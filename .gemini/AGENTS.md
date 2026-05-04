# Agent Instructions

Reference for any AI agent working on this codebase.

## Project Overview

`ibkr-go` is a Go CGO wrapper around the official Interactive Brokers C++ TWS API. It provides a channel-based Go interface for market data, order management, scanning, historical data, and account reconciliation.

### Key Files

| File | Purpose |
|---|---|
| `ibkr.go` | `IBKRClient` interface, types, constants, `go:generate` directive |
| `client.go` | `Client` struct (CGO implementation, build tag: `cgo`) |
| `callbacks.go` | CGO callback exports (build tag: `cgo`) |
| `cgo/bridge.h` | C bridge header |
| `cgo/bridge.cpp` | C bridge implementation (subclasses `DefaultEWrapper`) |
| `cgo/Makefile` | Builds `libibkr_bridge.a` from IBKR C++ source |
| `install.sh` | Curl-pipe-bash installer for consumers |
| `install.sh` | One-line installer (local or curl\|bash) |

### Build Modes

- `CGO_ENABLED=1 go build` — full build with IBKR C++ bridge
- `CGO_ENABLED=0 go build` — types and interface only (no `Client` struct)

### Testing

```bash
# Integration tests (requires IBKR Desktop/Gateway on localhost:4001)
CGO_ENABLED=1 go test -v -tags integration -timeout 30s
```

## Conventional Commits

All commits must use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): short description
```

### Types

| Type | When |
|---|---|
| `feat` | New feature or capability |
| `fix` | Bug fix |
| `refactor` | Code restructuring (no behavior change) |
| `docs` | Documentation only |
| `test` | Adding or modifying tests |
| `chore` | Maintenance (deps, config, cleanup) |
| `perf` | Performance improvement |

### Scopes

| Scope | Covers |
|---|---|
| `client` | `client.go`, `callbacks.go` — the CGO client |
| `bridge` | `cgo/bridge.h`, `cgo/bridge.cpp` — the C++ shim |
| `types` | `ibkr.go` — interface and type definitions |
| `install` | `install.sh` — build and install script |
| `ci` | `.github/workflows/` — CI configuration |

### Examples

```bash
git commit -m "feat(client): add ReqContractDetails method"
git commit -m "fix(bridge): handle null symbol in position callback"
git commit -m "docs: update install instructions for macOS"
git commit -m "chore(install): bump IBKR API version to 1046.01"
git commit -m "test: add historical data integration test"
git commit -m "refactor(types): split TickType into price and size categories"
```

## Commit Hygiene

**Each commit on `main` should be a complete, logical unit of work.** Not "WIP", not "fix typo from last commit".

- **While working**, small intermediate commits are fine
- **Before pushing**, squash related commits:

```bash
git log --oneline -10
git rebase -i origin/main
```

### Linear History

- **Never `git merge`** — always rebase
- **Always `git pull --rebase`**
- On feature branches, `git push --force-with-lease` is fine after rewriting history

## Coding Standards

### Go

- **CGO files** use build tag `//go:build cgo`
- **Types and interface** in `ibkr.go` have no build tag (available without CGO)
- **Channel buffers**: 1000 for ticks, 100 for orders/executions/positions
- **Timeouts**: 5s for synchronous requests (`ReqPositions`, `ReqOpenOrders`)
- **Global singleton**: `globalClient` is required because CGO exports can't carry Go closure state
- **Memory**: Always `C.free()` strings passed to C via `C.CString()`

### C++ Bridge

- Subclass `DefaultEWrapper` — only override the callbacks we need
- Keep the bridge minimal — don't expose more IBKR API than the Go interface requires
- Use C function pointers (not C++ virtual dispatch) for Go callbacks

## Push Safety

- **Never push without presenting a plan first**
- Before pushing, summarize what commits will be pushed and whether tests passed
- Wait for user approval before `git push`
