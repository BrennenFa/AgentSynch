package types

// ChatEntry is a single line in the orchestrator chat history.
type ChatEntry struct {
	From string // "you" | "claude" | "system"
	Text string
}

// OrchestratorEvent is what the orchestrator goroutine sends back to BubbleTea.
type OrchestratorEvent struct {
	Response  string   // Claude's text (SPAWN lines already stripped)
	SpawnMsgs []string // status string per spawned agent
	Err       string   // non-empty on failure
}

// ChatState is the slice of model state that the ui package needs to render the chat panel.
type ChatState struct {
	Mode    bool
	Input   string
	History []ChatEntry
}
