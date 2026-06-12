package objects

type Task struct {
	ID          string  `yaml:"id"`
	Title       string  `yaml:"title"`
	Description string  `yaml:"description"`
	Status      string  `yaml:"status"`
	ClaimedBy   *string `yaml:"claimed_by"`
	ClaimedAt   *string `yaml:"claimed_at"`
	CreatedAt   string  `yaml:"created_at"`
	FinishedAt  *string `yaml:"finished_at"`
	Output      *string `yaml:"output"`
	Error       *string `yaml:"error"`
}

type TaskStore struct {
	Tasks []Task `yaml:"tasks"`
}
