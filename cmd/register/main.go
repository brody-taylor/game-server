package main

import (
	"flag"

	"game-server/internal/config"
	"game-server/internal/discordbot/discordcmd"
)

var doClear = flag.Bool("clear", false, "clear all currently registered commands")

func main() {
	flag.Parse()

	cfg := config.New()
	if err := cfg.Load(); err != nil {
		panic(err)
	}

	c := discordcmd.New(cfg)
	if err := c.Connect(); err != nil {
		panic(err)
	}

	var err error
	if *doClear {
		err = c.Clear()
	} else {
		err = c.Register()
	}
	if err != nil {
		panic(err)
	}
}
