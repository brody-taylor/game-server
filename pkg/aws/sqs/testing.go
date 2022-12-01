package sqs

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/mock"
)

const (
	ConnectMethod            = "Connect"
	ConnectWithSessionMethod = "ConnectWithSession"
	GetSessionMethod         = "GetSession"
	SendMethod               = "Send"
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

func (m *MockClient) Send(queueUrl string, msg string) error {
	args := m.Called(queueUrl, msg)
	return args.Error(0)
}