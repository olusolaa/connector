package secretsmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/connector-recruitment/internal/domain"
	"github.com/connector-recruitment/pkg/logger"
)

type ClientOption = func(*secretsmanager.Options)

type ManagerAPI interface {
	CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)
}

type Client struct {
	client ManagerAPI
}

func WithRetryMaxAttempts(attempts int) ClientOption {
	return func(o *secretsmanager.Options) {
		o.Retryer = retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = attempts
		})
	}
}

func WithRetryMaxBackoff(duration time.Duration) ClientOption {
	return func(o *secretsmanager.Options) {
		o.Retryer = retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxBackoff = duration
		})
	}
}

func NewClient(cfg aws.Config, opts ...ClientOption) domain.SecretsManager {
	hasRetryOptions := false
	for _, opt := range opts {
		optFunc := opt
		if optFunc != nil {
			dummyOpts := &secretsmanager.Options{}
			optFunc(dummyOpts)
			if dummyOpts.Retryer != nil {
				hasRetryOptions = true
				break
			}
		}
	}

	var options []func(*secretsmanager.Options)

	if !hasRetryOptions {
		options = append(options, func(o *secretsmanager.Options) {
			o.Retryer = retry.NewStandard(func(so *retry.StandardOptions) {
				so.MaxAttempts = 3
				so.MaxBackoff = 30 * time.Second
			})
		})
	}

	options = append(options, opts...)

	return &Client{
		client: secretsmanager.NewFromConfig(cfg, options...),
	}
}

func (c *Client) StoreToken(ctx context.Context, secretName, token string) error {
	logger.Info().Str("secret_name", secretName).Msg("Attempting to create secret")
	_, err := c.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         &secretName,
		SecretString: &token,
	})
	if err != nil {
		if isResourceExistsError(err) {
			logger.Warn().Str("secret_name", secretName).Msg("Secret already exists; updating instead")
			_, err = c.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
				SecretId:     &secretName,
				SecretString: &token,
			})
			if err != nil {
				logger.Error().Err(err).Str("secret_name", secretName).Msg("Failed to update secret")
				return fmt.Errorf("update secret: %w", err)
			}
			logger.Info().Str("secret_name", secretName).Msg("Secret updated successfully")
			return nil
		}
		logger.Error().Err(err).Str("secret_name", secretName).Msg("Failed to create secret")
		return fmt.Errorf("create secret: %w", err)
	}
	return nil
}

func (c *Client) GetToken(ctx context.Context, secretName string) (string, error) {
	logger.Info().Str("secret_name", secretName).Msg("Retrieving secret value")
	out, err := c.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		logger.Error().Err(err).Str("secret_name", secretName).Msg("Failed to get secret value")
		return "", fmt.Errorf("get secret value: %w", err)
	}
	if out.SecretString == nil {
		logger.Error().Str("secret_name", secretName).Msg("Secret string is nil")
		return "", fmt.Errorf("secret string is nil")
	}
	return *out.SecretString, nil
}

func (c *Client) DeleteToken(ctx context.Context, secretName string) error {
	logger.Info().Str("secret_name", secretName).Msg("Deleting secret")
	input := &secretsmanager.DeleteSecretInput{
		SecretId:             &secretName,
		RecoveryWindowInDays: aws.Int64(30),
	}
	_, err := c.client.DeleteSecret(ctx, input)
	if err != nil {
		logger.Error().Err(err).Str("secret_name", secretName).Msg("Failed to delete secret")
		return fmt.Errorf("delete secret: %w", err)
	}
	logger.Info().Str("secret_name", secretName).Msg("Secret deletion initiated")
	return nil
}

func isResourceExistsError(err error) bool {
	var resourceExistsErr *types.ResourceExistsException
	return errors.As(err, &resourceExistsErr)
}
