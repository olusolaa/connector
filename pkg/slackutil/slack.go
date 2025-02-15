package slackutil

import (
	"context"
	"fmt"
	"log"

	"github.com/connector-recruitment/internal/domain"
	pgRepo "github.com/connector-recruitment/internal/infrastructure/postgres"
	"github.com/jmoiron/sqlx"
)

func SendSlackMessage(ctx context.Context, connectorID string, message string, db *sqlx.DB, smClient domain.SecretsManager, slackClient domain.SlackClient) error {
	repository := pgRepo.NewConnectorRepository(db)
	id, err := domain.ParseUUID(connectorID)
	if err != nil {
		return fmt.Errorf("invalid connector id: %w", err)
	}
	conn, err := repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to retrieve connector: %w", err)
	}

	secretName := fmt.Sprintf("connector-%s-%s", conn.WorkspaceID, conn.TenantID)
	token, err := smClient.GetToken(ctx, secretName)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if err := slackClient.SendMessage(ctx, token, conn.DefaultChannelID, message); err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}

	log.Printf("Message sent successfully to channel %s", conn.DefaultChannelID)
	return nil
}
