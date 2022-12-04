package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"game-server/internal/config"
)

func Test_Load(t *testing.T) {
	// Set game config path to mock
	os.Setenv(config.EnvGameConfig, "testdata/gameconfig.json")
	defer os.Unsetenv(config.EnvGameConfig)

	cfg := config.New()

	assert.NoError(t, cfg.Load())
}
