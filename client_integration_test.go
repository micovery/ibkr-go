//go:build cgo && integration
// +build cgo,integration

// client_integration_test.go — Live integration tests against IBKR paper trading.
//
// Prerequisites:
//   - IBKR Desktop or IB Gateway running on localhost:4001 (paper account)
//   - API connections enabled in IBKR settings
//   - ./install.sh (builds and installs libibkr_bridge.a)
//
// Run all:
//   CGO_ENABLED=1 go test -v -tags integration -timeout 60s
//
// Run one:
//   CGO_ENABLED=1 go test -v -tags integration -run TestPositions -timeout 30s

package ibkr

import (
	"testing"
	"time"
)

const (
	ibkrHost     = "127.0.0.1"
	ibkrPort     = 4001
	ibkrClientID = 99 // Use a high client ID to avoid conflicts
)

// connect is a test helper that returns a connected client or skips the test.
func connect(t *testing.T) *Client {
	t.Helper()
	c := NewClient()
	err := c.Connect(ibkrHost, ibkrPort, ibkrClientID)
	if err != nil {
		t.Skipf("IBKR not available at %s:%d — skipping: %v", ibkrHost, ibkrPort, err)
	}
	time.Sleep(500 * time.Millisecond)
	return c
}

func TestConnect(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	if !c.IsConnected() {
		t.Fatal("Expected IsConnected() to return true after Connect()")
	}
}

func TestPositions(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	positions, err := c.ReqPositions()
	if err != nil {
		t.Fatalf("ReqPositions failed: %v", err)
	}
	t.Logf("Found %d position(s)", len(positions))
	for _, p := range positions {
		t.Logf("  %s: qty=%d avgCost=%.2f", p.Ticker, p.Qty, p.AvgCost)
		if p.Ticker == "" {
			t.Error("Position has empty ticker")
		}
	}
}

func TestOpenOrders(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	orders, err := c.ReqOpenOrders()
	if err != nil {
		t.Fatalf("ReqOpenOrders failed: %v", err)
	}
	t.Logf("Found %d open order(s)", len(orders))
	for _, o := range orders {
		t.Logf("  #%d %s %s %d x %s $%.2f status=%s",
			o.OrderID, o.Action, o.Ticker, o.Qty, o.OrderType, o.Price, o.Status)
		if o.OrderID == 0 {
			t.Error("Order has zero OrderID")
		}
		if o.Status == "" {
			t.Error("Order has empty status")
		}
	}
}

func TestMarketData(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	ticker := "AAPL"
	reqID := 2001
	tickCh, err := c.ReqMktData(reqID, ticker, "")
	if err != nil {
		t.Fatalf("ReqMktData failed: %v", err)
	}
	defer c.CancelMktData(reqID)

	deadline := time.After(5 * time.Second)
	tickCount := 0

	for done := false; !done; {
		select {
		case tick := <-tickCh:
			if tick.ReqID != reqID {
				continue
			}
			tickCount++
			if tickCount <= 5 {
				t.Logf("  tick: type=%d price=%.4f size=%d", tick.TickType, tick.Price, tick.Size)
			}
		case <-deadline:
			done = true
		}
	}

	t.Logf("Received %d ticks for %s", tickCount, ticker)
	if tickCount == 0 {
		t.Log("⚠ No ticks received — market may be closed")
	}
}

func TestMarketDepth(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	ticker := "AAPL"
	reqID := 2002
	domCh, err := c.ReqMktDepth(reqID, ticker, 5)
	if err != nil {
		t.Fatalf("ReqMktDepth failed: %v", err)
	}
	defer c.CancelMktDepth(reqID)

	deadline := time.After(5 * time.Second)
	levelCount := 0

	for done := false; !done; {
		select {
		case level := <-domCh:
			levelCount++
			if levelCount <= 5 {
				t.Logf("  L2: pos=%d side=%d price=%.2f size=%d op=%d",
					level.Position, level.Side, level.Price, level.Size, level.Operation)
			}
		case <-deadline:
			done = true
		}
	}

	t.Logf("Received %d L2 updates for %s", levelCount, ticker)
	if levelCount == 0 {
		t.Log("⚠ No L2 data — market may be closed or no L2 subscription")
	}
}

func TestHistoricalData(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	ticker := "AAPL"
	reqID := 2003
	barCh, err := c.ReqHistoricalData(reqID, ticker, "1 D", "5 mins")
	if err != nil {
		t.Fatalf("ReqHistoricalData failed: %v", err)
	}

	select {
	case bars := <-barCh:
		t.Logf("Received %d historical bars for %s", len(bars), ticker)
		if len(bars) == 0 {
			t.Error("Expected at least 1 historical bar")
		}
		for i, b := range bars {
			if i >= 3 {
				t.Logf("  ... (%d more bars)", len(bars)-3)
				break
			}
			t.Logf("  %s O=%.2f H=%.2f L=%.2f C=%.2f V=%d",
				b.Time, b.Open, b.High, b.Low, b.Close, b.Volume)
		}
		// Validate bar integrity
		if len(bars) > 0 {
			b := bars[0]
			if b.High < b.Low {
				t.Errorf("Invalid bar: High (%.2f) < Low (%.2f)", b.High, b.Low)
			}
			if b.Open == 0 || b.Close == 0 {
				t.Error("Bar has zero Open or Close price")
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for historical data")
	}
}

func TestScannerData(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	reqID := 2004
	resultCh, err := c.ReqScannerData(reqID, "TOP_PERC_GAIN", "STK", "STK.US.MAJOR", nil)
	if err != nil {
		t.Fatalf("ReqScannerData failed: %v", err)
	}

	select {
	case rows := <-resultCh:
		t.Logf("Scanner returned %d results", len(rows))
		if len(rows) == 0 {
			t.Log("⚠ No scanner results — market may be closed")
		}
		for i, r := range rows {
			if i >= 5 {
				t.Logf("  ... (%d more results)", len(rows)-5)
				break
			}
			t.Logf("  #%d %s (%s)", r.Rank, r.Ticker, r.Exchange)
			if r.Ticker == "" {
				t.Error("Scanner row has empty ticker")
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for scanner data")
	}
}

func TestReconnect(t *testing.T) {
	c := connect(t)

	// First connection
	if !c.IsConnected() {
		t.Fatal("Expected connected after Connect()")
	}

	// Disconnect
	c.Disconnect()
	time.Sleep(500 * time.Millisecond)

	// Reconnect with new client (IBKR C++ API doesn't support reconnecting same instance)
	c2 := NewClient()
	err := c2.Connect(ibkrHost, ibkrPort, ibkrClientID)
	if err != nil {
		t.Fatalf("Reconnect failed: %v", err)
	}
	defer c2.Disconnect()

	time.Sleep(500 * time.Millisecond)
	if !c2.IsConnected() {
		t.Fatal("Expected connected after reconnect")
	}

	// Verify the reconnected client can make requests
	positions, err := c2.ReqPositions()
	if err != nil {
		t.Fatalf("ReqPositions after reconnect failed: %v", err)
	}
	t.Logf("Reconnected — found %d position(s)", len(positions))
}

func TestOrderLifecycle(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	// Use IBKR's next valid order ID (sent on connect)
	ticker := "SPY"
	orderID := c.NextOrderID()
	limitPrice := 1.00 // $1 limit — will never fill

	// Safety net: always cancel on exit
	defer func() {
		c.CancelOrder(orderID)
		time.Sleep(2 * time.Second)
	}()

	t.Logf("Placing LMT BUY %s x1 @ $%.2f (orderID=%d)", ticker, limitPrice, orderID)
	err := c.PlaceOrder(orderID, ticker, "BUY", 1, "LMT", limitPrice, 0)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	// Wait for any order status
	statusCh := c.OrderStatusChan()
	select {
	case s := <-statusCh:
		t.Logf("  Status: %s (orderID=%d filled=%d remaining=%d)", s.Status, s.OrderID, s.Filled, s.Remaining)
	case <-time.After(5 * time.Second):
		t.Log("  ⚠ No order status within 5s (paper-trading popup may need acknowledgment)")
	}

	// Cancel
	t.Logf("Cancelling order %d...", orderID)
	c.CancelOrder(orderID)

	// Wait for cancel confirmation
	select {
	case s := <-statusCh:
		t.Logf("  Cancel status: %s", s.Status)
	case <-time.After(5 * time.Second):
		t.Log("  ⚠ No cancel status within 5s")
	}

	// Verify gone
	time.Sleep(1 * time.Second)
	orders, _ := c.ReqOpenOrders()
	for _, o := range orders {
		if o.OrderID == orderID {
			t.Errorf("Order %d still open after cancel (status=%s)", orderID, o.Status)
		}
	}
	t.Log("Order lifecycle complete")
}

func TestNextOrderID(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	ids := make([]int, 10)
	for i := range ids {
		ids[i] = c.NextOrderID()
	}

	t.Logf("Generated IDs: %v", ids)
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("IDs not monotonically increasing: ids[%d]=%d <= ids[%d]=%d",
				i, ids[i], i-1, ids[i-1])
		}
	}
	if ids[0] == 0 {
		t.Error("First ID is 0 — nextValidId callback may not have fired")
	}
}

func TestMultipleSubscriptions(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	type sub struct {
		symbol string
		reqID  int
	}
	subs := []sub{
		{"AAPL", 3001},
		{"MSFT", 3002},
		{"GOOG", 3003},
	}

	// Subscribe to all — they share the same tick channel
	var tickCh <-chan Tick
	for _, s := range subs {
		ch, err := c.ReqMktData(s.reqID, s.symbol, "")
		if err != nil {
			t.Fatalf("ReqMktData(%s) failed: %v", s.symbol, err)
		}
		defer c.CancelMktData(s.reqID)
		tickCh = ch
	}

	// Collect ticks for 3 seconds
	tickCounts := map[int]int{}
	deadline := time.After(3 * time.Second)
	for done := false; !done; {
		select {
		case tick := <-tickCh:
			tickCounts[tick.ReqID]++
		case <-deadline:
			done = true
		}
	}

	for _, s := range subs {
		t.Logf("  %s (reqID=%d): %d ticks", s.symbol, s.reqID, tickCounts[s.reqID])
	}
	total := 0
	for _, count := range tickCounts {
		total += count
	}
	if total == 0 {
		t.Log("⚠ No ticks received — market may be closed")
	} else {
		t.Logf("Total: %d ticks across %d tickers", total, len(tickCounts))
	}
}

func TestHistoricalBarSizes(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	tests := []struct {
		duration string
		barSize  string
		minBars  int
	}{
		{"1 D", "1 hour", 4},  // ~7 bars per day
		{"1 D", "5 mins", 20}, // ~78 bars per day
	}

	for i, tt := range tests {
		reqID := 4001 + i
		barCh, err := c.ReqHistoricalData(reqID, "SPY", tt.duration, tt.barSize)
		if err != nil {
			t.Fatalf("ReqHistoricalData(%s, %s) failed: %v", tt.duration, tt.barSize, err)
		}

		select {
		case bars := <-barCh:
			t.Logf("  %s / %s → %d bars", tt.duration, tt.barSize, len(bars))
			if len(bars) < tt.minBars {
				t.Errorf("Expected at least %d bars for %s/%s, got %d",
					tt.minBars, tt.duration, tt.barSize, len(bars))
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("Timed out waiting for %s/%s historical data", tt.duration, tt.barSize)
		}
	}
}

func TestOrderModify(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	ticker := "SPY"
	orderID := c.NextOrderID()

	defer func() {
		c.CancelOrder(orderID)
		time.Sleep(2 * time.Second)
	}()

	// Place at $1
	t.Logf("Placing LMT BUY %s @ $1.00 (orderID=%d)", ticker, orderID)
	err := c.PlaceOrder(orderID, ticker, "BUY", 1, "LMT", 1.00, 0)
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	statusCh := c.OrderStatusChan()
	select {
	case s := <-statusCh:
		t.Logf("  Initial status: %s", s.Status)
	case <-time.After(5 * time.Second):
		t.Log("  ⚠ No initial status within 5s")
	}

	// Modify to $2 (same orderID = modify in IBKR)
	t.Logf("Modifying order %d → $2.00", orderID)
	err = c.PlaceOrder(orderID, ticker, "BUY", 1, "LMT", 2.00, 0)
	if err != nil {
		t.Fatalf("PlaceOrder (modify) failed: %v", err)
	}

	select {
	case s := <-statusCh:
		t.Logf("  Modified status: %s", s.Status)
	case <-time.After(5 * time.Second):
		t.Log("  ⚠ No modify status within 5s")
	}

	// Cancel
	t.Logf("Cancelling order %d", orderID)
	c.CancelOrder(orderID)

	select {
	case s := <-statusCh:
		t.Logf("  Cancel status: %s", s.Status)
	case <-time.After(5 * time.Second):
		t.Log("  ⚠ No cancel status within 5s")
	}
}

func TestCancelNonexistent(t *testing.T) {
	c := connect(t)
	defer c.Disconnect()

	// Cancel a bogus order — should not crash
	bogusID := 999999999
	t.Logf("Cancelling nonexistent order %d...", bogusID)
	err := c.CancelOrder(bogusID)
	if err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
	// Give time for error callback to fire (should just log, not crash)
	time.Sleep(1 * time.Second)
	t.Log("No crash — graceful handling confirmed")
}

func TestRapidConnectDisconnect(t *testing.T) {
	for i := 0; i < 5; i++ {
		c := NewClient()
		err := c.Connect(ibkrHost, ibkrPort, ibkrClientID)
		if err != nil {
			t.Skipf("IBKR not available — skipping: %v", err)
		}
		time.Sleep(200 * time.Millisecond)

		if !c.IsConnected() {
			t.Errorf("Cycle %d: not connected after Connect()", i)
		}

		c.Disconnect()
		time.Sleep(200 * time.Millisecond)
	}
	t.Log("5 rapid connect/disconnect cycles completed without crash")
}
