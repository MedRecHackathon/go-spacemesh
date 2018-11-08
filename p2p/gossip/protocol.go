package gossip

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/MedRecHackathon/go-spacemesh/crypto"
	"github.com/MedRecHackathon/go-spacemesh/log"
	"github.com/MedRecHackathon/go-spacemesh/p2p/config"
	"github.com/MedRecHackathon/go-spacemesh/p2p/message"
	"github.com/MedRecHackathon/go-spacemesh/p2p/net"
	"github.com/MedRecHackathon/go-spacemesh/p2p/node"
	"sync"
	"time"
)

const PeerMessageQueueSize = 100

type Protocol interface {
	Broadcast(payload []byte) error
	Start() error
	Peer(pubkey string) (node.Node, net.Connection)
	RegisterPeer(node.Node, net.Connection)
	Disconnect(peer string)
	Shutdown()
}

type PeerSampler interface {
	SelectPeers(count int) []node.Node
}

type ConnectionFactory interface {
	GetConnection(address string, pk crypto.PublicKey) (net.Connection, error)
}

type NodeConPair struct {
	node.Node
	net.Connection
}

type Neighborhood struct {
	log.Log

	config config.SwarmConfig

	// we want make sure we don't hold to much inbound connections
	inbound map[string]*peer
	// we always try to reach a number of outbound peers
	outbound map[string]*peer

	outboundCount uint32
	inboundCount  uint32

	// connecting new outbound peers is in progress
	outboundMutex sync.Mutex
	// a channel where we recieve incoming connections
	inc chan NodeConPair

	// this channel tells us we got some more outbound peers
	out chan struct{}

	// closed peers are reported here
	remove chan string

	// we make sure we don't send a message twice
	oldMessageMu sync.RWMutex
	oldMessageQ  map[string]struct{}

	// an interface where we can consume samples of peers
	ps PeerSampler

	// an interface get/create connections with peers selected tf
	cp ConnectionFactory

	shutdown chan struct{}

	peersMutex sync.RWMutex
}

func NewNeighborhood(config config.SwarmConfig, ps PeerSampler, cp ConnectionFactory, log2 log.Log) *Neighborhood {
	return &Neighborhood{
		Log:         log2,
		config:      config,
		out:         make(chan struct{}, config.RandomConnections),
		outbound:    make(map[string]*peer, config.RandomConnections),
		inbound:     make(map[string]*peer, config.RandomConnections),
		inc:         make(chan NodeConPair, config.RandomConnections*2),
		oldMessageQ: make(map[string]struct{}), // todo : remember to drain this
		ps:          ps,
		cp:          cp,
	}
}

var _ Protocol = new(Neighborhood)

type peer struct {
	log.Log
	node.Node
	connected     time.Time
	conn          net.Connection
	knownMessages map[string]struct{}
	msgQ          chan []byte
}

func makePeer(node2 node.Node, c net.Connection, log log.Log) *peer {
	return &peer{
		log,
		node2,
		time.Now(),
		c,
		make(map[string]struct{}),
		make(chan []byte, PeerMessageQueueSize),
	}
}

func (p *peer) send(message []byte) error {
	if p.conn == nil || p.conn.Session() == nil {
		return fmt.Errorf("the connection does not exist for this peer")
	}
	return p.conn.Send(message)
}

func (p *peer) addMessage(msg []byte) error {
	// dont do anything if this peer know this msg
	if _, ok := p.knownMessages[hex.EncodeToString(msg)]; ok {
		return errors.New("already got this msg")
	}

	// check if connection and session are ok
	c := p.conn
	session := c.Session()
	if c == nil || session == nil {
		// todo: refresh Neighborhood or create session
		return errors.New("no session")
	}

	data, err := message.PrepareMessage(session, msg)

	if err != nil {
		return err
	}

	select {
	case p.msgQ <- data:

	default:
		return errors.New("Q was full")

	}

	return nil
}

func (p *peer) start(dischann chan string) {
	// check on new outbound if they need something we have
	//c := make(chan []string)
	//t := time.NewTicker(time.Second * 5)
	for {
		select {
		case m := <-p.msgQ:
			err := p.send(m)
			if err != nil {
				// todo: handle errors
				log.Error("Failed sending message to this peer %v", p.Node.PublicKey().String())
				return
			}
		}
	}
}

func (s *Neighborhood) Shutdown() {
	// no need to shutdown con, conpool will do so in a shutdown. the morepeerreq won't work
	close(s.shutdown)
}

func (s *Neighborhood) Peer(pubkey string) (node.Node, net.Connection) {
	s.peersMutex.RLock()
	p, ok := s.outbound[pubkey]
	p2, ok2 := s.inbound[pubkey]
	s.peersMutex.RUnlock()
	if ok {
		return p.Node, p.conn
	}
	if ok2 {
		return p2.Node, p2.conn
	}
	return node.EmptyNode, nil

}

// the actual broadcast procedure, loop on outbound and add the message to their queues
func (s *Neighborhood) Broadcast(msg []byte) error {

	s.oldMessageMu.RLock()
	if _, ok := s.oldMessageQ[string(msg)]; ok {
		// todo : - have some more metrics for termination
		// todo	: - maybe tell the peer weg ot this message already?
		return errors.New("old message")
	}
	s.oldMessageMu.RUnlock()

	if len(s.outbound) == 0 {
		return errors.New("you have no outbound to broadcast to")
	}

	s.oldMessageMu.Lock()
	s.oldMessageQ[string(msg)] = struct{}{}
	s.oldMessageMu.Unlock()

	s.peersMutex.RLock()
	for p := range s.outbound {
		peer := s.outbound[p]
		err := peer.addMessage(msg)
		if err != nil {
			// report error and maybe replace this peer
			s.Errorf("Err adding message err=", err)
			continue
		}
		s.Debug("adding message to peer %v", peer.Pretty())
	}
	for p := range s.inbound {
		peer := s.inbound[p]
		err := peer.addMessage(msg)
		if err != nil {
			// report error and maybe replace this peer
			s.Errorf("Err adding message err=", err)
			continue
		}
		s.Debug("adding message to peer %v", peer.Pretty())
	}
	s.peersMutex.RUnlock()

	//TODO: if we didn't send to RandomConnections then try to other outbound.
	return nil
}

func (s *Neighborhood) Disconnect(peer string) {
	s.Info("Disconnect from outside")
	s.peersMutex.RLock()
	_, ok := s.outbound[peer]
	ln := len(s.outbound)
	s.peersMutex.RUnlock()
	s.removePeer(peer)
	if ok {
		num := s.config.RandomConnections - ln - 1
		if num > 0 {
			s.getMorePeers(num)
		}
	}
}

func (s *Neighborhood) getMorePeers(numpeers int) {
	s.Debug("getMorePeers %d", numpeers)
	s.peersMutex.RLock()
	if len(s.outbound) == s.config.RandomConnections {
		return
	}
	s.peersMutex.RUnlock()
	s.outboundMutex.Lock()
	type cnErr struct {
		n   node.Node
		c   net.Connection
		err error
	}

	res := make(chan cnErr, numpeers)

	s.Debug("Trying to get peer sample of %d", numpeers)
	// should provide us with random peers to connect to
	nds := s.ps.SelectPeers(numpeers)
	ndsLen := len(nds)
	if ndsLen == 0 {
		s.Debug("Peer sampler returned nothing.")
		// this gets busy at start so we spare a second
		time.Sleep(1 * time.Second)
		s.outboundMutex.Unlock()
		go s.getMorePeers(numpeers)
		return // zero samples here so no reason to proceed
	}

	s.Debug("trying to connect all %d sampled peers", ndsLen)

	// Try a connection to each peer.
	// TODO: try splitting the load and don't connect to more than X at a time
	for i := 0; i < ndsLen; i++ {
		s.Debug("issuing conn >> %v", nds[i].String())
		go func(nd node.Node, reportChan chan cnErr) {
			c, err := s.cp.GetConnection(nd.Address(), nd.PublicKey())
			s.Debug("Connection returned")
			reportChan <- cnErr{nd, c, err}
		}(nds[i], res)
	}

	i, j := 0, 0
	for cne := range res {
		i++ // We count i everytime to know when to close the channel

		if cne.err != nil {
			s.Error("can't establish connection with sampled peer %v, %v", cne.n.String(), cne.err)
			j++
			continue // this peer didn't work, todo: tell dht
		}

		p := makePeer(cne.n, cne.c, s.Log)
		s.peersMutex.Lock()
		//_, ok := s.outbound[cne.n.String()]
		_, ok2 := s.inbound[cne.n.String()]
		if ok2 {
			delete(s.inbound, cne.n.String())
		}
		//
		//if ok {
		//	s.outbound[cne.n.String()] = p
		//} else if ok2 {
		//	s.outbound[cne.n.String()] = p
		//}

		s.outbound[cne.n.String()] = p

		s.peersMutex.Unlock()
		go p.start(s.remove)
		s.Debug("Neighborhood: Added peer to peer list %v", cne.n.Pretty())

		if i == numpeers {
			close(res)
		}

	}

	if len(s.outbound) < s.config.RandomConnections {
		s.outboundMutex.Unlock()
		s.Debug("getMorePeers enough calling again")
		s.getMorePeers(numpeers)
		return
	}

	s.outboundMutex.Unlock()
	s.out <- struct{}{}
}

func (s *Neighborhood) removePeer(torm string) {
	s.peersMutex.RLock()
	_, ok := s.inbound[torm]
	_, ok2 := s.outbound[torm]
	s.peersMutex.RUnlock()

	var removefrom []map[string]*peer

	if ok {
		removefrom = append(removefrom, s.inbound)
	}
	if ok2 {
		removefrom = append(removefrom, s.outbound)
	}

	if len(removefrom) > 0 {
		s.peersMutex.Lock()
		for _, rf := range removefrom {
			delete(rf, torm)
		}
		s.peersMutex.Unlock()
	}
}

// Start Neighborhood manages the outbound we are connected to all the time
// It connects to config.RandomConnections and after that maintains this number
// of connections, if a connection is closed it should send a channel message that will
// trigger new connections to fill the requirement.
func (s *Neighborhood) Start() error {
	s.Info("Starting neighborhood")
	//TODO: Save and load persistent outbound ?

	// initial
	ret := make(chan struct{})

	go s.getMorePeers(s.config.RandomConnections)

	go func() {
		var o sync.Once
	loop:
		for {
			// TODO: inbound/outbound connections limit .
			select {
			//case torm := <-s.remove:
			//		go s.Disconnect(torm)
			case inc := <-s.inc:
				// todo : check the possibility of changing an existing peer's connection
				// try to assign the new peer
				s.peersMutex.RLock()
				_, ok := s.inbound[inc.Node.String()]
				_, ok2 := s.outbound[inc.Node.String()]
				s.peersMutex.RUnlock()

				if !ok && !ok2 {
					peer := makePeer(inc.Node, inc.Connection, s.Log)
					s.peersMutex.Lock()
					s.inbound[peer.Node.String()] = peer
					s.peersMutex.Unlock()
					go peer.start(s.remove)
				}
			case <-s.out:
				if len(s.outbound) >= s.config.RandomConnections {
					o.Do(func() { close(ret) })
				}

			case <-s.shutdown:
				break loop // maybe error ?
			}
		}
	}()

	<-ret
	s.Info("Neighborhood initialized with %v dialed peers", len(s.outbound))
	s.Info(spew.Sdump(s.outbound))

	return nil
}

func (s *Neighborhood) RegisterPeer(n node.Node, c net.Connection) {
	s.inc <- NodeConPair{n, c}
}
