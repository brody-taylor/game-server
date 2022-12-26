package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func NewTestConfig(t *testing.T, configFile []byte) *Config {
	cfg := New()
	cfg.Logger = NewTestLogger()

	err := cfg.loadGameConfigFile(configFile)
	require.NoError(t, err, "Could not load config file")

	return cfg
}

func NewTestLogger() *zap.Logger {
	return zap.NewNop()
}
