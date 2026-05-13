package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort = "8080"

	defaultCacheTTL = 1 * time.Hour

	defaultReqTimeout        = 10 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
	defaultWriteTimeout      = 15 * time.Second
	defaultIdleTimeout       = 60 * time.Second

	defaultShutdownTimeout = 10 * time.Second
)

type Config struct {
	Port string

	CacheTTL time.Duration

	RequestTimeout    time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

	ShutdownTimeout time.Duration

	AvitoUserID  string
	AvitoEnabled bool
}

func Load(log *slog.Logger) Config {
	avitoUserID := getEnv("AVITO_USER_ID", "")
	if avitoUserID == "" {
		log.Error("Avito user ID is not set, exiting")
		os.Exit(1)
	}

	return Config{
		Port: getEnv(os.Getenv("PORT"), defaultPort),

		CacheTTL: getEnvDuration("CACHE_TTL_SECONDS", defaultCacheTTL, time.Second, log),

		RequestTimeout:    getEnvDuration("REQUEST_TIMEOUT_MS", defaultReqTimeout, time.Millisecond, log),
		ReadHeaderTimeout: getEnvDuration("READ_HEADER_TIMEOUT_MS", defaultReadHeaderTimeout, time.Millisecond, log),
		WriteTimeout:      getEnvDuration("WRITE_TIMEOUT_MS", defaultWriteTimeout, time.Millisecond, log),
		IdleTimeout:       getEnvDuration("IDLE_TIMEOUT_MS", defaultIdleTimeout, time.Millisecond, log),

		ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT_MS", defaultShutdownTimeout, time.Millisecond, log),

		AvitoUserID:  avitoUserID,
		AvitoEnabled: getEnvBool("AVITO_ENABLED", true),
	}
}

func getEnv(envName string, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}

	return defaultValue
}

func getEnvDuration(envName string, defaultValue time.Duration, unit time.Duration, log *slog.Logger) time.Duration {
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Warn(
			"invalid duration, falling back to default",
			"name", envName,
			"input", value,
			"error", err,
			"default_value", defaultValue,
		)

		return defaultValue
	}

	result := time.Duration(parsed) * unit
	if result <= 0 {
		log.Warn(
			"duration is non-positive, falling back to default",
			"name", envName,
			"value", result,
			"default_value", defaultValue,
		)

		return defaultValue
	}

	return result
}

func getEnvBool(envName string, defaultValue bool) bool {
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return defaultValue
	}

	l := strings.ToLower(value)
	return l == "true" || l == "t" || l == "1" || l == "yes" || l == "y" || l == "on" || l == "enable" || l == "enabled"
}
