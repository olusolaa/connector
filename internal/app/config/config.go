package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/joho/godotenv"
)

type Config struct {
	GRPCPort          string
	DBDSN             string
	AWSRegion         string
	AWSEndpoint       string
	SlackBaseURL      string
	SlackClientID     string
	SlackClientSecret string
	SlackRedirectURL  string
	RedisAddr         string
	SlackScopes       string
	// Slack retry configuration
	SlackRetryMax       int
	SlackRetryWait      time.Duration
	SlackRateLimitRPS   float64
	SlackRateLimitBurst int
	// AWS retry configuration
	AWSRetryMaxAttempts int
	AWSRetryMaxBackoff  time.Duration
	// HTTP Server timeouts
	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	// Circuit breaker configuration
	CircuitBreakerInterval time.Duration
	CircuitBreakerTimeout  time.Duration
	// OAuth configuration
	OAuthStateTimeout time.Duration
	// Database configuration
	DBConnMaxLifetime time.Duration
	DBMaxIdleConns    int
	DBMaxOpenConns    int
	RedisDB           int
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, relying on environment variables")
	}

	cfg := &Config{
		GRPCPort:          getEnv("GRPC_PORT", "50051"),
		DBDSN:             getEnv("DB_DSN", "postgres://user:password@localhost:5432/connector?sslmode=disable"),
		AWSRegion:         getEnv("AWS_REGION", "us-east-1"),
		AWSEndpoint:       getEnv("AWS_ENDPOINT", "http://localhost:4566"),
		SlackBaseURL:      getEnv("SLACK_BASE_URL", "https://slack.com/api"),
		SlackClientID:     os.Getenv("SLACK_CLIENT_ID"),
		SlackClientSecret: os.Getenv("SLACK_CLIENT_SECRET"),
		SlackRedirectURL:  os.Getenv("SLACK_REDIRECT_URL"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		SlackScopes:       getEnv("SLACK_SCOPES", "chat:write,channels:read,groups:read"),
		// Slack retry configuration with defaults
		SlackRetryMax:       getEnvInt("SLACK_RETRY_MAX", 3),
		SlackRetryWait:      time.Duration(getEnvInt("SLACK_RETRY_WAIT_SECONDS", 5)) * time.Second,
		SlackRateLimitRPS:   getEnvFloat("SLACK_RATE_LIMIT_RPS", 1.0),
		SlackRateLimitBurst: getEnvInt("SLACK_RATE_LIMIT_BURST", 3),
		// AWS retry configuration with defaults
		AWSRetryMaxAttempts: getEnvInt("AWS_RETRY_MAX_ATTEMPTS", 5),
		AWSRetryMaxBackoff:  time.Duration(getEnvInt("AWS_RETRY_MAX_BACKOFF_SECONDS", 60)) * time.Second,
		// HTTP Server timeouts
		HTTPReadHeaderTimeout: time.Duration(getEnvInt("HTTP_READ_HEADER_TIMEOUT_SECONDS", 60)) * time.Second,
		HTTPReadTimeout:       time.Duration(getEnvInt("HTTP_READ_TIMEOUT_SECONDS", 60)) * time.Second,
		HTTPWriteTimeout:      time.Duration(getEnvInt("HTTP_WRITE_TIMEOUT_SECONDS", 60)) * time.Second,
		HTTPIdleTimeout:       time.Duration(getEnvInt("HTTP_IDLE_TIMEOUT_SECONDS", 120)) * time.Second,
		// Circuit breaker configuration
		CircuitBreakerInterval: time.Duration(getEnvInt("CIRCUIT_BREAKER_INTERVAL_SECONDS", 60)) * time.Second,
		CircuitBreakerTimeout:  time.Duration(getEnvInt("CIRCUIT_BREAKER_TIMEOUT_SECONDS", 30)) * time.Second,
		// OAuth configuration
		OAuthStateTimeout: time.Duration(getEnvInt("OAUTH_STATE_TIMEOUT_MINUTES", 15)) * time.Minute,
		// Database configuration
		DBConnMaxLifetime: time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MINUTES", 5)) * time.Minute,
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 100),
		RedisDB:           getEnvInt("REDIS_DB", 0),
	}

	if cfg.SlackClientID == "" || cfg.SlackClientSecret == "" || cfg.SlackRedirectURL == "" {
		return nil, fmt.Errorf("missing required Slack configuration (SLACK_CLIENT_ID, SLACK_CLIENT_SECRET, SLACK_REDIRECT_URL)")
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val, exists := os.LookupEnv(key); exists {
		if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
			return floatVal
		}
	}
	return defaultVal
}

func LoadAWSConfig(region, endpoint string) (aws.Config, error) {
	var optFns []func(*awsConfig.LoadOptions) error

	optFns = append(optFns, awsConfig.WithRegion(region))

	if strings.Contains(endpoint, "localhost") || strings.Contains(endpoint, "localstack") {
		optFns = append(optFns,
			awsConfig.WithEndpointResolverWithOptions(
				aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{
						URL:               endpoint,
						SigningRegion:     region,
						HostnameImmutable: true,
					}, nil
				})),
		)
	}

	return awsConfig.LoadDefaultConfig(context.Background(), optFns...)
}
