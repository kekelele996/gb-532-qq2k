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
	if !GapAccepted.CanTransition(GapResurveying) || !GapResurveying.CanTransition(GapResurveyed) || !GapResurveying.CanTransition(GapAccepted) {
		t.Fatal("resurvey task driven gap transitions should be allowed")
	}
	if GapAccepted.CanTransition(GapResurveyed) {
		t.Fatal("accepted -> resurveyed must go through a resurvey task")
	}
	if !IsTaskDrivenGapTransition(GapAccepted, GapResurveying) || !IsTaskDrivenGapTransition(GapResurveying, GapResurveyed) {
		t.Fatal("task driven transitions must be detectable")
	}
	if IsTaskDrivenGapTransition(GapDetected, GapReviewed) || IsTaskDrivenGapTransition(GapResurveyed, GapClosed) {
		t.Fatal("manual review transitions must not be flagged as task driven")
	}
	if SeverityForRatio(0.13) != SeverityCritical || SeverityForRatio(0.07) != SeverityMajor || SeverityForRatio(0.01) != SeverityMinor {
		t.Fatal("severity thresholds are inconsistent")
	}
}

func TestResurveyTaskTransitions(t *testing.T) {
	valid := [][2]ResurveyTaskState{{TaskPending, TaskInProgress}, {TaskInProgress, TaskPendingVerification}, {TaskPendingVerification, TaskCompleted}, {TaskPending, TaskCancelled}, {TaskInProgress, TaskCancelled}, {TaskPendingVerification, TaskCancelled}}
	for _, transition := range valid {
		if !transition[0].CanTransition(transition[1]) {
			t.Errorf("expected %s -> %s", transition[0], transition[1])
		}
	}
	if TaskPending.CanTransition(TaskCompleted) {
		t.Fatal("pending -> completed must be rejected")
	}
	if TaskInProgress.CanTransition(TaskPending) {
		t.Fatal("in_progress -> pending must be rejected")
	}
	if TaskCompleted.CanTransition(TaskInProgress) || TaskCancelled.CanTransition(TaskPending) {
		t.Fatal("finished states must be terminal")
	}
	if !TaskCompleted.Finished() || !TaskCancelled.Finished() || TaskPending.Finished() || TaskInProgress.Finished() || TaskPendingVerification.Finished() {
		t.Fatal("finished flag is inconsistent")
	}
	if _, err := ParseResurveyTaskState("unknown"); err == nil {
		t.Fatal("unknown task state must be rejected")
	}
}
