package grpc

import (
	"context"
	"fmt"

	"github.com/connector-recruitment/internal/app/connector"
	"github.com/connector-recruitment/internal/domain"
	"github.com/connector-recruitment/pkg/logger"
	connectorv1 "github.com/connector-recruitment/proto/gen/connector/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	connectorv1.UnimplementedConnectorServiceServer
	service *connector.Service
}

func NewHandler(svc *connector.Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) CreateConnector(ctx context.Context, req *connectorv1.CreateConnectorRequest) (*connectorv1.CreateConnectorResponse, error) {
	logger.Info().Msg("Received CreateConnector gRPC request")

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("CreateConnector request validation failed")
		return nil, connector.GRPCError(err)
	}
	input := connector.CreateInput{
		WorkspaceID:    req.WorkspaceId,
		TenantID:       req.TenantId,
		Token:          req.Token,
		DefaultChannel: req.DefaultChannelName,
	}
	conn, err := h.service.CreateConnector(ctx, input)
	if err != nil {
		return nil, connector.GRPCError(err)
	}

	return &connectorv1.CreateConnectorResponse{
		Connector: domainToProto(conn),
	}, nil
}

func (h *Handler) GetConnector(ctx context.Context, req *connectorv1.GetConnectorRequest) (*connectorv1.GetConnectorResponse, error) {
	logger.Info().Str("connector_id", req.Id).Msg("Received GetConnector gRPC request")

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("GetConnector request validation failed")
		return nil, connector.GRPCError(err)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connector.GRPCError(fmt.Errorf("invalid connector ID: %w", err))
	}
	connectorObj, err := h.service.GetConnector(ctx, id)
	if err != nil {
		return nil, connector.GRPCError(err)
	}
	return &connectorv1.GetConnectorResponse{
		Connector: domainToProto(connectorObj),
	}, nil
}

func (h *Handler) DeleteConnector(ctx context.Context, req *connectorv1.DeleteConnectorRequest) (*connectorv1.DeleteConnectorResponse, error) {
	logger.Info().Str("connector_id", req.Id).Msg("Received DeleteConnector gRPC request")

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("DeleteConnector request validation failed")
		return nil, connector.GRPCError(err)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connector.GRPCError(fmt.Errorf("invalid connector ID: %w", err))
	}
	if err := h.service.DeleteConnector(ctx, id, req.WorkspaceId, req.TenantId); err != nil {
		return nil, connector.GRPCError(err)
	}

	return &connectorv1.DeleteConnectorResponse{
		Message: "Connector deleted successfully",
	}, nil
}

func (h *Handler) GetOAuthV2URL(ctx context.Context, req *connectorv1.GetOAuthV2URLRequest) (*connectorv1.GetOAuthV2URLResponse, error) {
	logger.Info().Msg("Received GetOAuthV2URL gRPC request")

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("GetOAuthV2URL request validation failed")
		return nil, connector.GRPCError(err)
	}

	url, err := h.service.GetOAuthV2URL(ctx, req.RedirectUri)
	if err != nil {
		return nil, connector.GRPCError(err)
	}

	return &connectorv1.GetOAuthV2URLResponse{
		Url: url,
	}, nil
}

func (h *Handler) ExchangeOAuthCode(ctx context.Context, req *connectorv1.ExchangeOAuthCodeRequest) (*connectorv1.ExchangeOAuthCodeResponse, error) {
	logger.Info().Msg("Received ExchangeOAuthCode gRPC request")

	if err := req.Validate(); err != nil {
		logger.Warn().Err(err).Msg("ExchangeOAuthCode request validation failed")
		return nil, connector.GRPCError(err)
	}

	token, err := h.service.ExchangeOAuthCode(ctx, req.Code)
	if err != nil {
		return nil, connector.GRPCError(err)
	}

	return &connectorv1.ExchangeOAuthCodeResponse{
		AccessToken: token,
	}, nil
}

func domainToProto(conn *domain.Connector) *connectorv1.Connector {
	return &connectorv1.Connector{
		Id:               conn.ID.String(),
		WorkspaceId:      conn.WorkspaceID,
		TenantId:         conn.TenantID,
		DefaultChannelId: conn.DefaultChannelID,
		CreatedAt:        timestamppb.New(conn.CreatedAt),
		UpdatedAt:        timestamppb.New(conn.UpdatedAt),
		SecretVersion:    conn.SecretVersion,
	}
}
