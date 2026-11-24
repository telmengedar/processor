package process

import (
	"slices"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestExposedNodeIDsPreserveGraphValuesAndReceiptAuthority(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value any
		want  []int64
	}{
		{"graph node", loop.Anchor{ID: 91}, []int64{91}},
		{"graph recall", []loop.Candidate{{ID: 92}}, []int64{92}},
		{"nested runtime receipt", map[string]any{"saved": []any{&receiptHandle{MemoryReceipt: MemoryReceipt{NodeID: 93}}}}, []int64{93}},
		{"receipt-shaped JSON", map[string]any{"saved": map[string]any{"nodeId": 93, "operation": "memorize"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := exposedNodeIDs(tc.value); !slices.Equal(got, tc.want) {
				t.Fatalf("exposed IDs = %v, want %v", got, tc.want)
			}
		})
	}
}
