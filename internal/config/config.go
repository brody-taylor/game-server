package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
)

const (
	EnvGameConfig = "GAME_CONFIG_PATH"
)

type Config struct {
	Logger *zap.Logger
	games  map[string]*GameConfig
}

type GameConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	WorkingDir  string `json:"working_directory"`
	Run         struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	} `json:"run"`
	Message string `json:"message"`
	Stop    string `json:"stop"`
}

func New() *Config {
	return &Config{
		Logger: NewLogger(),
	}
}

func NewLogger() *zap.Logger {
	logCfg := zap.NewProductionConfig()
	logCfg.DisableStacktrace = true
	logger, _ := logCfg.Build()
	return logger
}

func (c *Config) Load() error {
	if err := c.loadGameConfig(); err != nil {
		return err
	}

	return nil
}

func (c *Config) GetGameConfig(game string) (*GameConfig, bool) {
	cfg, ok := c.games[strings.ToLower(game)]
	return cfg, ok
}

func (c *Config) GetGameNames() []string {
	names := make([]string, 0, len(c.games))
	for _, gameCfg := range c.games {
		names = append(names, gameCfg.Name)
	}
	return names
}

func (c *Config) loadGameConfig() error {
	filePath, ok := os.LookupEnv(EnvGameConfig)
	if !ok {
		return fmt.Errorf("missing env for game config path: [%s]", EnvGameConfig)
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	return c.loadGameConfigFile(fileData)
}

func (c *Config) loadGameConfigFile(fileData []byte) error {
	var gameCfgs []*GameConfig
	if err := json.Unmarshal(fileData, &gameCfgs); err != nil {
		return err
	}

	c.games = make(map[string]*GameConfig, len(gameCfgs))
	for _, gameCfg := range gameCfgs {
		gameName := strings.ToLower(gameCfg.Name)
		if _, ok := c.games[gameName]; ok {
			return fmt.Errorf("multiple configs found for game: [%s]", gameCfg.Name)
		}
		c.games[gameName] = gameCfg
	}

	return nil
}
