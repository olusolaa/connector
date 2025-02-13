package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/connector-recruitment/internal/domain"
)

type MockConnectorRepository struct {
	mock.Mock
}

func (m *MockConnectorRepository) Create(ctx context.Context, c *domain.Connector) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *MockConnectorRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Connector, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Connector), args.Error(1)
}

func (m *MockConnectorRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockConnectorRepository) ListConnectors(ctx context.Context, limit int, cursor *domain.ListCursor) ([]*domain.Connector, *domain.ListCursor, error) {
	args := m.Called(ctx, limit, cursor)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*domain.Connector), args.Get(1).(*domain.ListCursor), args.Error(2)
}

func (m *MockConnectorRepository) UpdateConnector(ctx context.Context, id uuid.UUID, token string) error {
	args := m.Called(ctx, id, token)
	return args.Error(0)
}
