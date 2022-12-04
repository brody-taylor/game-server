package gameserver

import (
	"github.com/stretchr/testify/mock"
)

const (
	// Mock server messages
	MockServerStartupMessage   = "Started mock server"
	MockServerShutdownResponse = "Got shutdown command"
	MockServerShutdownMessage  = "Closing mock server"

	LoadMethod = "Load"
	RunMethod  = "Run"
	StopMethod = "Stop"
)

// Ensure MockClient implements ClientIFace
var _ ClientIFace = (*MockClient)(nil)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) Run(game string) error {
	args := m.Called(game)
	return args.Error(0)
}

func (m *MockClient) Stop() error {
	args := m.Called()
	return args.Error(0)
}
