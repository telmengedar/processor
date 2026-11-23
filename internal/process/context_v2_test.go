package process

import (
	"strings"
	"testing"
)

func TestV2IncomingTransportAndRenderedContextShareBound(t *testing.T) {
	t.Parallel()
	step := Step{Prompt: strings.Repeat("{{context.message}}", 200), Context: []ContextItem{{Key: "message", Value: strings.Repeat("x", 100), Required: true}}}
	r := &transportRuntime{data: map[string]any{"intentional_background": strings.Repeat("b", 14000)}}
	messages, _, _, err := v2Messages(step, nil, nil, &Snapshot{}, r)
	typed, ok := err.(*Error)
	if !ok || typed.Code != "context_bound" || len(messages) != 0 {
		t.Fatalf("combined context exceeded32768 without rejection: error=%v messages=%d", err, len(messages))
	}
}
