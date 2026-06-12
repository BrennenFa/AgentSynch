package objects

type Task struct {
	ID          int64
	Title       string
	Description string
	Status      string
	ClaimedBy   *string
	ClaimedAt   *string
	CreatedAt   string
	FinishedAt  *string
	Output      *string
	Error       *string
}
