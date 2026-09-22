package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestShutdownWorkerStopsComponentsInOrder(t *testing.T) {
	var events []string
	add := func(event string) func() {
		return func() { events = append(events, event) }
	}
	if err := shutdownWorker(time.Second, shutdownOps{
		stopProducers:    add("producers"),
		waitHeartbeat:    add("heartbeat"),
		shutdownConsumer: add("consumer"),
		shutdownHTTP: func(context.Context) error {
			events = append(events, "http")
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{"producers", "heartbeat", "consumer", "http"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("shutdown order = %v, want %v", events, want)
	}
}

func TestShutdownWorkerClosesHTTPAfterGracefulShutdownFailure(t *testing.T) {
	wantErr := errors.New("shutdown timeout")
	var closed bool
	err := shutdownWorker(time.Second, shutdownOps{
		stopProducers:    func() {},
		waitHeartbeat:    func() {},
		shutdownConsumer: func() {},
		shutdownHTTP: func(context.Context) error {
			return wantErr
		},
		closeHTTP: func() error {
			closed = true
			return nil
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("shutdown error = %v, want %v", err, wantErr)
	}
	if !closed {
		t.Fatal("HTTP server was not force-closed after graceful shutdown failure")
	}
}
