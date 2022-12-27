package s3

import (
	"io"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/mock"
)

const (
	ConnectMethod            = "Connect"
	ConnectWithSessionMethod = "ConnectWithSession"
	GetSessionMethod         = "GetSession"
	GetFoldersMethod         = "GetFolders"
	PutMethod                = "Put"
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

func (m *MockClient) GetFolders(bucket string, depth int) ([]string, error) {
	args := m.Called(bucket, depth)
	if folders := args.Get(0); folders != nil {
		return folders.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockClient) Put(file io.ReadSeeker, bucket string, key string) error {
	args := m.Called(file, bucket, key)
	return args.Error(0)
}
