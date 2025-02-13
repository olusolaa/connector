#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configurable timeouts (can be overridden by environment variables)
POSTGRES_TIMEOUT=${POSTGRES_TIMEOUT:-30}
LOCALSTACK_TIMEOUT=${LOCALSTACK_TIMEOUT:-45}
REDIS_TIMEOUT=${REDIS_TIMEOUT:-15}

# Use dynamic ports with defaults
GRPC_PORT=${GRPC_PORT:-50051}
HTTP_PORT=${HTTP_PORT:-8080}
POSTGRES_PORT=${POSTGRES_PORT:-5432}
LOCALSTACK_PORT=${LOCALSTACK_PORT:-4566}
REDIS_PORT=${REDIS_PORT:-6379}

# Export environment variables for the Go servers
export DB_DSN="postgres://aryon:aryon@postgres:${POSTGRES_PORT}/aryondb?sslmode=disable"
export REDIS_ADDR="redis:${REDIS_PORT}"
export AWS_ENDPOINT="http://localstack:${LOCALSTACK_PORT}"
export GRPC_PORT="${GRPC_PORT}"
export HTTP_PORT="${HTTP_PORT}"

# Initialize AWS credentials for LocalStack
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1

# Wait for services with timeout
wait_for_service() {
    local service=$1
    local port=$2
    local service_name=$3
    local timeout=$4
    local start_time=$(date +%s)

    echo -e "${YELLOW}Waiting for $service_name to be ready...${NC}"
    while ! nc -z $service $port; do
        current_time=$(date +%s)
        elapsed=$((current_time - start_time))
        
        if [ $elapsed -gt $timeout ]; then
            echo -e "${RED}Timeout waiting for $service_name${NC}"
            return 1
        fi
        
        echo -n "."
        sleep 1
    done
    echo -e "\n${GREEN}$service_name is ready!${NC}"
    return 0
}

# Initialize LocalStack
init_localstack() {
    echo -e "${YELLOW}Initializing LocalStack...${NC}"
    
    # Wait for LocalStack to be fully up
    until curl -s http://localstack:4566/_localstack/health | grep -q '"secretsmanager": "available"'; do
        echo "Waiting for Secrets Manager to be available..."
        sleep 2
    done

    # Set up AWS credentials directory
    mkdir -p ~/.aws
    cat > ~/.aws/credentials << EOF
[default]
aws_access_key_id = test
aws_secret_access_key = test
region = us-east-1
EOF

    # Create a test secret to verify Secrets Manager is working
    aws --endpoint-url=http://localstack:4566 secretsmanager create-secret \
        --name test-secret \
        --secret-string "test-value" || true

    echo -e "${GREEN}LocalStack initialization completed${NC}"
}

# Track if all services are ready
services_ready=0

# Main startup sequence
for attempt in {1..3}; do
    if [ $attempt -gt 1 ]; then
        echo -e "${YELLOW}Retry attempt $attempt/3${NC}"
        sleep 5
    fi

    if wait_for_service postgres $POSTGRES_PORT "PostgreSQL" "$POSTGRES_TIMEOUT" && \
       wait_for_service localstack $LOCALSTACK_PORT "LocalStack" "$LOCALSTACK_TIMEOUT" && \
       wait_for_service redis $REDIS_PORT "Redis" "$REDIS_TIMEOUT"; then
        services_ready=1
        break
    fi
done

if [ $services_ready -eq 0 ]; then
    echo -e "${RED}Failed to connect to all required services after 3 attempts${NC}"
    echo -e "${YELLOW}Please check the service logs and try again${NC}"
    exit 1
fi

# Initialize LocalStack
init_localstack

# Generate SSL certificates if they don't exist
if [ ! -f "cert.pem" ] || [ ! -f "key.pem" ]; then
    echo "Generating SSL certificates..."
    openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
fi

# Start application servers
echo -e "${GREEN}Starting application servers...${NC}"
echo "Starting gRPC server on port ${GRPC_PORT}..."
/app/myapp 2>&1 | tee /tmp/server.log &
SERVER_PID=$!

# Print service URLs
echo -e "\n${GREEN}Services are running:${NC}"
echo "- gRPC Server: localhost:${GRPC_PORT}"
echo "- HTTP Server: localhost:${HTTP_PORT}"
echo "- PostgreSQL: localhost:${POSTGRES_PORT}"
echo "- LocalStack: localhost:${LOCALSTACK_PORT}"
echo "- Redis: localhost:${REDIS_PORT}"

# Trap SIGTERM and SIGINT
trap 'echo "Shutting down servers..."; kill $SERVER_PID; exit 0' TERM INT

# Monitor server processes
while kill -0 $SERVER_PID 2>/dev/null; do
    sleep 1
done 