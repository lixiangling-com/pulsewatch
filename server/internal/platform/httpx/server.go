package httpx

import (
	"net/http"
	"time"
)

func NewServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
		// WriteTimeout intentionally remains zero because future SSE responses
		// are long-lived. Streaming handlers will enforce their own deadlines.
	}
}
