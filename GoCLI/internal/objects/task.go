package objects

type TaskDependency struct {
	TaskID      int64 `json:"task_id"`
	DependsOnID int64 `json:"depends_on_id"`
}

type Task struct {
	ID           int64    `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	Plan         *string  `json:"plan"`
	ClaimedBy    *string  `json:"claimed_by"`
	ClaimedAt    *string  `json:"claimed_at"`
	CreatedAt    string   `json:"created_at"`
	FinishedAt   *string  `json:"finished_at"`
	Output       *string  `json:"output"`
	Error        *string  `json:"error"`
	HeartbeatAt  *string `json:"heartbeat_at"`
	Attempts     int     `json:"attempts"`
	SameBranch   bool    `json:"same_branch"`
	BranchName   *string `json:"branch_name"`
	GhURL        *string `json:"gh_url"`
	// SharedDocs is a JSON array of context_docs names relevant to this task (e.g. ["architecture","conventions"])
	SharedDocs   *string `json:"shared_docs"`
}
