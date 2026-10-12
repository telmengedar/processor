package loop

import "strings"

const (
	// HistoryExchangeLimit is how many of the newest earlier exchanges a turn is shown.
	HistoryExchangeLimit = 4
	// HistorySafetyLimit is the byte size past which the oldest whole exchanges are dropped; a runtime cut, never the shape of normal history.
	HistorySafetyLimit = 8_000
)

const sectionConversation = "EARLIER IN THIS CONVERSATION"

// Exchange is one earlier turn of the conversation: what was said to the bot and, when it answered, what it said back.
type Exchange struct {
	Input string `json:"input"`
	Reply string `json:"reply,omitempty"`
}

func renderConversation(exchanges []Exchange) string {
	if len(exchanges) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("===== " + sectionConversation + " =====")
	for _, e := range exchanges {
		b.WriteString("\nuser: ")
		b.WriteString(e.Input)
		if e.Reply != "" {
			b.WriteString("\nyou: ")
			b.WriteString(e.Reply)
		}
	}
	return b.String()
}

func selectExchanges(supplied []Exchange) (used []Exchange, section string, safetyCut int) {
	used = supplied[max(0, len(supplied)-HistoryExchangeLimit):]
	counted := len(used)

	section = renderConversation(used)
	for len(used) > 0 && len(section) > HistorySafetyLimit {
		used = used[1:]
		section = renderConversation(used)
	}
	return used, section, counted - len(used)
}
