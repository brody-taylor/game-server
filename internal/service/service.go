package service

import (
	"log"

	"game-server/internal/discordbot"
)

type Service struct {
	logger    *log.Logger
	botServer *discordbot.BotServer
}

func New() *Service {
	return &Service{
		logger:    log.Default(),
		botServer: discordbot.New(),
	}
}

func (s *Service) Run() {
	// Start discord bot
	// TODO: run in separate Goroutine
	if err := s.botServer.Run(); err != nil {
		s.logger.Panic(err)
	}
	s.logger.Print("Discord bot closed")
}
