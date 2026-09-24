package constants

import "fmt"

type ResurveyTaskState string

const (
	TaskPending             ResurveyTaskState = "pending"
	TaskInProgress          ResurveyTaskState = "in_progress"
	TaskPendingVerification ResurveyTaskState = "pending_verification"
	TaskCompleted           ResurveyTaskState = "completed"
	TaskCancelled           ResurveyTaskState = "cancelled"
)

var resurveyTaskTransitions = map[ResurveyTaskState]map[ResurveyTaskState]struct{}{
	TaskPending:             {TaskInProgress: {}, TaskCancelled: {}},
	TaskInProgress:          {TaskPendingVerification: {}, TaskCancelled: {}},
	TaskPendingVerification: {TaskCompleted: {}, TaskCancelled: {}},
	TaskCompleted:           {},
	TaskCancelled:           {},
}

func (s ResurveyTaskState) Valid() bool {
	_, ok := resurveyTaskTransitions[s]
	return ok
}

func (s ResurveyTaskState) CanTransition(target ResurveyTaskState) bool {
	allowed, ok := resurveyTaskTransitions[s]
	if !ok {
		return false
	}
	_, ok = allowed[target]
	return ok
}

// Finished 表示任务已结束（已完成或已取消），结束后释放缺口的在建占用。
func (s ResurveyTaskState) Finished() bool {
	return s == TaskCompleted || s == TaskCancelled
}

func ParseResurveyTaskState(value string) (ResurveyTaskState, error) {
	state := ResurveyTaskState(value)
	if !state.Valid() {
		return "", fmt.Errorf("unknown resurvey task state %q", value)
	}
	return state, nil
}
