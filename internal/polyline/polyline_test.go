package polyline

import (
	"reflect"
	"testing"
)

func TestDecode6(t *testing.T) {
	cases := []struct {
		name    string
		encoded string
		want    [][2]float64
	}{
		{
			name:    "empty",
			encoded: "",
			want:    nil,
		},
		{
			// Each point's delta is (+1, +1) micro-degrees: lat/lon deltas
			// of 1 zigzag-encode to value 2, which varint-encodes to the
			// single byte 'A' (2+63).
			name:    "positive deltas",
			encoded: "AAAA",
			want:    [][2]float64{{0.000001, 0.000001}, {0.000002, 0.000002}},
		},
		{
			// Second point backtracks in lat only: delta (-1, 0). -1
			// zigzags to 1 -> '@' (1+63); 0 zigzags to 0 -> '?' (0+63).
			name:    "negative delta",
			encoded: "AA@?",
			want:    [][2]float64{{0.000001, 0.000001}, {0, 0.000001}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Decode6(tc.encoded)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Decode6(%q) = %v, want %v", tc.encoded, got, tc.want)
			}
		})
	}
}
