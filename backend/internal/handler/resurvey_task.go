package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
	items, total, err := h.service.List(dto.ResurveyTaskQuery{CoverageGapID: uintQuery(c, "coverage_gap_id"), State: cleanQuery(c, "state"), Page: page, PageSize: size})
	if err != nil {
		writeServiceError(c, err, "补测执行单")
		return
	}
	api.Page(c, items, page, size, total)
}

func (h *ResurveyTaskHandler) Create(c *gin.Context) {
	var request dto.CreateResurveyTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		api.BindError(c, err)
		return
	}
	item, err := h.service.Create(request, actorFrom(c))
	if err != nil {
		writeServiceError(c, err, "补测执行单")
		return
	}
	api.Success(c, http.StatusCreated, item)
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
	item, err := h.service.Transition(id, request, actorFrom(c))
	if err != nil {
		writeServiceError(c, err, "补测执行单")
		return
	}
	api.Success(c, http.StatusOK, item)
}
