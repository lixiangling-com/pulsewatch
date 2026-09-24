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

	"github.com/gin-gonic/gin"
	"github.com/lixiangling-com/pulsewatch/server/internal/auth"
	"github.com/lixiangling-com/pulsewatch/server/internal/health"
	"github.com/lixiangling-com/pulsewatch/server/internal/monitor"
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
	if err := cfg.ValidateAPI(); err != nil {
		slog.Error("configuration failed", slog.String("error", err.Error()))
		return err
	}
	logger := logging.New("api", cfg.Environment)
	gin.SetMode(gin.ReleaseMode)

	postgres, err := database.New(cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres client initialization failed", slog.String("error", "invalid database configuration"))
		return err
	}
	defer postgres.Close()
	redisClient := queue.New(cfg.RedisAddr, cfg.RedisPassword)
	defer redisClient.Close()

	var draining atomic.Bool
	checker := health.NewChecker("api", postgres, redisClient, cfg.HealthTimeout, draining.Load)
	router := gin.New()
	router.Use(httpx.RequestID(), httpx.CORS(cfg.CORSAllowedOrigins), httpx.Recovery(logger), httpx.AccessLog(logger))
	healthHandler := health.NewHandler(checker)
	router.GET("/health/live", healthHandler.Live)
	router.GET("/health/ready", healthHandler.Ready)

	tokenManager, err := auth.NewTokenManager(cfg.JWTAccessSecret, cfg.JWTIssuer)
	if err != nil {
		logger.Error("auth initialization failed", slog.String("error", "invalid token configuration"))
		return err
	}
	authService := auth.NewService(auth.NewRepository(postgres), tokenManager)
	authHandler := auth.NewHandler(authService, cfg.IsDevelopment())
	authRoutes := router.Group("/api/v1/auth")
	authRoutes.POST("/register", authHandler.Register)
	authRoutes.POST("/login", authHandler.Login)
	authRoutes.POST("/refresh", authHandler.Refresh)
	authRoutes.POST("/logout", authHandler.Logout)
	authRoutes.GET("/me", tokenManager.RequireUser(), authHandler.Me)

	monitorService := monitor.NewService(monitor.NewRepository(postgres))
	monitorHandler := monitor.NewHandler(monitorService, logger)
	monitorRoutes := router.Group("/api/v1/monitors", tokenManager.RequireUser())
	monitor.RegisterRoutes(monitorRoutes, monitorHandler)
	notificationHandler := notification.NewHandler(postgres)
	notification.RegisterMonitorRoutes(monitorRoutes, notificationHandler)
	notificationRoutes := router.Group("/api/v1", tokenManager.RequireUser())
	notification.RegisterRoutes(notificationRoutes, notificationHandler)

	server := httpx.NewServer(cfg.HTTPAddr, router)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errorsCh := make(chan error, 1)
	go func() {
		logger.Info("api started", slog.String("addr", cfg.HTTPAddr))
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errorsCh <- serveErr
		}
	}()

	select {
	case serveErr := <-errorsCh:
		return serveErr
	case <-ctx.Done():
		draining.Store(true)
		logger.Info("api shutdown started")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			logger.Error("api graceful shutdown timed out", slog.String("error", "shutdown deadline exceeded"))
			_ = server.Close()
			return shutdownErr
		}
		logger.Info("api stopped")
		return nil
	}
}
