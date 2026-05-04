/*
 * bridge.h — C shim between Go (CGO) and the official IBKR C++ TWS API.
 *
 * This header declares only the functions we actually use from our
 * ibapi.Client interface. The full IBKR EWrapper has 100+ callbacks;
 * we only bridge the ~12 we need.
 *
 * Architecture:
 *   Go (RealClient) → CGO → bridge.h (C) → bridge.cpp (C++) → EClientSocket
 *   EWrapper callbacks → C function pointers → CGO exports → Go channels
 */
#ifndef IBKR_BRIDGE_H
#define IBKR_BRIDGE_H

#ifdef __cplusplus
extern "C" {
#endif

/* ── Lifecycle ── */
int  ibkr_connect(const char* host, int port, int clientId);
void ibkr_disconnect(void);
int  ibkr_is_connected(void);

/* Process incoming messages — call from a goroutine loop */
void ibkr_process_messages(int timeoutMs);

/* ── Market Data ── */
void ibkr_req_mkt_data(int reqId, const char* symbol, const char* genericTicks);
void ibkr_cancel_mkt_data(int reqId);

/* ── Level 2 ── */
void ibkr_req_mkt_depth(int reqId, const char* symbol, int numRows);
void ibkr_cancel_mkt_depth(int reqId);

/* ── Orders ── */
void ibkr_place_order(int orderId, const char* symbol, const char* action,
                      int qty, const char* orderType, double price, double auxPrice);
void ibkr_cancel_order(int orderId);
void ibkr_req_ids(void);

/* ── Reconciliation ── */
void ibkr_req_positions(void);
void ibkr_cancel_positions(void);
void ibkr_req_open_orders(void);

/* ── Scanner ── */
void ibkr_req_scanner(int reqId, const char* scanCode, const char* instrument,
                      const char* location);

/* ── Historical Data ── */
void ibkr_req_historical_data(int reqId, const char* symbol,
                              const char* duration, const char* barSize);

/* ── Callback registration ──
 * These are called from Go (via CGO) to register function pointers
 * that the C++ EWrapper subclass will invoke when events arrive.
 */

/* Tick data: (reqId, tickType, price, size, timestamp) */
typedef void (*tick_callback_t)(int reqId, int tickType, double price, int size, long long timestamp);
void ibkr_register_tick_callback(tick_callback_t cb);

/* Order status: (orderId, status, filled, remaining, avgFillPrice) */
typedef void (*order_status_callback_t)(int orderId, const char* status,
                                        int filled, int remaining, double avgFillPrice);
void ibkr_register_order_status_callback(order_status_callback_t cb);

/* Execution/fill: (orderId, execId, symbol, side, shares, price, time) */
typedef void (*execution_callback_t)(int orderId, const char* execId, const char* symbol,
                                      const char* side, int shares, double price, const char* time);
void ibkr_register_execution_callback(execution_callback_t cb);

/* Position: (symbol, qty, avgCost) — called for each position, then position_end */
typedef void (*position_callback_t)(const char* symbol, int qty, double avgCost);
typedef void (*position_end_callback_t)(void);
void ibkr_register_position_callback(position_callback_t cb, position_end_callback_t endCb);

/* Open order: (orderId, symbol, action, qty, orderType, price, auxPrice, status) */
typedef void (*open_order_callback_t)(int orderId, const char* symbol, const char* action,
                                       int qty, const char* orderType, double price,
                                       double auxPrice, const char* status);
typedef void (*open_order_end_callback_t)(void);
void ibkr_register_open_order_callback(open_order_callback_t cb, open_order_end_callback_t endCb);

/* L2 DOM: (reqId, position, operation, side, price, size) */
typedef void (*dom_callback_t)(int reqId, int position, int operation, int side,
                                double price, int size);
void ibkr_register_dom_callback(dom_callback_t cb);

/* Scanner: (reqId, rank, symbol) — called for each row, then scanner_end */
typedef void (*scanner_callback_t)(int reqId, int rank, const char* symbol);
typedef void (*scanner_end_callback_t)(int reqId);
void ibkr_register_scanner_callback(scanner_callback_t cb, scanner_end_callback_t endCb);

/* Historical bar: (reqId, time, open, high, low, close, volume) */
typedef void (*historical_callback_t)(int reqId, const char* time, double open, double high,
                                       double low, double close, long long volume);
typedef void (*historical_end_callback_t)(int reqId);
void ibkr_register_historical_callback(historical_callback_t cb, historical_end_callback_t endCb);

/* Error: (id, errorCode, errorMsg) */
typedef void (*error_callback_t)(int id, int errorCode, const char* errorMsg);
void ibkr_register_error_callback(error_callback_t cb);

/* News headline: (articleId, headline, timestamp) */
typedef void (*headline_callback_t)(const char* articleId, const char* headline, long long timestamp);
void ibkr_register_headline_callback(headline_callback_t cb);

/* Next valid order ID: (orderId) — sent on connect and on reqIds() */
typedef void (*next_valid_id_callback_t)(int orderId);
void ibkr_register_next_valid_id_callback(next_valid_id_callback_t cb);

#ifdef __cplusplus
}
#endif

#endif /* IBKR_BRIDGE_H */
