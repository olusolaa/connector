package testserver

import (
	"context"
	"database/sql"
	"fmt"
	httpTransport "github.com/connector-recruitment/internal/transport/http"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/connector-recruitment/internal/app/connector"
	"github.com/connector-recruitment/internal/domain"
	"github.com/connector-recruitment/internal/infrastructure/aws/secretsmanager"
	pg "github.com/connector-recruitment/internal/infrastructure/postgres" // alias for postgres package
	grpcHandler "github.com/connector-recruitment/internal/transport/grpc"
	conV1 "github.com/connector-recruitment/proto/gen/connector/v1"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

///////////////////////////////////////////////////////////////////////////////
// PostgreSQL Container
///////////////////////////////////////////////////////////////////////////////

// PostgresContainer encapsulates a test container running PostgreSQL.
type PostgresContainer struct {
	Container testcontainers.Container
	DSN       string
}

func NewPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
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
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Postgres container host: %w", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		container.Terminate(ctx)
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
		container.Terminate(ctx)
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

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		container.Terminate(ctx)
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

func NewRedisContainer(ctx context.Context) (*RedisContainer, *redis.Client, error) {
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
		return nil, nil, fmt.Errorf("failed to start Redis container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "6379")
	if err != nil {
		container.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to get Redis container port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to get Redis container host: %w", err)
	}

	address := fmt.Sprintf("%s:%s", host, mappedPort.Port())

	rdb := redis.NewClient(&redis.Options{
		Addr: address,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		container.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &RedisContainer{
		Container: container,
		Address:   address,
	}, rdb, nil
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
	GrpcConn          *grpc.ClientConn
	GrpcServer        *grpc.Server
	BufListener       *bufconn.Listener
	Repository        domain.ConnectorRepository
	SecretsManager    domain.SecretsManager
	SecretsContainer  *SecretsManagerContainer
	RedisClient       *redis.Client
	RedisContainer    *RedisContainer
	DB                *sql.DB
	PostgresContainer *PostgresContainer
	Service           *connector.Service
	OAuthManager      *connector.OAuthStateManager
	HTTPTestServer    *httptest.Server
	HTTPAddress       string
	CleanupFunc       func()
}

func SetupIntegrationTestServer(t *testing.T) *IntegrationTestServer {
	ctx := context.Background()

	// (1) Start Postgres, LocalStack, Redis, etc. (unchanged)
	pgContainer, err := NewPostgresContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start Postgres container: %v", err)
	}
	db, err := pg.InitDatabase(ctx, pgContainer.DSN)
	if err != nil {
		t.Fatalf("failed to initialize Postgres: %v", err)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to get caller info")
	}
	migrationsPath := filepath.Join(filepath.Dir(filename), "../../../migrations")
	if err := pg.RunMigrations(db, migrationsPath); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	repo := pg.NewConnectorRepository(db)

	secretsContainer, err := NewSecretsManagerContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start LocalStack container: %v", err)
	}
	secretsManager := secretsContainer.Client

	redisContainer, redisClient, err := NewRedisContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start Redis container: %v", err)
	}

	oauthManager := connector.NewOAuthStateManager(redisClient, 5*time.Minute)
	slackClient := NewFakeSlackClient()

	// (2) Create the main Service
	svc := connector.NewService(repo, secretsManager, slackClient, connector.WithOAuthManager(oauthManager))

	//---------------------------------------------------------------------
	// gRPC Setup
	//---------------------------------------------------------------------
	grpcHandler := grpcHandler.NewHandler(svc)
	bufListener := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	conV1.RegisterConnectorServiceServer(grpcServer, grpcHandler)

	go func() {
		if err := grpcServer.Serve(bufListener); err != nil {
			log.Fatalf("gRPC server exited with error: %v", err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return bufListener.Dial()
	}
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}

	//---------------------------------------------------------------------
	// (3) HTTP Setup
	//---------------------------------------------------------------------
	httpHandler := httpTransport.NewHandler(svc, oauthManager)

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/health", httpHandler.Health)
	httpMux.HandleFunc("/oauth/callback", httpHandler.OAuthCallback)

	httpTestServer := httptest.NewServer(httpMux)
	httpAddr := httpTestServer.URL

	//---------------------------------------------------------------------
	// (4) Define Cleanup
	//---------------------------------------------------------------------
	cleanup := func() {
		httpTestServer.Close()

		conn.Close()
		grpcServer.Stop()
		bufListener.Close()

		if db != nil {
			db.Close()
		}
		if err := secretsContainer.Cleanup(ctx); err != nil {
			t.Errorf("failed to cleanup LocalStack container: %v", err)
		}
		if err := redisContainer.Cleanup(ctx); err != nil {
			t.Errorf("failed to cleanup Redis container: %v", err)
		}
		if err := pgContainer.Cleanup(ctx); err != nil {
			t.Errorf("failed to cleanup Postgres container: %v", err)
		}
	}

	//---------------------------------------------------------------------
	// (5) Return the test server struct
	//---------------------------------------------------------------------
	return &IntegrationTestServer{
		GrpcConn:          conn,
		GrpcServer:        grpcServer,
		BufListener:       bufListener,
		Repository:        repo,
		SecretsManager:    secretsManager,
		SecretsContainer:  secretsContainer,
		RedisClient:       redisClient,
		RedisContainer:    redisContainer,
		DB:                db,
		PostgresContainer: pgContainer,
		Service:           svc,
		OAuthManager:      oauthManager,
		HTTPTestServer:    httpTestServer,
		HTTPAddress:       httpAddr,
		CleanupFunc:       cleanup,
	}
}

///////////////////////////////////////////////////////////////////////////////
// A simple Fake Slack Client implementation
///////////////////////////////////////////////////////////////////////////////

// NewFakeSlackClient returns a dummy Slack client.
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
