package config

import (
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr    string
	LogLevel    slog.Level
	MaxUploadMB int64
	DBHost string
}

func MustLoad() Config {
	httpAddr := getEnv("HTTP_ADDR", ":8080")

	levelStr := strings.ToLower(getEnv("LOG_LEVEL", "info"))
	var lvl slog.Level
	switch levelStr {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		log.Fatalf("unknown LOG_LEVEL=%s", levelStr)
	}

	maxMBStr := getEnv("MAX_UPLOAD_MB", "200")
	maxMB, err := strconv.ParseInt(maxMBStr, 10, 64)
	if err != nil || maxMB <= 0 {
		log.Fatalf("invalid MAX_UPLOAD_MB=%s", maxMBStr)
	}

	dbHost := getEnv("DB_HOST", "localhost")

	return Config{
		HTTPAddr:    httpAddr,
		LogLevel:    lvl,
		MaxUploadMB: maxMB,
		DBHost:      dbHost,
	}
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}