package app

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/MedRecHackathon/go-spacemesh/filesystem"
)

func TestApp(t *testing.T) {
	filesystem.SetupTestSpacemeshDataFolders(t, "app_test")

	// remove all injected test flags for now
	os.Args = []string{"/go-spacemesh", "--json-server=true"}

	go Main()

	<-EntryPointCreated

	assert.NotNil(t, App)

	<-App.NodeInitCallback

	assert.NotNil(t, App.P2P)
	assert.NotNil(t, App)

	// app should exit based on this signal
	ExitApp <- true

	filesystem.DeleteSpacemeshDataFolders(t)

}

func TestParseConfig(t *testing.T) {
	t.Skip() // to do test this for real
	//err := config.LoadConfig("./config.toml")
	//assert.Nil(t, err)
	//
	//_, err = ParseConfig()
	//assert.Nil(t, err)
}
