package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	checktask "github.com/lixiangling-com/pulsewatch/server/internal/check"
	"github.com/lixiangling-com/pulsewatch/server/internal/check/checker"
	"github.com/lixiangling-com/pulsewatch/server/internal/check/consumer"
	"github.com/lixiangling-com/pulsewatch/server/internal/check/dispatcher"
	"github.com/lixiangling-com/pulsewatch/server/internal/check/scheduler"
	"github.com/lixiangling-com/pulsewatch/server/internal/health"
	"github.com/lixiangling-com/pulsewatch/server/internal/notification"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/config"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/database"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/httpx"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/logging"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/queue"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration failed", slog.String("error", err.Error()))
		return err
	}
	logger := logging.New("worker", cfg.Environment)
	gin.SetMode(gin.ReleaseMode)

	postgres, err := database.New(cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres client initialization failed", slog.String("error", "invalid database configuration"))
		return err
	}
	defer postgres.Close()
	redisClient := queue.New(cfg.RedisAddr, cfg.RedisPassword)
	defer redisClient.Close()
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
	defer asynqClient.Close()

	workerServer := asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.RedisAddr, Password: cfg.RedisPassword}, asynq.Config{
		Concurrency:     10,
		ShutdownTimeout: cfg.ShutdownTimeout,
	})
	consumerProcessor := consumer.NewRepository(postgres, checker.NewHTTPChecker(cfg.IsDevelopment()))
	consumerHandler := consumer.New(consumerProcessor, logger)
	mux := asynq.NewServeMux()
	mux.HandleFunc(checktask.TypeCheckRun, consumerHandler.HandleCheckRun)
	mailConsumer := notification.NewConsumer(postgres, notification.SMTPSender{Addr: cfg.SMTPAddr, From: cfg.SMTPFrom}, logger)
	mux.HandleFunc(notification.TypeMailSend, mailConsumer.HandleMail)
	if err := workerServer.Start(mux); err != nil {
		return err
	}
	defer workerServer.Shutdown()
	producerCtx, stopProducers := context.WithCancel(context.Background())
	defer stopProducers()
	schedulerLoop := scheduler.New(scheduler.NewRepository(postgres), cfg.SchedulerInterval, cfg.SchedulerBatchSize, logger)
	dispatcherLoop := dispatcher.New(dispatcher.NewRepository(postgres), asynqClient, cfg.DispatchInterval, cfg.DispatchBatchSize, logger)
	notificationDispatcher := notification.NewDispatcher(notification.NewStore(postgres), asynqClient, cfg.DispatchInterval, cfg.DispatchBatchSize, logger)
	go schedulerLoop.Run(producerCtx)
	go dispatcherLoop.Run(producerCtx)
	go notificationDispatcher.Run(producerCtx)

	var draining atomic.Bool
	checker := health.NewChecker("worker", postgres, redisClient, cfg.HealthTimeout, draining.Load)
	router := gin.New()
	router.Use(httpx.RequestID(), httpx.Recovery(logger), httpx.AccessLog(logger))
	healthHandler := health.NewHandler(checker)
	router.GET("/health/live", healthHandler.Live)
	router.GET("/health/ready", healthHandler.Ready)

	server := httpx.NewServer(cfg.WorkerHTTPAddr, router)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errorsCh := make(chan error, 1)
	go func() {
		logger.Info("worker started", slog.String("probe_addr", cfg.WorkerHTTPAddr))
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errorsCh <- serveErr
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		for {
			select {
			case <-ticker.C:
				logger.Info("worker heartbeat")
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case serveErr := <-errorsCh:
		return serveErr
	case <-ctx.Done():
		draining.Store(true)
		logger.Info("worker shutdown started")
		if shutdownErr := shutdownWorker(cfg.ShutdownTimeout, shutdownOps{
			stopProducers:    stopProducers,
			waitHeartbeat:    func() { <-heartbeatDone },
			shutdownConsumer: workerServer.Shutdown,
			shutdownHTTP:     server.Shutdown,
			closeHTTP:        server.Close,
		}); shutdownErr != nil {
			logger.Error("worker graceful shutdown timed out", slog.String("error", "shutdown deadline exceeded"))
			return shutdownErr
		}
		logger.Info("worker stopped")
		return nil
	}
}
