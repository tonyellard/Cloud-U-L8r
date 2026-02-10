package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/tonyellard/ess-enn-ess/internal/admin"
	"github.com/tonyellard/ess-enn-ess/internal/config"
	"github.com/tonyellard/ess-enn-ess/internal/server"
)

func main() {
	// Parse flags
	configFile := flag.String("config", "./config/config.yaml", "Path to configuration file")
	flag.Parse()

	// Set up logging
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))

	// Load configuration
	logger.Info("Loading configuration", "file", *configFile)
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create server
	logger.Info("Creating SNS server", "api_port", cfg.Server.APIPort, "admin_port", cfg.Server.AdminPort)
	snsServer := server.NewServer(cfg, logger)
	adminServer := admin.NewServer(cfg, logger, snsServer.GetTopicStore(), snsServer.GetActivityLogger())

	// Start both servers in goroutines
	serverErrors := make(chan error, 2)
	var wg sync.WaitGroup

	// Start SNS API server
	wg.Add(1)
	go func() {
		defer wg.Done()
		serverErrors <- snsServer.Start()
	}()

	// Start Admin dashboard server
	wg.Add(1)
	go func() {
		defer wg.Done()
		serverErrors <- adminServer.Start()
	}()

	logger.Info("SNS emulator started successfully")
	logger.Info("API endpoint", "url", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.APIPort))
	logger.Info("Admin dashboard", "url", fmt.Sprintf("http://localhost:%d", cfg.Server.AdminPort))
	logger.Info("Health check", "url", fmt.Sprintf("http://%s:%d/health", cfg.Server.Host, cfg.Server.APIPort))

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		logger.Info("Shutdown signal received, stopping servers...")
		if err := snsServer.Stop(); err != nil {
			logger.Error("Error stopping SNS server", "error", err)
		}
		if err := adminServer.Stop(); err != nil {
			logger.Error("Error stopping admin server", "error", err)
		}
		wg.Wait()
	case err := <-serverErrors:
		if err != nil {
			logger.Error("Server error", "error", err)
		}
	}

	logger.Info("SNS emulator stopped")
}
