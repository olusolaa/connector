package domain

import (
	"time"

	"github.com/google/uuid"
)

type Connector struct {
	ID               uuid.UUID
	WorkspaceID      string
	TenantID         string
	DefaultChannelID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	SecretVersion    string
}

func ParseUUID(id string) (uuid.UUID, error) {
	return uuid.Parse(id)
}
