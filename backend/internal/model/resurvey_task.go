package model

import "time"

type ResurveyTask struct {
	ID            uint         `json:"id" gorm:"primaryKey"`
	CoverageGapID uint         `json:"coverage_gap_id" gorm:"not null;index"`
	Assignee      string       `json:"assignee" gorm:"size:80;not null"`
	PlannedDate   time.Time    `json:"planned_date" gorm:"type:date;not null"`
	TaskState     string       `json:"task_state" gorm:"size:24;not null;index"`
	Note          string       `json:"note" gorm:"size:600;not null;default:''"`
	ActiveGapKey  *uint        `json:"-" gorm:"column:active_gap_key;uniqueIndex:idx_resurvey_active_gap"`
	Version       uint         `json:"version" gorm:"not null;default:1"`
	CreatedBy     uint         `json:"created_by" gorm:"not null"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	CoverageGap   *CoverageGap `json:"coverage_gap,omitempty" gorm:"foreignKey:CoverageGapID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (ResurveyTask) TableName() string { return "resurvey_tasks" }
