package store

import (
	"context"
	"database/sql"
	"fmt"

	"agentsynch/internal/objects"
)

// allColumns is the full column list used by every read query.
const allColumns = `id, title, description, status, plan, claimed_by, claimed_at, created_at,
	finished_at, output, error, heartbeat_at, attempts, validator_id, validation_claimed_at,
	branch_name, gh_url`

func scanTask(row interface {
	Scan(...any) error
}) (objects.Task, error) {
	var t objects.Task
	err := row.Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Plan,
		&t.ClaimedBy, &t.ClaimedAt, &t.CreatedAt,
		&t.FinishedAt, &t.Output, &t.Error,
		&t.HeartbeatAt, &t.Attempts, &t.ValidatorID, &t.ValidationClaimedAt,
		&t.BranchName, &t.GhURL,
	)
	return t, err
}

func AddTask(db *sql.DB, task objects.Task, deps []int64) (int64, error) {
	if len(deps) > 0 {
		task.Status = "blocked"
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// insert tasks based on the given items
	result, err := tx.Exec(
		`INSERT INTO tasks (title, description, status, plan, claimed_by, claimed_at, created_at, finished_at, output, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Title, task.Description, task.Status, task.Plan,
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

	for _, depID := range deps {
		_, err := tx.Exec(
			`INSERT INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)`,
			id, depID,
		)
		if err != nil {
			return 0, fmt.Errorf("invalid dependency task-%d: %w", depID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
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

	depRows, err := db.Query(`SELECT depends_on_id FROM task_dependencies WHERE task_id = ? ORDER BY depends_on_id`, id)
	if err != nil {
		return nil, err
	}
	defer depRows.Close()
	for depRows.Next() {
		var depID int64
		if err := depRows.Scan(&depID); err != nil {
			return nil, err
		}
		t.Dependencies = append(t.Dependencies, depID)
	}
	if err := depRows.Err(); err != nil {
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
	idxByID := make(map[int64]int)

	// package tasks into a struct
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		idxByID[t.ID] = len(tasks)
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	depRows, err := db.Query(`SELECT task_id, depends_on_id FROM task_dependencies ORDER BY task_id, depends_on_id`)
	if err != nil {
		return nil, err
	}
	defer depRows.Close()

	for depRows.Next() {
		var taskID, dependsOnID int64
		if err := depRows.Scan(&taskID, &dependsOnID); err != nil {
			return nil, err
		}
		if idx, ok := idxByID[taskID]; ok {
			tasks[idx].Dependencies = append(tasks[idx].Dependencies, dependsOnID)
		}
	}
	return tasks, depRows.Err()
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
