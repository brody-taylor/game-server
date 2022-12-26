package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"game-server/internal/testing/mockserver"
)

func main() {
	fmt.Println(mockserver.StartupMessage)
	ctx, end := context.WithTimeout(context.Background(), 30*time.Second)

	go func() {
		defer end()
		in := bufio.NewScanner(os.Stdin)
		for in.Scan() {
			line := in.Text()
			switch {
			case line == mockserver.StopCommand:
				fmt.Println(mockserver.ShutdownResponse)
				return

			case strings.HasPrefix(line, mockserver.MessageCommand):
				msg := strings.TrimPrefix(line, mockserver.MessageCommand)
				msg = strings.TrimPrefix(msg, " ")
				fmt.Println(msg)

			default:
				fmt.Printf("Unknown command: [%s]\n", line)
			}
		}
	}()

	<-ctx.Done()
	fmt.Println(mockserver.ShutdownMessage)
}
