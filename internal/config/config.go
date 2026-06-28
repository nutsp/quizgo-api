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
	RedisContentDB     int
	RedisUserDB        int
	RedisResultDB      int
	RedisRuntimeDB     int
	RedisCacheEnabled  bool
	JWTSecret          string
	JWTExpiresIn       time.Duration
	CORSAllowedOrigins []string
	AutoMigrate        bool
	AutoSeed           bool
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	FrontendURL        string
	OAuthStateSecret   string
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
	redisContentDB, err := strconv.Atoi(getEnv("REDIS_CONTENT_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_CONTENT_DB: %w", err)
	}
	redisUserDB, err := strconv.Atoi(getEnv("REDIS_USER_DB", "1"))
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_USER_DB: %w", err)
	}
	redisResultDB, err := strconv.Atoi(getEnv("REDIS_RESULT_DB", "2"))
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_RESULT_DB: %w", err)
	}
	redisRuntimeDB, err := strconv.Atoi(getEnv("REDIS_RUNTIME_DB", "3"))
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_RUNTIME_DB: %w", err)
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
		RedisContentDB:     redisContentDB,
		RedisUserDB:        redisUserDB,
		RedisResultDB:      redisResultDB,
		RedisRuntimeDB:     redisRuntimeDB,
		RedisCacheEnabled:  getEnvBool("REDIS_CACHE_ENABLED", true),
		JWTSecret:          getEnv("JWT_SECRET", "change-me"),
		JWTExpiresIn:       jwtExpires,
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		AutoMigrate:        getEnvBool("AUTO_MIGRATE", true),
		AutoSeed:           getEnvBool("AUTO_SEED", true),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/oauth/google/callback"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:3000"),
		OAuthStateSecret:   getEnv("OAUTH_STATE_SECRET", "change-me"),
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
