package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/connector-recruitment/internal/app/connector"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRedisClient struct {
	mock.Mock
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.Called(ctx, key, value, expiration)
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("OK")
	return cmd
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.Called(ctx, keys[0])
	cmd := redis.NewIntCmd(ctx)
	if keys[0] == "oauth_state:invalid-state" {
		cmd.SetVal(0)
	} else {
		cmd.SetVal(1)
	}
	return cmd
}

func setupOAuthTest() (*mockRedisClient, *connector.OAuthStateManager) {
	client := new(mockRedisClient)
	manager := connector.NewOAuthStateManager(client, time.Minute)
	return client, manager
}

func TestOAuthStateManager_GenerateState(t *testing.T) {
	client, manager := setupOAuthTest()
	ctx := context.Background()

	client.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	state, err := manager.GenerateState(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, state)
	assert.Len(t, state, 43)

	client.AssertExpectations(t)
}

func TestOAuthStateManager_ValidateState(t *testing.T) {
	client, manager := setupOAuthTest()
	ctx := context.Background()

	client.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	client.On("Del", mock.Anything, mock.Anything).Return(nil)

	state, err := manager.GenerateState(ctx)
	require.NoError(t, err)

	err = manager.ValidateState(ctx, state)
	assert.NoError(t, err)

	err = manager.ValidateState(ctx, "invalid-state")
	assert.Error(t, err)

	client.AssertExpectations(t)
}
