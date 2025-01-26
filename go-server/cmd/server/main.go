package server

import (
	"context"
	"log/slog"
	"net"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"google.golang.org/grpc"
)

func main() {
	// Setup AWS session for LocalStack
	awsConfig := &aws.Config{
		Credentials:      credentials.NewStaticCredentials("test", "test", ""),
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String("http://localhost:4566"), // LocalStack default
		S3ForcePathStyle: aws.Bool(true),
	}
	sess := session.Must(session.NewSession(awsConfig))
	smClient := secretsmanager.New(sess)

	_, err := smClient.CreateSecretWithContext(context.Background(), &secretsmanager.CreateSecretInput{
		Name:         aws.String("slack-connector"),
		SecretString: aws.String("my-super-secret"),
	})
	if err != nil {
		slog.Error("failed to create secret", "err", err)
	}

	// Create a new gRPC server
	grpcServer := grpc.NewServer()

	// Listen on port 50051
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		slog.Error("failed to listen: %v", err)
	}

	slog.Info("Starting Slack Connector gRPC server on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("failed to serve: %v", err)
	}
}
