package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/yaml.v3"

	"github.com/tonyellard/ess-enn-ess/internal/admin"
	"github.com/tonyellard/ess-enn-ess/internal/config"
	"github.com/tonyellard/ess-enn-ess/internal/server"
	"github.com/tonyellard/ess-enn-ess/internal/subscription"
	"github.com/tonyellard/ess-enn-ess/internal/topic"
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

	// Create SNS server
	logger.Info("Creating SNS server", "api_port", cfg.Server.APIPort)
	snsServer := server.NewServer(cfg, logger)

	// Try to load existing topics and subscriptions from the config file
	loadExportedState(*configFile, snsServer.GetTopicStore(), snsServer.GetSubscriptionStore(), logger)

	// Register admin dashboard routes on the same SNS server
	dashboardHandler, apiHandlers := admin.GetAdminRouteHandlers(cfg, logger,
		snsServer.GetTopicStore(), snsServer.GetSubscriptionStore(), snsServer.GetActivityLogger())
	snsServer.RegisterAdminRoutes(dashboardHandler, apiHandlers)

	// Start server
	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- snsServer.Start()
	}()

	logger.Info("SNS emulator started successfully")
	logger.Info("API endpoint", "url", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.APIPort))
	logger.Info("Admin dashboard", "url", fmt.Sprintf("http://localhost:%d/admin", cfg.Server.APIPort))
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

// loadExportedState loads topics and subscriptions from an exported config file
func loadExportedState(filePath string, topicStore *topic.Store, subStore *subscription.Store, logger *slog.Logger) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// File doesn't exist or can't be read - this is OK for fresh start
		return
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		logger.Warn("Failed to unmarshal config file for state loading", "error", err)
		return
	}

	// Load topics
	if topicsData, ok := raw["topics"].([]interface{}); ok {
		for _, topicData := range topicsData {
			var t topic.Topic
			// Re-marshal and unmarshal to convert to struct
			topicYaml, err := yaml.Marshal(topicData)
			if err != nil {
				logger.Warn("Failed to marshal topic data", "error", err)
				continue
			}
			if err := yaml.Unmarshal(topicYaml, &t); err != nil {
				logger.Warn("Failed to unmarshal topic", "error", err)
				continue
			}
			topicStore.Restore(&t)
		}
		logger.Info("Loaded topics from config file", "count", len(topicsData))
	}

	// Load subscriptions
	if subsData, ok := raw["subscriptions"].([]interface{}); ok {
		for _, subData := range subsData {
			var s subscription.Subscription
			// Re-marshal and unmarshal to convert to struct
			subYaml, err := yaml.Marshal(subData)
			if err != nil {
				logger.Warn("Failed to marshal subscription data", "error", err)
				continue
			}
			if err := yaml.Unmarshal(subYaml, &s); err != nil {
				logger.Warn("Failed to unmarshal subscription", "error", err)
				continue
			}
			subStore.Restore(&s)
		}
		logger.Info("Loaded subscriptions from config file", "count", len(subsData))
	}
}
