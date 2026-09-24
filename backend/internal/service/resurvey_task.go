package service

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"sonar-survey-coverage-planner/backend/internal/constants"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/model"
	"sonar-survey-coverage-planner/backend/internal/repository"
	"sonar-survey-coverage-planner/backend/pkg/api"
)

// ResurveyTaskView 同时返回任务与缺口最新快照，页面可一次刷新两侧状态。
type ResurveyTaskView struct {
	Task model.ResurveyTask `json:"task"`
	Gap  model.CoverageGap `json:"gap"`
}

type ResurveyTaskService struct {
	repository *repository.ResurveyTaskRepository
	gaps       *repository.CoverageGapRepository
	audit      *AuditService
}

func NewResurveyTaskService(repository *repository.ResurveyTaskRepository, gaps *repository.CoverageGapRepository, audit *AuditService) *ResurveyTaskService {
	return &ResurveyTaskService{repository: repository, gaps: gaps, audit: audit}
}

func (s *ResurveyTaskService) List(query dto.ResurveyTaskQuery) ([]model.ResurveyTask, int64, error) {
	return s.repository.List(query)
}

func (s *ResurveyTaskService) Get(id uint) (model.ResurveyTask, error) {
	item, err := s.repository.Get(id)
	if err != nil {
		return item, mapDatabaseError(err, "补测执行单")
	}
	return item, nil
}

// Create 只允许已接受补测（accepted）的缺口建单；同一缺口已有未结束任务时返回 409。
func (s *ResurveyTaskService) Create(request dto.CreateResurveyTaskRequest, actor Actor) (ResurveyTaskView, error) {
	gap, err := s.gaps.Get(request.CoverageGapID)
	if err != nil {
		return ResurveyTaskView{}, mapDatabaseError(err, "覆盖缺口")
	}
	if gap.GapState != string(constants.GapAccepted) {
		return ResurveyTaskView{}, api.Conflict("GAP_NOT_ACCEPTED", "只有已接受补测的缺口才能创建补测执行单", nil)
	}
	plannedDate, err := request.PlannedAt()
	if err != nil {
		return ResurveyTaskView{}, api.BadRequest("PLANNED_DATE_INVALID", "计划日期必须是 YYYY-MM-DD 格式", err)
	}
	if plannedDate.Before(dayFloor(time.Now())) {
		return ResurveyTaskView{}, api.Unprocessable("PLANNED_DATE_PAST", "计划日期不能早于今天", nil)
	}
	assignee := strings.TrimSpace(request.AssigneeName)
	if len([]rune(assignee)) < 2 {
		return ResurveyTaskView{}, api.BadRequest("ASSIGNEE_REQUIRED", "执行人至少 2 个字符", nil)
	}

	task := model.ResurveyTask{
		CoverageGapID: gap.ID,
		AssigneeName:  assignee,
		PlannedDate:   plannedDate,
		TaskState:     string(constants.TaskPending),
		ExecutionNote: strings.TrimSpace(request.Note),
		CreatedBy:     actor.UserID,
		Version:       1,
	}
	var updatedGap model.CoverageGap
	var created model.ResurveyTask
	txErr := s.repository.Transaction(func(tx *gorm.DB) error {
		var txErr error
		updatedGap, txErr = s.repository.CreateInTx(tx, &task, request.ExpectedGapVer)
		if txErr != nil {
			return txErr
		}
		if txErr = s.audit.RecordTx(tx, actor, "resurvey_task.create", "resurvey_task", task.ID, nil, task, map[string]any{
			"gap_id":         gap.ID,
			"assignee":       assignee,
			"planned_date":   plannedDate.Format("2006-01-02"),
			"gap_before":     gap.GapState,
			"gap_after":      updatedGap.GapState,
			"expected_gap_v": request.ExpectedGapVer,
		}); txErr != nil {
			return txErr
		}
		created = task
		return nil
	})
	if txErr != nil {
		return ResurveyTaskView{}, mapDatabaseError(txErr, "补测执行单")
	}
	reload, err := s.repository.Get(created.ID)
	if err != nil {
		return ResurveyTaskView{}, mapDatabaseError(err, "补测执行单")
	}
	// 事务外重载缺口以带上 SurveyArea 预加载，避免页面丢失测区信息。
	gapView, err := s.gaps.Get(updatedGap.ID)
	if err != nil {
		return ResurveyTaskView{}, mapDatabaseError(err, "覆盖缺口")
	}
	return ResurveyTaskView{Task: reload, Gap: gapView}, nil
}

// Transition 推进任务状态机；完成与取消由复核人确认，同时驱动缺口状态迁移。
func (s *ResurveyTaskService) Transition(id uint, request dto.ResurveyTaskTransitionRequest, actor Actor) (ResurveyTaskView, error) {
	before, err := s.repository.Get(id)
	if err != nil {
		return ResurveyTaskView{}, mapDatabaseError(err, "补测执行单")
	}
	from, err := constants.ParseResurveyTaskState(before.TaskState)
	if err != nil {
		return ResurveyTaskView{}, fmt.Errorf("stored task state: %w", err)
	}
	target, err := constants.ParseResurveyTaskState(request.TargetState)
	if err != nil {
		return ResurveyTaskView{}, api.Unprocessable("RESURVEY_TASK_STATE_INVALID", "目标任务状态无效", err)
	}
	if !from.CanTransition(target) {
		return ResurveyTaskView{}, api.Conflict("RESURVEY_TASK_TRANSITION_INVALID", fmt.Sprintf("补测单不能从 %s 迁移到 %s", from, target), nil)
	}
	if !actorMayTransition(actor.Role, target) {
		return ResurveyTaskView{}, api.Forbidden("当前角色无权将补测单推进到该状态")
	}
	note := strings.TrimSpace(request.Note)
	if (target == constants.TaskDone || target == constants.TaskCanceled) && len([]rune(note)) < 4 {
		field := "复验结论"
		if target == constants.TaskCanceled {
			field = "取消原因"
		}
		return ResurveyTaskView{}, api.Unprocessable("RESURVEY_TASK_NOTE_REQUIRED", field+"至少 4 个字符", nil)
	}

	gapBefore, err := s.gaps.Get(before.CoverageGapID)
	if err != nil {
		return ResurveyTaskView{}, mapDatabaseError(err, "覆盖缺口")
	}
	fields, gapExplanation := s.buildTransitionFields(before, gapBefore, target, note)
	var updatedTask model.ResurveyTask
	var updatedGap model.CoverageGap
	txErr := s.repository.Transaction(func(tx *gorm.DB) error {
		var txErr error
		updatedTask, updatedGap, txErr = s.repository.TransitionInTx(tx, id, request.ExpectedVersion, string(from), string(target), fields, gapExplanation)
		if txErr != nil {
			return txErr
		}
		metadata := map[string]any{"from": string(from), "to": string(target), "note": note, "gap_state": updatedGap.GapState}
		if txErr = s.audit.RecordTx(tx, actor, "resurvey_task.transition", "resurvey_task", id, before, updatedTask, metadata); txErr != nil {
			return txErr
		}
		gapAction := ""
		switch target {
		case constants.TaskDone:
			gapAction = "coverage.resurvey_confirmed"
		case constants.TaskCanceled:
			gapAction = "coverage.resurvey_cancelled"
		}
		if gapAction != "" {
			if txErr = s.audit.RecordTx(tx, actor, gapAction, "coverage_gap", before.CoverageGapID, map[string]any{"gap_state": "retesting"}, updatedGap, map[string]any{"task_id": id, "note": note}); txErr != nil {
				return txErr
			}
		}
		return nil
	})
	if txErr != nil {
		return ResurveyTaskView{}, mapDatabaseError(txErr, "补测执行单")
	}
	// 事务外重载以带上预加载的缺口/测区快照。
	taskView, err := s.repository.Get(updatedTask.ID)
	if err != nil {
		return ResurveyTaskView{}, mapDatabaseError(err, "补测执行单")
	}
	gapView, err := s.gaps.Get(updatedGap.ID)
	if err != nil {
		return ResurveyTaskView{}, mapDatabaseError(err, "覆盖缺口")
	}
	return ResurveyTaskView{Task: taskView, Gap: gapView}, nil
}

func (s *ResurveyTaskService) buildTransitionFields(before model.ResurveyTask, gapBefore model.CoverageGap, target constants.ResurveyTaskState, note string) (map[string]any, string) {
	now := time.Now().UTC()
	fields := map[string]any{}
	gapExplanation := ""
	switch target {
	case constants.TaskRunning:
		fields["started_at"] = gormExprNow(before.StartedAt, now)
		if note != "" {
			fields["execution_note"] = appendNote(before.ExecutionNote, note)
		}
	case constants.TaskVerifying:
		fields["submitted_at"] = now
		fields["execution_note"] = appendNote(before.ExecutionNote, note)
	case constants.TaskDone:
		fields["completed_at"] = now
		fields["review_note"] = appendNote(before.ReviewNote, note)
		gapExplanation = truncateExplanation(fmt.Sprintf("%s 补测完成：复核人确认补测执行单 #%d，%s", strings.TrimSpace(gapBefore.Explanation), before.ID, note))
	case constants.TaskCanceled:
		fields["canceled_at"] = now
		fields["cancel_reason"] = appendNote(before.CancelReason, note)
		gapExplanation = truncateExplanation(fmt.Sprintf("%s 补测取消：补测执行单 #%d 被取消，%s", strings.TrimSpace(gapBefore.Explanation), before.ID, note))
	}
	return fields, gapExplanation
}

// gormExprNow 保留既有时间戳（如复验退回执行中时不覆盖首次开始时间）。
func gormExprNow(existing *time.Time, now time.Time) any {
	if existing != nil {
		return *existing
	}
	return now
}

func appendNote(existing, note string) string {
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return note
	}
	if note == "" {
		return existing
	}
	return existing + " | " + note
}

func dayFloor(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// actorMayTransition：执行类动作限管理员与数据处理员；复验确认、退回和取消限管理员与复核人。
func actorMayTransition(role string, target constants.ResurveyTaskState) bool {
	switch target {
	case constants.TaskRunning, constants.TaskVerifying:
		return role == constants.RoleAdmin || role == constants.RoleDataProcessor
	case constants.TaskDone, constants.TaskCanceled:
		return role == constants.RoleAdmin || role == constants.RoleReviewer
	default:
		return false
	}
}

// truncateExplanation 将缺口说明保持在 coverage_gaps.explanation(1200) 长度以内。
func truncateExplanation(value string) string {
	runes := []rune(value)
	if len(runes) <= 1180 {
		return value
	}
	return string(runes[:1180]) + "…"
}
