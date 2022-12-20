package monitor

import (
	"time"

	"github.com/google/gopacket"
	"github.com/stretchr/testify/mock"
)

const (
	StartMethod = "Start"
	CloseMethod = "Close"
)

// Ensure MockClient implements ClientIFace
var _ ClientIFace = (*MockClient)(nil)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) Start(ports []int32) (chan struct{}, error) {
	args := m.Called(ports)
	if c := args.Get(0); c != nil {
		return args.Get(0).(chan struct{}), args.Error(1)
	}
	return args.Get(0).(chan struct{}), args.Error(1)
}

func (m *MockClient) Close() {
	m.Called()
}

const (
	mockPacketHandlerCloseMethod   = "Close"
	mockPacketHandlerPacketsMethod = "Packets"
)

// Ensure mockPacketHandler implements packetHandler and packetSource
var _ packetHandler = (*mockPacketHandler)(nil)
var _ packetSource = (*mockPacketHandler)(nil)

type mockPacketHandler struct {
	mock.Mock
}

func (m *mockPacketHandler) Close() {
	m.Called()
}

func (m *mockPacketHandler) Packets() chan gopacket.Packet {
	args := m.Called()
	return args.Get(0).(chan gopacket.Packet)
}

var _ gopacket.Packet = (*mockPacket)(nil)

type mockPacket struct {
	gopacket.Packet
	Timestamp time.Time
}

func (m *mockPacket) Metadata() *gopacket.PacketMetadata {
	metadata := &gopacket.PacketMetadata{}
	metadata.Timestamp = m.Timestamp
	return metadata
}
