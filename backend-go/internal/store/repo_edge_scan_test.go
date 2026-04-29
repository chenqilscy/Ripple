package store

import (
	"testing"
	"time"
)

// TestScanEdge_Strength 验证 scanEdge 正确读取 vals[9] 的 float64 strength 字段。
func TestScanEdge_Strength(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	cases := []struct {
		name     string
		vals     []any
		wantStr  float64
	}{
		{
			name: "strength=0.75",
			vals: []any{"id-1", "lake-1", "src-1", "dst-1", "relates", "", "owner-1", now, nil, float64(0.75)},
			wantStr: 0.75,
		},
		{
			name: "strength absent (len=9)",
			vals: []any{"id-1", "lake-1", "src-1", "dst-1", "relates", "", "owner-1", now, nil},
			wantStr: 0,
		},
		{
			name: "strength=nil (coalesce returned nil somehow)",
			vals: []any{"id-1", "lake-1", "src-1", "dst-1", "relates", "", "owner-1", now, nil, nil},
			wantStr: 0,
		},
		{
			name: "strength=0.0",
			vals: []any{"id-1", "lake-1", "src-1", "dst-1", "relates", "", "owner-1", now, nil, float64(0)},
			wantStr: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := scanEdge(tc.vals)
			if e.Strength != tc.wantStr {
				t.Fatalf("scanEdge Strength: want %v, got %v", tc.wantStr, e.Strength)
			}
		})
	}
}
