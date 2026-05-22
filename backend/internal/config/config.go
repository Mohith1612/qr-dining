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
	BaseDomain      string // BASE_DOMAIN: e.g. "dining.example.com". When set, enables subdomain-based tenant extraction.
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
	cfg.Server.BaseDomain = getenv("BASE_DOMAIN", "")

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

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	// Warn (non-fatal) when CORS origins are unconfigured in production.
	if len(cfg.CORS.AllowedOrigins) == 0 && cfg.Server.GinMode == "release" {
		fmt.Fprintf(os.Stderr, "warn: CORS_ALLOWED_ORIGINS is empty in release mode — all WebSocket origins will be accepted\n")
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Server.RateLimitRPM <= 0 {
		return fmt.Errorf("RATE_LIMIT_RPM must be > 0, got %d", c.Server.RateLimitRPM)
	}
	validModes := map[string]bool{"debug": true, "test": true, "release": true}
	if !validModes[c.Server.GinMode] {
		return fmt.Errorf("GIN_MODE must be one of debug/test/release, got %q", c.Server.GinMode)
	}
	if c.DB.MaxConns < 1 || c.DB.MaxConns > 200 {
		return fmt.Errorf("DB_MAX_CONNS must be between 1 and 200, got %d", c.DB.MaxConns)
	}
	if c.DB.MinConns < 0 || c.DB.MinConns > c.DB.MaxConns {
		return fmt.Errorf("DB_MIN_CONNS must be >= 0 and <= DB_MAX_CONNS (%d), got %d", c.DB.MaxConns, c.DB.MinConns)
	}
	if c.Server.ReadTimeout >= c.Server.WriteTimeout {
		return fmt.Errorf("READ_TIMEOUT (%s) must be less than WRITE_TIMEOUT (%s)", c.Server.ReadTimeout, c.Server.WriteTimeout)
	}
	return nil
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
		fmt.Fprintf(os.Stderr, "warn: invalid %s=%q: %v — using default %s\n", key, s, err, fallback)
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
