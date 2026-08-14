package store

import (
	"context"
	"database/sql"

	"agentsynch/internal/objects"
)

// allColumns is the full column list used by every read query.
const allColumns = `id, title, COALESCE(description, ''), status, claimed_by, claimed_at, created_at,
	finished_at, output, error, heartbeat_at, attempts, same_branch, branch_name, gh_url, tmux_window`

func scanTask(row interface {
	Scan(...any) error
}) (objects.Task, error) {
	var t objects.Task
	var sameBranchInt int
	err := row.Scan(
		&t.ID, &t.Title, &t.Description, &t.Status,
		&t.ClaimedBy, &t.ClaimedAt, &t.CreatedAt,
		&t.FinishedAt, &t.Output, &t.Error,
		&t.HeartbeatAt, &t.Attempts,
		&sameBranchInt, &t.BranchName, &t.GhURL, &t.TmuxWindow,
	)
	t.SameBranch = sameBranchInt == 1
	return t, err
}

// DeleteTask permanently removes a task from the database.
func DeleteTask(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func AddTask(db *sql.DB, task objects.Task) (int64, error) {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// insert tasks based on the given items
	result, err := tx.Exec(
		`INSERT INTO tasks (title, description, status, claimed_by, claimed_at, created_at, finished_at, output, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Title, task.Description, task.Status,
		task.ClaimedBy, task.ClaimedAt, task.CreatedAt,
		task.FinishedAt, task.Output, task.Error,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, tx.Commit()
}

func GetTask(db *sql.DB, id int64) (*objects.Task, error) {
	row := db.QueryRow(`SELECT `+allColumns+` FROM tasks WHERE id = ?`, id)
	t, err := scanTask(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func listTasksQuery(db *sql.DB, includeArchived bool, statusFilter string) ([]objects.Task, error) {
	query := `SELECT ` + allColumns + ` FROM tasks`
	var args []any

	if statusFilter != "" {
		query += ` WHERE status = ?`
		args = append(args, statusFilter)
	} else if !includeArchived {
		// archived tasks are soft-deleted; hide them from normal views
		query += ` WHERE status != 'archived'`
	}
	query += ` ORDER BY id`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []objects.Task

	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListTasks returns all tasks except archived ones.
func ListTasks(db *sql.DB) ([]objects.Task, error) {
	return listTasksQuery(db, false, "")
}

// ListAllTasks returns all tasks including archived ones (full history).
func ListAllTasks(db *sql.DB) ([]objects.Task, error) {
	return listTasksQuery(db, true, "")
}

// ListTasksByStatus returns tasks matching the given status.
func ListTasksByStatus(db *sql.DB, status string) ([]objects.Task, error) {
	return listTasksQuery(db, true, status)
}
