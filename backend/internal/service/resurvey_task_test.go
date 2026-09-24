package service

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sonar-survey-coverage-planner/backend/internal/constants"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/model"
	"sonar-survey-coverage-planner/backend/internal/repository"
	"sonar-survey-coverage-planner/backend/pkg/api"
)

var fixtureSeq int64

func newTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	seq := atomic.AddInt64(&fixtureSeq, 1)
	dsn := fmt.Sprintf("file:mem-%d?mode=memory&cache=shared", seq)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.SurveyArea{}, &model.CoverageGap{}, &model.ResurveyTask{}, &model.AuditEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_resurvey_task_open
		ON resurvey_tasks (coverage_gap_id)
		WHERE task_state IN ('pending', 'running', 'awaiting_review')`).Error; err != nil {
		t.Fatalf("partial index: %v", err)
	}
	return db
}

func newTaskFixture(t *testing.T, state string) (*ResurveyTaskService, *repository.ResurveyTaskRepository, model.CoverageGap) {
	t.Helper()
	db := newTaskTestDB(t)
	user := model.User{Username: "tester-" + state, DisplayName: "测试员", Role: constants.RoleReviewer, PasswordHash: "x", Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	area := model.SurveyArea{AreaCode: "AREA-T1", Name: "测试测区", BoundaryGeoJSON: []byte(`{"type":"Feature","geometry":{"type":"Polygon","coordinates":[]}}`), TargetResolutionM: 20, CoordinateSystem: "EPSG:32650", DefaultSwathM: 180, OwnerTeam: "team", Status: constants.AreaActive, Version: 1}
	if err := db.Create(&area).Error; err != nil {
		t.Fatalf("create area: %v", err)
	}
	gap := model.CoverageGap{SurveyAreaID: area.ID, SourceRunIDs: []byte("[1]"), GapGeoJSON: []byte(`{"type":"Feature","geometry":{"type":"Polygon","coordinates":[]}}`), Severity: string(constants.SeverityMajor), RecommendedLineGeoJSON: []byte(`{"type":"Feature","geometry":{"type":"LineString","coordinates":[]}}`), AlgorithmVersion: "grid-cover-v1.0.0", InputHash: "hash-" + state, GapState: state, Explanation: "基准说明", Version: 1, DetectedAt: time.Now().UTC()}
	if err := db.Create(&gap).Error; err != nil {
		t.Fatalf("create gap: %v", err)
	}
	gapRepo := repository.NewCoverageGapRepository(db)
	taskRepo := repository.NewResurveyTaskRepository(db)
	audit := NewAuditService(repository.NewSupportRepository(db))
	return NewResurveyTaskService(taskRepo, gapRepo, audit), taskRepo, gap
}

func createTaskRequest(gapID, version uint) dto.CreateResurveyTaskRequest {
	return dto.CreateResurveyTaskRequest{CoverageGapID: gapID, AssigneeName: "李补测", PlannedDate: time.Now().UTC().Format("2006-01-02"), ExpectedGapVer: version}
}

func TestResurveyTaskFullLifecycle(t *testing.T) {
	service, _, gap := newTaskFixture(t, string(constants.GapAccepted))
	actor := Actor{RequestID: "req-1", UserID: 1, Username: "reviewer", Role: constants.RoleReviewer}

	created, err := service.Create(createTaskRequest(gap.ID, gap.Version), actor)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if created.Task.TaskState != string(constants.TaskPending) || created.Gap.GapState != string(constants.GapRetesting) {
		t.Fatalf("unexpected states after create task=%s gap=%s", created.Task.TaskState, created.Gap.GapState)
	}

	// 同一缺口不能有第二张未结束任务：缺口已进入 retesting，建单被 409 拒绝；
	// 部分唯一索引与事务再为并发点击兜底（见 TestResurveyTaskOpenIndexRace）。
	if _, err := service.Create(createTaskRequest(gap.ID, created.Gap.Version), actor); err == nil {
		t.Fatal("second open task must be rejected")
	} else if appErr, ok := err.(*api.AppError); !ok || appErr.Status != 409 {
		t.Fatalf("expected 409 conflict, got %v", err)
	}

	// 旧页面携带建单前的 gap 版本也会撞版本，但此时已经被上面的 open 检查先拦下；
	// 这里直接验证陈旧 task 版本提交推进返回 409。
	stale := dto.ResurveyTaskTransitionRequest{TargetState: "running", ExpectedVersion: 999}
	if _, err := service.Transition(created.Task.ID, stale, actor); err == nil {
		t.Fatal("stale task version must conflict")
	}

	execActor := Actor{RequestID: "req-2", UserID: 2, Username: "processor", Role: constants.RoleDataProcessor}
	running, err := service.Transition(created.Task.ID, dto.ResurveyTaskTransitionRequest{TargetState: "running", ExpectedVersion: created.Task.Version, Note: "开始作业"}, execActor)
	if err != nil {
		t.Fatalf("start task: %v", err)
	}
	if running.Gap.GapState != string(constants.GapRetesting) {
		t.Fatalf("gap should stay retesting while task runs, got %s", running.Gap.GapState)
	}
	submitted, err := service.Transition(created.Task.ID, dto.ResurveyTaskTransitionRequest{TargetState: "awaiting_review", ExpectedVersion: running.Task.Version, Note: "补测航迹已回传"}, execActor)
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// data_processor 无权确认完成。
	if _, err := service.Transition(created.Task.ID, dto.ResurveyTaskTransitionRequest{TargetState: "completed", ExpectedVersion: submitted.Task.Version, Note: "数据合格确认"}, execActor); err == nil {
		t.Fatal("processor must not complete re-verification")
	}
	done, err := service.Transition(created.Task.ID, dto.ResurveyTaskTransitionRequest{TargetState: "completed", ExpectedVersion: submitted.Task.Version, Note: "数据合格确认"}, actor)
	if err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if done.Task.TaskState != string(constants.TaskDone) || done.Gap.GapState != string(constants.GapResurveyed) {
		t.Fatalf("after completion task=%s gap=%s", done.Task.TaskState, done.Gap.GapState)
	}
}

func TestResurveyTaskCancelReturnsGapToAccepted(t *testing.T) {
	service, _, gap := newTaskFixture(t, string(constants.GapAccepted))
	actor := Actor{RequestID: "req-3", UserID: 1, Username: "reviewer", Role: constants.RoleReviewer}
	created, err := service.Create(createTaskRequest(gap.ID, gap.Version), actor)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	canceled, err := service.Transition(created.Task.ID, dto.ResurveyTaskTransitionRequest{TargetState: "canceled", ExpectedVersion: created.Task.Version, Note: "海况突变取消"}, actor)
	if err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	if canceled.Task.TaskState != string(constants.TaskCanceled) || canceled.Gap.GapState != string(constants.GapAccepted) {
		t.Fatalf("after cancel task=%s gap=%s", canceled.Task.TaskState, canceled.Gap.GapState)
	}
	// 缺口回到 accepted 后允许重新建单。
	recreated, err := service.Create(createTaskRequest(gap.ID, canceled.Gap.Version), actor)
	if err != nil {
		t.Fatalf("recreate task after cancellation: %v", err)
	}
	if recreated.Task.TaskState != string(constants.TaskPending) {
		t.Fatalf("new task should be pending, got %s", recreated.Task.TaskState)
	}
}

func TestResurveyTaskCreateGuards(t *testing.T) {
	// 非 accepted 缺口禁止建单。
	service, _, detectedGap := newTaskFixture(t, string(constants.GapDetected))
	actor := Actor{RequestID: "req-4", UserID: 1, Username: "reviewer", Role: constants.RoleReviewer}
	if _, err := service.Create(createTaskRequest(detectedGap.ID, detectedGap.Version), actor); err == nil {
		t.Fatal("detected gap must not accept a resurvey task")
	}

	// 陈旧缺口版本（旧页面）提交建单必须 409。
	service2, _, acceptedGap := newTaskFixture(t, string(constants.GapAccepted))
	request := createTaskRequest(acceptedGap.ID, acceptedGap.Version+5)
	if _, err := service2.Create(request, actor); err == nil {
		t.Fatal("stale gap version must conflict")
	}

	// 过去的计划日期被拒绝。
	past := createTaskRequest(acceptedGap.ID, acceptedGap.Version)
	past.PlannedDate = time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	if _, err := service2.Create(past, actor); err == nil {
		t.Fatal("past planned date must be rejected")
	}
}

func TestResurveyTaskInvalidTransitions(t *testing.T) {
	service, _, gap := newTaskFixture(t, string(constants.GapAccepted))
	actor := Actor{RequestID: "req-5", UserID: 1, Username: "reviewer", Role: constants.RoleReviewer}
	created, err := service.Create(createTaskRequest(gap.ID, gap.Version), actor)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	// pending 直接 completed 属于非法迁移。
	if _, err := service.Transition(created.Task.ID, dto.ResurveyTaskTransitionRequest{TargetState: "completed", ExpectedVersion: created.Task.Version, Note: "跳过流程确认"}, actor); err == nil {
		t.Fatal("pending -> completed must be rejected")
	}
	// 取消/完成必须附 4 字以上说明：先推进到待复验。
	exec := Actor{RequestID: "req-6", UserID: 2, Username: "processor", Role: constants.RoleDataProcessor}
	running, err := service.Transition(created.Task.ID, dto.ResurveyTaskTransitionRequest{TargetState: "running", ExpectedVersion: created.Task.Version}, exec)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	submitted, err := service.Transition(created.Task.ID, dto.ResurveyTaskTransitionRequest{TargetState: "awaiting_review", ExpectedVersion: running.Task.Version}, exec)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := service.Transition(created.Task.ID, dto.ResurveyTaskTransitionRequest{TargetState: "completed", ExpectedVersion: submitted.Task.Version, Note: "不足"}, actor); err == nil {
		t.Fatal("completion note shorter than 4 runes must be rejected")
	}
}

// TestOpenTaskPartialIndexEnforced 直接验证数据库部分唯一索引：
// 同一缺口两张未结束任务必须被数据库拒绝，结束后可再建。
func TestOpenTaskPartialIndexEnforced(t *testing.T) {
	_, repo, gap := newTaskFixture(t, string(constants.GapAccepted))
	first := model.ResurveyTask{CoverageGapID: gap.ID, AssigneeName: "甲", PlannedDate: time.Now().UTC(), TaskState: string(constants.TaskPending), CreatedBy: 1, Version: 1}
	if err := repo.Transaction(func(tx *gorm.DB) error { return tx.Create(&first).Error }); err != nil {
		t.Fatalf("first open task: %v", err)
	}
	second := model.ResurveyTask{CoverageGapID: gap.ID, AssigneeName: "乙", PlannedDate: time.Now().UTC(), TaskState: string(constants.TaskRunning), CreatedBy: 1, Version: 1}
	if err := repo.Transaction(func(tx *gorm.DB) error { return tx.Create(&second).Error }); err == nil {
		t.Fatal("database must reject two open resurvey tasks for one gap")
	}
	if err := repo.Transaction(func(tx *gorm.DB) error {
		return tx.Model(&model.ResurveyTask{}).Where("id = ?", first.ID).Update("task_state", string(constants.TaskDone)).Error
	}); err != nil {
		t.Fatalf("finish first: %v", err)
	}
	third := model.ResurveyTask{CoverageGapID: gap.ID, AssigneeName: "丙", PlannedDate: time.Now().UTC(), TaskState: string(constants.TaskPending), CreatedBy: 1, Version: 1}
	if err := repo.Transaction(func(tx *gorm.DB) error { return tx.Create(&third).Error }); err != nil {
		t.Fatalf("new open task after previous one finished should be allowed: %v", err)
	}
}
