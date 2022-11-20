package main

import (
	"log"
	
	"game-server/internal/discordbot"
)

func main() {
	bot := discordbot.New()
	if err := bot.Run(); err != nil {
		log.Fatal(err)
	}
}
