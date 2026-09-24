package dto

import (
	"time"
)

// CreateResurveyTaskRequest 为已接受补测的缺口建立补测执行单。
type CreateResurveyTaskRequest struct {
	CoverageGapID uint   `json:"coverage_gap_id" binding:"required,gt=0"`
	AssigneeName  string `json:"assignee_name" binding:"required,min=2,max=80"`
	// PlannedDate 仅接收 YYYY-MM-DD（本地规划日期），服务层归一化到 UTC 零点。
	PlannedDate    string `json:"planned_date" binding:"required,datetime=2006-01-02"`
	ExpectedGapVer uint   `json:"expected_gap_version" binding:"required,gt=0"`
	Note           string `json:"note" binding:"omitempty,max=500"`
}

type ResurveyTaskTransitionRequest struct {
	TargetState     string `json:"target_state" binding:"required,oneof=running awaiting_review completed canceled"`
	ExpectedVersion uint   `json:"expected_version" binding:"required,gt=0"`
	Note            string `json:"note" binding:"omitempty,max=500"`
}

type ResurveyTaskQuery struct {
	CoverageGapID uint
	State         string
	OpenOnly      bool
	Page          int
	PageSize      int
}

// PlannedAt 解析建单请求中的计划日期并归一化。
func (r CreateResurveyTaskRequest) PlannedAt() (time.Time, error) {
	planned, err := time.ParseInLocation("2006-01-02", r.PlannedDate, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return planned.UTC(), nil
}
