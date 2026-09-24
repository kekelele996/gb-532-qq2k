package model

import "time"

// ResurveyTask 记录“已接受补测”缺口的补测执行单：执行人、计划日期与任务状态机。
// 同一缺口至多一张未结束任务由部分唯一索引 idx_resurvey_task_open 保证（见 config 迁移）。
type ResurveyTask struct {
	ID            uint         `json:"id" gorm:"primaryKey"`
	CoverageGapID uint         `json:"coverage_gap_id" gorm:"not null;index:idx_resurvey_gap_state,priority:1"`
	AssigneeName  string       `json:"assignee_name" gorm:"size:80;not null"`
	PlannedDate   time.Time    `json:"planned_date" gorm:"not null;index"`
	TaskState     string       `json:"task_state" gorm:"size:24;not null;index:idx_resurvey_gap_state,priority:2;index:idx_resurvey_open_state"`
	ExecutionNote string       `json:"execution_note" gorm:"size:600;not null;default:''"`
	ReviewNote    string       `json:"review_note" gorm:"size:600;not null;default:''"`
	CancelReason  string       `json:"cancel_reason" gorm:"size:600;not null;default:''"`
	CreatedBy     uint         `json:"created_by" gorm:"not null"`
	StartedAt     *time.Time   `json:"started_at,omitempty"`
	SubmittedAt   *time.Time   `json:"submitted_at,omitempty"`
	CompletedAt   *time.Time   `json:"completed_at,omitempty"`
	CanceledAt    *time.Time   `json:"canceled_at,omitempty"`
	Version       uint         `json:"version" gorm:"not null;default:1"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	CoverageGap   *CoverageGap `json:"coverage_gap,omitempty" gorm:"foreignKey:CoverageGapID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (ResurveyTask) TableName() string { return "resurvey_tasks" }
