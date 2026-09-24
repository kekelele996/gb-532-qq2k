package constants

import "fmt"

type ResurveyTaskState string

const (
	TaskPending   ResurveyTaskState = "pending"
	TaskRunning   ResurveyTaskState = "running"
	TaskVerifying ResurveyTaskState = "awaiting_review"
	TaskDone      ResurveyTaskState = "completed"
	TaskCanceled  ResurveyTaskState = "canceled"
)

// ResurveyTaskOpenStates 是尚未结束的补测单状态，同一缺口在此集合内至多一张任务。
var ResurveyTaskOpenStates = []ResurveyTaskState{TaskPending, TaskRunning, TaskVerifying}

var resurveyTaskTransitions = map[ResurveyTaskState]map[ResurveyTaskState]struct{}{
	TaskPending:   {TaskRunning: {}, TaskCanceled: {}},
	TaskRunning:   {TaskVerifying: {}, TaskCanceled: {}},
	TaskVerifying: {TaskDone: {}, TaskRunning: {}, TaskCanceled: {}},
	TaskDone:      {},
	TaskCanceled:  {},
}

func (s ResurveyTaskState) Valid() bool {
	_, ok := resurveyTaskTransitions[s]
	return ok
}

func (s ResurveyTaskState) Open() bool {
	for _, state := range ResurveyTaskOpenStates {
		if s == state {
			return true
		}
	}
	return false
}

func (s ResurveyTaskState) CanTransition(target ResurveyTaskState) bool {
	allowed, ok := resurveyTaskTransitions[s]
	if !ok {
		return false
	}
	_, ok = allowed[target]
	return ok
}

func ParseResurveyTaskState(value string) (ResurveyTaskState, error) {
	state := ResurveyTaskState(value)
	if !state.Valid() {
		return "", fmt.Errorf("unknown resurvey task state %q", value)
	}
	return state, nil
}
