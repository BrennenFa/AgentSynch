package store

import (
	"context"
	"database/sql"
	"time"
)

func FinishTask(db *sql.DB, id int64, output string) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	finishedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := tx.Exec(
		`UPDATE tasks SET status = 'finished', finished_at = ?, output = ? WHERE id = ? AND status = 'claimed'`,
		finishedAt, output, id,
	)
	if err := validateResults(result, err, id, "claimed"); err != nil {
		return err
	}

	// unblock any dependent tasks whose other dependencies are all finished
	_, err = tx.Exec(`
		UPDATE tasks
		SET status = 'available'
		WHERE status = 'blocked'
		  AND id NOT IN (
		      SELECT d.task_id
		      FROM task_dependencies d
		      JOIN tasks dep ON dep.id = d.depends_on_id
		      WHERE dep.status != 'finished'
		  )
	`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func ErrorTask(db *sql.DB, id int64, errMsg string) error {
	// TODO -- Add a way to check go back and solve errors

	finishedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE tasks SET status = 'error', finished_at = ?, error = ? WHERE id = ? AND status = 'claimed'`,
		finishedAt, errMsg, id,
	)
	return validateResults(result, err, id, "claimed")
}

func WritePlan(db *sql.DB, id int64, plan string) error {

	// add in a plan for how the task will be completed

	result, err := db.Exec(
		`UPDATE tasks SET plan = ? WHERE id = ? AND status = 'claimed'`,
		plan, id,
	)
	return validateResults(result, err, id, "claimed")
}

// SetBranchName records the branch an agent created. Only valid on claimed tasks.
func SetBranchName(db *sql.DB, id int64, branchName string) error {
	result, err := db.Exec(
		`UPDATE tasks SET branch_name = ? WHERE id = ? AND status = 'claimed'`,
		branchName, id,
	)
	return validateResults(result, err, id, "claimed")
}

// ArchiveTask transitions a finished or error task to archived (soft-delete).
func ArchiveTask(db *sql.DB, id int64) error {
	result, err := db.Exec(
		`UPDATE tasks SET status = 'archived' WHERE id = ? AND status IN ('finished', 'error')`,
		id,
	)
	return validateResults(result, err, id, "finished or error")
}
