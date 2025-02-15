package connector

import (
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrInvalidInput = errors.New("invalid input provided")
	ErrNotFound     = errors.New("record not found")
)

func GRPCError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func handleAWSError(err error) error {
	var notFoundErr *types.ResourceNotFoundException
	var invalidRequestErr *types.InvalidRequestException
	switch {
	case errors.As(err, &notFoundErr):
		return ErrNotFound
	case errors.As(err, &invalidRequestErr):
		return ErrInvalidInput
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
