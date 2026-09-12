package httpx

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowsConfiguredOriginAndCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS([]string{"http://localhost:5173"}))
	router.GET("/health", func(c *gin.Context) { c.Status(200) })

	record := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/health", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	router.ServeHTTP(record, request)

	if record.Code != 200 || record.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" || record.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("unexpected CORS response: status=%d headers=%v", record.Code, record.Header())
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS([]string{"http://localhost:5173"}))
	router.GET("/health", func(c *gin.Context) { c.Status(200) })

	record := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/health", nil)
	request.Header.Set("Origin", "https://evil.example")
	router.ServeHTTP(record, request)

	if record.Code != 403 {
		t.Fatalf("expected 403 for unknown origin, got %d", record.Code)
	}
}

func TestRequestIDRejectsUnsafeHeaderAndGeneratesOne(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/health", func(c *gin.Context) { c.Status(200) })

	record := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/health", nil)
	request.Header.Set("X-Request-ID", "bad value\n")
	router.ServeHTTP(record, request)

	requestID := record.Header().Get("X-Request-ID")
	if record.Code != 200 || requestID == "" || requestID == "bad value\n" {
		t.Fatalf("expected generated request ID, got %q", requestID)
	}
}
