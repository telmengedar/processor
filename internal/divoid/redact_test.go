package divoid

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

const (
	malformedBaseURL                = "http://user:secret@[::1"
	unreachableCredentialedBaseURL  = "http://sk-secretkey@127.0.0.1:1"
	unreachableCredentialedFragment = "sk-secretkey"
	malformedCredentialUser         = "user"
	malformedCredentialPass         = "secret"
)

func TestGetOnAMalformedBaseURLRedactsTheCredentialFromTheBuildRequestError(t *testing.T) {
	t.Parallel()

	c := NewClient(malformedBaseURL, "k", nil, testLogger())
	err := c.get(context.Background(), "/api/probe", url.Values{}, &struct{}{})

	assertNoCredential(t, err, malformedCredentialUser, malformedCredentialPass)
	assertContains(t, err, "[::1")
	assertContains(t, err, "/api/probe")
}

func TestGetOnAnUnreachableHostRedactsTheCredentialFromTheRequestFailedError(t *testing.T) {
	t.Parallel()

	c := NewClient(unreachableCredentialedBaseURL, "k", nil, testLogger())
	err := c.get(context.Background(), "/api/probe", url.Values{}, &struct{}{})

	assertNoCredential(t, err, unreachableCredentialedFragment)
	assertContains(t, err, "127.0.0.1:1")
}

func TestPostOnAMalformedBaseURLRedactsTheCredentialFromTheBuildRequestError(t *testing.T) {
	t.Parallel()

	c := NewClient(malformedBaseURL, "k", nil, testLogger())
	err := c.post(context.Background(), "/api/probe", "application/json", []byte("{}"), nil)

	assertNoCredential(t, err, malformedCredentialUser, malformedCredentialPass)
	assertContains(t, err, "[::1")
	assertContains(t, err, "/api/probe")
}

func TestRemoveOnAMalformedBaseURLRedactsTheCredentialFromTheBuildRequestError(t *testing.T) {
	t.Parallel()

	c := NewClient(malformedBaseURL, "k", nil, testLogger())
	err := c.remove(context.Background(), "/api/probe")

	assertNoCredential(t, err, malformedCredentialUser, malformedCredentialPass)
	assertContains(t, err, "[::1")
	assertContains(t, err, "/api/probe")
}

func TestSendOnAnUnreachableHostRedactsTheCredentialFromTheRequestFailedError(t *testing.T) {
	t.Parallel()

	c := NewClient(unreachableCredentialedBaseURL, "k", nil, testLogger())
	err := c.post(context.Background(), "/api/probe", "application/json", []byte("{}"), nil)

	assertNoCredential(t, err, unreachableCredentialedFragment)
	assertContains(t, err, "127.0.0.1:1")
}

func TestPatchOnAMalformedBaseURLRedactsTheCredentialFromTheBuildRequestError(t *testing.T) {
	t.Parallel()

	c := NewClient(malformedBaseURL, "k", nil, testLogger())
	err := c.patch(context.Background(), "/api/probe", "application/json-patch+json", []byte("[]"))

	assertNoCredential(t, err, malformedCredentialUser, malformedCredentialPass)
	assertContains(t, err, "[::1")
	assertContains(t, err, "/api/probe")
}

func assertNoCredential(t *testing.T, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("got nil error, want a request failure to inspect")
	}
	for _, f := range fragments {
		if strings.Contains(err.Error(), f) {
			t.Fatalf("error still carries the credential fragment %q: %q", f, err.Error())
		}
	}
}

func assertContains(t *testing.T, err error, fragment string) {
	t.Helper()
	if err == nil {
		t.Fatal("got nil error, want a request failure to inspect")
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("error lost %q, want it to survive redaction: %q", fragment, err.Error())
	}
}
