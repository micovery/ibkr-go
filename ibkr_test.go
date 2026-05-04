package ibkr

import "testing"

// TestTickTypeConstants verifies the IBKR tick type IDs match the official spec.
func TestTickTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      TickType
		expected int
	}{
		{"TickBidSize", TickBidSize, 0},
		{"TickBidPrice", TickBidPrice, 1},
		{"TickAskPrice", TickAskPrice, 2},
		{"TickAskSize", TickAskSize, 3},
		{"TickLastPrice", TickLastPrice, 4},
		{"TickLastSize", TickLastSize, 5},
		{"TickVolume", TickVolume, 8},
	}
	for _, tt := range tests {
		if int(tt.got) != tt.expected {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.expected)
		}
	}
}

// TestTypeDefaults verifies zero-value structs are usable.
func TestTypeDefaults(t *testing.T) {
	var tick Tick
	if tick.ReqID != 0 || tick.Price != 0 || tick.Size != 0 {
		t.Error("zero-value Tick should have all zero fields")
	}

	var pos BrokerPosition
	if pos.Ticker != "" || pos.Qty != 0 || pos.AvgCost != 0 {
		t.Error("zero-value BrokerPosition should have all zero fields")
	}

	var order OpenOrder
	if order.OrderID != 0 || order.Status != "" {
		t.Error("zero-value OpenOrder should have all zero fields")
	}
}
