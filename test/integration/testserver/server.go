package testserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/connector-recruitment/internal/infrastructure/redis"
	"github.com/connector-recruitment/pkg/logger"
	"github.com/connector-recruitment/pkg/resilience"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	httpTransport "github.com/connector-recruitment/internal/transport/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/connector-recruitment/internal/app/config"
	"github.com/connector-recruitment/internal/app/connector"
	"github.com/connector-recruitment/internal/domain"
	"github.com/connector-recruitment/internal/infrastructure/aws/secretsmanager"
	pg "github.com/connector-recruitment/internal/infrastructure/postgres" // alias for postgres package
	grpcHandler "github.com/connector-recruitment/internal/transport/grpc"
	conV1 "github.com/connector-recruitment/proto/gen/connector/v1"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

///////////////////////////////////////////////////////////////////////////////
// PostgreSQL Container
///////////////////////////////////////////////////////////////////////////////

type PostgresContainer struct {
	Container testcontainers.Container
	DSN       string
}

func NewPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("failed to get caller info")
	}
	initSQLPath := filepath.Join(filepath.Dir(filename), "../../../migrations/001_init.sql")

	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      initSQLPath,
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
				FileMode:          0644,
			},
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Postgres container: %w", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Postgres container host: %w", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Postgres container port: %w", err)
	}
	dsn := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())
	return &PostgresContainer{
		Container: container,
		DSN:       dsn,
	}, nil
}

func (p *PostgresContainer) Cleanup(ctx context.Context) error {
	if p.Container != nil {
		return p.Container.Terminate(ctx)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// LocalStack Secrets Manager Container
///////////////////////////////////////////////////////////////////////////////

type SecretsManagerContainer struct {
	Container testcontainers.Container
	Client    domain.SecretsManager
	Endpoint  string
}

func NewSecretsManagerContainer(ctx context.Context) (*SecretsManagerContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "localstack/localstack:latest",
		ExposedPorts: []string{"4566/tcp"},
		Env: map[string]string{
			"SERVICES":              "secretsmanager",
			"DEFAULT_REGION":        "us-east-1",
			"AWS_ACCESS_KEY_ID":     "test",
			"AWS_SECRET_ACCESS_KEY": "test",
		},
		WaitingFor: wait.ForListeningPort("4566/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start LocalStack container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "4566")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	endpoint := fmt.Sprintf("http://localhost:%s", mappedPort.Port())

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:   "aws",
			URL:           endpoint,
			SigningRegion: "us-east-1",
		}, nil
	})

	cfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithRegion("us-east-1"),
		awsConfig.WithEndpointResolverWithOptions(customResolver),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create AWS config: %w", err)
	}

	client := secretsmanager.NewClient(cfg)

	return &SecretsManagerContainer{
		Container: container,
		Client:    client,
		Endpoint:  endpoint,
	}, nil
}

func (s *SecretsManagerContainer) Cleanup(ctx context.Context) error {
	if s.Container != nil {
		logs, err := s.Container.Logs(ctx)
		if err == nil {
			if logContent, err := io.ReadAll(logs); err == nil {
				log.Printf("LocalStack container logs:\n%s", string(logContent))
			}
		}
		return s.Container.Terminate(ctx)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// Redis Container for OAuth State Manager
///////////////////////////////////////////////////////////////////////////////

type RedisContainer struct {
	Container testcontainers.Container
	Address   string
}

func NewRedisContainer(ctx context.Context) (*RedisContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Redis container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "6379")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Redis container port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Redis container host: %w", err)
	}

	return &RedisContainer{
		Container: container,
		Address:   fmt.Sprintf("%s:%s", host, mappedPort.Port()),
	}, nil
}

func (r *RedisContainer) Cleanup(ctx context.Context) error {
	if r.Container != nil {
		return r.Container.Terminate(ctx)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// IntegrationTestServer: Bringing It All Together
///////////////////////////////////////////////////////////////////////////////

type IntegrationTestServer struct {
	GrpcConn     *grpc.ClientConn
	OAuthManager *connector.OAuthStateManager
	HTTPAddress  string
}

func SetupIntegrationTestServer(t *testing.T) *IntegrationTestServer {
	// Use a cancelable context so that any background operations can be stopped.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// (1) Start Postgres
	pgContainer, err := NewPostgresContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start Postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Cleanup(ctx); err != nil {
			t.Errorf("failed to cleanup Postgres container: %v", err)
		}
	})
	db, err := pg.InitDb(ctx, pgContainer.DSN)
	if err != nil {
		t.Fatalf("failed to initialize Postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close database connection: %v", err)
		}
	})
	repo := pg.NewConnectorRepository(db)

	// (2) Start LocalStack Secrets Manager
	secretsContainer, err := NewSecretsManagerContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start LocalStack container: %v", err)
	}
	t.Cleanup(func() {
		if err := secretsContainer.Cleanup(ctx); err != nil {
			t.Errorf("failed to cleanup LocalStack container: %v", err)
		}
	})
	secretsManager := secretsContainer.Client

	// (3) Start Redis
	redisContainer, err := NewRedisContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start Redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := redisContainer.Cleanup(ctx); err != nil {
			t.Errorf("failed to cleanup Redis container: %v", err)
		}
	})
	redisClient, err := redis.IntClient(ctx, redisContainer.Address)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	t.Cleanup(func() {
		if err := redisClient.Close(); err != nil {
			t.Errorf("failed to close Redis client: %v", err)
		}
	})
	oauthManager := connector.NewOAuthStateManager(redisClient, 5*time.Minute)
	slackClient := NewFakeSlackClient()

	testConfig := &config.Config{
		CircuitBreakerInterval: 60 * time.Second,
		CircuitBreakerTimeout:  30 * time.Second,
		OAuthStateTimeout:      5 * time.Minute,
	}

	// (4) Create the main Service
	svc := connector.NewService(repo, secretsManager, slackClient,
		resilience.New("github.com/connector-recruitment", testConfig),
		connector.WithOAuthManager(oauthManager))

	//---------------------------------------------------------------------
	// gRPC Setup
	//---------------------------------------------------------------------
	grpcSvcHandler := grpcHandler.NewHandler(svc)

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	bufListener := bufconn.Listen(bufSize)

	conV1.RegisterConnectorServiceServer(grpcServer, grpcSvcHandler)

	go func() {
		if err := grpcServer.Serve(bufListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("gRPC server exited with error: %v", err)
		}
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		if err := bufListener.Close(); err != nil {
			t.Errorf("failed to close bufnet listener: %v", err)
		}
	})

	dialer := func(context.Context, string) (net.Conn, error) {
		return bufListener.Dial()
	}

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("failed to close gRPC connection: %v", err)
		}
	})

	//---------------------------------------------------------------------
	// HTTP Setup
	//---------------------------------------------------------------------
	httpHandler := httpTransport.NewHandler(svc, oauthManager)
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/health", httpHandler.Health)
	httpMux.HandleFunc("/oauth/callback", httpHandler.OAuthCallback)

	httpTestServer := httptest.NewServer(httpMux)
	t.Cleanup(httpTestServer.Close)
	httpAddr := httpTestServer.URL

	//---------------------------------------------------------------------
	// (5) Return the test server struct
	//---------------------------------------------------------------------
	return &IntegrationTestServer{
		GrpcConn:     conn,
		OAuthManager: oauthManager,
		HTTPAddress:  httpAddr,
	}
}

///////////////////////////////////////////////////////////////////////////////
// A simple Fake Slack Client implementation
///////////////////////////////////////////////////////////////////////////////

func NewFakeSlackClient() domain.SlackClient {
	return &fakeSlackClient{}
}

type fakeSlackClient struct{}

func (f *fakeSlackClient) ResolveChannelID(ctx context.Context, token, channelName string) (string, error) {
	return "C1234567890", nil
}

func (f *fakeSlackClient) SendMessage(ctx context.Context, token, channelID, message string) error {
	log.Printf("fakeSlackClient.SendMessage: token=%s, channelID=%s, message=%s", token, channelID, message)
	return nil
}

func (f *fakeSlackClient) ExchangeCode(ctx context.Context, code string) (string, error) {
	return "exchanged-token", nil
}

func (f *fakeSlackClient) GetOAuthV2URL(state string) (string, error) {
	return "https://example.com/oauth?state=" + state, nil
}
