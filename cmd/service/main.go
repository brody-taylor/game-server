package main

import (
	"game-server/internal/service"
)

func main() {
	s := service.New()
	s.Run()
}
