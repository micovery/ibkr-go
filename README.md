# ibkr-go

A Go wrapper around the **official Interactive Brokers C++ TWS API** via CGO. Provides a clean, channel-based Go interface for market data, order management, scanning, historical data, and account reconciliation.

> **This is not a protocol reimplementation.** It links directly against IBKR's official `EClientSocket` and `EWrapper` classes via a minimal C bridge layer.

## Architecture

```
Go (ibkr.Client)  →  CGO  →  bridge.h/cpp (C shim)  →  EClientSocket (official C++)
                                      ↑
EWrapper callbacks  →  C function ptrs  →  //export Go funcs  →  Go channels
```

## Install

One command. No cloning required.

```bash
curl -sSL https://raw.githubusercontent.com/micovery/ibkr-go/main/install.sh | bash
```

This downloads the IBKR C++ API, builds the bridge library, and installs it system-wide.

Then use it in any Go project:

```bash
go get github.com/micovery/ibkr-go@latest
CGO_ENABLED=1 go build ./...
```

To uninstall:

```bash
curl -sSL https://raw.githubusercontent.com/micovery/ibkr-go/main/install.sh | bash -s -- --uninstall
```

### System Requirements

- **OS:** Linux (Ubuntu/Debian)
- **Packages:** `g++`, `libprotobuf-dev`, `libintelrdfpmath-dev`, `protobuf-compiler` (auto-installed by the script)

## Usage

```go
package main

import (
    "fmt"
    "time"

    ibkr "github.com/micovery/ibkr-go"
)

func main() {
    client := ibkr.NewClient()

    if err := client.Connect("127.0.0.1", 4001, 1); err != nil {
        panic(err)
    }
    defer client.Disconnect()
    time.Sleep(500 * time.Millisecond)

    // List positions
    positions, _ := client.ReqPositions()
    for _, p := range positions {
        fmt.Printf("%s: qty=%d avgCost=%.2f\n", p.Ticker, p.Qty, p.AvgCost)
    }

    // Stream market data
    tickCh, _ := client.ReqMktData(1001, "AAPL", "")
    for i := 0; i < 10; i++ {
        tick := <-tickCh
        fmt.Printf("type=%d price=%.2f size=%d\n", tick.TickType, tick.Price, tick.Size)
    }
    client.CancelMktData(1001)
}
```

## Interface

The `IBKRClient` interface covers:

| Category | Methods |
|---|---|
| **Connection** | `Connect`, `Disconnect`, `IsConnected` |
| **Market Data** | `ReqMktData`, `CancelMktData` |
| **L2 Depth** | `ReqMktDepth`, `CancelMktDepth` |
| **Orders** | `PlaceOrder`, `ModifyOrder`, `CancelOrder`, `OrderStatusChan`, `ExecutionChan` |
| **Scanner** | `ReqScannerData` |
| **Historical** | `ReqHistoricalData` |
| **News** | `HeadlineChan` |
| **Reconciliation** | `ReqPositions`, `ReqOpenOrders` |

## Testing

```bash
# Integration tests (requires IBKR Desktop/Gateway on localhost:4001)
CGO_ENABLED=1 go test -v -tags integration -timeout 30s
```

## License

MIT — see [LICENSE](LICENSE).

The IBKR C++ API itself is under the [IB API Non-Commercial License](https://interactivebrokers.github.io/) and is **not** included in this repository.
