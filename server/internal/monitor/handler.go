package monitor

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lixiangling-com/pulsewatch/server/internal/auth"
	api "github.com/lixiangling-com/pulsewatch/server/internal/gen/api"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/httpx"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

func RegisterRoutes(routes *gin.RouterGroup, handler *Handler) {
	routes.GET("", handler.List)
	routes.POST("", handler.Create)
	routes.GET("/:id", handler.Get)
	routes.PATCH("/:id", handler.Update)
	routes.DELETE("/:id", handler.Delete)
	routes.POST("/:id/pause", handler.Pause)
	routes.POST("/:id/resume", handler.Resume)
}

func (h *Handler) List(c *gin.Context) {
	userID, ok := authenticatedUser(c)
	if !ok {
		h.writeUnauthorized(c, "list")
		return
	}
	page, ok := queryInteger(c, "page", 1)
	if !ok {
		h.writeError(c, userID, uuid.Nil, "list", &ValidationError{Fields: map[string][]string{"page": {"页码必须是正整数"}}})
		return
	}
	pageSize, ok := queryInteger(c, "page_size", 20)
	if !ok {
		h.writeError(c, userID, uuid.Nil, "list", &ValidationError{Fields: map[string][]string{"page_size": {"每页数量必须是正整数"}}})
		return
	}
	result, err := h.service.List(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		h.writeError(c, userID, uuid.Nil, "list", err)
		return
	}
	items := make([]api.Monitor, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toResponse(item))
	}
	h.log(c, userID, uuid.Nil, "list", "success")
	c.JSON(http.StatusOK, api.MonitorListResponse{
		Items: items,
		Meta:  api.PaginationMeta{Page: result.Number, PageSize: result.Size, Total: int(result.Total)},
	})
}

func (h *Handler) Create(c *gin.Context) {
	userID, ok := authenticatedUser(c)
	if !ok {
		h.writeUnauthorized(c, "create")
		return
	}
	var request api.CreateMonitorRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		h.writeError(c, userID, uuid.Nil, "create", &ValidationError{Fields: map[string][]string{"body": {"请求 JSON 无效或包含未知字段"}}})
		return
	}
	created, err := h.service.Create(c.Request.Context(), userID, CreateInput{
		Name: request.Name, URL: request.Url, IntervalMinutes: int(request.IntervalMinutes), ExpectedStatus: request.ExpectedStatus,
	})
	if err != nil {
		h.writeError(c, userID, uuid.Nil, "create", err)
		return
	}
	h.log(c, userID, created.ID, "create", "success")
	c.JSON(http.StatusCreated, toResponse(created))
}

func (h *Handler) Get(c *gin.Context) {
	h.withID(c, "get", func(userID, id uuid.UUID) (Monitor, error) {
		return h.service.Get(c.Request.Context(), userID, id)
	})
}

func (h *Handler) Update(c *gin.Context) {
	userID, id, ok := h.userAndID(c, "update")
	if !ok {
		return
	}
	var request api.UpdateMonitorRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		h.writeError(c, userID, id, "update", &ValidationError{Fields: map[string][]string{"body": {"请求 JSON 无效或包含未知字段"}}})
		return
	}
	var interval *int
	if request.IntervalMinutes != nil {
		value := int(*request.IntervalMinutes)
		interval = &value
	}
	updated, err := h.service.Update(c.Request.Context(), userID, id, UpdateInput{
		Name: request.Name, URL: request.Url, IntervalMinutes: interval, ExpectedStatus: request.ExpectedStatus,
	})
	if err != nil {
		h.writeError(c, userID, id, "update", err)
		return
	}
	h.log(c, userID, id, "update", "success")
	c.JSON(http.StatusOK, toResponse(updated))
}

func (h *Handler) Pause(c *gin.Context) {
	h.withID(c, "pause", func(userID, id uuid.UUID) (Monitor, error) {
		return h.service.Pause(c.Request.Context(), userID, id)
	})
}

func (h *Handler) Resume(c *gin.Context) {
	h.withID(c, "resume", func(userID, id uuid.UUID) (Monitor, error) {
		return h.service.Resume(c.Request.Context(), userID, id)
	})
}

func (h *Handler) Delete(c *gin.Context) {
	userID, id, ok := h.userAndID(c, "delete")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), userID, id); err != nil {
		h.writeError(c, userID, id, "delete", err)
		return
	}
	h.log(c, userID, id, "delete", "success")
	c.Status(http.StatusNoContent)
}

func (h *Handler) withID(c *gin.Context, operation string, action func(uuid.UUID, uuid.UUID) (Monitor, error)) {
	userID, id, ok := h.userAndID(c, operation)
	if !ok {
		return
	}
	result, err := action(userID, id)
	if err != nil {
		h.writeError(c, userID, id, operation, err)
		return
	}
	h.log(c, userID, id, operation, "success")
	c.JSON(http.StatusOK, toResponse(result))
}

func (h *Handler) userAndID(c *gin.Context, operation string) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := authenticatedUser(c)
	if !ok {
		h.writeUnauthorized(c, operation)
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.writeError(c, userID, uuid.Nil, operation, ErrNotFound)
		return uuid.Nil, uuid.Nil, false
	}
	return userID, id, true
}

func authenticatedUser(c *gin.Context) (uuid.UUID, bool) {
	return auth.UserIDFromContext(c)
}

func (h *Handler) writeUnauthorized(c *gin.Context, operation string) {
	h.log(c, uuid.Nil, uuid.Nil, operation, "unauthorized")
	httpx.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "认证已失效，请重新登录", nil)
}

func (h *Handler) writeError(c *gin.Context, userID, monitorID uuid.UUID, operation string, err error) {
	var invalid *ValidationError
	switch {
	case errors.As(err, &invalid):
		h.log(c, userID, monitorID, operation, "validation_error")
		httpx.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请检查输入", invalid.Fields)
	case errors.Is(err, ErrLimit):
		h.log(c, userID, monitorID, operation, "limit_reached")
		httpx.WriteError(c, http.StatusConflict, "MONITOR_LIMIT_REACHED", "每个用户最多创建 20 个监控", nil)
	case errors.Is(err, ErrNotFound):
		h.log(c, userID, monitorID, operation, "not_found")
		httpx.WriteError(c, http.StatusNotFound, "MONITOR_NOT_FOUND", "监控不存在", nil)
	default:
		h.log(c, userID, monitorID, operation, "internal_error")
		httpx.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用", nil)
	}
}

func (h *Handler) log(c *gin.Context, userID, monitorID uuid.UUID, operation, result string) {
	h.logger.Info("monitor operation",
		slog.String("request_id", c.GetString(httpx.RequestIDKey)),
		slog.String("user_id", userID.String()),
		slog.String("monitor_id", monitorID.String()),
		slog.String("operation", operation),
		slog.String("result", result),
	)
}

func decodeStrictJSON(c *gin.Context, destination any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func queryInteger(c *gin.Context, name string, defaultValue int) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return defaultValue, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil && value > 0
}

func toResponse(value Monitor) api.Monitor {
	response := api.Monitor{
		Id: value.ID, UserId: value.UserID, Name: value.Name, Url: value.URL,
		IntervalMinutes: api.MonitorIntervalMinutes(value.IntervalMinutes),
		ExpectedStatus:  value.ExpectedStatus, Status: api.MonitorStatus(value.Status),
		ConfigVersion: value.ConfigVersion, NextCheckAt: value.NextCheckAt,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
	response.LastCheckedAt = value.LastCheckedAt
	if value.LastLatencyMS != nil {
		latency := int(*value.LastLatencyMS)
		response.LastLatencyMs = &latency
	}
	return response
}
