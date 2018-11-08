package p2p

import (
	"encoding/hex"
	"github.com/MedRecHackathon/go-spacemesh/log"
	"testing"
	"time"

	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/MedRecHackathon/go-spacemesh/crypto"
	"github.com/MedRecHackathon/go-spacemesh/p2p/config"
	"github.com/MedRecHackathon/go-spacemesh/p2p/message"
	"github.com/MedRecHackathon/go-spacemesh/p2p/net"
	"github.com/MedRecHackathon/go-spacemesh/p2p/node"
	"github.com/MedRecHackathon/go-spacemesh/p2p/pb"
	"github.com/MedRecHackathon/go-spacemesh/p2p/service"
	"github.com/MedRecHackathon/go-spacemesh/timesync"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"sync"
	"sync/atomic"
)

func p2pTestInstance(t testing.TB, config config.Config) *swarm {
	port, err := node.GetUnboundedPort()
	assert.NoError(t, err, "Error getting a port", err)
	config.TCPPort = port
	p, err := newSwarm(config, true, true)
	assert.NoError(t, err, "Error creating p2p stack, err: %v", err)
	assert.NotNil(t, p)
	return p
}

func startInstance(t testing.TB, s *swarm) {
	err := s.Start()
	assert.NoError(t, err)
}

const exampleProtocol = "EX"
const examplePayload = "Example"

func TestNew(t *testing.T) {
	s, err := New(config.DefaultConfig())
	assert.NoError(t, err, err)
	err = s.Start()
	assert.NoError(t, err, err)
	assert.NotNil(t, s, "its nil")
	s.Shutdown()
}

func Test_newSwarm(t *testing.T) {
	s, err := newSwarm(config.DefaultConfig(), true, false)
	assert.NoError(t, err)
	err = s.Start()
	assert.NoError(t, err, err)
	assert.NotNil(t, s)
	s.Shutdown()
}

func TestSwarm_Start(t *testing.T) {
	s, err := newSwarm(config.DefaultConfig(), true, false)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	err = s.Start()
	assert.NoError(t, err)
	assert.Equal(t, atomic.LoadUint32(&s.started), uint32(1))
	err = s.Start()
	assert.Error(t, err)
	s.Shutdown()
}

func TestSwarm_Shutdown(t *testing.T) {
	s, err := newSwarm(config.DefaultConfig(), true, false)
	assert.NoError(t, err)
	err = s.Start()
	assert.NoError(t, err, err)
	s.Shutdown()

	select {
	case _, ok := <-s.shutdown:
		assert.False(t, ok)
	case <-time.After(1 * time.Second):
		t.Error("Failed to shutdown")
	}
}

func TestSwarm_ShutdownNoStart(t *testing.T) {
	s, err := newSwarm(config.DefaultConfig(), true, false)
	assert.NoError(t, err)
	s.Shutdown()
}

func TestSwarm_RegisterProtocolNoStart(t *testing.T) {
	s, err := newSwarm(config.DefaultConfig(), true, false)
	msgs := s.RegisterProtocol("Anton")
	assert.NotNil(t, msgs)
	assert.NoError(t, err)
	s.Shutdown()
}

func TestSwarm_processMessage(t *testing.T) {
	s := swarm{}
	s.lNode, _ = node.GenerateTestNode(t)
	r := node.GenerateRandomNodeData()
	c := &net.ConnectionMock{}
	c.SetRemotePublicKey(r.PublicKey())
	ime := net.IncomingMessageEvent{Message: []byte("0"), Conn: c}
	s.processMessage(ime) // should error

	assert.True(t, c.Closed())
}

func TestSwarm_authAuthor(t *testing.T) {
	// create a message

	priv, pub, err := crypto.GenerateKeyPair()

	assert.NoError(t, err, err)
	assert.NotNil(t, priv)
	assert.NotNil(t, pub)

	pm := &pb.ProtocolMessage{
		Metadata: message.NewProtocolMessageMetadata(pub, exampleProtocol, false),
		Payload:  []byte(examplePayload),
	}
	ppm, err := proto.Marshal(pm)
	assert.NoError(t, err, "cant marshal msg ", err)

	// sign it
	s, err := priv.Sign(ppm)
	assert.NoError(t, err, "cant sign ", err)
	ssign := hex.EncodeToString(s)

	pm.Metadata.AuthorSign = ssign

	vererr := message.AuthAuthor(pm)
	assert.NoError(t, vererr)

	priv2, pub2, err := crypto.GenerateKeyPair()

	assert.NoError(t, err, err)
	assert.NotNil(t, priv2)
	assert.NotNil(t, pub2)

	s, err = priv2.Sign(ppm)
	assert.NoError(t, err, "cant sign ", err)
	ssign = hex.EncodeToString(s)

	pm.Metadata.AuthorSign = ssign

	vererr = message.AuthAuthor(pm)
	assert.Error(t, vererr)
}

func TestSwarm_SignAuth(t *testing.T) {
	n, _ := node.GenerateTestNode(t)
	pm := &pb.ProtocolMessage{
		Metadata: message.NewProtocolMessageMetadata(n.PublicKey(), exampleProtocol, false),
		Payload:  []byte(examplePayload),
	}

	err := message.SignMessage(n.PrivateKey(), pm)
	assert.NoError(t, err)

	err = message.AuthAuthor(pm)

	assert.NoError(t, err)
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func sendDirectMessage(t *testing.T, sender *swarm, recvPub string, inChan chan service.Message, checkpayload bool) {
	rand := RandString(5)
	sender.lNode.Info("(%v) TEST Message from %v To %v ", rand, sender.lNode.String(), recvPub)
	payload := []byte(RandString(10))
	err := sender.SendMessage(recvPub, exampleProtocol, payload)
	assert.NoError(t, err)
	select {
	case msg := <-inChan:
		if checkpayload {
			assert.Equal(t, msg.Data(), payload)
		}
		assert.Equal(t, msg.Sender().String(), sender.lNode.String())
		sender.lNode.Info("(%v) Sent succesfully", rand)
		break
	case <-time.After(5 * time.Second):
		t.Error("Took too much time to recieve")
	}
}

func TestSwarm_RoundTrip(t *testing.T) {
	p1 := p2pTestInstance(t, config.DefaultConfig())
	p2 := p2pTestInstance(t, config.DefaultConfig())

	startInstance(t, p1)
	startInstance(t, p2)

	exchan1 := p1.RegisterProtocol(exampleProtocol)
	assert.Equal(t, exchan1, p1.protocolHandlers[exampleProtocol])
	exchan2 := p2.RegisterProtocol(exampleProtocol)
	assert.Equal(t, exchan2, p2.protocolHandlers[exampleProtocol])

	p2.dht.Update(p1.lNode.Node)

	sendDirectMessage(t, p2, p1.lNode.PublicKey().String(), exchan1, true)
	sendDirectMessage(t, p1, p2.lNode.PublicKey().String(), exchan2, true)
}

func TestSwarm_MultipleMessages(t *testing.T) {
	p1 := p2pTestInstance(t, config.DefaultConfig())
	p2 := p2pTestInstance(t, config.DefaultConfig())

	startInstance(t, p1)
	startInstance(t, p2)

	exchan1 := p1.RegisterProtocol(exampleProtocol)
	assert.Equal(t, exchan1, p1.protocolHandlers[exampleProtocol])
	exchan2 := p2.RegisterProtocol(exampleProtocol)
	assert.Equal(t, exchan2, p2.protocolHandlers[exampleProtocol])

	err := p2.SendMessage(p1.lNode.String(), exampleProtocol, []byte(examplePayload))
	assert.Error(t, err, "ERR") // should'nt be in routing table
	p2.dht.Update(p1.lNode.Node)

	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() { sendDirectMessage(t, p2, p1.lNode.String(), exchan1, false); wg.Done() }()
	}
	wg.Wait()
}

func TestSwarm_RegisterProtocol(t *testing.T) {
	const numPeers = 100
	nodechan := make(chan *swarm)
	cfg := config.DefaultConfig()
	for i := 0; i < numPeers; i++ {
		go func() { // protocols are registered before starting the node and read after that.
			// there ins't an actual need to sync them.
			nod := p2pTestInstance(t, cfg)
			nod.RegisterProtocol(exampleProtocol) // this is example
			nodechan <- nod
		}()
	}
	i := 0
	for r := range nodechan {
		_, ok := r.protocolHandlers[exampleProtocol]
		assert.True(t, ok)
		_, ok = r.protocolHandlers["/dht/1.0/find-node/"]
		assert.True(t, ok)
		i++
		if i == numPeers {
			close(nodechan)
		}
	}
}

func TestSwarm_onRemoteClientMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	id, err := node.NewNodeIdentity(cfg, "0.0.0.0:0000", false)
	assert.NoError(t, err, "we cant make node ?")

	p := p2pTestInstance(t, config.DefaultConfig())
	nmock := &net.ConnectionMock{}
	nmock.SetRemotePublicKey(id.PublicKey())

	// Test bad format

	msg := []byte("badbadformat")
	imc := net.IncomingMessageEvent{nmock, msg}
	err = p.onRemoteClientMessage(imc)
	assert.Equal(t, err, ErrBadFormat1)

	// Test out of sync

	realmsg := &pb.CommonMessageData{
		SessionId: []byte("test"),
		Payload:   []byte("test"),
		Timestamp: time.Now().Add(timesync.MaxAllowedMessageDrift + time.Minute).Unix(),
	}
	bin, _ := proto.Marshal(realmsg)

	imc.Message = bin

	err = p.onRemoteClientMessage(imc)
	assert.Equal(t, err, ErrOutOfSync)

	// Test no payload

	cmd := &pb.CommonMessageData{
		SessionId: []byte("test"),
		Payload:   []byte(""),
		Timestamp: time.Now().Unix(),
	}

	bin, _ = proto.Marshal(cmd)
	imc.Message = bin
	err = p.onRemoteClientMessage(imc)

	assert.Equal(t, err, ErrNoPayload)

	// Test No Session

	cmd.Payload = []byte("test")

	bin, _ = proto.Marshal(cmd)
	imc.Message = bin

	err = p.onRemoteClientMessage(imc)
	assert.Equal(t, err, ErrNoSession)

	// Test bad session

	session := &net.SessionMock{}
	session.SetDecrypt(nil, errors.New("fail"))
	imc.Conn.SetSession(session)

	err = p.onRemoteClientMessage(imc)
	assert.Equal(t, err, ErrFailDecrypt)

	// Test bad format again

	session.SetDecrypt([]byte("wont_format_fo_protocol_message"), nil)

	err = p.onRemoteClientMessage(imc)
	assert.Equal(t, err, ErrBadFormat2)

	// Test bad auth sign

	goodmsg := &pb.ProtocolMessage{
		Metadata: message.NewProtocolMessageMetadata(id.PublicKey(), exampleProtocol, false), // not signed
		Payload:  []byte(examplePayload),
	}

	goodbin, _ := proto.Marshal(goodmsg)

	cmd.Payload = goodbin
	bin, _ = proto.Marshal(cmd)
	imc.Message = bin
	session.SetDecrypt(goodbin, nil)

	err = p.onRemoteClientMessage(imc)
	assert.Equal(t, err, ErrAuthAuthor)

	// Test no protocol

	err = message.SignMessage(id.PrivateKey(), goodmsg)
	assert.NoError(t, err, err)

	goodbin, _ = proto.Marshal(goodmsg)
	cmd.Payload = goodbin
	bin, _ = proto.Marshal(cmd)
	imc.Message = bin
	session.SetDecrypt(goodbin, nil)

	err = p.onRemoteClientMessage(imc)
	assert.Equal(t, err, ErrNoProtocol)

	// Test no err

	c := p.RegisterProtocol(exampleProtocol)
	go func() { <-c }()

	err = p.onRemoteClientMessage(imc)

	assert.NoError(t, err)

	// todo : test gossip codepaths.
}

func testBootstrapping(t testing.TB, bootnodes int, nodes int, rcon int) []*swarm {
	bufchan := make(chan *swarm, nodes)

	bnarr := []string{}

	for k := 0; k < bootnodes; k++ {
		bn := p2pTestInstance(t, config.DefaultConfig())
		bn.lNode.Info("This is a bootnode - %v", bn.lNode.Node.String())
		bnarr = append(bnarr, node.StringFromNode(bn.lNode.Node))
		err := bn.Start()
		assert.NoError(t, err)
	}

	cfg := config.DefaultConfig()
	cfg.SwarmConfig.Bootstrap = true
	cfg.SwarmConfig.RandomConnections = rcon
	cfg.SwarmConfig.BootstrapNodes = bnarr

	var wg sync.WaitGroup

	for j := 0; j < nodes; j++ {
		wg.Add(1)
		go func() {
			sw := p2pTestInstance(t, cfg)
			err := sw.Start()
			assert.NoError(t, err)
			err = sw.waitForBoot()
			if assert.NoError(t, err) {
				bufchan <- sw
			}
			wg.Done()
		}()
	}

	wg.Wait()
	close(bufchan)
	swarms := []*swarm{}
	for s := range bufchan {
		swarms = append(swarms, s)
	}

	return swarms
}

// this is a multi-value bootstrap test. it will run all configured numbers in the top of the function
// TODO: add more params (bucketsize, alpha, etc..). to make this illustrate test from the test plan in our wiki
// so it can be used as model for integration/systems tests.
func TestBootstrap(t *testing.T) {
	bootnodes := []int{1}
	nodes := []int{10}
	rcon := []int{3}

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < len(nodes); i++ {
		t.Run(fmt.Sprintf("Peers:%v/randconn:%v", nodes[i], rcon[i]), func(t *testing.T) {
			swarms := testBootstrapping(t, bootnodes[i], nodes[i], rcon[i])
			assert.Equal(t, len(swarms), nodes)
		})
	}
}

func TestBootstrapAndMessages(t *testing.T) {

	bootnodes := []int{1}
	nodes := []int{10}
	rcon := []int{3}

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < len(nodes); i++ {
		t.Run(fmt.Sprintf("Peers:%v/randconn:%v", nodes[i], rcon[i]), func(t *testing.T) {
			swarms := testBootstrapping(t, bootnodes[i], nodes[i], rcon[i])
			if assert.Equal(t, len(swarms), nodes[i]) {
				log.Info("All nodes bootstrapped ")
				rand.Seed(time.Now().UnixNano())
				for z := 0; z < 10; z++ {

					randnode := swarms[rand.Int31n(int32(len(swarms)-1))]
					randnode2 := swarms[rand.Int31n(int32(len(swarms)-1))]

					for (randnode == nil || randnode2 == nil) || randnode.lNode.String() == randnode2.lNode.String() {
						randnode = swarms[rand.Int31n(int32(len(swarms)-1))]
						randnode2 = swarms[rand.Int31n(int32(len(swarms)-1))]
					}

					randnode.RegisterProtocol(exampleProtocol)
					recv := randnode2.RegisterProtocol(exampleProtocol)

					sendDirectMessage(t, randnode, randnode2.lNode.PublicKey().String(), recv, true)
				}
			}
		})
	}

}
