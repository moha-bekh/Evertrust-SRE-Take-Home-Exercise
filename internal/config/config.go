package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddress     string
	DatabasePath      string
	MigrationsPath    string
	QueueSize         int
	HTTPReadTimeout   time.Duration
	HTTPWriteTimeout  time.Duration
	HTTPIdleTimeout   time.Duration
	InspectionTimeout time.Duration
	ShutdownTimeout   time.Duration
}

func FromEnv() Config {
	return Config{
		ListenAddress:     env("LISTEN_ADDRESS", ":8080"),
		DatabasePath:      env("DATABASE_PATH", "certificate-inspector.db"),
		MigrationsPath:    env("MIGRATIONS_PATH", "migrations"),
		QueueSize:         envInt("QUEUE_SIZE", 100),
		HTTPReadTimeout:   envDuration("HTTP_READ_TIMEOUT", 5*time.Second),
		HTTPWriteTimeout:  envDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
		HTTPIdleTimeout:   envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		InspectionTimeout: envDuration("INSPECTION_TIMEOUT", 5*time.Second),
		ShutdownTimeout:   envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
