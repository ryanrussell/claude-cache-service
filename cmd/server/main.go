package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/ryanrussell/claude-cache-service/internal/api"
	"github.com/ryanrussell/claude-cache-service/internal/cache"
	"github.com/ryanrussell/claude-cache-service/internal/config"
	"github.com/ryanrussell/claude-cache-service/internal/worker"
)

func main() {
	// Initialize logger
	logger := zerolog.New(os.Stdout).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Caller().
		Logger()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Set log level from config
	if cfg.Debug {
		logger = logger.Level(zerolog.DebugLevel)
	}

	logger.Info().
		Str("version", cfg.Version).
		Str("port", cfg.Port).
		Msg("Starting Claude Cache Service")

	// Initialize cache manager
	cacheManager, err := cache.NewManager(cfg.CacheDir, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize cache manager")
	}
	defer func() {
		if err := cacheManager.Close(); err != nil {
			logger.Error().Err(err).Msg("Failed to close cache manager")
		}
	}()

	// Initialize update worker
	updateWorker := worker.NewUpdateWorker(cacheManager, logger, cfg)

	// Start scheduled updates
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go updateWorker.Start(ctx)

	// Initialize API server
	server := api.NewServer(cfg, cacheManager, logger)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		logger.Info().Msg("Shutting down gracefully...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error().Err(err).Msg("Failed to shutdown server gracefully")
		}
	}()

	// Start the server
	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Info().Str("address", addr).Msg("Starting API server")

	if err := server.Run(addr); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start server")
	}
}
