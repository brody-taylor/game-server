package service

import (
	"log"

	"game-server/internal/config"
	discordbot "game-server/internal/discord/bot"
	"game-server/internal/gameserver"
)

type Service struct {
	cfg    *config.Config
	logger *log.Logger

	gameClient gameserver.ClientIFace
	botServer  *discordbot.BotServer
}

func New() *Service {
	cfg := config.New()
	gameClient := gameserver.New(cfg)

	return &Service{
		cfg:    cfg,
		logger: log.Default(),

		gameClient: gameClient,
		botServer:  discordbot.New(gameClient),
	}
}

func (s *Service) Run() {
	// Load game config
	if err := s.cfg.Load(); err != nil {
		panic(err)
	}

	// Start discord bot
	// TODO: run in separate Goroutine
	if err := s.botServer.Connect(); err != nil {
		s.logger.Panic(err)
	}
	if err := s.botServer.Run(); err != nil {
		s.logger.Panic(err)
	}
	s.logger.Print("Discord bot closed")
}
