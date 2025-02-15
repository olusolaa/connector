package main

import (
	"context"
	"fmt"
	"github.com/connector-recruitment/internal/infrastructure/redis"
	"github.com/connector-recruitment/pkg/resilience"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/connector-recruitment/internal/app/config"
	appConnector "github.com/connector-recruitment/internal/app/connector"
	awsSM "github.com/connector-recruitment/internal/infrastructure/aws/secretsmanager"
	pgRepo "github.com/connector-recruitment/internal/infrastructure/postgres"
	slackInfra "github.com/connector-recruitment/internal/infrastructure/slack"
	grpcTransport "github.com/connector-recruitment/internal/transport/grpc"
	httpTransport "github.com/connector-recruitment/internal/transport/http"
	"github.com/connector-recruitment/pkg/logger"
	"github.com/connector-recruitment/pkg/observability"

	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load configuration")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tp, err := observability.InitTracer(ctx, "connector-recruitment")
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize tracer")
	}
	defer func() {
		if shutdownErr := observability.ShutdownTracer(ctx, tp); shutdownErr != nil {
			logger.Error().Err(shutdownErr).Msg("error shutting down tracer provider")
		}
	}()

	db, err := pgRepo.InitDb(ctx, cfg.DBDSN)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer db.Close()

	redisClient, err := redis.IntClient(ctx, cfg.RedisAddr)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer redisClient.Close()

	awsCfg, err := config.LoadAWSConfig(cfg.AWSRegion, cfg.AWSEndpoint)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load AWS config")
	}
	smClient := awsSM.NewClient(awsCfg,
		awsSM.WithRetryMaxAttempts(cfg.AWSRetryMaxAttempts),
		awsSM.WithRetryMaxBackoff(cfg.AWSRetryMaxBackoff),
	)

	slackClient := slackInfra.NewSlackClient(
		cfg.SlackBaseURL,
		cfg.SlackClientID,
		cfg.SlackClientSecret,
		cfg.SlackRedirectURL,
		cfg.SlackScopes,
		slackInfra.WithRetry(cfg.SlackRetryMax, cfg.SlackRetryWait),
		slackInfra.WithRateLimit(cfg.SlackRateLimitRPS, cfg.SlackRateLimitBurst),
	)

	repository := pgRepo.NewConnectorRepository(db)
	oauthManager := appConnector.NewOAuthStateManager(redisClient, cfg.OAuthStateTimeout)
	service := appConnector.NewService(repository, smClient, slackClient,
		resilience.New("github.com/connector-recruitment", cfg), appConnector.WithOAuthManager(oauthManager))

	grpcServer, lis, err := grpcTransport.NewServer(service, cfg.GRPCPort)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create gRPC server")
	}

	httpServer := httpTransport.NewHTTPServer(service, oauthManager, cfg)

	errCh := make(chan error, 2)

	go func() {
		logger.Info().Str("port", cfg.GRPCPort).Msg("Starting gRPC server")
		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			errCh <- fmt.Errorf("grpc server error: %w", serveErr)
		}
	}()

	go func() {
		logger.Info().Str("port", "8080").Msg("Starting HTTP server with TLS")
		if serveErr := httpServer.ListenAndServeTLS("cert.pem", "key.pem"); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- fmt.Errorf("http server error: %w", serveErr)
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info().Msg("Shutdown signal received; stopping servers...")

	case serverErr := <-errCh:
		logger.Error().Err(serverErr).Msg("Server encountered an error; shutting down...")
	}

	go func() {
		logger.Info().Msg("Attempting gRPC graceful stop")
		grpcServer.GracefulStop()
		logger.Info().Msg("gRPC server stopped gracefully")
	}()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("HTTP server shutdown error")
	} else {
		logger.Info().Msg("HTTP server stopped gracefully")
	}

	logger.Info().Msg("All servers stopped. Exiting.")
}
