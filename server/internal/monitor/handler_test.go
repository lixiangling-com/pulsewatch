package monitor

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lixiangling-com/pulsewatch/server/internal/auth"
	api "github.com/lixiangling-com/pulsewatch/server/internal/gen/api"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/httpx"
)

func TestHandlerLifecycleAndStableErrors(t *testing.T) {
	userID := uuid.New()
	router := newMonitorTestRouter(userID, NewService(newMemoryStore()))

	created := performRequest(router, http.MethodPost, "/api/v1/monitors", `{"name":"API","url":"https://example.com/health","interval_minutes":5,"expected_status":200}`)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"status":"pending"`) || !strings.Contains(created.Body.String(), `"config_version":1`) {
		t.Fatalf("create response = %d %s", created.Code, created.Body.String())
	}
	createdMonitor := decodeMonitorResponse(t, created)

	listed := performRequest(router, http.MethodGet, "/api/v1/monitors", "")
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"page":1`) || !strings.Contains(listed.Body.String(), `"page_size":20`) || !strings.Contains(listed.Body.String(), `"total":1`) {
		t.Fatalf("list response = %d %s", listed.Code, listed.Body.String())
	}

	unknown := performRequest(router, http.MethodPost, "/api/v1/monitors", `{"name":"API","url":"https://example.com","interval_minutes":5,"expected_status":200,"secret":true}`)
	if unknown.Code != http.StatusUnprocessableEntity || !strings.Contains(unknown.Body.String(), `"code":"VALIDATION_ERROR"`) {
		t.Fatalf("unknown-field response = %d %s", unknown.Code, unknown.Body.String())
	}

	badPage := performRequest(router, http.MethodGet, "/api/v1/monitors?page_size=101", "")
	if badPage.Code != http.StatusUnprocessableEntity || !strings.Contains(badPage.Body.String(), `"VALIDATION_ERROR"`) {
		t.Fatalf("bad-page response = %d %s", badPage.Code, badPage.Body.String())
	}

	missing := performRequest(router, http.MethodGet, "/api/v1/monitors/"+uuid.NewString(), "")
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), `"code":"MONITOR_NOT_FOUND"`) {
		t.Fatalf("missing response = %d %s", missing.Code, missing.Body.String())
	}

	path := "/api/v1/monitors/" + createdMonitor.Id.String()
	updated := performRequest(router, http.MethodPatch, path, `{"url":"https://example.org/ready"}`)
	if value := decodeMonitorResponse(t, updated); updated.Code != http.StatusOK || value.ConfigVersion != 2 || value.Status != api.MonitorStatus("pending") {
		t.Fatalf("update response = %d %s", updated.Code, updated.Body.String())
	}
	paused := performRequest(router, http.MethodPost, path+"/pause", "")
	if value := decodeMonitorResponse(t, paused); paused.Code != http.StatusOK || value.ConfigVersion != 3 || value.Status != api.MonitorStatus("paused") {
		t.Fatalf("pause response = %d %s", paused.Code, paused.Body.String())
	}
	repeatedPause := performRequest(router, http.MethodPost, path+"/pause", "")
	if value := decodeMonitorResponse(t, repeatedPause); value.ConfigVersion != 3 {
		t.Fatalf("repeated pause response = %d %s", repeatedPause.Code, repeatedPause.Body.String())
	}
	resumed := performRequest(router, http.MethodPost, path+"/resume", "")
	if value := decodeMonitorResponse(t, resumed); resumed.Code != http.StatusOK || value.ConfigVersion != 4 || value.Status != api.MonitorStatus("pending") {
		t.Fatalf("resume response = %d %s", resumed.Code, resumed.Body.String())
	}
	deleted := performRequest(router, http.MethodDelete, path, "")
	if deleted.Code != http.StatusNoContent || deleted.Body.Len() != 0 {
		t.Fatalf("delete response = %d %s", deleted.Code, deleted.Body.String())
	}
	repeatedDelete := performRequest(router, http.MethodDelete, path, "")
	if repeatedDelete.Code != http.StatusNotFound {
		t.Fatalf("repeated delete response = %d %s", repeatedDelete.Code, repeatedDelete.Body.String())
	}
}

func TestHandlerRequiresUserAndRejectsEmptyPatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewService(newMemoryStore())
	handler := NewHandler(service, logger)

	unauthorizedRouter := gin.New()
	unauthorizedRouter.Use(httpx.RequestID())
	RegisterRoutes(unauthorizedRouter.Group("/api/v1/monitors"), handler)
	unauthorized := performRequest(unauthorizedRouter, http.MethodGet, "/api/v1/monitors", "")
	if unauthorized.Code != http.StatusUnauthorized || !strings.Contains(unauthorized.Body.String(), `"code":"UNAUTHORIZED"`) {
		t.Fatalf("unauthorized response = %d %s", unauthorized.Code, unauthorized.Body.String())
	}

	userID := uuid.New()
	router := newMonitorTestRouter(userID, service)
	created, err := service.Create(t.Context(), userID, validCreate("API"))
	if err != nil {
		t.Fatal(err)
	}
	empty := performRequest(router, http.MethodPatch, "/api/v1/monitors/"+created.ID.String(), `{}`)
	if empty.Code != http.StatusUnprocessableEntity || !strings.Contains(empty.Body.String(), `"body"`) {
		t.Fatalf("empty patch response = %d %s", empty.Code, empty.Body.String())
	}
}

func TestHandlerMapsLimitAndDoesNotLogURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	store := newMemoryStore()
	service := NewService(store)
	for index := 0; index < MaxPerUser; index++ {
		if _, err := service.Create(t.Context(), userID, validCreate("Monitor")); err != nil {
			t.Fatal(err)
		}
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := gin.New()
	router.Use(httpx.RequestID(), func(c *gin.Context) {
		c.Set(auth.UserIDContextKey, userID)
		c.Next()
	})
	RegisterRoutes(router.Group("/api/v1/monitors"), NewHandler(service, logger))
	response := performRequest(router, http.MethodPost, "/api/v1/monitors", `{"name":"Private","url":"https://example.com/health?token=do-not-log","interval_minutes":5,"expected_status":200}`)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"MONITOR_LIMIT_REACHED"`) {
		t.Fatalf("limit response = %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(logs.String(), "do-not-log") || !strings.Contains(logs.String(), `"operation":"create"`) || !strings.Contains(logs.String(), `"result":"limit_reached"`) {
		t.Fatalf("monitor log = %s", logs.String())
	}
}

func newMonitorTestRouter(userID uuid.UUID, service *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := gin.New()
	router.Use(httpx.RequestID(), func(c *gin.Context) {
		c.Set(auth.UserIDContextKey, userID)
		c.Next()
	})
	RegisterRoutes(router.Group("/api/v1/monitors"), NewHandler(service, logger))
	return router
}

func performRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-Request-ID", "monitor-test")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeMonitorResponse(t *testing.T, recorder *httptest.ResponseRecorder) api.Monitor {
	t.Helper()
	var response api.Monitor
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode monitor response %q: %v", recorder.Body.String(), err)
	}
	return response
}
