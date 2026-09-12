package health

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/httpx"
)

type fakePinger struct {
	err   error
	delay time.Duration
}

func (f fakePinger) Ping(ctx context.Context) error {
	if f.delay > 0 {
		timer := time.NewTimer(f.delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.err
}

func TestCheckerReturnsReadyWithParallelDependencies(t *testing.T) {
	started := time.Now()
	checker := NewChecker("api", fakePinger{delay: 30 * time.Millisecond}, fakePinger{delay: 30 * time.Millisecond}, time.Second, nil)

	result, ready := checker.Check(context.Background())

	if !ready || result.Status != "ready" {
		t.Fatalf("expected ready result, got ready=%v result=%#v", ready, result)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("dependencies were not checked in parallel: elapsed=%s", elapsed)
	}
	if result.Dependencies["postgres"].Status != "up" || result.Dependencies["redis"].Status != "up" {
		t.Fatalf("unexpected dependencies: %#v", result.Dependencies)
	}
}

func TestCheckerReportsFailuresWithoutRawErrors(t *testing.T) {
	secretError := errors.New("postgres://secret-password@db:5432/pulsewatch")
	checker := NewChecker("api", fakePinger{err: secretError}, fakePinger{err: context.DeadlineExceeded}, time.Second, nil)

	result, ready := checker.Check(context.Background())

	if ready || result.Status != "not_ready" {
		t.Fatalf("expected not_ready result, got ready=%v result=%#v", ready, result)
	}
	if result.Dependencies["postgres"].ErrorCode != "unavailable" {
		t.Fatalf("unexpected postgres error: %#v", result.Dependencies["postgres"])
	}
	if result.Dependencies["redis"].ErrorCode != "timeout" {
		t.Fatalf("unexpected redis error: %#v", result.Dependencies["redis"])
	}
	if result.Dependencies["postgres"].Status != "down" || result.Dependencies["redis"].Status != "down" {
		t.Fatalf("unexpected dependency statuses: %#v", result.Dependencies)
	}
}

func TestCheckerReportsDrainingAsNotReady(t *testing.T) {
	checker := NewChecker("worker", fakePinger{}, fakePinger{}, time.Second, func() bool { return true })
	result, ready := checker.Check(context.Background())
	if ready || result.Status != "not_ready" {
		t.Fatalf("expected draining worker to be not ready: ready=%v result=%#v", ready, result)
	}
}

func TestCheckerHonorsTimeoutWhenPingerIgnoresContext(t *testing.T) {
	checker := NewChecker("api", blockingPinger{}, fakePinger{}, 20*time.Millisecond, nil)
	started := time.Now()
	result, ready := checker.Check(context.Background())
	if ready || result.Status != "not_ready" {
		t.Fatalf("expected timeout result: ready=%v result=%#v", ready, result)
	}
	if elapsed := time.Since(started); elapsed > 150*time.Millisecond {
		t.Fatalf("checker exceeded timeout: %s", elapsed)
	}
	if result.Dependencies["postgres"].ErrorCode != "timeout" {
		t.Fatalf("expected postgres timeout, got %#v", result.Dependencies["postgres"])
	}
}

type blockingPinger struct{}

func (blockingPinger) Ping(context.Context) error {
	time.Sleep(100 * time.Millisecond)
	return nil
}

func TestHealthHandlersSetResponseHeadersAndRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(httpx.RequestID())
	checker := NewChecker("api", fakePinger{}, fakePinger{}, time.Second, nil)
	handler := NewHandler(checker)
	router.GET("/health/live", handler.Live)
	router.GET("/health/ready", handler.Ready)

	record := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/health/live", nil)
	request.Header.Set("X-Request-ID", "health-test-1")
	router.ServeHTTP(record, request)

	if record.Code != 200 || record.Header().Get("X-Request-ID") != "health-test-1" {
		t.Fatalf("unexpected live response: status=%d headers=%v", record.Code, record.Header())
	}
	if record.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing no-store cache header")
	}

	record = httptest.NewRecorder()
	request = httptest.NewRequest("GET", "/health/ready", nil)
	request.Header.Set("X-Request-ID", "health-test-2")
	router.ServeHTTP(record, request)
	if record.Code != 200 || record.Header().Get("X-Request-ID") != "health-test-2" {
		t.Fatalf("unexpected ready response: status=%d headers=%v", record.Code, record.Header())
	}
}
