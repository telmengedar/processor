package ports

import (
	"fmt"
	"log/slog"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/openaicompat"
)

// Suppression is the control an adapter sends to keep the model's reasoning channel off, and where that control comes from.
type Suppression struct {
	Control  string
	Standing string
}

// ThinkingSuppression returns the control the protocol's adapter sends on every request to keep the model's reasoning channel off.
func ThinkingSuppression(protocol string) (Suppression, error) {
	switch protocol {
	case boot.ProtocolOpenAICompat:
		return Suppression{Control: openaicompat.ThinkingSuppression, Standing: openaicompat.ThinkingSuppressionStanding}, nil
	case boot.ProtocolOllama:
		return Suppression{Control: ollama.ThinkingSuppression, Standing: ollama.ThinkingSuppressionStanding}, nil
	}
	return Suppression{}, fmt.Errorf("model protocol %q has no adapter in this binary, so no thinking-suppression control is in force", protocol)
}

// StateThinkingSuppression writes the one boot line naming the control the protocol's adapter sends on the named call path to keep the reasoning channel off.
func StateThinkingSuppression(logger *slog.Logger, path, protocol string) error {
	suppression, err := ThinkingSuppression(protocol)
	if err != nil {
		return err
	}
	logger.Info("thinking suppressed on every model call",
		"path", path, "protocol", protocol, "control", suppression.Control, "standing", suppression.Standing)
	return nil
}
