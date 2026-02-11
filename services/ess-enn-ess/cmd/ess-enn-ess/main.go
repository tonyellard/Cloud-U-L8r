package main

import (
"flag"
"fmt"
"log/slog"
"os"
"os/signal"
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

// Create SNS server
logger.Info("Creating SNS server", "api_port", cfg.Server.APIPort)
snsServer := server.NewServer(cfg, logger)

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
