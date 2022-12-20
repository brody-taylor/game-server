package monitor

import (
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_New(t *testing.T) {
	timeout := time.Minute

	m := New(timeout)

	require.NotNil(t, m)
	assert.NotEmpty(t, m.lastTime)
	assert.Equal(t, timeout, m.timeout)
	assert.NotNil(t, m.done)
}

func Test_Client_start(t *testing.T) {
	timeout := 30 * time.Second
	pktChan := make(chan gopacket.Packet)

	// Speed up check rate
	defer func(origRate time.Duration) {
		checkRate = origRate
	}(checkRate)
	checkRate = 0

	// Setup PCAP mock
	pcapMock := new(mockPacketHandler)
	pcapMock.On(mockPacketHandlerPacketsMethod).Return(pktChan)
	pcapMock.On(mockPacketHandlerCloseMethod).Return()

	c := New(timeout)
	c.handler = pcapMock
	c.packets = pcapMock

	done := c.start()

	// Simulate traffic
	for i := 0; i < 10; i++ {
		pkt := &mockPacket{
			Timestamp: time.Now(),
		}
		pktChan <- pkt
	}

	// Ensure done channel is still open
	select {
	case <-done:
		require.Fail(t, "Channel not expected to be closed")
	case <-time.After(time.Millisecond):
	}

	// Simulate stale traffic to trigger timeout
	timeoutPkt := new(mockPacket)
	timeoutPkt.Timestamp = time.Now().Add(-2 * timeout)
	pktChan <- timeoutPkt

	// Ensure done channel and packet handler were closed
	select {
	case <-done:
	case <-time.After(time.Millisecond):
		assert.Fail(t, "Channel was not closed")
	}
	pcapMock.AssertCalled(t, mockPacketHandlerCloseMethod)
}
