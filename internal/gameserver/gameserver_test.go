package gameserver

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"game-server/internal/config"
)

func Test_Run_and_Stop(t *testing.T) {
	// Override shutdown delay
	defer func(origDelay time.Duration) {
		ServerShutdownDelay = origDelay
	}(ServerShutdownDelay)
	newDelay := 10 * time.Millisecond
	ServerShutdownDelay = newDelay

	// Get test config
	testCfg, err := config.NewTestConfig()
	require.NoError(t, err)

	// Start mock server
	c := New(testCfg)
	require.NoError(t, c.Run(config.MockGameName))
	time.Sleep(10 * time.Millisecond)

	// Record stdout from mock server
	var out []string
	go func() {
		buf := bufio.NewReader(c.running.out)
		for {
			line, err := buf.ReadString('\n')
			if err != nil {
				return
			}
			out = append(out, line)
		}
	}()

	// Stop mock server
	require.NoError(t, c.Stop())

	// Check running status was cleared
	assert.Nil(t, c.running)

	// Check for warning message and graceful shutdown
	didWarningMsg := false
	didGracefulShutdown := false
	for _, line := range out {
		if strings.Contains(line, fmt.Sprintf(ServerShutdownWarning, newDelay)) {
			didWarningMsg = true
		} else if strings.Contains(line, MockServerShutdownResponse) {
			didGracefulShutdown = true
		}
	}
	assert.True(t, didWarningMsg, "Did not get shutdown warning message")
	assert.True(t, didGracefulShutdown, "Server did not shutdown gracefully")
}
