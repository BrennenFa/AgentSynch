package server

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"agentsynch/internal/store"
)

func reap(db *sql.DB, ch chan string) {
	results, err := store.ReapZombies(db, zombieTimeout)
	if err != nil {
		ch <- fmt.Sprintf("[%s] reaper error: %v", time.Now().Format("2006-01-02 15:04:05"), err)
		return
	}

	if len(results) == 0 {
		ch <- fmt.Sprintf("[%s] 0 tasks reaped", time.Now().Format("2006-01-02 15:04:05"))
		return
	}

	parts := make([]string, len(results))
	for i, r := range results {
		parts[i] = fmt.Sprintf("task-%d \"%s\" → %s", r.TaskID, r.Title, r.NewStatus)
	}
	ch <- fmt.Sprintf("[%s] %d tasks reaped - %s",
		time.Now().Format("2006-01-02 15:04:05"),
		len(results),
		strings.Join(parts, ", "),
	)
}
