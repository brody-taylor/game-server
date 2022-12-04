package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"game-server/internal/config"
	"game-server/internal/gameserver"
)

func main() {
	fmt.Println(gameserver.MockServerStartupMessage)
	ctx, end := context.WithTimeout(context.Background(), 30*time.Second)

	go func() {
		defer end()
		in := bufio.NewScanner(os.Stdin)
		for in.Scan() {
			line := in.Text()
			switch {
			case line == config.MockGameStopCommand:
				fmt.Println(gameserver.MockServerShutdownResponse)
				return

			case strings.HasPrefix(line, config.MockGameMessageCommand):
				msg := strings.TrimPrefix(line, config.MockGameMessageCommand)
				msg = strings.TrimPrefix(msg, " ")
				fmt.Println(msg)

			default:
				fmt.Printf("Unknown command: [%s]\n", line)
			}
		}
	}()

	<-ctx.Done()
	fmt.Println(gameserver.MockServerShutdownMessage)
}
