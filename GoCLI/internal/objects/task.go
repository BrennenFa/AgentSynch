package objects

type TaskDependency struct {
	TaskID      int64
	DependsOnID int64
}

type Task struct {
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
	HeartbeatAt  *string
	Attempts     int
}
