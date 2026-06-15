package objects

type TaskDependency struct {
	TaskID      int64 `json:"task_id"`
	DependsOnID int64 `json:"depends_on_id"`
}

type Task struct {
<<<<<<< HEAD
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
	Dependencies []int64  `json:"dependencies"`
	HeartbeatAt          *string `json:"heartbeat_at"`
	Attempts             int     `json:"attempts"`
	ValidatorID          *string `json:"validator_id"`
	ValidationClaimedAt  *string `json:"validation_claimed_at"`
	SameBranch           bool    `json:"same_branch"`
	BranchName           *string `json:"branch_name"`
	GhURL                *string `json:"gh_url"`
=======
	ID          int64
	Title       string
	Description string
	Status      string
	Plan        *string
	ClaimedBy   *string
	ClaimedAt   *string
	CreatedAt   string
	FinishedAt  *string
	Output      *string
	Error       *string
	Dependencies []int64
	HeartbeatAt          *string
	Attempts             int
	ValidatorID          *string
	ValidationClaimedAt  *string
	SameBranch           bool
	BranchName           *string
	GhURL                *string
>>>>>>> validation_loop
}
