package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mocks2 "github.com/connector-recruitment/test/unit/mocks"

	"github.com/connector-recruitment/internal/app/connector"
	"github.com/connector-recruitment/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

func TestRotationService_Start(t *testing.T) {
	tests := []struct {
		name        string
		connector   *domain.Connector
		setupMocks  func(*mocks2.MockConnectorRepository, *mocks2.MockSecretsManager)
		expectError bool
	}{
		{
			name: "successful rotation",
			connector: &domain.Connector{
				ID:               uuid.New(),
				WorkspaceID:      "workspace1",
				TenantID:         "tenant1",
				DefaultChannelID: "C123",
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
				SecretVersion:    "v1",
			},
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager) {
				repo.On("ListConnectors", mock.Anything, mock.Anything, mock.Anything).
					Return([]*domain.Connector{
						{
							ID:               uuid.New(),
							WorkspaceID:      "workspace1",
							TenantID:         "tenant1",
							DefaultChannelID: "C123",
							CreatedAt:        time.Now(),
							UpdatedAt:        time.Now(),
							SecretVersion:    "v1",
						},
					}, &domain.ListCursor{}, nil)
				sm.On("StoreToken", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
					Return(nil)
				repo.On("UpdateConnector", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
					Return(nil)
			},
			expectError: false,
		},
		{
			name: "list connectors failure",
			connector: &domain.Connector{
				ID:               uuid.New(),
				WorkspaceID:      "workspace1",
				TenantID:         "tenant1",
				DefaultChannelID: "C123",
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
				SecretVersion:    "v1",
			},
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager) {
				repo.On("ListConnectors", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, nil, errors.New("list error"))
			},
			expectError: true,
		},
		{
			name: "store token failure",
			connector: &domain.Connector{
				ID:               uuid.New(),
				WorkspaceID:      "workspace1",
				TenantID:         "tenant1",
				DefaultChannelID: "C123",
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
				SecretVersion:    "v1",
			},
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager) {
				repo.On("ListConnectors", mock.Anything, mock.Anything, mock.Anything).
					Return([]*domain.Connector{
						{
							ID:               uuid.New(),
							WorkspaceID:      "workspace1",
							TenantID:         "tenant1",
							DefaultChannelID: "C123",
							CreatedAt:        time.Now(),
							UpdatedAt:        time.Now(),
							SecretVersion:    "v1",
						},
					}, &domain.ListCursor{}, nil)
				sm.On("StoreToken", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
					Return(errors.New("store error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mocks2.MockConnectorRepository)
			sm := new(mocks2.MockSecretsManager)
			tt.setupMocks(repo, sm)

			service := connector.NewRotationService(repo, sm, time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
			defer cancel()

			go service.Start(ctx)
			<-ctx.Done()

			repo.AssertExpectations(t)
			sm.AssertExpectations(t)
		})
	}
}
