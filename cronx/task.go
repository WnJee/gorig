package cronx

import (
	"time"

	"github.com/robfig/cron/v3"
)

// TaskInfo represents metadata about a scheduled task.
type TaskInfo struct {
	EntryID  cron.EntryID `json:"entryID"`
	Spec     string       `json:"spec"`
	Name     string       `json:"name"`
	NextRun  time.Time    `json:"nextRun"`
	PrevRun  time.Time    `json:"prevRun"`
}

// PersistTaskStatus represents persistent task state.
type PersistTaskStatus string

const (
	TaskPending PersistTaskStatus = "pending"
	TaskRunning PersistTaskStatus = "running"
	TaskDone    PersistTaskStatus = "done"
	TaskFailed  PersistTaskStatus = "failed"
)
