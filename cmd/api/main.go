// не судите строго, Go не мой язык к сожалению, познакомился с ним
// только на этом курсе (

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pricesapi/internal/config"
	"pricesapi/internal/db"
	"pricesapi/internal/httpapi"
)

func main() {
	cfg := config.MustLoad()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	pool, err := db.Open(cfg.DBHost)
	if err != nil {
		logger.Error("db open failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		logger.Error("db migrate failed", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      httpapi.NewRouter(pool, logger, cfg),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("http server started", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("shutting down")
	_ = srv.Shutdown(ctx)
	logger.Info("bye")
}
