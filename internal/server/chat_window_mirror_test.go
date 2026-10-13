package server

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

var chatWindowDeclaration = regexp.MustCompile(`(?m)^HISTORY_WINDOW\s*=\s*([0-9]+)\s*$`)

func TestTheChatClientsWindowIsAtLeastTwiceTheExchangesATurnUses(t *testing.T) {
	source, err := os.ReadFile(scriptsDir + "/chat.py")
	if err != nil {
		t.Fatalf("reading chat.py: %v", err)
	}
	match := chatWindowDeclaration.FindSubmatch(source)
	if match == nil {
		t.Fatalf("chat.py declares no HISTORY_WINDOW as a bare decimal integer")
	}
	window, err := strconv.Atoi(string(match[1]))
	if err != nil {
		t.Fatalf("HISTORY_WINDOW %q: %v", match[1], err)
	}
	if window < 2*loop.HistoryExchangeLimit {
		t.Errorf("chat.py holds a window of %d exchanges, under twice the %d a turn uses (design §6.2)", window, loop.HistoryExchangeLimit)
	}
}
