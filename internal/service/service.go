package service

import (
	"go.uber.org/zap"

	"game-server/internal/config"
	discordbot "game-server/internal/discord/bot"
	"game-server/internal/gameserver"
)

type Service struct {
	cfg *config.Config

	gameClient gameserver.ClientIFace
	botServer  *discordbot.BotServer
}

func New() *Service {
	cfg := config.New()
	gameClient := gameserver.New(cfg)

	return &Service{
		cfg: cfg,

		gameClient: gameClient,
		botServer:  discordbot.New(cfg, gameClient),
	}
}

func (s *Service) Run() {
	// Load game config
	if err := s.cfg.Load(); err != nil {
		s.cfg.Logger.Panic("Failed to load config", zap.Error(err))
	}

	// Start discord bot
	// TODO: run in separate Goroutine
	if err := s.botServer.Connect(); err != nil {
		s.cfg.Logger.Panic("Discord bot could not connect to required services", zap.Error(err))
	}
	if err := s.botServer.Run(); err != nil {
		s.cfg.Logger.Panic("Failed to run Discord bot", zap.Error(err))
	}
	s.cfg.Logger.Info("Discord bot closed")
}
