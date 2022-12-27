package service

import (
	"time"

	"go.uber.org/zap"

	"game-server/internal/backup"
	"game-server/internal/config"
	discordbot "game-server/internal/discord/bot"
	"game-server/internal/gameserver"
	"game-server/pkg/monitor"
)

const inactivityThreshold = 15 * time.Minute

type Service struct {
	cfg *config.Config

	gameClient gameserver.ClientIFace
	botServer  *discordbot.BotServer
	monitor    monitor.ClientIFace
	backup     *backup.Client
}

func New() *Service {
	cfg := config.New()
	gameClient := gameserver.New(cfg)

	return &Service{
		cfg: cfg,

		gameClient: gameClient,
		botServer:  discordbot.New(cfg, gameClient),
		monitor:    monitor.New(inactivityThreshold),
		backup:     backup.New(cfg),
	}
}

func (s *Service) Run() {
	// Load game config
	if err := s.cfg.Load(); err != nil {
		s.cfg.Logger.Panic("failed to load config", zap.Error(err))
	}

	// Start discord bot
	go func() {
		s.cfg.Logger.Info("starting discord bot")
		if err := s.botServer.Connect(); err != nil {
			s.cfg.Logger.Panic("discord bot could not connect to required services", zap.Error(err))
		}
		if err := s.botServer.Run(); err != nil {
			s.cfg.Logger.Panic("failed to run discord bot", zap.Error(err))
		}
		s.cfg.Logger.Info("discord bot closed")
	}()

	// Start monitoring server activity
	inactive, err := s.monitor.Start(s.cfg.GetGamePorts())
	if err != nil {
		s.cfg.Logger.Panic("failed to monitor server activity", zap.Error(err))
	}

	// Await inactivity before triggering shutdown
	<-inactive
	s.cfg.Logger.Info("game server is inactive, initiating shutdown")
	s.gracefulShutdown()
}

func (s *Service) gracefulShutdown() {
	// Flushes log buffer, if any
	defer s.cfg.Logger.Sync()

	// Shutdown game server if currently running
	if _, running := s.gameClient.IsRunning(); running {
		if err := s.gameClient.Stop(); err != nil {
			s.cfg.Logger.Error("could not shutdown game server", zap.Error(err))
		}
	}

	// Stop monitoring server activity
	s.monitor.Close()

	// Stop Discord bot server
	if err := s.botServer.Stop(); err != nil {
		s.cfg.Logger.Error("could not shutdown discord bot", zap.Error(err))
	}

	// Backup game save data
	if err := s.backup.DoBackup(); err != nil {
		s.cfg.Logger.Error("error encountered backing up save data", zap.Error(err))
	}
}
