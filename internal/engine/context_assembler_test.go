package engine

import "testing"

func TestDynamicSearchLimit(t *testing.T) {
	tests := []struct {
		nodeCount int
		want      int
	}{
		{0, 10},
		{500, 10},
		{999, 10},
		{1000, 15},
		{3000, 15},
		{4999, 15},
		{5000, 20},
		{10000, 20},
		{14999, 20},
		{15000, 25},
		{50000, 25},
	}

	for _, tt := range tests {
		got := dynamicSearchLimit(tt.nodeCount)
		if got != tt.want {
			t.Errorf("dynamicSearchLimit(%d) = %d, want %d", tt.nodeCount, got, tt.want)
		}
	}
}
