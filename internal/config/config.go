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
	AppEnv             string
	AppPort            string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBSSLMode          string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	JWTSecret          string
	JWTExpiresIn       time.Duration
	CORSAllowedOrigins []string
	AutoMigrate        bool
	AutoSeed           bool
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	jwtExpires, err := time.ParseDuration(getEnv("JWT_EXPIRES_IN", "24h"))
	if err != nil {
		return nil, fmt.Errorf("parse JWT_EXPIRES_IN: %w", err)
	}

	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_DB: %w", err)
	}

	cfg := &Config{
		AppEnv:             getEnv("APP_ENV", "local"),
		AppPort:            getEnv("APP_PORT", "8080"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "appuser"),
		DBPassword:         getEnv("DB_PASSWORD", "appsecret"),
		DBName:             getEnv("DB_NAME", "virtual_exam"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            redisDB,
		JWTSecret:          getEnv("JWT_SECRET", "change-me"),
		JWTExpiresIn:       jwtExpires,
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		AutoMigrate:        getEnvBool("AUTO_MIGRATE", true),
		AutoSeed:           getEnvBool("AUTO_SEED", true),
	}

	return cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func splitCSV(s string) []string {
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
