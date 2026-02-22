// SPDX-License-Identifier: Apache-2.0
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tonyellard/scheduler/internal/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := server.Config{
		Region:      envOrDefault("REGION", "us-east-1"),
		AccountID:   envOrDefault("ACCOUNT_ID", "000000000000"),
		SQSEndpoint: envOrDefault("SQS_ENDPOINT", "http://ess-queue-ess:9320"),
		SNSEndpoint: envOrDefault("SNS_ENDPOINT", "http://ess-enn-ess:9330"),
	}

	handler := server.NewRouter(logger, cfg)
	port := envOrDefault("PORT", "9342")

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	logger.Info("scheduler starting", "port", port, "region", cfg.Region)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
