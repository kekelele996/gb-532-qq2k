package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sonar-survey-coverage-planner/backend/internal/constants"
	"sonar-survey-coverage-planner/backend/internal/dto"
	"sonar-survey-coverage-planner/backend/internal/service"
	"sonar-survey-coverage-planner/backend/pkg/api"
)

type ResurveyTaskHandler struct{ service *service.ResurveyTaskService }

func NewResurveyTaskHandler(service *service.ResurveyTaskService) *ResurveyTaskHandler {
	return &ResurveyTaskHandler{service: service}
}

func (h *ResurveyTaskHandler) List(c *gin.Context) {
	page, size := pageQuery(c)
	items, total, err := h.service.List(dto.ResurveyTaskQuery{
		CoverageGapID: uintQuery(c, "coverage_gap_id"),
		State:         cleanQuery(c, "state"),
		OpenOnly:      c.Query("open_only") == "true",
		Page:          page,
		PageSize:      size,
	})
	if err != nil {
		writeServiceError(c, err, "补测执行单")
		return
	}
	api.Page(c, items, page, size, total)
}

func (h *ResurveyTaskHandler) Get(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	item, err := h.service.Get(id)
	if err != nil {
		writeServiceError(c, err, "补测执行单")
		return
	}
	api.Success(c, http.StatusOK, item)
}

func (h *ResurveyTaskHandler) Create(c *gin.Context) {
	var request dto.CreateResurveyTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		api.BindError(c, err)
		return
	}
	view, err := h.service.Create(request, actorFrom(c))
	if err != nil {
		writeServiceError(c, err, "补测执行单")
		return
	}
	api.Success(c, http.StatusCreated, view)
}

func (h *ResurveyTaskHandler) Transition(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var request dto.ResurveyTaskTransitionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		api.BindError(c, err)
		return
	}
	if !h.allowedTransition(c, request.TargetState) {
		return
	}
	view, err := h.service.Transition(id, request, actorFrom(c))
	if err != nil {
		writeServiceError(c, err, "补测执行单")
		return
	}
	api.Success(c, http.StatusOK, view)
}

// allowedTransition 在路由 RBAC 之上做按目标状态的细粒度权限校验。
func (h *ResurveyTaskHandler) allowedTransition(c *gin.Context, target string) bool {
	role := c.GetString("role")
	executors := map[string]struct{}{constants.RoleAdmin: {}, constants.RoleDataProcessor: {}}
	reviewers := map[string]struct{}{constants.RoleAdmin: {}, constants.RoleReviewer: {}}
	allowed := false
	switch target {
	case "running", "awaiting_review":
		_, allowed = executors[role]
	case "completed":
		_, allowed = reviewers[role]
	case "canceled":
		_, allowed = reviewers[role]
	}
	if !allowed {
		api.WriteError(c, api.Forbidden("当前角色无权将补测单推进到该状态"))
		return false
	}
	return true
}
