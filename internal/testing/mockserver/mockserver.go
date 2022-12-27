package mockserver

import (
	_ "embed"
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"game-server/internal/config"
)

//go:embed config.json
var configFile []byte

const (
	// Mock game config data
	GameName       = "MockGame"
	MessageCommand = "/message"
	StopCommand    = "/stop"

	// Mock server messages
	StartupMessage   = "Started mock server"
	ShutdownResponse = "Got shutdown command"
	ShutdownMessage  = "Closing mock server"
)

var SaveFilePaths = []string{
	"savedata/savefile1.txt",
	"savedata/savedir/savefile2.txt",
	"savedata/savedir/savefile3",
}

func GetConfig(t *testing.T) *config.Config {
	cfg := config.NewTestConfig(t, configFile)

	// Set working directory
	_, filePath, _, ok := runtime.Caller(0)
	require.True(t, ok, "No caller information getting mock server working directory")
	workingDir := path.Dir(filePath)
	for _, game := range cfg.GetGameNames() {
		gameCfg, _ := cfg.GetGameConfig(game)
		gameCfg.WorkingDir = workingDir
	}

	return cfg
}
