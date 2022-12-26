package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"game-server/internal/config"
)

func Test_Load(t *testing.T) {
	// Set game config path to mock
	t.Setenv(config.EnvGameConfig, "testdata/emptyconfig.json")

	cfg := config.New()

	assert.NoError(t, cfg.Load())
}
