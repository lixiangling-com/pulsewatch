// Command mocktarget is a development-only HTTP target used to exercise checks.
package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /target", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if delay, _ := strconv.Atoi(query.Get("delay_ms")); delay > 0 && delay <= 30000 {
			timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-r.Context().Done():
				return
			}
		}
		if redirect := query.Get("redirect"); redirect != "" {
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		}
		status, _ := strconv.Atoi(query.Get("status"))
		if status < 100 || status > 599 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, "mock target status=%d\n", status)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	log.Fatal(http.ListenAndServe(":8080", mux))
}
