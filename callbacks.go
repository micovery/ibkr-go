//go:build cgo
// +build cgo

package ibkr

/*
#include "bridge.h"

// Forward declarations — implemented below as Go exports
extern void goTickCallback(int reqId, int tickType, double price, int size, long long timestamp);
extern void goOrderStatusCallback(int orderId, char* status, int filled, int remaining, double avgFillPrice);
extern void goExecutionCallback(int orderId, char* execId, char* symbol, char* side, int shares, double price, char* time);
extern void goPositionCallback(char* symbol, int qty, double avgCost);
extern void goPositionEndCallback();
extern void goOpenOrderCallback(int orderId, char* symbol, char* action, int qty, char* orderType, double price, double auxPrice, char* status);
extern void goOpenOrderEndCallback();
extern void goDOMCallback(int reqId, int position, int operation, int side, double price, int size);
extern void goScannerCallback(int reqId, int rank, char* symbol);
extern void goScannerEndCallback(int reqId);
extern void goHistoricalCallback(int reqId, char* time, double open, double high, double low, double close, long long volume);
extern void goHistoricalEndCallback(int reqId);
extern void goErrorCallback(int id, int errorCode, char* errorMsg);
extern void goHeadlineCallback(char* articleId, char* headline, long long timestamp);
extern void goNextValidIdCallback(int orderId);
*/
import "C"
import (
	"log"
	"time"
)

// globalClient is the singleton Client that CGO callbacks route to.
// This is necessary because CGO callbacks are C function pointers — they
// can't carry Go closure state. Only one Client can be active at a time.
var globalClient *Client

func (c *Client) registerCallbacks() {
	globalClient = c
	C.ibkr_register_tick_callback(C.tick_callback_t(C.goTickCallback))
	C.ibkr_register_order_status_callback(C.order_status_callback_t(C.goOrderStatusCallback))
	C.ibkr_register_execution_callback(C.execution_callback_t(C.goExecutionCallback))
	C.ibkr_register_position_callback(C.position_callback_t(C.goPositionCallback), C.position_end_callback_t(C.goPositionEndCallback))
	C.ibkr_register_open_order_callback(C.open_order_callback_t(C.goOpenOrderCallback), C.open_order_end_callback_t(C.goOpenOrderEndCallback))
	C.ibkr_register_dom_callback(C.dom_callback_t(C.goDOMCallback))
	C.ibkr_register_scanner_callback(C.scanner_callback_t(C.goScannerCallback), C.scanner_end_callback_t(C.goScannerEndCallback))
	C.ibkr_register_historical_callback(C.historical_callback_t(C.goHistoricalCallback), C.historical_end_callback_t(C.goHistoricalEndCallback))
	C.ibkr_register_error_callback(C.error_callback_t(C.goErrorCallback))
	C.ibkr_register_headline_callback(C.headline_callback_t(C.goHeadlineCallback))
	C.ibkr_register_next_valid_id_callback(C.next_valid_id_callback_t(C.goNextValidIdCallback))
}

// ── CGO exports — called from C++ via function pointers ──

//export goTickCallback
func goTickCallback(reqId C.int, tickType C.int, price C.double, size C.int, timestamp C.longlong) {
	if globalClient == nil {
		return
	}
	t := Tick{
		ReqID:     int(reqId),
		TickType:  TickType(int(tickType)),
		Price:     float64(price),
		Size:      int64(size),
		Timestamp: int64(timestamp),
	}
	if t.Timestamp == 0 {
		t.Timestamp = time.Now().UnixMilli()
	}
	select {
	case globalClient.tickCh <- t:
	default:
		// Drop tick if channel full — back-pressure
	}
}

//export goOrderStatusCallback
func goOrderStatusCallback(orderId C.int, status *C.char, filled C.int, remaining C.int, avgFillPrice C.double) {
	if globalClient == nil {
		return
	}
	globalClient.orderStatusCh <- OrderStatus{
		OrderID:   int(orderId),
		Status:    C.GoString(status),
		Filled:    int(filled),
		Remaining: int(remaining),
	}
}

//export goExecutionCallback
func goExecutionCallback(orderId C.int, execId *C.char, symbol *C.char, side *C.char, shares C.int, price C.double, execTime *C.char) {
	if globalClient == nil {
		return
	}
	globalClient.executionCh <- Execution{
		OrderID: int(orderId),
		Ticker:  C.GoString(symbol),
		Side:    C.GoString(side),
		Shares:  int(shares),
		Price:   float64(price),
	}
}

//export goPositionCallback
func goPositionCallback(symbol *C.char, qty C.int, avgCost C.double) {
	if globalClient == nil {
		return
	}
	globalClient.positionCh <- BrokerPosition{
		Ticker:  C.GoString(symbol),
		Qty:     int(qty),
		AvgCost: float64(avgCost),
	}
}

//export goPositionEndCallback
func goPositionEndCallback() {
	if globalClient == nil {
		return
	}
	select {
	case globalClient.positionDone <- struct{}{}:
	default:
	}
}

//export goOpenOrderCallback
func goOpenOrderCallback(orderId C.int, symbol *C.char, action *C.char, qty C.int, orderType *C.char, price C.double, auxPrice C.double, status *C.char) {
	if globalClient == nil {
		return
	}
	globalClient.openOrderCh <- OpenOrder{
		OrderID:   int(orderId),
		Ticker:    C.GoString(symbol),
		Action:    C.GoString(action),
		Qty:       int(qty),
		OrderType: C.GoString(orderType),
		Price:     float64(price),
		AuxPrice:  float64(auxPrice),
		Status:    C.GoString(status),
	}
}

//export goOpenOrderEndCallback
func goOpenOrderEndCallback() {
	if globalClient == nil {
		return
	}
	select {
	case globalClient.openOrderDone <- struct{}{}:
	default:
	}
}

//export goDOMCallback
func goDOMCallback(reqId C.int, position C.int, operation C.int, side C.int, price C.double, size C.int) {
	if globalClient == nil {
		return
	}
	globalClient.domMu.Lock()
	ch, ok := globalClient.domChans[int(reqId)]
	globalClient.domMu.Unlock()
	if ok {
		select {
		case ch <- DOMLevel{
			Position:  int(position),
			Operation: int(operation),
			Side:      int(side),
			Price:     float64(price),
			Size:      int(size),
		}:
		default:
		}
	}
}

//export goScannerCallback
func goScannerCallback(reqId C.int, rank C.int, symbol *C.char) {
	if globalClient == nil {
		return
	}
	globalClient.scannerMu.Lock()
	if ch, ok := globalClient.scanners[int(reqId)]; ok {
		_ = ch // rows accumulated in scannerEnd
	}
	globalClient.scannerMu.Unlock()
}

//export goScannerEndCallback
func goScannerEndCallback(reqId C.int) {
	if globalClient == nil {
		return
	}
	globalClient.scannerMu.Lock()
	ch, ok := globalClient.scanners[int(reqId)]
	delete(globalClient.scanners, int(reqId))
	globalClient.scannerMu.Unlock()
	if ok {
		ch <- nil
		close(ch)
	}
}

//export goHistoricalCallback
func goHistoricalCallback(reqId C.int, barTime *C.char, open C.double, high C.double, low C.double, close_ C.double, volume C.longlong) {
	if globalClient == nil {
		return
	}
	globalClient.histMu.Lock()
	globalClient.histBars[int(reqId)] = append(globalClient.histBars[int(reqId)], HistoricalBar{
		Time:   C.GoString(barTime),
		Open:   float64(open),
		High:   float64(high),
		Low:    float64(low),
		Close:  float64(close_),
		Volume: int64(volume),
	})
	globalClient.histMu.Unlock()
}

//export goHistoricalEndCallback
func goHistoricalEndCallback(reqId C.int) {
	if globalClient == nil {
		return
	}
	globalClient.histMu.Lock()
	doneCh, ok := globalClient.histDoneChs[int(reqId)]
	globalClient.histMu.Unlock()
	if ok {
		select {
		case doneCh <- struct{}{}:
		default:
		}
	}
}

//export goErrorCallback
func goErrorCallback(id C.int, errorCode C.int, errorMsg *C.char) {
	log.Printf("[IBKR] Error %d (id=%d): %s", int(errorCode), int(id), C.GoString(errorMsg))
}

//export goHeadlineCallback
func goHeadlineCallback(articleId *C.char, headline *C.char, timestamp C.longlong) {
	if globalClient == nil {
		return
	}
	select {
	case globalClient.headlineCh <- HeadlineArticle{
		ArticleID: C.GoString(articleId),
		Headline:  C.GoString(headline),
	}:
	default:
	}
}

//export goNextValidIdCallback
func goNextValidIdCallback(orderId C.int) {
	if globalClient == nil {
		return
	}
	globalClient.nextOrderID.Store(int64(orderId))
}
