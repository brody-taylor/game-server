package monitor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

const device = "any"

var checkRate = time.Second

// Ensure Client implements ClientIFace
var _ ClientIFace = (*Client)(nil)

type ClientIFace interface {
	Start(ports []int32) (chan struct{}, error)
	Close()
}

type Client struct {
	lastTime time.Time
	timeout  time.Duration
	done     chan struct{}
	mu       sync.Mutex

	handler packetHandler
	packets packetSource
}

type packetHandler interface {
	Close()
}

type packetSource interface {
	Packets() chan gopacket.Packet
}

func New(timeout time.Duration) *Client {
	return &Client{
		lastTime: time.Now(),
		timeout:  timeout,
		done:     make(chan struct{}),
	}
}

func (c *Client) Start(ports []int32) (chan struct{}, error) {
	filter := getPortFilter(ports)
	if filter == "" {
		return nil, fmt.Errorf("no ports specified")
	}

	// Get PCAP handler
	h, err := pcap.OpenLive(device, 0, false, pcap.BlockForever)
	if err != nil {
		return nil, err
	}
	c.handler = h
	c.packets = gopacket.NewPacketSource(h, h.LinkType())

	// Set port filter
	if err := h.SetBPFFilter(filter); err != nil {
		h.Close()
		return nil, err
	}

	return c.start(), nil
}

func (c *Client) Close() {
	c.handler.Close()
	close(c.done)
}

func (c *Client) start() chan struct{} {
	c.startPacketHandler()
	c.monitor()
	return c.done
}

func (c *Client) startPacketHandler() {
	// Saves the timestamp from the most recent packet
	go func() {
		for pkt := range c.packets.Packets() {
			timestamp := pkt.Metadata().Timestamp
			c.mu.Lock()
			c.lastTime = timestamp
			c.mu.Unlock()
		}
	}()
}

func (c *Client) monitor() {
	// Monitors time since the last packet until it surpasses the timeout value
	go func() {
		defer c.Close()

		var diff time.Duration
		for diff < c.timeout {
			select {
			case <-c.done:
				return
			default:
				time.Sleep(checkRate)
				c.mu.Lock()
				diff = time.Since(c.lastTime)
				c.mu.Unlock()
			}
		}
	}()
}

func getPortFilter(ports []int32) string {
	if len(ports) < 1 {
		return ""
	}

	stringPorts := make([]string, len(ports))
	for i, port := range ports {
		stringPorts[i] = fmt.Sprint(port)
	}

	return fmt.Sprintf("port (%s)", strings.Join(stringPorts, " or "))
}
