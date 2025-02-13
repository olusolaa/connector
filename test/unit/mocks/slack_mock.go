package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockSlackClient struct {
	mock.Mock
}

func (m *MockSlackClient) ResolveChannelID(ctx context.Context, token, channelName string) (string, error) {
	args := m.Called(ctx, token, channelName)
	return args.String(0), args.Error(1)
}

func (m *MockSlackClient) SendMessage(ctx context.Context, token, channelID, message string) error {
	args := m.Called(ctx, token, channelID, message)
	return args.Error(0)
}

func (m *MockSlackClient) ExchangeCode(ctx context.Context, code string) (string, error) {
	args := m.Called(ctx, code)
	return args.String(0), args.Error(1)
}

func (m *MockSlackClient) GetOAuthV2URL(state string) (string, error) {
	args := m.Called(state)
	return args.String(0), args.Error(1)
}
