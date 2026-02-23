// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/tonyellard/ess-enn-ess/internal/admin"
	"github.com/tonyellard/ess-enn-ess/internal/config"
	"github.com/tonyellard/ess-enn-ess/internal/server"
)

func main() {
	// Set up logging
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))

	// Build configuration from environment variables
	cfg := buildConfigFromEnv()

	// Create SNS server
	logger.Info("Creating SNS server", "api_port", cfg.Server.APIPort)
	snsServer := server.NewServer(cfg, logger)

	// Register admin API routes on the same SNS server
	apiHandlers := admin.GetAdminRouteHandlers(cfg, logger,
		snsServer.GetTopicStore(), snsServer.GetSubscriptionStore(), snsServer.GetActivityLogger())
	snsServer.RegisterAdminRoutes(apiHandlers)

	// Start server
	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- snsServer.Start()
	}()

	logger.Info("SNS emulator started successfully")
	logger.Info("API endpoint", "url", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.APIPort))
	logger.Info("Health check", "url", fmt.Sprintf("http://%s:%d/health", cfg.Server.Host, cfg.Server.APIPort))

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		logger.Info("Shutdown signal received, stopping server...")
		if err := snsServer.Stop(); err != nil {
			logger.Error("Error stopping server", "error", err)
		}
	case err := <-serverErrors:
		if err != nil && err.Error() != "http: Server closed" {
			logger.Error("Server error", "error", err)
		}
	}

	logger.Info("SNS emulator stopped")
}

// buildConfigFromEnv creates a Config from environment variables, falling back to defaults.
func buildConfigFromEnv() *config.Config {
	cfg := config.Default()

	if v := os.Getenv("API_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.APIPort = port
		}
	}
	if v := os.Getenv("ADMIN_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.AdminPort = port
			cfg.Admin.Port = port
		}
	}
	if v := os.Getenv("HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("REGION"); v != "" {
		cfg.AWS.Region = v
		cfg.SQS.Region = v
	}
	if v := os.Getenv("ACCOUNT_ID"); v != "" {
		cfg.AWS.AccountId = v
	}
	if v := os.Getenv("SQS_ENDPOINT"); v != "" {
		cfg.SQS.Endpoint = v
	}
	if v := os.Getenv("SQS_ENABLED"); v != "" {
		cfg.SQS.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("AUTO_CONFIRM_SUBSCRIPTIONS"); v != "" {
		cfg.Developer.AutoConfirmSubscriptions = v == "true" || v == "1"
	}

	return cfg
}
