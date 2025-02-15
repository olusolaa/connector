package connector

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/connector-recruitment/internal/domain"
	"github.com/connector-recruitment/pkg/logger"
)

type RotationService struct {
	repo           domain.ConnectorRepository
	secretsManager domain.SecretsManager
	interval       time.Duration
}

func NewRotationService(repo domain.ConnectorRepository, sm domain.SecretsManager, interval time.Duration) *RotationService {
	return &RotationService{
		repo:           repo,
		secretsManager: sm,
		interval:       interval,
	}
}

func (rs *RotationService) Start(ctx context.Context) {
	ticker := time.NewTicker(rs.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rs.rotateSecrets(ctx)
		case <-ctx.Done():
			logger.Info().Msg("Secret rotation stopped.")
			return
		}
	}
}

func (rs *RotationService) rotateSecrets(ctx context.Context) {
	logger.Info().Msg("Starting secret rotation...")

	const pageSize = 100
	var cursor *domain.ListCursor

	for {
		connectors, nextCursor, err := rs.repo.ListConnectors(ctx, pageSize, cursor)
		if err != nil {
			logger.Error().Err(err).Msg("Error fetching connectors for rotation")
			return
		}

		for _, connector := range connectors {
			connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

			if err := rs.rotateConnectorSecret(connCtx, connector); err != nil {
				logger.Warn().
					Err(err).
					Str("connector_id", connector.ID.String()).
					Msg("Error rotating secret for connector")
				cancel()
				continue
			}

			cancel()
			logger.Info().
				Str("connector_id", connector.ID.String()).
				Msg("Successfully rotated secret for connector")
		}

		if nextCursor == nil {
			break
		}

		cursor = nextCursor
	}

	logger.Info().Msg("Secret rotation completed.")
}

func (rs *RotationService) rotateConnectorSecret(ctx context.Context, connector domain.Connector) error {
	newToken, err := generateNewTokenForConnector(connector)
	if err != nil {
		return fmt.Errorf("failed to generate new token: %w", err)
	}

	secretName := fmt.Sprintf("connector-%s-%s", connector.WorkspaceID, connector.TenantID)
	if err := rs.secretsManager.StoreToken(ctx, secretName, newToken); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	if err := rs.repo.UpdateConnector(ctx, connector.ID, newToken); err != nil {
		if rollbackErr := rs.secretsManager.StoreToken(ctx, secretName, connector.SecretVersion); rollbackErr != nil {
			logger.Warn().
				Err(rollbackErr).
				Str("connector_id", connector.ID.String()).
				Msg("Failed to rollback secret for connector")
		}
		return fmt.Errorf("failed to update connector: %w", err)
	}

	return nil
}

func generateNewTokenForConnector(conn domain.Connector) (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	randomPart := base64.RawURLEncoding.EncodeToString(randomBytes)

	token := fmt.Sprintf("%s:%d:%s", conn.ID, time.Now().Unix(), randomPart)

	return token, nil
}
