package repository

import (
	"fmt"

	"gorm.io/gorm"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/model"
)

type ResurveyTaskRepository struct{ db *gorm.DB }

func NewResurveyTaskRepository(db *gorm.DB) *ResurveyTaskRepository {
	return &ResurveyTaskRepository{db: db}
}

func (r *ResurveyTaskRepository) WithTx(tx *gorm.DB) *ResurveyTaskRepository {
	return &ResurveyTaskRepository{db: tx}
}

func (r *ResurveyTaskRepository) List(query dto.ResurveyTaskQuery) ([]model.ResurveyTask, int64, error) {
	db := r.db.Model(&model.ResurveyTask{})
	if query.CoverageGapID > 0 {
		db = db.Where("coverage_gap_id = ?", query.CoverageGapID)
	}
	if query.State != "" {
		db = db.Where("task_state = ?", query.State)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count resurvey tasks: %w", err)
	}
	var items []model.ResurveyTask
	if err := db.Order("created_at DESC, id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list resurvey tasks: %w", err)
	}
	return items, total, nil
}

func (r *ResurveyTaskRepository) Get(id uint) (model.ResurveyTask, error) {
	var item model.ResurveyTask
	if err := r.db.First(&item, id).Error; err != nil {
		return item, fmt.Errorf("get resurvey task: %w", err)
	}
	return item, nil
}

func (r *ResurveyTaskRepository) Create(item *model.ResurveyTask) error {
	if err := r.db.Create(item).Error; err != nil {
		return fmt.Errorf("create resurvey task: %w", err)
	}
	return nil
}

func (r *ResurveyTaskRepository) Transition(id, expectedVersion uint, from, to, note string, finished bool) (model.ResurveyTask, error) {
	updates := map[string]any{"task_state": to, "version": gorm.Expr("version + 1")}
	if note != "" {
		updates["note"] = note
	}
	if finished {
		updates["active_gap_key"] = nil
	}
	result := r.db.Model(&model.ResurveyTask{}).Where("id = ? AND version = ? AND task_state = ?", id, expectedVersion, from).Updates(updates)
	if result.Error != nil {
		return model.ResurveyTask{}, fmt.Errorf("transition resurvey task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ResurveyTask{}, ErrVersionConflict
	}
	return r.Get(id)
}
