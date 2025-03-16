package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auditcue/integration-framework/internal/api"
	"github.com/auditcue/integration-framework/internal/config"
	"github.com/auditcue/integration-framework/internal/db"
	"github.com/auditcue/integration-framework/internal/queue"
	"github.com/auditcue/integration-framework/pkg/logger"
)

func main() {
	// Initialize logger
	logger := logger.NewLogger()
	logger.Info("Starting AuditCue Integration Framework")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", "error", err)
	}

	// Initialize database
	database, err := db.NewDatabase(cfg.Database)
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.Migrate(); err != nil {
		logger.Fatal("Failed to run database migrations", "error", err)
	}

	// Initialize job queue
	jobQueue, err := queue.NewQueue(cfg.Queue)
	if err != nil {
		logger.Fatal("Failed to initialize job queue", "error", err)
	}
	defer jobQueue.Close()

	// Start workers
	workers := queue.StartWorkers(jobQueue, cfg.Workers.Count, database, logger)
	defer workers.Stop()

	// Create router and initialize API
	router := api.SetupRouter(cfg, database, jobQueue, logger)

	// Configure server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", "error", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second)
	defer cancel()

	// Doesn't block if no connections, but will wait until the timeout deadline
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exiting")
}
