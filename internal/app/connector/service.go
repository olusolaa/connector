package connector

import (
	"context"
	"fmt"
	"time"

	"github.com/connector-recruitment/internal/domain"
	"github.com/connector-recruitment/pkg/logger"
	"github.com/connector-recruitment/pkg/resilience"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServiceOption func(*Service)

func WithOAuthManager(manager *OAuthStateManager) ServiceOption {
	return func(s *Service) {
		s.oauthManager = manager
	}
}

type Service struct {
	repo           domain.ConnectorRepository
	secretsManager domain.SecretsManager
	slackClient    domain.SlackClient
	cb             *resilience.CircuitBreaker
	oauthManager   *OAuthStateManager
}

func NewService(repo domain.ConnectorRepository, sm domain.SecretsManager, sc domain.SlackClient, opts ...ServiceOption) *Service {
	s := &Service{
		repo:           repo,
		secretsManager: sm,
		slackClient:    sc,
		cb:             resilience.New("github.com/connector-recruitment"),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Service) CreateConnector(ctx context.Context, input CreateInput) (*domain.Connector, error) {
	if len(input.WorkspaceID) < 3 || len(input.TenantID) == 0 || len(input.Token) == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid input: workspace_id, tenant_id, and token are required")
	}

	result, err := s.cb.Execute(ctx, func() (interface{}, error) {
		logger.Info().
			Str("default_channel", input.DefaultChannel).
			Msg("Resolving channel ID")

		channelID, err := s.slackClient.ResolveChannelID(ctx, input.Token, input.DefaultChannel)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to resolve channel ID")
			return nil, fmt.Errorf("failed to resolve channel: %w", err)
		}

		secretName := fmt.Sprintf("connector-%s-%s", input.WorkspaceID, input.TenantID)
		logger.Info().
			Str("secret_name", secretName).
			Msg("Storing Slack token in Secrets Manager (not logging token value)")

		if err := s.secretsManager.StoreToken(ctx, secretName, input.Token); err != nil {
			logger.Error().Err(err).Msg("Failed to store token in Secrets Manager")
			return nil, fmt.Errorf("failed to store secret: %w", err)
		}

		now := time.Now().UTC()
		conn := &domain.Connector{
			ID:               uuid.New(),
			WorkspaceID:      input.WorkspaceID,
			TenantID:         input.TenantID,
			DefaultChannelID: channelID,
			CreatedAt:        now,
			UpdatedAt:        now,
			SecretVersion:    "v1",
		}

		logger.Info().Interface("connector", conn).Msg("Creating connector in DB")
		if err := s.repo.Create(ctx, conn); err != nil {
			logger.Error().Err(err).Msg("Failed to create connector in database")
			cleanupErr := s.secretsManager.DeleteToken(ctx, secretName)
			if cleanupErr != nil {
				logger.Error().Err(cleanupErr).
					Msg("Failed to clean up secret after DB error")
			}
			return nil, fmt.Errorf("failed to create connector: %w", err)
		}

		return conn, nil
	})
	if err != nil {
		logger.Error().Err(err).Msg("Circuit breaker execution failed for CreateConnector")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return result.(*domain.Connector), nil
}

func (s *Service) GetConnector(ctx context.Context, id uuid.UUID) (*domain.Connector, error) {
	conn, err := s.repo.GetByID(ctx, id)
	if err != nil {
		logger.Warn().Err(err).Str("connector_id", id.String()).Msg("Connector not found")
		return nil, status.Error(codes.NotFound, "connector not found")
	}
	return conn, nil
}

func (s *Service) DeleteConnector(ctx context.Context, id uuid.UUID, workspaceID, tenantID string) error {
	logger.Info().Str("connector_id", id.String()).Msg("Attempting to delete connector")

	if err := s.repo.Delete(ctx, id); err != nil {
		logger.Warn().Err(err).Str("connector_id", id.String()).Msg("Failed to delete connector in DB")
		return status.Error(codes.NotFound, "failed to delete connector")
	}

	secretName := fmt.Sprintf("connector-%s-%s", workspaceID, tenantID)
	if err := s.secretsManager.DeleteToken(ctx, secretName); err != nil {
		logger.Error().Err(err).Str("secret_name", secretName).Msg("Failed to delete token from Secrets Manager")
		return status.Error(codes.Internal, "failed to delete secret")
	}

	return nil
}

func (s *Service) GetOAuthV2URL(ctx context.Context, redirectURI string) (string, error) {
	logger.Info().Msg("Generating OAuth V2 state")
	state, err := s.oauthManager.GenerateState(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to generate OAuth state")
		return "", fmt.Errorf("failed to generate state: %w", err)
	}

	url, err := s.slackClient.GetOAuthV2URL(state)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get Slack OAuth V2 URL")
		return "", fmt.Errorf("failed to get Slack OAuth V2 URL: %w", err)
	}

	logger.Info().Msg("OAuth V2 URL generated successfully")
	return url, nil
}

func (s *Service) ExchangeOAuthCode(ctx context.Context, code string) (string, error) {
	logger.Info().Msg("Exchanging OAuth code with Slack")
	token, err := s.slackClient.ExchangeCode(ctx, code)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to exchange OAuth code with Slack")
		return "", fmt.Errorf("failed to exchange OAuth code: %w", err)
	}
	logger.Info().Msg("Slack OAuth code exchanged successfully")
	return token, nil
}
