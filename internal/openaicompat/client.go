// Package openaicompat is the model adapter: a client for the
// OpenAI-compatible chat-completions protocol — the interface every local
// runtime serves, and the reference implementation besides (design
// #10521's ruling; docs/architecture/m1-skeleton-loop.md §5.3, §6.6).
//
// Named for the protocol, not a vendor (#6836 applied deliberately): it
// will mostly point at things that are not OpenAI. It implements
// loop.ModelPort, which is declared in internal/loop and constructed here
// only as a value passed into main (design §5.3, §8.3).
//
// What it may not do, because it is the ruling's whole content: it may
// not leak a provider's field names, finish-reason strings, or error
// shapes past its own boundary. The loop's TerminalReason values are the
// only vocabulary that crosses out of this package (design §9.2's
// neutrality falsifier — grep internal/loop for any provider vocabulary;
// it must return nothing).
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

// DefaultTimeout bounds the model call when the caller does not supply its
// own *http.Client. It is deliberately its own constant, and deliberately
// not divoid.DefaultTimeout (design §8.4a): a hosted endpoint answers in
// seconds, but a local model on CPU — the ruling's own target — can take
// minutes for a few hundred tokens. This exists to stop a hung socket, not
// to enforce a latency budget; there is no latency budget at M1, because
// the caller is a human with curl.
const DefaultTimeout = 5 * time.Minute

// recallToolName is the one tool M1 ships (design §2.4): supplementary
// recall, the same operation Assemble already performs, exposed to the
// model for what assembly missed.
const recallToolName = "recall"

// Compile-time assertion that Client satisfies the port the loop declares.
var _ loop.ModelPort = (*Client)(nil)

// Client is a client for the OpenAI-compatible chat-completions protocol.
// Standard library only (design §10.9).
type Client struct {
	baseURL    string
	modelID    string
	apiKey     string
	httpClient *http.Client
}

// NewClient builds a Client against baseURL, requesting modelID on every
// call. apiKey is sent as "Authorization: Bearer <apiKey>" only when
// non-empty (design §8.1) — an empty apiKey means no header is sent at
// all, never an empty one; cmd/processor's boot configuration is what
// turns "absent" and "present but empty" into that one non-ambiguous
// value before it ever reaches here. httpClient is used verbatim when
// non-nil; otherwise a new *http.Client with DefaultTimeout applied is
// built — never http.DefaultClient, which carries no timeout.
func NewClient(baseURL, modelID, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		modelID:    modelID,
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// Judge runs one judgement step (design §8.3's model port). It sends one
// fixed request shape (design §6.6) — it does not probe or adapt to the
// endpoint — and translates the response into the loop's own vocabulary
// before returning.
func (c *Client) Judge(ctx context.Context, in loop.JudgeInput) (loop.JudgeResult, error) {
	reqBody := chatRequest{
		Model:    c.modelID,
		Messages: buildMessages(in),
		// MaxTokens is loop.MaxOutputTokens, not a package-local constant
		// (design §8.4): the output-token cap is one of the run record's
		// Limits (design §8.2), so it lives where the record is built and
		// this adapter references it — never a second copy that could drift.
		MaxTokens: loop.MaxOutputTokens,
		Tools:     []wireTool{recallTool()},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: unexpected status %d: %s", resp.StatusCode, readUpstreamMessage(resp.Body))
	}

	var wire chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return loop.JudgeResult{}, fmt.Errorf("openaicompat: decode response: %w", err)
	}

	return translate(wire)
}

// readUpstreamMessage extracts a human-readable message from an error
// response for the wrapped error's text, per C41's shape (an object under
// "error" with a "message" field). It never fails the call: a body that
// isn't the expected error shape just yields empty, and the status code
// alone is still reported by the caller.
func readUpstreamMessage(r io.Reader) string {
	body, _ := io.ReadAll(io.LimitReader(r, 4096))
	var e wireError
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return string(body)
}
