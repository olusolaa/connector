package connector

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

type OAuthStateManager struct {
	validPeriod time.Duration
	redisClient RedisClient
}

func NewOAuthStateManager(redisClient RedisClient, validPeriod time.Duration) *OAuthStateManager {
	return &OAuthStateManager{
		validPeriod: validPeriod,
		redisClient: redisClient,
	}
}

func (om *OAuthStateManager) GenerateState(ctx context.Context) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(buf)

	err := om.redisClient.Set(ctx, fmt.Sprintf("oauth_state:%s", state), "1", om.validPeriod).Err()
	if err != nil {
		return "", fmt.Errorf("store state: %w", err)
	}

	return state, nil
}

func (om *OAuthStateManager) ValidateState(ctx context.Context, state string) error {
	key := fmt.Sprintf("oauth_state:%s", state)
	deleted, err := om.redisClient.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("validate state: %w", err)
	}
	if deleted == 0 {
		return errors.New("state not found or expired")
	}
	return nil
}
