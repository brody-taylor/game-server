package gameserver

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	EnvGameConfig = "GAME_CONFIG_PATH"

	ServerShutdownWarning = "Server shutting down in %s"
)

var (
	ServerShutdownDelay   time.Duration = 30 * time.Second
	ServerShutdownTimeout time.Duration = 10 * time.Second
)

// Ensure Client implements ClientIFace
var _ ClientIFace = (*Client)(nil)

type ClientIFace interface {
	Load() error
	Run(game string) error
	Stop() error
}

type Client struct {
	games   map[string]gameConfig
	running *server
}

type gameConfig struct {
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

func New() *Client {
	return &Client{}
}

func (c *Client) Load() error {
	// Load config
	cfg, err := readAndParseConfig()
	if err != nil {
		return err
	}

	c.games = make(map[string]gameConfig, len(cfg))
	for _, gameCfg := range cfg {
		c.games[strings.ToLower(gameCfg.Name)] = gameCfg
	}
	return nil
}

func (c *Client) Run(game string) error {
	gameCfg, ok := c.games[strings.ToLower(game)]
	if !ok {
		return fmt.Errorf("no configuration for game: [%s]", game)
	}

	// Stop currently running game
	if c.running != nil {
		if err := c.Stop(); err != nil {
			return fmt.Errorf("could not stop current game server: %w", err)
		}
	}

	s, err := newGameServer(gameCfg)
	if err != nil {
		return err
	}

	if err := s.run.Start(); err != nil {
		return err
	}
	c.running = s
	return nil
}

func (c *Client) Stop() error {
	// Check that a server is running
	if c.running == nil {
		return fmt.Errorf("no running server to stop")
	}

	// Attempt stop
	if err := c.running.stopServer(); err != nil {
		return err
	}

	// Clear running status after successful stop
	c.running = nil
	return nil
}

type server struct {
	run    *exec.Cmd
	stop   string
	msg    string
	in     io.Writer
	out    io.Reader
	outErr io.Reader
}

func newGameServer(cfg gameConfig) (*server, error) {
	s := &server{
		run:  exec.Command(cfg.Run.Command, cfg.Run.Args...),
		stop: cfg.Stop,
		msg:  cfg.Message,
	}

	// Specify working directory
	if dir := cfg.WorkingDir; dir != "" {
		// Convert path to absolute if relative
		if !filepath.IsAbs(dir) {
			var err error
			if dir, err = filepath.Abs(dir); err != nil {
				return nil, err
			}
		}

		s.run.Dir = dir
	}

	// Get input writer and output readers
	in, err := s.run.StdinPipe()
	if err != nil {
		return nil, err
	}
	s.in = in
	out, err := s.run.StdoutPipe()
	if err != nil {
		return nil, err
	}
	s.out = out
	outErr, err := s.run.StderrPipe()
	if err != nil {
		return nil, err
	}
	s.outErr = outErr

	return s, nil
}

func (s *server) stopServer() error {
	// Send shutdown warning and delay
	warningMsg := fmt.Sprintf(ServerShutdownWarning, ServerShutdownDelay)
	warningMsg = fmt.Sprintf("%s %s\n", s.msg, warningMsg)
	s.in.Write([]byte(warningMsg))
	time.Sleep(ServerShutdownDelay)

	// Try graceful shutdown
	s.in.Write(append([]byte(s.stop), '\n'))
	wait := make(chan error)
	go func() {
		wait <- s.run.Wait()
	}()

	select {
	// Await graceful shutdown
	case err := <-wait:
		return err

	// Force shutdown on timeout
	case <-time.After(ServerShutdownTimeout):
		return s.run.Process.Kill()
	}
}

func readAndParseConfig() ([]gameConfig, error) {
	filePath, ok := os.LookupEnv(EnvGameConfig)
	if !ok {
		return nil, fmt.Errorf("missing env for game config path: [%s]", EnvGameConfig)
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config []gameConfig
	return config, json.Unmarshal(fileData, &config)
}
