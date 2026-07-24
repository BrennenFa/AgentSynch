package worker

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// spawnHeartbeat starts a detached background heartbeat process for the given task.
func spawnHeartbeat(taskID int64) {
	hb := exec.Command(os.Args[0], "heartbeat", "--id", fmt.Sprintf("%d", taskID))
	hb.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := hb.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not start heartbeat for task-%d: %v\n", taskID, err)
		return
	}
	hb.Process.Release()
}
