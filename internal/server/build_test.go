package server

import (
	"context"
	"slices"
	"testing"
)

const buildSentinel = "0f3a9c1d7be24a58a6c0d1e29b4f7a35c8e61d02"

func TestTheVerboseBodyCarriesTheBuildTheTurnWasStampedWith(t *testing.T) {
	t.Parallel()

	turn := replyTurn(&scriptedModel{step: answeringWith("an answer")})
	turn.Build = buildSentinel

	members, _ := topLevelKeys(t, postTo(t, context.Background(), turn, "/runs?verbose=true"))

	if got, want := stringMember(t, members, "build"), buildSentinel; got != want {
		t.Fatalf("verbose build = %q, want %q", got, want)
	}
}

func TestTheDefaultBodyOfAStampedTurnStillCarriesOnlyReplyWrittenAndProduced(t *testing.T) {
	t.Parallel()

	turn := replyTurn(&scriptedModel{step: answeringWith("an answer")})
	turn.Build = buildSentinel

	_, keys := topLevelKeys(t, postTo(t, context.Background(), turn, "/runs"))

	if want := []string{"produced", "reply", "written"}; !slices.Equal(keys, want) {
		t.Fatalf("default body keys = %q, want %q — the build must not reach the default body", keys, want)
	}
}
