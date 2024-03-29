package instance

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/mock"
)

const (
	ConnectMethod            = "Connect"
	ConnectWithSessionMethod = "ConnectWithSession"
	GetSessionMethod         = "GetSession"
	GetInstanceStateMethod   = "GetInstanceState"
	GetInstanceAddressMethod = "GetInstanceAddress"
	StartInstanceMethod      = "StartInstance"
)

// Ensure MockClient implements ClientIFace
var _ ClientIFace = (*MockClient)(nil)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) Connect() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockClient) ConnectWithSession(awsSessions *session.Session) {
	_ = m.Called(awsSessions)
}

func (m *MockClient) GetSession() *session.Session {
	args := m.Called()
	return args.Get(0).(*session.Session)
}

func (m *MockClient) GetInstanceState(id string) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockClient) GetInstanceAddress(id string) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockClient) StartInstance(id string) error {
	args := m.Called(id)
	return args.Error(0)
}
