package constants

import "testing"

func TestRunStateTransitions(t *testing.T) {
	valid := [][2]RunState{{RunImported, RunQualityChecked}, {RunQualityChecked, RunProcessing}, {RunProcessing, RunProcessed}, {RunProcessed, RunSuperseded}}
	for _, transition := range valid {
		if !transition[0].CanTransition(transition[1]) {
			t.Errorf("expected %s -> %s", transition[0], transition[1])
		}
	}
	if RunImported.CanTransition(RunProcessed) {
		t.Fatal("imported -> processed must be rejected")
	}
	if RunRejected.CanTransition(RunProcessing) {
		t.Fatal("rejected state must be terminal")
	}
}

func TestGapStateAndSeverity(t *testing.T) {
	if !GapDetected.CanTransition(GapReviewed) {
		t.Fatal("detected -> reviewed should be allowed")
	}
	if GapDetected.CanTransition(GapClosed) {
		t.Fatal("detected -> closed must be rejected")
	}
	if GapAccepted.CanTransition(GapResurveyed) {
		t.Fatal("accepted -> resurveyed must be driven by a resurvey task, not a manual transition")
	}
	if GapAccepted.CanTransition(GapRetesting) {
		t.Fatal("accepted -> retesting must be driven by task creation")
	}
	if GapRetesting.CanTransition(GapResurveyed) || GapRetesting.CanTransition(GapAccepted) {
		t.Fatal("retesting exits only through task completion or cancellation")
	}
	if SeverityForRatio(0.13) != SeverityCritical || SeverityForRatio(0.07) != SeverityMajor || SeverityForRatio(0.01) != SeverityMinor {
		t.Fatal("severity thresholds are inconsistent")
	}
}

func TestResurveyTaskTransitions(t *testing.T) {
	valid := [][2]ResurveyTaskState{
		{TaskPending, TaskRunning},
		{TaskRunning, TaskVerifying},
		{TaskVerifying, TaskDone},
		{TaskVerifying, TaskRunning},
		{TaskPending, TaskCanceled},
		{TaskRunning, TaskCanceled},
		{TaskVerifying, TaskCanceled},
	}
	for _, transition := range valid {
		if !transition[0].CanTransition(transition[1]) {
			t.Errorf("expected %s -> %s", transition[0], transition[1])
		}
	}
	if TaskPending.CanTransition(TaskDone) {
		t.Fatal("pending tasks must be executed before re-verification")
	}
	if TaskRunning.CanTransition(TaskDone) {
		t.Fatal("running tasks must be submitted for re-verification first")
	}
	if TaskDone.CanTransition(TaskCanceled) || TaskCanceled.CanTransition(TaskRunning) {
		t.Fatal("finished tasks are terminal")
	}
	if !TaskRunning.Open() || TaskDone.Open() || TaskCanceled.Open() {
		t.Fatal("open-state classification is inconsistent")
	}
}
