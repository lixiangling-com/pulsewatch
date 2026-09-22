package main

import (
	"context"
	"time"
)

type shutdownOps struct {
	stopProducers    func()
	waitHeartbeat    func()
	shutdownConsumer func()
	shutdownHTTP     func(context.Context) error
	closeHTTP        func() error
}

func shutdownWorker(timeout time.Duration, ops shutdownOps) error {
	ops.stopProducers()
	ops.waitHeartbeat()
	ops.shutdownConsumer()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := ops.shutdownHTTP(shutdownCtx); err != nil {
		if ops.closeHTTP != nil {
			_ = ops.closeHTTP()
		}
		return err
	}
	return nil
}
