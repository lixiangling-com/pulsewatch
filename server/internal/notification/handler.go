package notification

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lixiangling-com/pulsewatch/server/db/sqlc"
	"github.com/lixiangling-com/pulsewatch/server/internal/auth"
	api "github.com/lixiangling-com/pulsewatch/server/internal/gen/api"
)

type Handler struct{ queries *sqlc.Queries }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{queries: sqlc.New(pool)} }
func RegisterRoutes(router *gin.RouterGroup, h *Handler) {
	router.GET("/notifications", h.ListNotifications)
	router.PATCH("/notifications/:id/read", h.MarkRead)
}
func RegisterMonitorRoutes(router *gin.RouterGroup, h *Handler) {
	router.GET("/:id/incidents", h.ListIncidents)
}
func (h *Handler) ListNotifications(c *gin.Context) {
	user, ok := auth.UserIDFromContext(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	page, size := pageParams(c)
	rows, err := h.queries.ListNotificationsByUser(c, sqlc.ListNotificationsByUserParams{UserID: pgUUID(user), PageOffset: int32((page - 1) * size), PageSize: int32(size)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR"})
		return
	}
	items := make([]api.Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, notificationResponse(row))
	}
	count, err := h.queries.CountNotificationsByUser(c, pgUUID(user))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "meta": api.PaginationMeta{Page: page, PageSize: size, Total: int(count)}})
}
func (h *Handler) MarkRead(c *gin.Context) {
	user, ok := auth.UserIDFromContext(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	row, err := h.queries.MarkNotificationReadByUser(c, sqlc.MarkNotificationReadByUserParams{ID: pgUUID(id), UserID: pgUUID(user)})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR"})
		return
	}
	c.JSON(http.StatusOK, notificationResponse(row))
}
func (h *Handler) ListIncidents(c *gin.Context) {
	user, ok := auth.UserIDFromContext(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	monitorID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	if _, err := h.queries.GetMonitorByIDAndUser(c, sqlc.GetMonitorByIDAndUserParams{ID: pgUUID(monitorID), UserID: pgUUID(user)}); errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR"})
		return
	}
	page, size := pageParams(c)
	rows, err := h.queries.ListIncidentsByMonitorAndUser(c, sqlc.ListIncidentsByMonitorAndUserParams{MonitorID: pgUUID(monitorID), UserID: pgUUID(user), PageOffset: int32((page - 1) * size), PageSize: int32(size)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR"})
		return
	}
	count, err := h.queries.CountIncidentsByMonitorAndUser(c, sqlc.CountIncidentsByMonitorAndUserParams{MonitorID: pgUUID(monitorID), UserID: pgUUID(user)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR"})
		return
	}
	items := make([]api.Incident, 0, len(rows))
	for _, row := range rows {
		var resolved *time.Time
		if row.ResolvedAt.Valid {
			value := row.ResolvedAt.Time
			resolved = &value
		}
		items = append(items, api.Incident{Id: row.ID.Bytes, MonitorId: row.MonitorID.Bytes, OpenedAt: row.OpenedAt.Time, ResolvedAt: resolved, ErrorCode: row.ErrorCode, ErrorSummary: optionalText(row.ErrorSummary), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time})
	}
	c.JSON(http.StatusOK, api.IncidentListResponse{Items: items, Meta: api.PaginationMeta{Page: page, PageSize: size, Total: int(count)}})
}
func pageParams(c *gin.Context) (int, int) {
	page, size := 1, 20
	if v, err := strconv.Atoi(c.Query("page")); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("page_size")); err == nil && v > 0 && v <= 100 {
		size = v
	}
	return page, size
}
func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
func optionalTime(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	value := v.Time
	return &value
}
func optionalText(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	value := v.String
	return &value
}
func notificationResponse(row sqlc.Notification) api.Notification {
	return api.Notification{Id: row.ID.Bytes, UserId: row.UserID.Bytes, MonitorId: row.MonitorID.Bytes, IncidentId: row.IncidentID.Bytes, Type: api.NotificationType(row.Type), DedupeKey: row.DedupeKey, Title: row.Title, Body: row.Body, Status: api.NotificationStatus(row.Status), ReadAt: optionalTime(row.ReadAt), QueuedAt: optionalTime(row.QueuedAt), SentAt: optionalTime(row.SentAt), ErrorCode: optionalText(row.ErrorCode), ErrorSummary: optionalText(row.ErrorSummary), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}
