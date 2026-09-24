package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/model"
)

type ResurveyTaskRepository struct{ db *gorm.DB }

func NewResurveyTaskRepository(db *gorm.DB) *ResurveyTaskRepository {
	return &ResurveyTaskRepository{db: db}
}

func (r *ResurveyTaskRepository) List(query dto.ResurveyTaskQuery) ([]model.ResurveyTask, int64, error) {
	db := r.db.Model(&model.ResurveyTask{})
	if query.CoverageGapID > 0 {
		db = db.Where("coverage_gap_id = ?", query.CoverageGapID)
	}
	if query.State != "" {
		db = db.Where("task_state = ?", query.State)
	}
	if query.OpenOnly {
		db = db.Where("task_state IN ?", []string{"pending", "running", "awaiting_review"})
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count resurvey tasks: %w", err)
	}
	var items []model.ResurveyTask
	if err := db.Preload("CoverageGap.SurveyArea").Order("created_at DESC, id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list resurvey tasks: %w", err)
	}
	return items, total, nil
}

func (r *ResurveyTaskRepository) Get(id uint) (model.ResurveyTask, error) {
	var item model.ResurveyTask
	if err := r.db.Preload("CoverageGap.SurveyArea").First(&item, id).Error; err != nil {
		return item, fmt.Errorf("get resurvey task: %w", err)
	}
	return item, nil
}

// OpenCount 返回缺口当前未结束（pending/running/awaiting_review）任务数。
func (r *ResurveyTaskRepository) OpenCount(tx *gorm.DB, gapID uint) (int64, error) {
	var count int64
	if err := tx.Model(&model.ResurveyTask{}).
		Where("coverage_gap_id = ? AND task_state IN ?", gapID, []string{"pending", "running", "awaiting_review"}).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count open resurvey tasks: %w", err)
	}
	return count, nil
}

// CreateInTx 要求缺口版本与状态同时匹配 accepted；缺口状态被推到 retesting。
// 任一条件不满足都返回 ErrVersionConflict，让旧页面提交收到 409。
func (r *ResurveyTaskRepository) CreateInTx(tx *gorm.DB, item *model.ResurveyTask, expectedGapVersion uint) (model.CoverageGap, error) {
	result := tx.Model(&model.CoverageGap{}).
		Where("id = ? AND version = ? AND gap_state = ?", item.CoverageGapID, expectedGapVersion, "accepted").
		Updates(map[string]any{"gap_state": "retesting", "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return model.CoverageGap{}, fmt.Errorf("mark gap retesting: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.CoverageGap{}, ErrVersionConflict
	}
	if err := tx.Create(item).Error; err != nil {
		if isDuplicatedKey(err) {
			return model.CoverageGap{}, ErrOpenTaskExists
		}
		return model.CoverageGap{}, fmt.Errorf("create resurvey task: %w", err)
	}
	var gap model.CoverageGap
	if err := tx.First(&gap, item.CoverageGapID).Error; err != nil {
		return model.CoverageGap{}, fmt.Errorf("reload coverage gap: %w", err)
	}
	return gap, nil
}

// TransitionInTx 按任务版本与当前状态做条件更新；confirmed=true 时缺口进入 resurveyed，
// canceled=true 时缺口退回 accepted。
func (r *ResurveyTaskRepository) TransitionInTx(tx *gorm.DB, id, expectedVersion uint, from, to string, fields map[string]any, gapExplanation string) (model.ResurveyTask, model.CoverageGap, error) {
	updates := map[string]any{"task_state": to, "version": gorm.Expr("version + 1")}
	for key, value := range fields {
		updates[key] = value
	}
	result := tx.Model(&model.ResurveyTask{}).Where("id = ? AND version = ? AND task_state = ?", id, expectedVersion, from).Updates(updates)
	if result.Error != nil {
		return model.ResurveyTask{}, model.CoverageGap{}, fmt.Errorf("transition resurvey task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return model.ResurveyTask{}, model.CoverageGap{}, ErrVersionConflict
	}

	var task model.ResurveyTask
	if err := tx.First(&task, id).Error; err != nil {
		return model.ResurveyTask{}, model.CoverageGap{}, fmt.Errorf("reload resurvey task: %w", err)
	}
	gapTarget := ""
	switch to {
	case "completed":
		gapTarget = "resurveyed"
	case "canceled":
		gapTarget = "accepted"
	}
	var gap model.CoverageGap
	if gapTarget != "" {
		gapResult := tx.Model(&model.CoverageGap{}).
			Where("id = ? AND gap_state = ?", task.CoverageGapID, "retesting").
			Updates(map[string]any{"gap_state": gapTarget, "explanation": gapExplanation, "version": gorm.Expr("version + 1")})
		if gapResult.Error != nil {
			return model.ResurveyTask{}, model.CoverageGap{}, fmt.Errorf("transition gap with resurvey task: %w", gapResult.Error)
		}
		if gapResult.RowsAffected == 0 {
			return model.ResurveyTask{}, model.CoverageGap{}, ErrVersionConflict
		}
	}
	if err := tx.First(&gap, task.CoverageGapID).Error; err != nil {
		return model.ResurveyTask{}, model.CoverageGap{}, fmt.Errorf("reload coverage gap: %w", err)
	}
	return task, gap, nil
}

func (r *ResurveyTaskRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// isDuplicatedKey 兼容 Postgres 与 SQLite 的唯一约束冲突。
func isDuplicatedKey(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "UNIQUE constraint failed") || strings.Contains(message, "duplicate key value")
}
