//go:build cgo
// +build cgo

// client.go implements IBKRClient using CGO to call the official
// IBKR C++ TWS API (EClientSocket/EWrapper) via the C bridge layer.
//
// Build prerequisites:
//
//	go generate ./...   (builds libibkr_bridge.a via cgo/Makefile)

package ibkr

/*
#cgo pkg-config: ibkr_bridge
#include "bridge.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// Client wraps the official IBKR C++ TWS API via CGO.
// It implements the IBKRClient interface.
type Client struct {
	mu        sync.RWMutex
	connected bool

	// Event channels
	tickCh        chan Tick
	orderStatusCh chan OrderStatus
	executionCh   chan Execution
	headlineCh    chan HeadlineArticle

	// Internal channels for synchronous request collection
	positionCh    chan BrokerPosition
	positionDone  chan struct{}
	openOrderCh   chan OpenOrder
	openOrderDone chan struct{}

	// Scanner channels: reqId → result channel
	scannerMu sync.Mutex
	scanners  map[int]chan []ScannerRow

	// Historical channels: reqId → bar collector
	histMu      sync.Mutex
	histBars    map[int][]HistoricalBar
	histDoneChs map[int]chan struct{}

	// L2 DOM channels: reqId → channel
	domMu    sync.Mutex
	domChans map[int]chan DOMLevel

	// Message processing stop channel
	stopCh chan struct{}

	// Next valid order ID (set by IBKR on connect, updated atomically)
	nextOrderID atomic.Int64
}

// NewClient creates a new IBKR client backed by the official C++ API.
func NewClient() *Client {
	c := &Client{
		tickCh:        make(chan Tick, 1000),
		orderStatusCh: make(chan OrderStatus, 100),
		executionCh:   make(chan Execution, 100),
		headlineCh:    make(chan HeadlineArticle, 100),
		positionCh:    make(chan BrokerPosition, 100),
		positionDone:  make(chan struct{}, 1),
		openOrderCh:   make(chan OpenOrder, 100),
		openOrderDone: make(chan struct{}, 1),
		scanners:      make(map[int]chan []ScannerRow),
		histBars:      make(map[int][]HistoricalBar),
		histDoneChs:   make(map[int]chan struct{}),
		domChans:      make(map[int]chan DOMLevel),
		stopCh:        make(chan struct{}),
	}

	// Register CGO callbacks
	c.registerCallbacks()

	return c
}

// ── IBKRClient interface implementation ──

func (c *Client) Connect(host string, port int, clientID int) error {
	cHost := C.CString(host)
	defer C.free(unsafe.Pointer(cHost))

	ok := C.ibkr_connect(cHost, C.int(port), C.int(clientID))
	if ok == 0 {
		return fmt.Errorf("failed to connect to IBKR at %s:%d", host, port)
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	// Start message processing goroutine
	go c.processLoop()

	log.Printf("[IBKR] Connected to %s:%d (clientID=%d)", host, port, clientID)
	return nil
}

func (c *Client) Disconnect() {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	close(c.stopCh)
	C.ibkr_disconnect()
	log.Println("[IBKR] Disconnected")
}

func (c *Client) IsConnected() bool {
	return C.ibkr_is_connected() != 0
}

func (c *Client) ReqMktData(reqID int, symbol string, genericTicks string) (<-chan Tick, error) {
	cSymbol := C.CString(symbol)
	cTicks := C.CString(genericTicks)
	defer C.free(unsafe.Pointer(cSymbol))
	defer C.free(unsafe.Pointer(cTicks))

	C.ibkr_req_mkt_data(C.int(reqID), cSymbol, cTicks)
	return c.tickCh, nil
}

func (c *Client) CancelMktData(reqID int) {
	C.ibkr_cancel_mkt_data(C.int(reqID))
}

func (c *Client) ReqMktDepth(reqID int, symbol string, numRows int) (<-chan DOMLevel, error) {
	cSymbol := C.CString(symbol)
	defer C.free(unsafe.Pointer(cSymbol))

	ch := make(chan DOMLevel, 100)
	c.domMu.Lock()
	c.domChans[reqID] = ch
	c.domMu.Unlock()

	C.ibkr_req_mkt_depth(C.int(reqID), cSymbol, C.int(numRows))
	return ch, nil
}

func (c *Client) CancelMktDepth(reqID int) {
	C.ibkr_cancel_mkt_depth(C.int(reqID))
	c.domMu.Lock()
	delete(c.domChans, reqID)
	c.domMu.Unlock()
}

func (c *Client) PlaceOrder(orderID int, symbol, action string, qty int, orderType string, price, auxPrice float64) error {
	cSymbol := C.CString(symbol)
	cAction := C.CString(action)
	cType := C.CString(orderType)
	defer C.free(unsafe.Pointer(cSymbol))
	defer C.free(unsafe.Pointer(cAction))
	defer C.free(unsafe.Pointer(cType))

	C.ibkr_place_order(C.int(orderID), cSymbol, cAction, C.int(qty), cType, C.double(price), C.double(auxPrice))
	return nil
}

func (c *Client) ModifyOrder(orderID int, symbol, action string, qty int, orderType string, price, auxPrice float64) error {
	// IBKR uses PlaceOrder with same orderID to modify
	return c.PlaceOrder(orderID, symbol, action, qty, orderType, price, auxPrice)
}

func (c *Client) CancelOrder(orderID int) error {
	C.ibkr_cancel_order(C.int(orderID))
	return nil
}

func (c *Client) ReqPositions() ([]BrokerPosition, error) {
	// Drain any stale data
	for len(c.positionCh) > 0 {
		<-c.positionCh
	}
	for len(c.positionDone) > 0 {
		<-c.positionDone
	}

	C.ibkr_req_positions()

	timeout := time.After(5 * time.Second)
	var positions []BrokerPosition
	for {
		select {
		case p := <-c.positionCh:
			positions = append(positions, p)
		case <-c.positionDone:
			C.ibkr_cancel_positions()
			return positions, nil
		case <-timeout:
			C.ibkr_cancel_positions()
			return positions, nil
		}
	}
}

func (c *Client) ReqOpenOrders() ([]OpenOrder, error) {
	// Drain any stale data
	for len(c.openOrderCh) > 0 {
		<-c.openOrderCh
	}
	for len(c.openOrderDone) > 0 {
		<-c.openOrderDone
	}

	C.ibkr_req_open_orders()

	timeout := time.After(5 * time.Second)
	var orders []OpenOrder
	for {
		select {
		case o := <-c.openOrderCh:
			orders = append(orders, o)
		case <-c.openOrderDone:
			return orders, nil
		case <-timeout:
			return orders, nil
		}
	}
}

func (c *Client) ReqScannerData(reqID int, scanCode, instrument, location string, filters map[string]string) (<-chan []ScannerRow, error) {
	cScan := C.CString(scanCode)
	cInst := C.CString(instrument)
	cLoc := C.CString(location)
	defer C.free(unsafe.Pointer(cScan))
	defer C.free(unsafe.Pointer(cInst))
	defer C.free(unsafe.Pointer(cLoc))

	ch := make(chan []ScannerRow, 1)
	c.scannerMu.Lock()
	c.scanners[reqID] = ch
	c.scannerMu.Unlock()

	C.ibkr_req_scanner(C.int(reqID), cScan, cInst, cLoc)
	return ch, nil
}

func (c *Client) ReqHistoricalData(reqID int, symbol, duration, barSize string) (<-chan []HistoricalBar, error) {
	cSymbol := C.CString(symbol)
	cDur := C.CString(duration)
	cBar := C.CString(barSize)
	defer C.free(unsafe.Pointer(cSymbol))
	defer C.free(unsafe.Pointer(cDur))
	defer C.free(unsafe.Pointer(cBar))

	doneCh := make(chan struct{}, 1)
	c.histMu.Lock()
	c.histBars[reqID] = nil
	c.histDoneChs[reqID] = doneCh
	c.histMu.Unlock()

	resultCh := make(chan []HistoricalBar, 1)

	C.ibkr_req_historical_data(C.int(reqID), cSymbol, cDur, cBar)

	go func() {
		<-doneCh
		c.histMu.Lock()
		bars := c.histBars[reqID]
		delete(c.histBars, reqID)
		delete(c.histDoneChs, reqID)
		c.histMu.Unlock()
		resultCh <- bars
	}()

	return resultCh, nil
}

func (c *Client) OrderStatusChan() <-chan OrderStatus { return c.orderStatusCh }
func (c *Client) ExecutionChan() <-chan Execution     { return c.executionCh }
func (c *Client) HeadlineChan() <-chan HeadlineArticle { return c.headlineCh }

// NextOrderID returns the next valid order ID and atomically increments
// the internal counter. IBKR sends the initial value on connect via the
// nextValidId callback. Each call returns a unique, monotonically
// increasing order ID safe for use with PlaceOrder.
func (c *Client) NextOrderID() int {
	return int(c.nextOrderID.Add(1) - 1)
}

// ── Message processing loop ──

func (c *Client) processLoop() {
	for {
		select {
		case <-c.stopCh:
			return
		default:
			C.ibkr_process_messages(C.int(100))
		}
	}
}
