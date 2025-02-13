package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ConnectorRepository interface {
	Create(ctx context.Context, c *Connector) error
	GetByID(ctx context.Context, id uuid.UUID) (*Connector, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListConnectors(ctx context.Context, limit int, cursor *ListCursor) ([]*Connector, *ListCursor, error)
	UpdateConnector(ctx context.Context, id uuid.UUID, token string) error
}

type ListCursor struct {
	UpdatedAt time.Time
	ID        uuid.UUID
}

type SecretsManager interface {
	StoreToken(ctx context.Context, secretName, token string) error
	GetToken(ctx context.Context, secretName string) (string, error)
	DeleteToken(ctx context.Context, secretName string) error
}

type SlackClient interface {
	ResolveChannelID(ctx context.Context, token, channelName string) (string, error)
	SendMessage(ctx context.Context, token, channelID, message string) error
	ExchangeCode(ctx context.Context, code string) (string, error)
	GetOAuthV2URL(state string) (string, error)
}
