package dto

type CreateResurveyTaskRequest struct {
	CoverageGapID uint   `json:"coverage_gap_id" binding:"required,gt=0"`
	Assignee      string `json:"assignee" binding:"required,min=2,max=80"`
	PlannedDate   string `json:"planned_date" binding:"required"`
}

type ResurveyTaskTransitionRequest struct {
	TargetState     string `json:"target_state" binding:"required,oneof=in_progress pending_verification completed cancelled"`
	ExpectedVersion uint   `json:"expected_version" binding:"required,gt=0"`
	Note            string `json:"note" binding:"omitempty,max=600"`
}

type ResurveyTaskQuery struct {
	CoverageGapID uint
	State         string
	Page          int
	PageSize      int
}
