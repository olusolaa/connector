package app_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/connector-recruitment/internal/app/config"
	"github.com/connector-recruitment/pkg/resilience"

	mocks2 "github.com/connector-recruitment/test/unit/mocks"

	"github.com/connector-recruitment/internal/app/connector"
	"github.com/connector-recruitment/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupServiceTest() (*mocks2.MockConnectorRepository, *mocks2.MockSecretsManager, *mocks2.MockSlackClient, *mocks2.MockRedisClient, *connector.Service) {
	repo := new(mocks2.MockConnectorRepository)
	sm := new(mocks2.MockSecretsManager)
	sc := new(mocks2.MockSlackClient)
	rd := new(mocks2.MockRedisClient)
	oauthManager := connector.NewOAuthStateManager(rd, time.Minute)
	testConfig := &config.Config{
		CircuitBreakerInterval: 60 * time.Second,
		CircuitBreakerTimeout:  30 * time.Second,
	}
	cb := resilience.New("github.com/connector-recruitment", testConfig)
	service := connector.NewService(repo, sm, sc, cb, connector.WithOAuthManager(oauthManager))
	return repo, sm, sc, rd, service
}

func TestService_CreateConnector(t *testing.T) {
	tests := []struct {
		name        string
		input       connector.CreateInput
		setupMocks  func(*mocks2.MockConnectorRepository, *mocks2.MockSecretsManager, *mocks2.MockSlackClient)
		expectError bool
	}{
		{
			name: "successful creation",
			input: connector.CreateInput{
				WorkspaceID:    "workspace123",
				TenantID:       "tenant123",
				Token:          "valid-token-12345",
				DefaultChannel: "general",
			},
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager, sc *mocks2.MockSlackClient) {
				sc.On("ResolveChannelID", mock.Anything, "valid-token-12345", "general").
					Return("C123", nil)
				sm.On("StoreToken", mock.Anything, "connector-workspace123-tenant123", "valid-token-12345").
					Return(nil)
				repo.On("Create", mock.Anything, mock.MatchedBy(func(c *domain.Connector) bool {
					return c.WorkspaceID == "workspace123" &&
						c.TenantID == "tenant123" &&
						c.DefaultChannelID == "C123"
				})).Return(nil)
			},
			expectError: false,
		},
		{
			name: "invalid input",
			input: connector.CreateInput{
				WorkspaceID: "w",
				TenantID:    "tenant123",
			},
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager, sc *mocks2.MockSlackClient) {
			},
			expectError: true,
		},
		{
			name: "channel resolution failure",
			input: connector.CreateInput{
				WorkspaceID:    "workspace123",
				TenantID:       "tenant123",
				Token:          "valid-token-12345",
				DefaultChannel: "general",
			},
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager, sc *mocks2.MockSlackClient) {
				sc.On("ResolveChannelID", mock.Anything, "valid-token-12345", "general").
					Return("", errors.New("channel not found"))
			},
			expectError: true,
		},
		{
			name: "token storage failure",
			input: connector.CreateInput{
				WorkspaceID:    "workspace123",
				TenantID:       "tenant123",
				Token:          "valid-token-12345",
				DefaultChannel: "general",
			},
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager, sc *mocks2.MockSlackClient) {
				sc.On("ResolveChannelID", mock.Anything, "valid-token-12345", "general").
					Return("C123", nil)
				sm.On("StoreToken", mock.Anything, "connector-workspace123-tenant123", "valid-token-12345").
					Return(errors.New("storage error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, sm, sc, _, service := setupServiceTest()
			tt.setupMocks(repo, sm, sc)

			connector, err := service.CreateConnector(context.Background(), tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, connector)
				assert.Equal(t, tt.input.WorkspaceID, connector.WorkspaceID)
				assert.Equal(t, tt.input.TenantID, connector.TenantID)
			}

			repo.AssertExpectations(t)
			sm.AssertExpectations(t)
			sc.AssertExpectations(t)
		})
	}
}

func TestService_GetConnector(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*mocks2.MockConnectorRepository)
		expectError bool
	}{
		{
			name: "successful retrieval",
			setupMocks: func(repo *mocks2.MockConnectorRepository) {
				connector := &domain.Connector{
					ID:               uuid.New(),
					WorkspaceID:      "workspace123",
					TenantID:         "tenant123",
					DefaultChannelID: "C123",
					CreatedAt:        time.Now(),
					UpdatedAt:        time.Now(),
					SecretVersion:    "v1",
				}
				repo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).
					Return(connector, nil)
			},
			expectError: false,
		},
		{
			name: "not found",
			setupMocks: func(repo *mocks2.MockConnectorRepository) {
				repo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).
					Return(nil, sql.ErrNoRows)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, _, _, _, service := setupServiceTest()
			tt.setupMocks(repo)

			connector, err := service.GetConnector(context.Background(), uuid.New())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, connector)
			}

			repo.AssertExpectations(t)
		})
	}
}

func TestService_DeleteConnector(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*mocks2.MockConnectorRepository, *mocks2.MockSecretsManager)
		expectError bool
	}{
		{
			name: "successful deletion",
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager) {
				repo.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).
					Return(nil)
				sm.On("DeleteToken", mock.Anything, mock.AnythingOfType("string")).
					Return(nil)
			},
			expectError: false,
		},
		{
			name: "repository deletion failure",
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager) {
				repo.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).
					Return(sql.ErrNoRows)
			},
			expectError: true,
		},
		{
			name: "secret deletion failure",
			setupMocks: func(repo *mocks2.MockConnectorRepository, sm *mocks2.MockSecretsManager) {
				repo.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).
					Return(nil)
				sm.On("DeleteToken", mock.Anything, mock.AnythingOfType("string")).
					Return(errors.New("deletion error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, sm, _, _, service := setupServiceTest()
			tt.setupMocks(repo, sm)

			err := service.DeleteConnector(context.Background(), uuid.New(), "workspace123", "tenant123")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			repo.AssertExpectations(t)
			sm.AssertExpectations(t)
		})
	}
}

func TestService_GetOAuthV2URL(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*mocks2.MockSlackClient, *mocks2.MockRedisClient)
		expectError bool
	}{
		{
			name: "successful URL generation",
			setupMocks: func(sc *mocks2.MockSlackClient, rc *mocks2.MockRedisClient) {
				rc.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).
					Return(redis.NewStatusCmd(context.Background()))
				sc.On("GetOAuthV2URL", mock.AnythingOfType("string")).
					Return("https://slack.com/oauth/v2/authorize?state=123", nil)
			},
			expectError: false,
		},
		{
			name: "URL generation failure",
			setupMocks: func(sc *mocks2.MockSlackClient, rc *mocks2.MockRedisClient) {
				rc.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).
					Return(redis.NewStatusCmd(context.Background()))
				sc.On("GetOAuthV2URL", mock.AnythingOfType("string")).
					Return("", errors.New("generation error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, sc, rd, service := setupServiceTest()
			tt.setupMocks(sc, rd)

			url, err := service.GetOAuthV2URL(context.Background(), "http://localhost/callback")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, url)
			}

			sc.AssertExpectations(t)
			rd.AssertExpectations(t)
		})
	}
}

func TestService_ExchangeOAuthCode(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*mocks2.MockSlackClient)
		expectError bool
	}{
		{
			name: "successful code exchange",
			setupMocks: func(sc *mocks2.MockSlackClient) {
				sc.On("ExchangeCode", mock.Anything, "test-code").
					Return("xoxb-test-token", nil)
			},
			expectError: false,
		},
		{
			name: "exchange failure",
			setupMocks: func(sc *mocks2.MockSlackClient) {
				sc.On("ExchangeCode", mock.Anything, "test-code").
					Return("", errors.New("exchange error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, sc, _, service := setupServiceTest()
			tt.setupMocks(sc)

			token, err := service.ExchangeOAuthCode(context.Background(), "test-code")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, token)
			}

			sc.AssertExpectations(t)
		})
	}
}
