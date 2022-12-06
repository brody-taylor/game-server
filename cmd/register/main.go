package main

import (
	"game-server/internal/config"
	"game-server/internal/discordbot/discordcmd"
)

func main() {
	cfg := config.New()
	if err := cfg.Load(); err != nil {
		panic(err)
	}

	c := discordcmd.New(cfg)
	if err := c.Connect(); err != nil {
		panic(err)
	}
	if err := c.Register(); err != nil {
		panic(err)
	}
}
