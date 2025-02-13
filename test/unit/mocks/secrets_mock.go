package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockSecretsManager struct {
	mock.Mock
}

func (m *MockSecretsManager) StoreToken(ctx context.Context, secretName, token string) error {
	args := m.Called(ctx, secretName, token)
	return args.Error(0)
}

func (m *MockSecretsManager) GetToken(ctx context.Context, secretName string) (string, error) {
	args := m.Called(ctx, secretName)
	return args.String(0), args.Error(1)
}

func (m *MockSecretsManager) DeleteToken(ctx context.Context, secretName string) error {
	args := m.Called(ctx, secretName)
	return args.Error(0)
}
