package main

import (
	"log"
	
	"game-server/httpserver"
)

func main() {
	if err := httpserver.Run(); err != nil {
		log.Fatal(err)
	}
}
