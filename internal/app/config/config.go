package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

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
