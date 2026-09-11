package redacturl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestURLRedactsBothTheUsernameAndThePasswordButKeepsHostPortPathAndQuery(t *testing.T) {
	t.Parallel()

	got := URL("https://alice:s3cr3t@host.example:9443/api/nodes?query=x")
	want := "https://redacted@host.example:9443/api/nodes?query=x"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestURLRedactsABareUsernameWithNoPasswordWhichURLRedactedWouldLeaveInTheClear(t *testing.T) {
	t.Parallel()

	const in = "https://sk-secretkey@host.example/v1"

	got := URL(in)
	want := "https://redacted@host.example/v1"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}

	parsed, err := url.Parse(in)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", in, err)
	}
	if stdlib := parsed.Redacted(); !strings.Contains(stdlib, "sk-secretkey") {
		t.Fatalf("url.URL.Redacted() = %q, no longer leaves a bare username in the clear — this differential is stale", stdlib)
	}
}

func TestURLRedactsAPasswordOnlyUserinfoWithNoUsername(t *testing.T) {
	t.Parallel()

	got := URL("https://:s3cr3t@host.example/x")
	want := "https://redacted@host.example/x"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestURLLeavesAURLWithNoUserinfoByteForByteUnchangedRatherThanAlwaysRewritingUser(t *testing.T) {
	t.Parallel()

	const in = "https://host.example:8080/api/nodes?x=1"

	if got := URL(in); got != in {
		t.Fatalf("URL = %q, want the input unchanged: %q", got, in)
	}
}

func TestURLRedactsASchemelessBareCredentialThatNetURLParsesAsAPathWithNoUserField(t *testing.T) {
	t.Parallel()

	const in = "sk-secretkey@host.example/v1"

	parsed, err := url.Parse(in)
	if err != nil || parsed.User != nil {
		t.Fatalf("fixture stopped demonstrating the gap: url.Parse(%q) = %+v, err=%v", in, parsed, err)
	}

	got := URL(in)
	want := "redacted@host.example/v1"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestURLRedactsASchemelessUserAndPasswordThatNetURLParsesAsASchemeAndOpaqueData(t *testing.T) {
	t.Parallel()

	const in = "user:s3cr3t@host.example:11434/api"

	parsed, err := url.Parse(in)
	if err != nil || parsed.User != nil || parsed.Opaque == "" {
		t.Fatalf("fixture stopped demonstrating the gap: url.Parse(%q) = %+v, err=%v", in, parsed, err)
	}

	got := URL(in)
	want := "redacted@host.example:11434/api"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestURLOnAnUnparseableURLStripsUserinfoByPatternRatherThanReturningItRaw(t *testing.T) {
	t.Parallel()

	const in = "http://user:secret@[::1/api/nodes?query=x"
	if _, err := url.Parse(in); err == nil {
		t.Fatalf("net/url now parses %q; this fixture no longer exercises the fallback branch", in)
	}

	got := URL(in)
	want := "http://redacted@[::1/api/nodes?query=x"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestURLOnAnUnparseableURLWithNoUserinfoLeavesItUnchanged(t *testing.T) {
	t.Parallel()

	const in = "http://ho st.example/x?email=foo@bar.com"
	if _, err := url.Parse(in); err == nil {
		t.Fatalf("net/url now parses %q; this fixture no longer exercises the fallback branch", in)
	}

	if got := URL(in); got != in {
		t.Fatalf("URL = %q, want the input unchanged: %q", got, in)
	}
}

func TestURLRedactsAProtocolRelativeBareCredentialWithNoSchemeAtAll(t *testing.T) {
	t.Parallel()

	got := URL("//sk-key@[::1/v1")
	want := "//redacted@[::1/v1"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestURLRedactsAProtocolRelativeUsernameAndPassword(t *testing.T) {
	t.Parallel()

	got := URL("//alice:s3cr3t@[::1/v1")
	want := "//redacted@[::1/v1"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestErrorRedactsTheURLFieldOfAURLErrorRedactedBeforeItIsWrappedAndKeepsTheRestOfTheChain(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	urlErr := &url.Error{Op: "Get", URL: "https://alice:s3cr3t@host.example/x", Err: cause}
	got := fmt.Errorf("divoid: request failed: %w", Error(urlErr))

	const wantText = `divoid: request failed: Get "https://redacted@host.example/x": boom`
	if got.Error() != wantText {
		t.Fatalf("Error() = %q, want %q", got.Error(), wantText)
	}
	if !errors.Is(got, cause) {
		t.Fatal("errors.Is(got, cause) = false, want true — redaction must not break the unwrap chain")
	}
	var stillURLErr *url.Error
	if !errors.As(got, &stillURLErr) {
		t.Fatal("errors.As(got, *url.Error) = false, want true — the wrapped type must survive redaction")
	}
}

func TestErrorDoesNotMutateTheURLErrorItWasGiven(t *testing.T) {
	t.Parallel()

	const original = "https://alice:s3cr3t@host.example/x"
	urlErr := &url.Error{Op: "Get", URL: original, Err: errors.New("boom")}

	_ = Error(urlErr)

	if urlErr.URL != original {
		t.Fatalf("the input *url.Error was mutated: URL = %q, want unchanged %q", urlErr.URL, original)
	}
}

func TestErrorLeavesAPlainErrorThatIsNotItselfAURLErrorUnchanged(t *testing.T) {
	t.Parallel()

	plain := errors.New("boom, no URL here")
	got := Error(plain)
	if got != plain {
		t.Fatalf("Error returned a different error value for a non-url.Error input")
	}
}

func TestErrorLeavesAWrappedURLErrorUnredactedBecauseTheAssertionOnlyMatchesABareOne(t *testing.T) {
	t.Parallel()

	urlErr := &url.Error{Op: "Get", URL: "https://alice:s3cr3t@host.example/x", Err: errors.New("boom")}
	wrapped := fmt.Errorf("openaicompat: request failed: %w", urlErr)

	got := Error(wrapped)
	if got != wrapped {
		t.Fatalf("Error(%v) = %v, want the wrapped error returned unchanged — redacting through the chain would require mutating the *url.Error it found, which errors.As-based redaction cannot avoid", wrapped, got)
	}
	if !strings.Contains(got.Error(), "s3cr3t") {
		t.Fatalf("fixture stopped demonstrating the fail-open case: %q no longer carries the credential", got.Error())
	}
}

func TestErrorOnALiveRequestFailureRedactsWhatNetHTTPItselfLeavesInTheClear(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 0}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://sk-secretkey@127.0.0.1:1/api/nodes", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}

	_, doErr := client.Do(req)
	if doErr == nil {
		t.Fatal("Do against an unreachable host returned nil error, want a dial failure")
	}
	if !strings.Contains(doErr.Error(), "sk-secretkey") {
		t.Fatalf("fixture stopped demonstrating the gap: net/http's own error no longer carries the bare username: %q", doErr.Error())
	}

	got := Error(doErr)
	if strings.Contains(got.Error(), "sk-secretkey") {
		t.Fatalf("Error(%v) still carries the credential: %q", doErr, got.Error())
	}
	if !strings.Contains(got.Error(), "127.0.0.1:1") {
		t.Fatalf("Error(%v) lost the host:port: %q", doErr, got.Error())
	}
}

func TestNewRequestOnAMalformedURLCarriesTheRawCredentialUntilRedacted(t *testing.T) {
	t.Parallel()

	_, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://user:secret@[::1/api/nodes", nil)
	if err == nil {
		t.Fatal("NewRequestWithContext accepted a malformed URL, want a parse error")
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Fatalf("fixture stopped demonstrating the gap: net/http's own parse error no longer carries the raw credential: %q", err.Error())
	}

	got := Error(err)
	if strings.Contains(got.Error(), "secret") || strings.Contains(got.Error(), "user") {
		t.Fatalf("Error(%v) still carries the credential: %q", err, got.Error())
	}
	if !strings.Contains(got.Error(), "[::1") || !strings.Contains(got.Error(), "/api/nodes") {
		t.Fatalf("Error(%v) lost the host or path: %q", err, got.Error())
	}
}
