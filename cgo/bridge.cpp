/*
 * bridge.cpp — C++ implementation of the IBKR CGO bridge.
 *
 * Subclasses DefaultEWrapper (official no-op stubs) and only overrides
 * the ~12 callbacks our ibapi.Client interface needs.
 *
 * Build: compile with the IBKR C++ source files into a static library.
 */

#include "bridge.h"

// IBKR C++ API headers
#include "EClientSocket.h"
#include "EReader.h"
#include "EReaderOSSignal.h"
#include "DefaultEWrapper.h"
#include "Contract.h"
#include "Order.h"
#include "OrderCancel.h"
#include "Execution.h"
#include "OrderState.h"
#include "ScannerSubscription.h"
#include "CommissionAndFeesReport.h"
#include "CommonDefs.h"
#include "Decimal.h"
#include "bar.h"

// Helper to convert IBKR's Decimal (BID64) to double
static inline double toDouble(Decimal d) { return DecimalFunctions::decimalToDouble(d); }
static inline Decimal fromDouble(double d) { return DecimalFunctions::doubleToDecimal(d); }

#include <cstring>
#include <string>
#include <memory>
#include <mutex>

// ── Registered callbacks (set from Go via CGO) ──
static tick_callback_t          g_tickCb          = nullptr;
static order_status_callback_t  g_orderStatusCb   = nullptr;
static execution_callback_t     g_executionCb     = nullptr;
static position_callback_t      g_positionCb      = nullptr;
static position_end_callback_t  g_positionEndCb   = nullptr;
static open_order_callback_t    g_openOrderCb     = nullptr;
static open_order_end_callback_t g_openOrderEndCb = nullptr;
static dom_callback_t           g_domCb           = nullptr;
static scanner_callback_t       g_scannerCb       = nullptr;
static scanner_end_callback_t   g_scannerEndCb    = nullptr;
static historical_callback_t    g_historicalCb    = nullptr;
static historical_end_callback_t g_historicalEndCb = nullptr;
static error_callback_t         g_errorCb         = nullptr;
static headline_callback_t      g_headlineCb      = nullptr;
static next_valid_id_callback_t g_nextValidIdCb   = nullptr;

// ── Minimal EWrapper subclass (inherits all no-op stubs) ──
class BridgeWrapper : public DefaultEWrapper {
public:
    // Market data
    void tickPrice(TickerId tickerId, TickType field, double price,
                   const TickAttrib& attribs) override {
        if (g_tickCb) g_tickCb((int)tickerId, (int)field, price, 0, 0);
    }

    void tickSize(TickerId tickerId, TickType field, Decimal size) override {
        if (g_tickCb) g_tickCb((int)tickerId, (int)field, 0.0, (int)toDouble(size), 0);
    }

    // Order status
    void orderStatus(OrderId orderId, const std::string& status,
                     Decimal filled, Decimal remaining,
                     double avgFillPrice, long long permId, int parentId,
                     double lastFillPrice, int clientId,
                     const std::string& whyHeld, double mktCapPrice) override {
        if (g_orderStatusCb) {
            g_orderStatusCb((int)orderId, status.c_str(),
                           (int)toDouble(filled),
                           (int)toDouble(remaining),
                           avgFillPrice);
        }
    }

    // Execution (fill)
    void execDetails(int reqId, const Contract& contract,
                     const Execution& exec) override {
        if (g_executionCb) {
            g_executionCb((int)exec.orderId, exec.execId.c_str(),
                         contract.symbol.c_str(), exec.side.c_str(),
                         (int)toDouble(exec.shares), exec.price,
                         exec.time.c_str());
        }
    }

    // Positions
    void position(const std::string& account, const Contract& contract,
                  Decimal pos, double avgCost) override {
        if (g_positionCb) {
            g_positionCb(contract.symbol.c_str(), (int)toDouble(pos), avgCost);
        }
    }

    void positionEnd() override {
        if (g_positionEndCb) g_positionEndCb();
    }

    // Open orders
    void openOrder(OrderId orderId, const Contract& contract,
                   const Order& order, const OrderState& state) override {
        if (g_openOrderCb) {
            g_openOrderCb((int)orderId, contract.symbol.c_str(),
                         order.action.c_str(), (int)toDouble(order.totalQuantity),
                         order.orderType.c_str(), order.lmtPrice,
                         order.auxPrice, state.status.c_str());
        }
    }

    void openOrderEnd() override {
        if (g_openOrderEndCb) g_openOrderEndCb();
    }

    // L2 DOM
    void updateMktDepth(TickerId id, int position, int operation,
                        int side, double price, Decimal size) override {
        if (g_domCb) g_domCb((int)id, position, operation, side, price, (int)toDouble(size));
    }

    // Scanner
    void scannerData(int reqId, int rank,
                     const ContractDetails& contractDetails,
                     const std::string& distance, const std::string& benchmark,
                     const std::string& projection, const std::string& legsStr) override {
        if (g_scannerCb) {
            g_scannerCb(reqId, rank, contractDetails.contract.symbol.c_str());
        }
    }

    void scannerDataEnd(int reqId) override {
        if (g_scannerEndCb) g_scannerEndCb(reqId);
    }

    // Historical data
    void historicalData(TickerId reqId, const Bar& bar) override {
        if (g_historicalCb) {
            g_historicalCb((int)reqId, bar.time.c_str(),
                          bar.open, bar.high, bar.low, bar.close,
                          (long long)toDouble(bar.volume));
        }
    }

    void historicalDataEnd(int reqId, const std::string& startDateStr,
                           const std::string& endDateStr) override {
        if (g_historicalEndCb) g_historicalEndCb(reqId);
    }

    // Error handling (v10.37 signature includes errorTime)
    void error(int id, time_t errorTime, int errorCode,
               const std::string& errorString,
               const std::string& advancedOrderRejectJson) override {
        if (g_errorCb) g_errorCb(id, errorCode, errorString.c_str());
    }

    // News headlines
    void tickNews(int tickerId, time_t timeStamp, const std::string& providerCode,
                  const std::string& articleId, const std::string& headline,
                  const std::string& extraData) override {
        if (g_headlineCb) {
            g_headlineCb(articleId.c_str(), headline.c_str(), (long long)timeStamp);
        }
    }

    // Next valid order ID — sent automatically on connect and on reqIds()
    void nextValidId(OrderId orderId) override {
        if (g_nextValidIdCb) g_nextValidIdCb((int)orderId);
    }
};

// ── Global state ──
static EReaderOSSignal   g_signal(2000); // 2s timeout
static BridgeWrapper     g_wrapper;
static EClientSocket     g_client(&g_wrapper, &g_signal);
static std::unique_ptr<EReader> g_reader;
static std::mutex        g_mutex;

// ── C API implementation ──

extern "C" {

int ibkr_connect(const char* host, int port, int clientId) {
    std::lock_guard<std::mutex> lock(g_mutex);
    bool ok = g_client.eConnect(host, (unsigned int)port, clientId, false);
    if (ok) {
        g_reader = std::make_unique<EReader>(&g_client, &g_signal);
        g_reader->start();
    }
    return ok ? 1 : 0;
}

void ibkr_disconnect() {
    std::lock_guard<std::mutex> lock(g_mutex);
    g_client.eDisconnect();
    g_reader.reset();
}

int ibkr_is_connected() {
    return g_client.isConnected() ? 1 : 0;
}

void ibkr_process_messages(int timeoutMs) {
    g_signal.waitForSignal();
    std::lock_guard<std::mutex> lock(g_mutex);
    if (g_reader) {
        g_reader->processMsgs();
    }
}

void ibkr_req_mkt_data(int reqId, const char* symbol, const char* genericTicks) {
    std::lock_guard<std::mutex> lock(g_mutex);
    Contract c;
    c.symbol = symbol;
    c.secType = "STK";
    c.exchange = "SMART";
    c.currency = "USD";
    g_client.reqMktData(reqId, c, genericTicks, false, false, TagValueListSPtr());
}

void ibkr_cancel_mkt_data(int reqId) {
    std::lock_guard<std::mutex> lock(g_mutex);
    g_client.cancelMktData(reqId);
}

void ibkr_req_mkt_depth(int reqId, const char* symbol, int numRows) {
    std::lock_guard<std::mutex> lock(g_mutex);
    Contract c;
    c.symbol = symbol;
    c.secType = "STK";
    c.exchange = "SMART";
    c.currency = "USD";
    g_client.reqMktDepth(reqId, c, numRows, false, TagValueListSPtr());
}

void ibkr_cancel_mkt_depth(int reqId) {
    std::lock_guard<std::mutex> lock(g_mutex);
    g_client.cancelMktDepth(reqId, false);
}

void ibkr_place_order(int orderId, const char* symbol, const char* action,
                      int qty, const char* orderType, double price, double auxPrice) {
    std::lock_guard<std::mutex> lock(g_mutex);
    Contract c;
    c.symbol = symbol;
    c.secType = "STK";
    c.exchange = "SMART";
    c.currency = "USD";

    Order o;
    o.action = action;
    o.totalQuantity = fromDouble((double)qty);
    o.orderType = orderType;
    o.lmtPrice = price;
    o.auxPrice = auxPrice;

    g_client.placeOrder(orderId, c, o);
}

void ibkr_cancel_order(int orderId) {
    std::lock_guard<std::mutex> lock(g_mutex);
    OrderCancel oc;
    g_client.cancelOrder(orderId, oc);
}

void ibkr_req_ids() {
    std::lock_guard<std::mutex> lock(g_mutex);
    g_client.reqIds(-1);
}

void ibkr_req_positions() {
    std::lock_guard<std::mutex> lock(g_mutex);
    g_client.reqPositions();
}

void ibkr_cancel_positions() {
    std::lock_guard<std::mutex> lock(g_mutex);
    g_client.cancelPositions();
}

void ibkr_req_open_orders() {
    std::lock_guard<std::mutex> lock(g_mutex);
    g_client.reqOpenOrders();
}

void ibkr_req_scanner(int reqId, const char* scanCode, const char* instrument,
                      const char* location) {
    std::lock_guard<std::mutex> lock(g_mutex);
    ScannerSubscription ss;
    ss.scanCode = scanCode;
    ss.instrument = instrument;
    ss.locationCode = location;
    g_client.reqScannerSubscription(reqId, ss, TagValueListSPtr(), TagValueListSPtr());
}

void ibkr_req_historical_data(int reqId, const char* symbol,
                              const char* duration, const char* barSize) {
    std::lock_guard<std::mutex> lock(g_mutex);
    Contract c;
    c.symbol = symbol;
    c.secType = "STK";
    c.exchange = "SMART";
    c.currency = "USD";
    g_client.reqHistoricalData(reqId, c, "", duration, barSize,
                               "TRADES", 1, 1, false, TagValueListSPtr());
}

// ── Callback registration ──

void ibkr_register_tick_callback(tick_callback_t cb) { g_tickCb = cb; }
void ibkr_register_order_status_callback(order_status_callback_t cb) { g_orderStatusCb = cb; }
void ibkr_register_execution_callback(execution_callback_t cb) { g_executionCb = cb; }
void ibkr_register_position_callback(position_callback_t cb, position_end_callback_t endCb) {
    g_positionCb = cb; g_positionEndCb = endCb;
}
void ibkr_register_open_order_callback(open_order_callback_t cb, open_order_end_callback_t endCb) {
    g_openOrderCb = cb; g_openOrderEndCb = endCb;
}
void ibkr_register_dom_callback(dom_callback_t cb) { g_domCb = cb; }
void ibkr_register_scanner_callback(scanner_callback_t cb, scanner_end_callback_t endCb) {
    g_scannerCb = cb; g_scannerEndCb = endCb;
}
void ibkr_register_historical_callback(historical_callback_t cb, historical_end_callback_t endCb) {
    g_historicalCb = cb; g_historicalEndCb = endCb;
}
void ibkr_register_error_callback(error_callback_t cb) { g_errorCb = cb; }
void ibkr_register_headline_callback(headline_callback_t cb) { g_headlineCb = cb; }
void ibkr_register_next_valid_id_callback(next_valid_id_callback_t cb) { g_nextValidIdCb = cb; }

} // extern "C"
