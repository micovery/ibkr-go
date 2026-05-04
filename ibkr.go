// Package ibkr provides a Go wrapper around the official Interactive Brokers
// C++ TWS API via CGO. It exposes a minimal, channel-based interface covering
// market data, order management, scanning, historical data, and account
// reconciliation.
//
// The package ships the C bridge source (cgo/) but NOT the IBKR C++ API source,
// which is under the IB API Non-Commercial License (Section 3.3 prohibits
// redistribution). The install.sh script automates the download + build.
//
// Setup:
//
//	go generate ./...                    # downloads IBKR API + builds libibkr_bridge.a
//	CGO_ENABLED=1 go build ./...         # compiles the Go wrapper
//
//go:generate ./install.sh
package ibkr

// TickType represents the type of market data tick from IBKR.
type TickType int

const (
	TickBidPrice  TickType = 1
	TickAskPrice  TickType = 2
	TickLastPrice TickType = 4
	TickBidSize   TickType = 0
	TickAskSize   TickType = 3
	TickLastSize  TickType = 5
	TickVolume    TickType = 8
)

// Tick represents a single market data tick from the IBKR feed.
type Tick struct {
	ReqID     int
	TickType  TickType
	Price     float64
	Size      int64
	Timestamp int64 // Unix milliseconds
}

// DOMLevel represents a single level in the Level 2 order book.
type DOMLevel struct {
	Position    int
	MarketMaker string
	Operation   int // 0=insert, 1=update, 2=delete
	Side        int // 0=ask, 1=bid
	Price       float64
	Size        int
}

// OrderStatus represents an order status update from IBKR.
type OrderStatus struct {
	OrderID   int
	Status    string // Submitted, Filled, Cancelled, Inactive
	Filled    int
	Remaining int
	AvgPrice  float64
	LastPrice float64
}

// Execution represents a fill/execution report from IBKR.
type Execution struct {
	OrderID  int
	ExecID   string
	Ticker   string
	Side     string // BOT, SLD
	Shares   int
	Price    float64
	Time     string
	Exchange string
}

// BrokerPosition represents an actual position held at the broker.
type BrokerPosition struct {
	Ticker  string
	Qty     int     // Positive = long, negative = short
	AvgCost float64 // Average cost per share
}

// OpenOrder represents an order currently live on the broker.
type OpenOrder struct {
	OrderID   int
	Ticker    string
	Action    string  // BUY, SELL
	Qty       int
	OrderType string  // MKT, STP, LMT
	Price     float64 // lmtPrice
	AuxPrice  float64 // auxPrice (stop price for STP orders)
	Status    string  // Submitted, PreSubmitted, Filled, Cancelled, Inactive
}

// HeadlineArticle represents a news headline from IBKR BroadTape.
type HeadlineArticle struct {
	ArticleID string
	Provider  string
	Headline  string
	Time      string
}

// ScannerRow represents a single result from an IBKR scanner subscription.
type ScannerRow struct {
	Rank     int
	Ticker   string
	Exchange string
}

// HistoricalBar represents a single historical OHLCV bar.
type HistoricalBar struct {
	Time   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// IBKRClient is the interface for interacting with the IBKR API.
// The concrete Client struct in this package implements it via CGO.
// Consumers can also implement this interface for mocking/testing.
type IBKRClient interface {
	// Connect establishes the connection to the broker.
	Connect(host string, port int, clientID int) error

	// Disconnect cleanly tears down the connection.
	Disconnect()

	// IsConnected returns true if the client is actively connected.
	IsConnected() bool

	// ── Market Data ──

	// ReqMktData subscribes to real-time market data for a ticker.
	ReqMktData(reqID int, ticker string, genericTicks string) (<-chan Tick, error)

	// CancelMktData unsubscribes from market data for the given request ID.
	CancelMktData(reqID int)

	// ── Level 2 Market Depth ──

	// ReqMktDepth subscribes to Level 2 order book data.
	ReqMktDepth(reqID int, ticker string, numRows int) (<-chan DOMLevel, error)

	// CancelMktDepth unsubscribes from L2 data for the given request ID.
	CancelMktDepth(reqID int)

	// ── Orders ──

	// PlaceOrder submits a new order to the broker.
	PlaceOrder(orderID int, ticker string, action string, qty int, orderType string, price float64, auxPrice float64) error

	// ModifyOrder modifies an existing order (used for ratcheting stops).
	ModifyOrder(orderID int, ticker string, action string, qty int, orderType string, price float64, auxPrice float64) error

	// CancelOrder cancels an existing order.
	CancelOrder(orderID int) error

	// OrderStatusChan returns the channel for receiving order status updates.
	OrderStatusChan() <-chan OrderStatus

	// ExecutionChan returns the channel for receiving execution/fill reports.
	ExecutionChan() <-chan Execution

	// ── Scanner ──

	// ReqScannerData requests a one-shot scanner snapshot.
	ReqScannerData(reqID int, scanCode string, instrument string, locationCode string, params map[string]string) (<-chan []ScannerRow, error)

	// ── Historical Data ──

	// ReqHistoricalData requests historical bars for a ticker.
	ReqHistoricalData(reqID int, ticker string, duration string, barSize string) (<-chan []HistoricalBar, error)

	// ── News ──

	// HeadlineChan returns the channel for receiving BroadTape news headlines.
	HeadlineChan() <-chan HeadlineArticle

	// ── Reconciliation (crash recovery) ──

	// ReqPositions requests a snapshot of all current positions.
	ReqPositions() ([]BrokerPosition, error)

	// ReqOpenOrders requests all open orders for this clientId.
	ReqOpenOrders() ([]OpenOrder, error)
}
