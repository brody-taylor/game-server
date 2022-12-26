package gameserver

import "github.com/stretchr/testify/mock"

const (
	LoadMethod      = "Load"
	RunMethod       = "Run"
	IsRunningMethod = "IsRunning"
	StopMethod      = "Stop"
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

func (m *MockClient) IsRunning() (string, bool) {
	args := m.Called()
	return args.String(0), args.Bool(1)
}

func (m *MockClient) Stop() error {
	args := m.Called()
	return args.Error(0)
}
