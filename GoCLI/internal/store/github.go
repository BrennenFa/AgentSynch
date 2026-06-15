package store

import (
	"database/sql"

	"agentsynch/internal/objects"
)

// SetDbGit records the GitHub PR/issue URL and changes status to 'archived'
func SetDbGit(db *sql.DB, id int64, url string) error {
	// null guard for idempotency
	result, err := db.Exec(
		`UPDATE tasks SET gh_url = ?, status = 'archived' WHERE id = ? AND gh_url IS NULL`,
		url, id,
	)
	return validateResults(result, err, id, "unprocessed (gh_url IS NULL)")
}

// ListFinishedForGH returns finished tasks that have not yet had a GH action created.
func ListFinishedForGH(db *sql.DB) ([]objects.Task, error) {
	rows, err := db.Query(
		`SELECT id, title, description, plan, output, branch_name FROM tasks WHERE status = 'finished' AND gh_url IS NULL ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []objects.Task

	// iterate through all returned rows
	for rows.Next() {
		var t objects.Task

		// scan the row into a Task struct; handle nullable fields with pointers
		err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Plan, &t.Output, &t.BranchName)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListErrorsForGH returns error tasks that have not yet had a GH issue created.
func ListErrorsForGH(db *sql.DB) ([]objects.Task, error) {
	rows, err := db.Query(
		`SELECT id, title, description, error FROM tasks WHERE status = 'error' AND gh_url IS NULL ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []objects.Task

	// iterate through all returned rows
	for rows.Next() {
		var t objects.Task

		// scan the row into a Task struct; handle nullable fields with pointers
		err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Error)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
