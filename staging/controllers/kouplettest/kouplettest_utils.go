package kouplettest

import (
	"time"

	"github.com/jgwest/kouplet-operator/controllers/shared"
)

// ScheduledTasks ...
type ScheduledTasks struct {
	tasks []*ScheduledTaskEntry
}

func (st *ScheduledTasks) addTasks(task ScheduledTaskEntry) {
	st.tasks = append(st.tasks, &task)
}

func (st *ScheduledTasks) runTasksIfNeeded(db *KoupletDB, ctx KTContext) {

	now := time.Now()

	for _, task := range st.tasks {

		if now.After(task.nextScheduledTime) {
			task.nextScheduledTime = time.Now().Add(task.timeBetweenRuns)
			shared.LogInfo("Running task '" + task.name + "', next scheduled time " + task.nextScheduledTime.String())
			err := task.fxn(db, ctx)

			if err != nil {
				shared.LogErrorMsg(err, "Task '"+task.name+"' returned error.")
			}
		}
	}
}

// ScheduledTaskEntry ...
type ScheduledTaskEntry struct {
	name              string
	nextScheduledTime time.Time
	timeBetweenRuns   time.Duration
	fxn               ScheduleTaskFunc
}

// ScheduleTaskFunc ...
type ScheduleTaskFunc func(*KoupletDB, KTContext) error
