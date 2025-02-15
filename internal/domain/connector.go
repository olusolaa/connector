package domain

import (
	"time"

	"github.com/google/uuid"
)

type Connector struct {
	ID               uuid.UUID `db:"id"`
	WorkspaceID      string    `db:"workspace_id"`
	TenantID         string    `db:"tenant_id"`
	DefaultChannelID string    `db:"default_channel_id"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	SecretVersion    string    `db:"secret_version"`
}

func ParseUUID(id string) (uuid.UUID, error) {
	return uuid.Parse(id)
}
