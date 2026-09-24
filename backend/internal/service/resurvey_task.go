package service

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"sonar-survey-coverage-planner/backend/internal/constants"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/model"
	"sonar-survey-coverage-planner/backend/internal/repository"
	"sonar-survey-coverage-planner/backend/pkg/api"
)

type ResurveyTaskService struct {
	tasks *repository.ResurveyTaskRepository
	gaps  *repository.CoverageGapRepository
	audit *AuditService
}

func NewResurveyTaskService(tasks *repository.ResurveyTaskRepository, gaps *repository.CoverageGapRepository, audit *AuditService) *ResurveyTaskService {
	return &ResurveyTaskService{tasks: tasks, gaps: gaps, audit: audit}
}

func (s *ResurveyTaskService) List(query dto.ResurveyTaskQuery) ([]model.ResurveyTask, int64, error) {
	return s.tasks.List(query)
}

func (s *ResurveyTaskService) Get(id uint) (model.ResurveyTask, error) {
	item, err := s.tasks.Get(id)
	if err != nil {
		return item, mapDatabaseError(err, "补测执行单")
	}
	return item, nil
}

func (s *ResurveyTaskService) Create(request dto.CreateResurveyTaskRequest, actor Actor) (model.ResurveyTask, error) {
	plannedDate, err := time.Parse("2006-01-02", request.PlannedDate)
	if err != nil {
		return model.ResurveyTask{}, api.Unprocessable("PLANNED_DATE_INVALID", "计划日期必须为 YYYY-MM-DD 格式", err)
	}
	gap, err := s.gaps.Get(request.CoverageGapID)
	if err != nil {
		return model.ResurveyTask{}, mapDatabaseError(err, "覆盖缺口")
	}
	if gap.GapState != string(constants.GapAccepted) {
		return model.ResurveyTask{}, api.Conflict("GAP_NOT_ACCEPTED", "仅接受补测状态的缺口可创建补测执行单", nil)
	}
	gapID := gap.ID
	item := model.ResurveyTask{CoverageGapID: gap.ID, Assignee: request.Assignee, PlannedDate: plannedDate, TaskState: string(constants.TaskPending), ActiveGapKey: &gapID, Version: 1, CreatedBy: actor.UserID}
	var updatedGap model.CoverageGap
	err = s.gaps.DB().Transaction(func(tx *gorm.DB) error {
		txTasks := s.tasks.WithTx(tx)
		txGaps := s.gaps.WithTx(tx)
		if txErr := txTasks.Create(&item); txErr != nil {
			return txErr
		}
		explanation := gap.Explanation + fmt.Sprintf(" 补测执行单已建立：执行人 %s，计划 %s。", request.Assignee, request.PlannedDate)
		var txErr error
		updatedGap, txErr = txGaps.Transition(gap.ID, gap.Version, string(constants.GapAccepted), string(constants.GapResurveying), explanation)
		return txErr
	})
	if err != nil {
		return model.ResurveyTask{}, mapResurveyTaskError(err)
	}
	if err := s.audit.Record(actor, "resurvey_task.create", "resurvey_task", item.ID, nil, item, map[string]any{"coverage_gap_id": gap.ID, "assignee": request.Assignee, "planned_date": request.PlannedDate}); err != nil {
		return item, err
	}
	if err := s.audit.Record(actor, "coverage.transition", "coverage_gap", gap.ID, gap, updatedGap, map[string]any{"resurvey_task_id": item.ID, "review_note": "补测执行单建立"}); err != nil {
		return item, err
	}
	return item, nil
}

func (s *ResurveyTaskService) Transition(id uint, request dto.ResurveyTaskTransitionRequest, actor Actor) (model.ResurveyTask, error) {
	before, err := s.Get(id)
	if err != nil {
		return model.ResurveyTask{}, err
	}
	from, err := constants.ParseResurveyTaskState(before.TaskState)
	if err != nil {
		return model.ResurveyTask{}, fmt.Errorf("stored resurvey task state: %w", err)
	}
	target, err := constants.ParseResurveyTaskState(request.TargetState)
	if err != nil {
		return model.ResurveyTask{}, api.Unprocessable("RESURVEY_TASK_STATE_INVALID", "目标任务状态无效", err)
	}
	if !from.CanTransition(target) {
		return model.ResurveyTask{}, api.Conflict("RESURVEY_TASK_TRANSITION_INVALID", fmt.Sprintf("不能从 %s 迁移到 %s", from, target), nil)
	}
	var updated model.ResurveyTask
	var gapBefore, gapAfter model.CoverageGap
	gapSynced := false
	err = s.gaps.DB().Transaction(func(tx *gorm.DB) error {
		txTasks := s.tasks.WithTx(tx)
		txGaps := s.gaps.WithTx(tx)
		var txErr error
		updated, txErr = txTasks.Transition(id, request.ExpectedVersion, before.TaskState, request.TargetState, request.Note, target.Finished())
		if txErr != nil {
			return txErr
		}
		if target == constants.TaskCompleted || target == constants.TaskCancelled {
			gapBefore, txErr = txGaps.Get(before.CoverageGapID)
			if txErr != nil {
				return txErr
			}
			gapTarget := constants.GapAccepted
			note := "补测执行单已取消，缺口回到接受补测。"
			if target == constants.TaskCompleted {
				gapTarget = constants.GapResurveyed
				note = "补测执行单复验完成，缺口进入已补测。"
			}
			gapAfter, txErr = txGaps.Transition(gapBefore.ID, gapBefore.Version, string(constants.GapResurveying), string(gapTarget), gapBefore.Explanation+" "+note)
			if txErr != nil {
				return txErr
			}
			gapSynced = true
		}
		return nil
	})
	if err != nil {
		return model.ResurveyTask{}, mapResurveyTaskError(err)
	}
	if err := s.audit.Record(actor, "resurvey_task.transition", "resurvey_task", id, before, updated, map[string]any{"note": request.Note}); err != nil {
		return updated, err
	}
	if gapSynced {
		if err := s.audit.Record(actor, "coverage.transition", "coverage_gap", gapBefore.ID, gapBefore, gapAfter, map[string]any{"resurvey_task_id": id}); err != nil {
			return updated, err
		}
	}
	return updated, nil
}

func mapResurveyTaskError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return api.Conflict("RESURVEY_TASK_ACTIVE_EXISTS", "同一缺口不能同时有两张未结束的补测执行单", err)
	case errors.Is(err, repository.ErrVersionConflict):
		return api.Conflict("VERSION_CONFLICT", "补测执行单或缺口状态已变化，请刷新后重试", err)
	default:
		return mapDatabaseError(err, "补测执行单")
	}
}
