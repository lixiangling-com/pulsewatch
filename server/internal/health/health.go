package health

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/httpx"
)

type Pinger interface {
	Ping(context.Context) error
}

type DependencyStatus struct {
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

type Response struct {
	Status       string                      `json:"status"`
	Service      string                      `json:"service"`
	Timestamp    time.Time                   `json:"timestamp"`
	RequestID    string                      `json:"request_id"`
	Dependencies map[string]DependencyStatus `json:"dependencies"`
}

type Checker struct {
	service  string
	postgres Pinger
	redis    Pinger
	timeout  time.Duration
	draining func() bool
}

func NewChecker(service string, postgres, redis Pinger, timeout time.Duration, draining func() bool) *Checker {
	return &Checker{service: service, postgres: postgres, redis: redis, timeout: timeout, draining: draining}
}

type Handler struct {
	checker *Checker
}

func NewHandler(checker *Checker) *Handler { return &Handler{checker: checker} }

func (h *Handler) Live(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, Response{
		Status:       "ok",
		Service:      h.checker.service,
		Timestamp:    time.Now().UTC(),
		RequestID:    requestID(c),
		Dependencies: map[string]DependencyStatus{},
	})
}

func (h *Handler) Ready(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	result, ready := h.checker.Check(c.Request.Context())
	result.RequestID = requestID(c)
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, result)
}

func (c *Checker) Check(parent context.Context) (Response, bool) {
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	defer cancel()

	result := Response{
		Status:       "ready",
		Service:      c.service,
		Timestamp:    time.Now().UTC(),
		Dependencies: make(map[string]DependencyStatus, 2),
	}
	ready := true
	type outcome struct {
		name   string
		status DependencyStatus
	}
	results := make(chan outcome, 2)

	check := func(name string, pinger Pinger) {
		started := time.Now()
		if pinger == nil {
			results <- outcome{name: name, status: DependencyStatus{Status: "down", ErrorCode: "unavailable"}}
			return
		}
		err := pinger.Ping(ctx)
		status := DependencyStatus{Status: "up", LatencyMS: time.Since(started).Milliseconds()}
		if err != nil {
			status.Status = "down"
			status.ErrorCode = classifyError(err, ctx)
			status.LatencyMS = 0
		}
		results <- outcome{name: name, status: status}
	}

	go check("postgres", c.postgres)
	go check("redis", c.redis)
	seen := make(map[string]struct{}, 2)
	for len(seen) < 2 {
		select {
		case outcome := <-results:
			seen[outcome.name] = struct{}{}
			result.Dependencies[outcome.name] = outcome.status
			if outcome.status.Status != "up" {
				ready = false
			}
		case <-ctx.Done():
			// A driver should honor ctx, but the checker must still honor its
			// own deadline if a third-party implementation does not.
			for _, name := range []string{"postgres", "redis"} {
				if _, ok := seen[name]; ok {
					continue
				}
				result.Dependencies[name] = DependencyStatus{Status: "down", ErrorCode: "timeout"}
			}
			ready = false
			goto done
		}
	}

done:
	if c.draining != nil && c.draining() {
		ready = false
	}
	if !ready {
		result.Status = "not_ready"
	}
	return result, ready
}

func classifyError(err error, ctx context.Context) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout"
	}
	return "unavailable"
}

func requestID(c *gin.Context) string {
	if value, ok := c.Get(httpx.RequestIDKey); ok {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return c.GetHeader("X-Request-ID")
}
