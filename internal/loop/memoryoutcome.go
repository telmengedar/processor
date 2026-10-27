package loop

type memoryOutcome struct {
	runID    string
	outcome  string
	status   string
	nodeType string
	nodeID   int64
}

func (t *Turn) setMemoryOutcome(outcome, status string, nodeID int64) {
	if t.memoryOutcome == nil || t.memoryOutcome.nodeID > 0 {
		return
	}
	t.memoryOutcome.outcome = outcome
	t.memoryOutcome.status = status
	t.memoryOutcome.nodeID = nodeID
}

func (t *Turn) logMemoryOutcome() {
	outcome := t.memoryOutcome
	attrs := []any{"runID", outcome.runID, "outcome", outcome.outcome, "status", outcome.status, "noteType", outcome.nodeType}
	if outcome.nodeID > 0 {
		attrs = append(attrs, "node", outcome.nodeID)
	}
	t.log().Info("run memory outcome", attrs...)
}
