package grpc

import (
	"context"
	"net"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	"github.com/connector-recruitment/internal/app/connector"
	connectorv1 "github.com/connector-recruitment/proto/gen/connector/v1"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var limiter = rate.NewLimiter(rate.Limit(10), 10)

func rateLimitingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	if err := limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func NewServer(svc *connector.Service, grpcPort string) (*grpc.Server, net.Listener, error) {
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			rateLimitingInterceptor,
			otelgrpc.UnaryServerInterceptor(),
		),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)

	handler := NewHandler(svc)
	connectorv1.RegisterConnectorServiceServer(server, handler)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	reflection.Register(server)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		return nil, nil, err
	}
	return server, lis, nil
}
