package domain_test

import (
	"testing"
	"time"

	"github.com/connector-recruitment/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestParseUUID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Valid UUID",
			input:   "123e4567-e89b-12d3-a456-426614174000",
			wantErr: false,
		},
		{
			name:    "Invalid UUID",
			input:   "invalid-uuid",
			wantErr: true,
		},
		{
			name:    "Empty UUID",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ParseUUID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.input, got.String())
			}
		})
	}
}

func TestConnector_Validation(t *testing.T) {
	validUUID := uuid.New()
	now := time.Now()

	tests := []struct {
		name      string
		connector domain.Connector
		isValid   bool
	}{
		{
			name: "Valid Connector",
			connector: domain.Connector{
				ID:               validUUID,
				WorkspaceID:      "workspace1",
				TenantID:         "tenant1",
				DefaultChannelID: "channel1",
				CreatedAt:        now,
				UpdatedAt:        now,
				SecretVersion:    "v1",
			},
			isValid: true,
		},
		{
			name: "Missing WorkspaceID",
			connector: domain.Connector{
				ID:               validUUID,
				TenantID:         "tenant1",
				DefaultChannelID: "channel1",
				CreatedAt:        now,
				UpdatedAt:        now,
				SecretVersion:    "v1",
			},
			isValid: false,
		},
		{
			name: "Missing TenantID",
			connector: domain.Connector{
				ID:               validUUID,
				WorkspaceID:      "workspace1",
				DefaultChannelID: "channel1",
				CreatedAt:        now,
				UpdatedAt:        now,
				SecretVersion:    "v1",
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.connector.ID)
			if tt.isValid {
				assert.NotEmpty(t, tt.connector.WorkspaceID)
				assert.NotEmpty(t, tt.connector.TenantID)
				assert.NotEmpty(t, tt.connector.DefaultChannelID)
				assert.False(t, tt.connector.CreatedAt.IsZero())
				assert.False(t, tt.connector.UpdatedAt.IsZero())
			}
		})
	}
}
