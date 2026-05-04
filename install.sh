#!/bin/bash
# install.sh — One-line installer for ibkr-go
#
# Install:  curl -sSL https://raw.githubusercontent.com/micovery/ibkr-go/main/install.sh | bash
# Uninstall: curl -sSL https://raw.githubusercontent.com/micovery/ibkr-go/main/install.sh | bash -s -- --uninstall

set -euo pipefail

IBKR_VERSION="1037.02"
IBKR_URL="https://interactivebrokers.github.io/downloads/twsapi_macunix.${IBKR_VERSION}.zip"
BRIDGE_URL="https://raw.githubusercontent.com/micovery/ibkr-go/main"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_LIB="/usr/local/lib"
INSTALL_INC="/usr/local/include/ibkr-go"
INSTALL_PC="/usr/local/lib/pkgconfig"
WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

if [ "${1:-}" = "--uninstall" ]; then
    echo "[1/1] Removing ibkr-go bridge from /usr/local..."
    sudo rm -f "$INSTALL_LIB/libibkr_bridge.a"
    sudo rm -rf "$INSTALL_INC"
    sudo rm -f "$INSTALL_PC/ibkr_bridge.pc"
    sudo ldconfig 2>/dev/null || true
    echo "Done."
    exit 0
fi

echo "Installing ibkr-go bridge (IBKR TWS API v${IBKR_VERSION})"
echo ""

# 1. System dependencies
echo "[1/6] Checking system dependencies..."
PKGS="g++ make pkg-config curl unzip protobuf-compiler libprotobuf-dev libintelrdfpmath-dev"
MISSING=""
for pkg in $PKGS; do
    dpkg -s "$pkg" &>/dev/null 2>&1 || MISSING="$MISSING $pkg"
done
if [ -n "$MISSING" ]; then
    echo "       Installing:$MISSING"
    sudo apt-get update -qq
    sudo apt-get install -y -qq $MISSING
fi
echo "       Done."

# 2. Download IBKR C++ API
echo "[2/6] Downloading IBKR TWS API..."
curl -sL "$IBKR_URL" -o "$WORKDIR/twsapi.zip"
echo "       Done."

# 3. Extract
echo "[3/6] Extracting C++ source..."
unzip -qo "$WORKDIR/twsapi.zip" -d "$WORKDIR/ibkr/"
CPP_DIR="$WORKDIR/ibkr/IBJts/source/cppclient/client"
[ -f "$CPP_DIR/EClientSocket.h" ] || { echo "ERROR: C++ source not found"; exit 1; }
echo "       Done."

# 4. Regenerate protobuf headers
echo "[4/6] Regenerating protobuf headers..."
PROTO_DIR="$WORKDIR/ibkr/IBJts/source/proto"
PB_OUT="$CPP_DIR/protobufUnix"
protoc --proto_path="$PROTO_DIR" --cpp_out="$PB_OUT" "$PROTO_DIR"/*.proto
echo "       Done."

# 5. Build
echo "[5/6] Building libibkr_bridge.a..."
if [ -f "$SCRIPT_DIR/cgo/bridge.h" ]; then
    # Running from repo checkout (e.g. CI) — use local files
    cp "$SCRIPT_DIR/cgo/bridge.h"   "$WORKDIR/bridge.h"
    cp "$SCRIPT_DIR/cgo/bridge.cpp" "$WORKDIR/bridge.cpp"
    cp "$SCRIPT_DIR/cgo/Makefile"   "$WORKDIR/Makefile"
else
    # Running via curl|bash — download from GitHub
    curl -sL "$BRIDGE_URL/cgo/bridge.h"   -o "$WORKDIR/bridge.h"
    curl -sL "$BRIDGE_URL/cgo/bridge.cpp" -o "$WORKDIR/bridge.cpp"
    curl -sL "$BRIDGE_URL/cgo/Makefile"   -o "$WORKDIR/Makefile"
fi
sed -i "s|^IBKR_SRC := .*|IBKR_SRC := $CPP_DIR|" "$WORKDIR/Makefile"
sed -i "s|^IBKR_PB  := .*|IBKR_PB  := $PB_OUT|"  "$WORKDIR/Makefile"
sed -i "s|^BRIDGE_DIR := .*|BRIDGE_DIR := $WORKDIR|" "$WORKDIR/Makefile"
(cd "$WORKDIR" && make -f Makefile -j"$(nproc)" >/dev/null 2>&1)
[ -f "$WORKDIR/libibkr_bridge.a" ] || { echo "ERROR: Build failed"; exit 1; }
echo "       Done."

# 6. Install
echo "[6/6] Installing to /usr/local..."
sudo mkdir -p "$INSTALL_INC" "$INSTALL_PC"
sudo cp "$WORKDIR/libibkr_bridge.a" "$INSTALL_LIB/"
sudo cp "$WORKDIR/bridge.h" "$INSTALL_INC/"
sudo tee "$INSTALL_PC/ibkr_bridge.pc" > /dev/null <<EOF
prefix=/usr/local
libdir=\${prefix}/lib
includedir=\${prefix}/include/ibkr-go

Name: ibkr_bridge
Description: IBKR C++ TWS API bridge for ibkr-go
Version: ${IBKR_VERSION}
Libs: -L\${libdir} -libkr_bridge -lstdc++ -lprotobuf -lbidgcc000 -lpthread
Cflags: -I\${includedir}
EOF
sudo ldconfig 2>/dev/null || true
echo "       Done."

echo ""
echo "Installed. Usage:"
echo "  go get github.com/micovery/ibkr-go@latest"
echo "  CGO_ENABLED=1 go build ./..."
