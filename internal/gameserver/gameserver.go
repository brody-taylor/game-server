package gameserver

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	"game-server/internal/config"

	"go.uber.org/zap"
)

const (
	loggerName = "game-client"

	ServerShutdownWarning = "Server shutting down in %s"
)

var (
	ServerShutdownDelay   time.Duration = 30 * time.Second
	ServerShutdownTimeout time.Duration = 10 * time.Second
)

// Ensure Client implements ClientIFace
var _ ClientIFace = (*Client)(nil)

type ClientIFace interface {
	Run(game string) error
	IsRunning() (gameName string, isRunning bool)
	Stop() error
}

type Client struct {
	cfg     *config.Config
	running *server
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
	}
}

func (c *Client) Run(game string) error {
	// Get game server
	s, err := newGameServer(c.cfg, game)
	if err != nil {
		return err
	}

	// Stop currently running game
	if c.running != nil {
		if err := c.Stop(); err != nil {
			return fmt.Errorf("could not stop current game server: %w", err)
		}
	}

	if err := s.run.Start(); err != nil {
		return err
	}
	c.running = s
	return nil
}

func (c *Client) IsRunning() (gameName string, isRunning bool) {
	if c.running != nil {
		return c.running.name, true
	}
	return "", false
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
	name   string
	logger *zap.Logger
	run    *exec.Cmd
	stop   string
	msg    string
	in     io.Writer
	out    io.Reader
	outErr io.Reader
}

func newGameServer(cfg *config.Config, game string) (*server, error) {
	gameCfg, ok := cfg.GetGameConfig(game)
	if !ok {
		return nil, fmt.Errorf("no configuration for game: [%s]", game)
	}

	s := &server{
		name: gameCfg.Name,
		logger: cfg.Logger.Named(loggerName).With(zap.String("game", gameCfg.Name)),
		run:  exec.Command(gameCfg.Run.Command, gameCfg.Run.Args...),
		stop: gameCfg.Stop,
		msg:  gameCfg.Message,
	}

	// Specify working directory
	if dir := gameCfg.WorkingDir; dir != "" {
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
		s.logger.Info("game server shutdown gracefully")
		return err

	// Force shutdown on timeout
	case <-time.After(ServerShutdownTimeout):
		s.logger.Error("could not shutdown game server gracefully, forcing shutdown")
		return s.run.Process.Kill()
	}
}
