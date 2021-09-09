package shared

import (
	"fmt"
	"time"
)

// ScheduledTasks ...
type ScheduledTasks struct {
	tasks []*scheduledTaskEntry
}

// AddTask ...
func (st *ScheduledTasks) AddTask(name string, nextScheduledTime time.Time, timeBetweenRuns time.Duration, fxn ScheduleTaskFunc) {

	LogInfo(fmt.Sprintf("AddTask called with %s %v %v", name, nextScheduledTime, timeBetweenRuns))

	st.tasks = append(st.tasks, &scheduledTaskEntry{
		name:              name,
		nextScheduledTime: nextScheduledTime,
		timeBetweenRuns:   timeBetweenRuns,
		fxn:               fxn,
	})
}

// RunTasksIfNeeded ...
func (st *ScheduledTasks) RunTasksIfNeeded(db interface{}, ctx interface{}) {

	now := time.Now()

	for _, task := range st.tasks {

		if now.After(task.nextScheduledTime) {
			task.nextScheduledTime = time.Now().Add(task.timeBetweenRuns)
			LogInfo("Running task '" + task.name + "', next scheduled time " + task.nextScheduledTime.String())
			err := task.fxn(db, ctx)

			if err != nil {
				LogErrorMsg(err, "Task '"+task.name+"' returned error.")
			}
		}
	}
}

// ScheduledTaskEntry ...
type scheduledTaskEntry struct {
	name              string
	nextScheduledTime time.Time
	timeBetweenRuns   time.Duration
	fxn               ScheduleTaskFunc
}

// ScheduleTaskFunc ...
type ScheduleTaskFunc func(interface{}, interface{}) error
