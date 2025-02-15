package domain_test

import (
	"errors"
	"testing"

	"github.com/connector-recruitment/internal/app/connector"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPCError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expectedCode  codes.Code
		expectedError string
	}{
		{
			name:          "invalid input error",
			err:           connector.ErrInvalidInput,
			expectedCode:  codes.InvalidArgument,
			expectedError: connector.ErrInvalidInput.Error(),
		},
		{
			name:          "connector not found error",
			err:           connector.ErrNotFound,
			expectedCode:  codes.NotFound,
			expectedError: connector.ErrNotFound.Error(),
		},
		{
			name:          "wrapped invalid input error",
			err:           errors.Join(errors.New("wrapped"), connector.ErrInvalidInput),
			expectedCode:  codes.InvalidArgument,
			expectedError: "wrapped\ninvalid input provided",
		},
		{
			name:          "wrapped not found error",
			err:           errors.Join(errors.New("wrapped"), connector.ErrNotFound),
			expectedCode:  codes.NotFound,
			expectedError: "wrapped\nrecord not found",
		},
		{
			name:          "unknown error",
			err:           errors.New("unknown error"),
			expectedCode:  codes.Internal,
			expectedError: "internal server error",
		},
		{
			name:          "nil error",
			err:           nil,
			expectedCode:  codes.Internal,
			expectedError: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcErr := connector.GRPCError(tt.err)
			st, ok := status.FromError(grpcErr)
			assert.True(t, ok, "expected gRPC status error")
			assert.Equal(t, tt.expectedCode, st.Code())
			assert.Equal(t, tt.expectedError, st.Message())
		})
	}
}
