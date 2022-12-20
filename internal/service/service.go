package service

import (
	"time"

	"go.uber.org/zap"

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
}

func New() *Service {
	cfg := config.New()
	gameClient := gameserver.New(cfg)

	return &Service{
		cfg: cfg,

		gameClient: gameClient,
		botServer:  discordbot.New(cfg, gameClient),
		monitor:    monitor.New(inactivityThreshold),
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
	// TODO: graceful shutdown
}
