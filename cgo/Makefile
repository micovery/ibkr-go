# Makefile — Builds the IBKR C++ bridge as a static library (libibkr_bridge.a)
#
# Usage:
#   make          — builds libibkr_bridge.a
#   make clean    — removes build artifacts
#
# Prerequisites:
#   apt install libprotobuf-dev libintelrdfpmath-dev
#
# The IBKR C++ API source must be at ../third_party/ibkr/IBJts/
# Download from: https://interactivebrokers.github.io/

IBKR_SRC := ../third_party/ibkr/IBJts/source/cppclient/client
IBKR_PB  := $(IBKR_SRC)/protobufUnix
BRIDGE_DIR := .

CXX := g++
CXXFLAGS := -std=c++17 -O2 -fPIC -DUSE_PTHREADS \
            -I$(IBKR_SRC) -I$(IBKR_PB) -I$(BRIDGE_DIR) \
            $(shell pkg-config --cflags protobuf) \
            -Wno-switch -Wno-unused-parameter -Wno-deprecated-declarations

# IBKR C++ source files
IBKR_SRCS := \
    $(IBKR_SRC)/EClient.cpp \
    $(IBKR_SRC)/EClientSocket.cpp \
    $(IBKR_SRC)/EDecoder.cpp \
    $(IBKR_SRC)/EDecoderUtils.cpp \
    $(IBKR_SRC)/EMessage.cpp \
    $(IBKR_SRC)/EMutex.cpp \
    $(IBKR_SRC)/EOrderDecoder.cpp \
    $(IBKR_SRC)/EReader.cpp \
    $(IBKR_SRC)/EReaderOSSignal.cpp \
    $(IBKR_SRC)/ESocket.cpp \
    $(IBKR_SRC)/EClientUtils.cpp \
    $(IBKR_SRC)/DefaultEWrapper.cpp \
    $(IBKR_SRC)/Decimal.cpp \
    $(IBKR_SRC)/ContractCondition.cpp \
    $(IBKR_SRC)/OrderCondition.cpp \
    $(IBKR_SRC)/OperatorCondition.cpp \
    $(IBKR_SRC)/MarginCondition.cpp \
    $(IBKR_SRC)/PercentChangeCondition.cpp \
    $(IBKR_SRC)/PriceCondition.cpp \
    $(IBKR_SRC)/TimeCondition.cpp \
    $(IBKR_SRC)/VolumeCondition.cpp \
    $(IBKR_SRC)/ExecutionCondition.cpp \
    $(IBKR_SRC)/SoftDollarTier.cpp \
    $(IBKR_SRC)/StdAfx.cpp \
    $(IBKR_SRC)/Utils.cpp

# Pre-generated protobuf sources
PB_SRCS := $(wildcard $(IBKR_PB)/*.pb.cc)

# Our bridge
BRIDGE_SRCS := $(BRIDGE_DIR)/bridge.cpp

ALL_SRCS := $(IBKR_SRCS) $(BRIDGE_SRCS)
ALL_OBJS := $(ALL_SRCS:.cpp=.o)
PB_OBJS  := $(PB_SRCS:.cc=.o)

TARGET := libibkr_bridge.a

.PHONY: all clean

all: $(TARGET)

$(TARGET): $(ALL_OBJS) $(PB_OBJS)
	ar rcs $@ $^
	@echo "Built $(TARGET)"

%.o: %.cpp
	$(CXX) $(CXXFLAGS) -c $< -o $@

%.o: %.cc
	$(CXX) $(CXXFLAGS) -c $< -o $@

clean:
	rm -f $(ALL_OBJS) $(PB_OBJS) $(TARGET)
	@echo "Cleaned"
