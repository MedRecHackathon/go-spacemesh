package gossip

import (
	"github.com/MedRecHackathon/go-spacemesh/log"
	"github.com/MedRecHackathon/go-spacemesh/p2p/config"
	"github.com/MedRecHackathon/go-spacemesh/p2p/net"
	"github.com/MedRecHackathon/go-spacemesh/p2p/node"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNeighborhood_Peer(t *testing.T) {
	n := &Neighborhood{outbound: make(map[string]*peer, 10)}
	ni := node.GenerateRandomNodeData()
	cn := &net.ConnectionMock{}
	cn.SetRemotePublicKey(ni.PublicKey())
	n.outbound[ni.String()] = makePeer(ni, cn, log.New("test", "", ""))
	np, c := n.Peer(ni.String())
	assert.Equal(t, ni, np)
	assert.Equal(t, cn, c)
}

func TestNeighborhood_Broadcast(t *testing.T) {
	n := NewNeighborhood(config.DefaultConfig().SwarmConfig, nil, nil, log.New("tesT", "", ""))
	err := n.Broadcast([]byte("msg"))
	assert.NoError(t, err)
	err = n.Broadcast([]byte("msg"))
	assert.Error(t, err)
	//todo test more
}
