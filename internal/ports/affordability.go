package ports

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
	"github.com/telmengedar/processor/internal/openaicompat"
)

// ClientBound returns the bound the protocol's adapter puts on one model call of its own accord.
func ClientBound(protocol string) (time.Duration, error) {
	switch protocol {
	case boot.ProtocolOpenAICompat:
		return openaicompat.DefaultTimeout, nil
	case boot.ProtocolOllama:
		return ollama.DefaultTimeout, nil
	}
	return 0, fmt.Errorf("model protocol %q has no adapter in this binary, so no client bound is in force", protocol)
}

// Affordability resolves the deployment's declared floors and the call sites a run issues under protocol.
func Affordability(protocol string, declared boot.FloorsConfig, runBound time.Duration) (loop.Floors, []loop.CallSite, error) {
	clientBound, err := ClientBound(protocol)
	if err != nil {
		return loop.Floors{}, nil, err
	}

	floors := loop.Floors{}
	if declared.TokensPerSecond != nil {
		floors.TokensPerSecond = *declared.TokensPerSecond
	}
	if declared.PromptBytesPerSecond != nil {
		floors.PromptBytesPerSecond = *declared.PromptBytesPerSecond
	}

	return floors.Resolved(), loop.CallSites(clientBound, runBound), nil
}

// StateAffordability writes the boot lines that report, per call site, what its budget costs at the declared floors and how much of the bound that binds it is left over.
func StateAffordability(logger *slog.Logger, floors loop.Floors, sites []loop.CallSite) {
	resolved := floors.Resolved()
	logger.Info("model rate floors declared: these are the deployment's assertions about its endpoint, not measurements of it",
		"tokensPerSecond", resolved.TokensPerSecond, "promptBytesPerSecond", resolved.PromptBytesPerSecond, "safetyFactor", loop.AffordabilitySafety)

	for _, headroom := range loop.Afford(sites, resolved) {
		if headroom.Affordable() {
			logger.Info("call site affordable at the declared floors",
				"site", headroom.Name, "budget", headroom.Budget, "promptCeiling", headroom.PromptCeiling,
				"bound", headroom.Bound, "generation", round(headroom.Generation), "prompt", round(headroom.Prompt),
				"allowed", round(headroom.Allowed), "headroom", round(headroom.Spare))
			continue
		}

		attrs := []any{
			"site", headroom.Name, "budget", headroom.Budget, "promptCeiling", headroom.PromptCeiling,
			"bound", headroom.Bound, "generation", round(headroom.Generation), "prompt", round(headroom.Prompt),
			"allowed", round(headroom.Allowed), "shortfall", round(-headroom.Spare),
			"declaredTokensPerSecond", resolved.TokensPerSecond, "requiredTokensPerSecond", headroom.RequiredTokensPerSecond,
			"declaredPromptBytesPerSecond", resolved.PromptBytesPerSecond, "requiredPromptBytesPerSecond", headroom.RequiredPromptBytesPerSecond,
		}

		logger.Warn(unaffordableSentence(headroom), attrs...)
	}
}

func unaffordableSentence(headroom loop.Headroom) string {
	prefix := fmt.Sprintf("call site %q is not affordable at the declared floors", headroom.Name)

	switch {
	case headroom.RequiredTokensPerSecond > 0 && headroom.RequiredPromptBytesPerSecond > 0:
		return fmt.Sprintf("%s: this host would have to deliver at least %.1f tokens per second, or process the prompt at %.0f bytes per second",
			prefix, headroom.RequiredTokensPerSecond, headroom.RequiredPromptBytesPerSecond)
	case headroom.RequiredTokensPerSecond > 0:
		return fmt.Sprintf("%s: this host would have to deliver at least %.1f tokens per second, and no prompt rate affords it on its own",
			prefix, headroom.RequiredTokensPerSecond)
	case headroom.RequiredPromptBytesPerSecond > 0:
		return fmt.Sprintf("%s: no generation rate affords it on its own, because the prompt alone claims the bound; this host would have to process the prompt at %.0f bytes per second",
			prefix, headroom.RequiredPromptBytesPerSecond)
	default:
		return fmt.Sprintf("%s: neither a faster generation rate nor a faster prompt rate affords it alone, because each half already claims the whole bound",
			prefix)
	}
}

func round(d time.Duration) time.Duration {
	return d.Round(time.Millisecond)
}
