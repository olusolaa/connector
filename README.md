# Connector Service

A gRPC service that manages Slack connectors with secure token storage and PostgreSQL metadata.

## Table of Contents
- [Quick Start](#quick-start)
- [Services](#services)
- [Endpoints](#endpoints)
    - [OAuth Flow](#1-oauth-flow)
    - [Create Connector](#2-create-connector)
    - [Get Connector](#3-get-connector)
    - [Delete Connector](#4-delete-connector)
    - [CLI Tool](#5-cli-tool)
- [Testing Guide](#testing-guide)
    - [Prerequisites](#prerequisites)
    - [Complete Testing Flow](#complete-testing-flow)
    - [Error Cases](#error-cases)
- [Development](#development)
    - [Project Structure](#project-structure)
    - [Development Commands](#development-commands)
- [Troubleshooting](#troubleshooting)

## Quick Start

1. **Prerequisites**
    - Docker and Docker Compose

2. **Setup**
   ```bash
   # Clone and enter the repository
   git clone <repository-url>
   cd connector-recruitment

   # Start the service
   ./setup.sh

   # Check the actual port mappings (IMPORTANT)
   docker-compose ps
   ```

The setup script will:
- Check for Docker requirements
- Prompt for Slack credentials (or use defaults)
- Start all required services
- Provide usage instructions

**Important Note About Ports**: When Docker starts the services, it may map the default ports to different values to avoid conflicts:
- gRPC Server: 50051 → may be mapped to another port (e.g., 50052)
- HTTP Server: 8080 → may be mapped to another port (e.g., 8081)
- LocalStack: 4566 → may be mapped to another port (e.g., 4567)
- PostgreSQL: 5432 → may be mapped to another port (e.g., 5433)
- Redis: 6379 → may be mapped to another port (e.g., 6380)

Always check the actual port mappings using `docker-compose ps` before running any commands.

## Project Structure
```
.
├── cmd/                    # Application entrypoints
│   ├── server/            # gRPC server
│   ├── httpserver/        # HTTP server for OAuth
│   └── cli/              # CLI tools
├── internal/              # Private application code
│   ├── app/              # Application logic
│   │   ├── config/       # Configuration
│   │   └── connector/    # Connector service
│   ├── domain/           # Business logic and interfaces
│   ├── infrastructure/   # External services implementation
│   │   ├── aws/         # AWS services
│   │   ├── postgres/    # Database
│   │   ├── redis/       # Cache
│   │   └── slack/       # Slack client
│   └── transport/       # Transport layer
│       └── grpc/        # gRPC implementation
├── pkg/                  # Public libraries
│   ├── resilience/      # Circuit breaker
│   ├── observability/   # Tracing and metrics
│   └── slackutil/       # Slack utilities
├── proto/               # Protocol buffer definitions
│   └── connector/       # Connector service protos
├── docker-compose.yml   # Docker services configuration
├── Dockerfile          # Main service Dockerfile
└── setup.sh           # Setup script
```

## Services

Once running, the following services are available:
- gRPC Server: `localhost:50051`
- HTTP Server: `localhost:8080`
- LocalStack: `localhost:4566`
- PostgreSQL: `localhost:5432`
- Redis: `localhost:6379`
- 
**Important**: When Docker starts the services, it may map the ports to different values to avoid conflicts. Always check the actual port mappings using `docker-compose ps`.

## Endpoints

### 1. OAuth Flow

Get OAuth URL and exchange code for token.

#### Get OAuth URL

**Request:**
```json
{
  "redirect_uri": "https://localhost:8080/oauth/callback"
}
```

**Response:**
```json
{
  "url": "https://slack.com/oauth/v2/authorize?client_id=7656730043137.8419590983429&scope=chat:write,channels:read&..."
}
```

**Example:**
```bash
# Replace 50051 with the actual mapped gRPC port from docker-compose ps
grpcurl -plaintext\
    -d '{
        "redirect_uri": "https://localhost:8080/oauth/callback"
    }' \
    localhost:50051 connector.v1.ConnectorService/GetOAuthV2URL | jq
```

#### Exchange OAuth Code

**Request:**
```json
{
  "code": "received_oauth_code"
}
```

**Response:**
```json
{
  "access_token": "xoxb-your-access-token"
}
```

**Example:**
```bash
# Replace 50051 with the actual mapped gRPC port from docker-compose ps
grpcurl -plaintext -d '{"code": "received_oauth_code"}' \
localhost:50051 connector.v1.ConnectorService/ExchangeOAuthCode | jq
```

### 2. Create Connector

Creates a new Slack connector with the specified configuration.

**Request:**
```json
{
  "workspace_id": "7656730043137",
  "tenant_id": "your-organization-id",
  "token": "xoxb-7656730043137-8419596279637-p6icqGwKPLsewHC9eALXrjl3",
  "default_channel_name": "all-moneta"
}
```

**Response:**
```json
{
  "connector": {
    "id": "0d2f2d58-c66d-442b-9dad-7b6793e21f8e",
    "workspace_id": "7656730043137",
    "tenant_id": "your-organization-id",
    "default_channel_id": "C07JRCP2S3Y",
    "created_at": "2024-02-09T12:54:17.035972Z",
    "updated_at": "2024-02-09T12:54:17.035972Z",
    "secret_version": "v1"
  }
}
```

**Example:**
```bash
# Replace 50051 with the actual mapped gRPC port from docker-compose ps
grpcurl -plaintext -d '{
  "workspace_id": "7656730043137",
  "tenant_id": "your-organization-id",
  "token": "<token_from_oauth_exchange>",
  "default_channel_name": "all-moneta"
}' localhost:50051 connector.v1.ConnectorService/CreateConnector
```

### 3. Get Connector

Retrieves a connector by its ID.

**Request:**
```json
{
  "id": "0d2f2d58-c66d-442b-9dad-7b6793e21f8e"
}
```

**Response:**
```json
{
  "connector": {
    "id": "0d2f2d58-c66d-442b-9dad-7b6793e21f8e",
    "workspace_id": "7656730043137",
    "tenant_id": "your-organization-id",
    "default_channel_id": "C07JRCP2S3Y",
    "created_at": "2024-02-09T12:54:17.035972Z",
    "updated_at": "2024-02-09T12:54:17.035972Z",
    "secret_version": "v1"
  }
}
```

**Example:**
```bash
# Replace 50051 with the actual mapped gRPC port from docker-compose ps
grpcurl -plaintext -emit-defaults \
    -d '{                 
        "id": "<token_from_oauth_exchange>"
    }' \
    localhost:50051 connector.v1.ConnectorService/GetConnector | jq
```

### 4. Delete Connector

Deletes a connector and its associated resources.

**Request:**
```json
{
  "id": "0d2f2d58-c66d-442b-9dad-7b6793e21f8e",
  "workspace_id": "7656730043137",
  "tenant_id": "your-organization-id"
}
```

**Response:**
```json
{
  "message": "Connector deleted successfully"
}
```

**Example:**
```bash
# Replace 50051 with the actual mapped gRPC port from docker-compose ps
grpcurl -plaintext -emit-defaults \
    -d '{
        "id": "<connector_id>",
        "workspace_id": "7656730043137",
        "tenant_id": "440"
    }' \
    localhost:50051 connector.v1.ConnectorService/DeleteConnector | jq
```

### 5. CLI Tool

The service includes a CLI tool for sending messages to Slack channels through a connector.

**Usage:**
```bash
DB_DSN="postgres://aryon:aryon@localhost:5433/aryondb?sslmode=disable" \
AWS_ENDPOINT="http://localhost:4567" go run go-server/cmd/cli/main.go \
--connector-id="50115dfb-23cb-4de4-b3db-77217c8c7c26" --message="Test message from connector service"
```

**Note**: The port numbers (5433 for PostgreSQL, 4567 for LocalStack) might be different on your system. Use `docker-compose ps` to check the actual port mappings.

**Parameters:**
- `--connector-id`: The UUID of the connector to use (required)
- `--message`: The message to send to the default Slack channel (required)



**Error Cases:**
- If either `--connector-id` or `--message` is missing, the CLI will return an error
- If the connector ID doesn't exist, the CLI will return an error
- If the Slack token is invalid or expired, the CLI will return an error

### Expected Results

1. **OAuth Flow**
    - OAuth URL generation should succeed
    - Code exchange should return valid token

2. **Connector Management**
    - Creation should succeed with valid data
    - Creation should fail with invalid data
    - Retrieval should succeed for valid ID
    - Retrieval should fail for invalid ID

3. **Message Testing**
    - Message sending should succeed with valid connector
    - Message sending should fail with invalid connector

4. **Cleanup Operations**
    - Deletion should succeed for existing connector
    - Deletion should fail for non-existent connector
    - Get after deletion should fail

### Important Notes

1. Replace placeholders in commands:
    - `<received_oauth_code>` with actual OAuth code
    - `<token_from_oauth_exchange>` with received token
    - `<connector_id>` with ID from create response

2. Steps are sequential and dependent:
    - OAuth flow must complete before creating connector
    - Connector must exist before sending messages
    - Clean up after testing to remove test data

3. For debugging:
    - View logs: `docker-compose logs -f app`
    - Access shell: `docker-compose exec app bash`
    - Restart services: `docker-compose restart`

## Development

### Development Commands

#### View Logs
```bash
docker-compose logs -f app
```

#### Access Container Shell
```bash
docker-compose exec app bash
```

#### Restart Services
```bash
docker-compose restart
```

#### Run Tests
```bash
# Run unit tests
go test -v ./test/unit/...

# Run integration tests
go test -v ./test/integration/...

# Run all tests
go test -v ./...
```

## Troubleshooting

If you encounter issues:

1. **Docker Not Running**
    - The setup script will attempt to start Docker
    - Follow the OS-specific instructions if manual start is needed

2. **Port Conflicts**
    - Ensure no other services are using the required ports:
        - gRPC Server: 50051
        - HTTP Server: 8080
        - LocalStack: 4566
        - PostgreSQL: 5432
        - Redis: 6379

3. **Service Issues**
    - View service logs: `docker-compose logs <service-name>`
    - Restart specific service: `docker-compose restart <service-name>`
    - Check service health: `docker-compose ps`