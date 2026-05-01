package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server ServerConfig
	DB     DBConfig
	Redis  RedisConfig
	Log    LogConfig
	CORS   CORSConfig
	Worker WorkerConfig
}

type ServerConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	GinMode         string
	TrustedProxies  []string
	RateLimitRPM    int
}

type DBConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

type RedisConfig struct {
	URL string
}

type LogConfig struct {
	Level  string
	Pretty bool
}

type CORSConfig struct {
	AllowedOrigins []string
}

type WorkerConfig struct {
	StaleSessionInterval  time.Duration
	PresenceExpiryInterval time.Duration
}

func Load() (*Config, error) {
	// Load .env if present — no-op in production where env vars are injected directly.
	_ = godotenv.Overload()

	cfg := &Config{}

	// Server
	port, err := parseInt("PORT", 8080)
	if err != nil {
		return nil, err
	}
	cfg.Server.Port = port
	cfg.Server.ReadTimeout = parseDuration("READ_TIMEOUT", 10*time.Second)
	cfg.Server.WriteTimeout = parseDuration("WRITE_TIMEOUT", 30*time.Second)
	cfg.Server.ShutdownTimeout = parseDuration("SHUTDOWN_TIMEOUT", 15*time.Second)
	cfg.Server.GinMode = getenv("GIN_MODE", "release")
	cfg.Server.TrustedProxies = splitComma("TRUSTED_PROXIES", "172.16.0.0/12")

	rateLimitRPM, err := parseInt("RATE_LIMIT_RPM", 60)
	if err != nil {
		return nil, err
	}
	cfg.Server.RateLimitRPM = rateLimitRPM

	// Database
	dbURL := getenv("DATABASE_URL", "")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	cfg.DB.URL = dbURL

	maxConns, err := parseInt("DB_MAX_CONNS", 20)
	if err != nil {
		return nil, err
	}
	cfg.DB.MaxConns = int32(maxConns)

	minConns, err := parseInt("DB_MIN_CONNS", 2)
	if err != nil {
		return nil, err
	}
	cfg.DB.MinConns = int32(minConns)

	cfg.DB.MaxConnLifetime = parseDuration("DB_MAX_CONN_LIFETIME", time.Hour)
	cfg.DB.MaxConnIdleTime = parseDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute)

	// Redis
	redisURL := getenv("REDIS_URL", "")
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is required")
	}
	cfg.Redis.URL = redisURL

	// Logging
	cfg.Log.Level = getenv("LOG_LEVEL", "info")
	cfg.Log.Pretty = getenv("LOG_PRETTY", "false") == "true"

	// CORS
	cfg.CORS.AllowedOrigins = splitComma("CORS_ALLOWED_ORIGINS", "")

	// Workers
	cfg.Worker.StaleSessionInterval = parseDuration("STALE_SESSION_INTERVAL", 5*time.Minute)
	cfg.Worker.PresenceExpiryInterval = parseDuration("PRESENCE_EXPIRY_INTERVAL", 60*time.Second)

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func parseInt(key string, fallback int) (int, error) {
	s := getenv(key, "")
	if s == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return v, nil
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	s := getenv(key, "")
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}

func splitComma(key, fallback string) []string {
	s := getenv(key, fallback)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
