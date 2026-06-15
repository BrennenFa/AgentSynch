package store

import (
	"context"
	"database/sql"
	"fmt"

	"agentsynch/internal/objects"
)

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
	sameBranchInt := 0
	if task.SameBranch {
		sameBranchInt = 1
	}
	result, err := tx.Exec(
		`INSERT INTO tasks (title, description, status, plan, claimed_by, claimed_at, created_at, finished_at, output, error, same_branch)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Title, task.Description, task.Status, task.Plan,
		task.ClaimedBy, task.ClaimedAt, task.CreatedAt,
		task.FinishedAt, task.Output, task.Error, sameBranchInt,
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
	var t objects.Task
	err := db.QueryRow(`SELECT id, title, description, status, plan, claimed_by, claimed_at, created_at, finished_at, output, error FROM tasks WHERE id = ?`, id).Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Plan,
		&t.ClaimedBy, &t.ClaimedAt, &t.CreatedAt,
		&t.FinishedAt, &t.Output, &t.Error,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func listTasksQuery(db *sql.DB, includeArchived bool) ([]objects.Task, error) {
	// TODO --- make more usable for agents
	query := `SELECT id, title, description, status, plan, claimed_by, claimed_at, created_at, finished_at, output, error FROM tasks`
	if !includeArchived {
		// archived tasks are soft-deleted; hide them from normal views
		query += ` WHERE status != 'archived'`
	}
	query += ` ORDER BY id`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []objects.Task
	idxByID := make(map[int64]int)

	// package tasks into a struct
	for rows.Next() {
		var t objects.Task
		err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.Status, &t.Plan,
			&t.ClaimedBy, &t.ClaimedAt, &t.CreatedAt,
			&t.FinishedAt, &t.Output, &t.Error,
		)
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
	return listTasksQuery(db, false)
}

// ListAllTasks returns all tasks including archived ones (full history).
func ListAllTasks(db *sql.DB) ([]objects.Task, error) {
	return listTasksQuery(db, true)
}
