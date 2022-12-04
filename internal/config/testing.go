package config

import (
	_ "embed"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed testdata/gameconfig.json
var mockGameConfigFile []byte

const (
	// Mock game config data
	MockGameName           = "MockGame"
	MockGameMessageCommand = "/message"
	MockGameStopCommand    = "/stop"
)

func NewTestConfig() (*Config, error) {
	cfg := New()
	if err := cfg.loadGameConfigFile(mockGameConfigFile); err != nil {
		return nil, err
	}

	// Convert working dir to abs so test cfg can be used by any pkg
	projRoot := getProjRoot()
	for game, gameCfg := range cfg.games {
		absWorkingDir := strings.Join([]string{projRoot, gameCfg.WorkingDir}, "\\")
		cfg.games[game].WorkingDir = absWorkingDir
	}

	return cfg, nil
}

func getProjRoot() string {
	const rootName = "game-server"

	_, b, _, _ := runtime.Caller(0)
	cwd := strings.Split(filepath.Dir(b), "\\")

	var projRoot string
	for i := len(cwd) - 1; i >= 0; i-- {
		if cwd[i] == rootName {
			projRoot = strings.Join(cwd[:i+1], "\\")
		}
	}

	return projRoot
}
