package ping

import (
	"github.com/MedRecHackathon/go-spacemesh/p2p/simulator"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPing_Ping(t *testing.T) {
	sim := simulator.New()
	node1 := sim.NewNode()
	node2 := sim.NewNode()

	p := New(node1)
	p2 := New(node2)

	pr, err := p.Ping(node2.String(), "hello")
	assert.NoError(t, err)
	assert.Equal(t, pr, responses["hello"])

	AddResponse("TEST", "T3ST")

	pr, err = p2.Ping(node1.String(), "TEST")
	assert.NoError(t, err)
	assert.Equal(t, pr, "T3ST")
}
