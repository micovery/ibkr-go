//go:build cgo
// +build cgo

package ibkr

import "testing"

// TestClientImplementsInterface verifies Client satisfies IBKRClient at compile time.
func TestClientImplementsInterface(t *testing.T) {
	var _ IBKRClient = (*Client)(nil)
}

// TestNewClient verifies the constructor initializes channels and maps.
func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.tickCh == nil {
		t.Error("tickCh not initialized")
	}
	if c.orderStatusCh == nil {
		t.Error("orderStatusCh not initialized")
	}
	if c.executionCh == nil {
		t.Error("executionCh not initialized")
	}
	if c.scanners == nil {
		t.Error("scanners map not initialized")
	}
	if c.domChans == nil {
		t.Error("domChans map not initialized")
	}
	if c.IsConnected() {
		t.Error("new client should not be connected")
	}
}

// TestConnectBadHost verifies Connect returns an error for a refused connection.
func TestConnectBadHost(t *testing.T) {
	c := NewClient()
	err := c.Connect("127.0.0.1", 1, 99) // port 1 — instant connection refused
	if err == nil {
		t.Error("Connect to refused port should return error")
		c.Disconnect()
	}
}

// TestChannelAccessors verifies the channel accessor methods return non-nil channels.
func TestChannelAccessors(t *testing.T) {
	c := NewClient()
	if c.OrderStatusChan() == nil {
		t.Error("OrderStatusChan returned nil")
	}
	if c.ExecutionChan() == nil {
		t.Error("ExecutionChan returned nil")
	}
	if c.HeadlineChan() == nil {
		t.Error("HeadlineChan returned nil")
	}
}
