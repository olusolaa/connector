#!/bin/bash

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}Starting Connector Service Setup...${NC}"

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "Linux" ;;
        Darwin*)    echo "Mac" ;;
        MINGW*)     echo "Windows" ;;
        *)          echo "Unknown" ;;
    esac
}

OS=$(detect_os)
echo "Detected OS: $OS"

# Check if ports are available
check_ports() {
    # Define service ports and names (one per line for clarity)
    local services="
grpc:50051:50060:gRPC Server
http:8080:8089:HTTP Server
localstack:4566:4575:LocalStack
postgres:5432:5442:PostgreSQL
redis:6379:6389:Redis"

    local has_changes=false

    echo -e "\n${GREEN}Checking service ports...${NC}"

    # Initialize arrays for selected ports
    GRPC_PORT=50051
    HTTP_PORT=8080
    LOCALSTACK_PORT=4566
    POSTGRES_PORT=5432
    REDIS_PORT=6379

    # Use a temp file to store port changes
    local temp_ports=$(mktemp)
    echo "GRPC_PORT=$GRPC_PORT" > "$temp_ports"
    echo "HTTP_PORT=$HTTP_PORT" >> "$temp_ports"
    echo "LOCALSTACK_PORT=$LOCALSTACK_PORT" >> "$temp_ports"
    echo "POSTGRES_PORT=$POSTGRES_PORT" >> "$temp_ports"
    echo "REDIS_PORT=$REDIS_PORT" >> "$temp_ports"

    # Check each service
    while IFS=: read -r service start_port end_port service_name; do
        # Skip empty lines
        [ -z "$service" ] && continue

        echo -n "Checking $service_name port ($start_port)... "

        # Try the default port first
        if ! docker ps --format '{{.Ports}}' | grep -q ":$start_port->" && ! lsof -i ":$start_port" > /dev/null 2>&1; then
            echo -e "${GREEN}OK${NC}"
            continue
        fi

        echo -e "${YELLOW}IN USE${NC}"
        echo -e "  ${YELLOW}→ Trying alternative ports...${NC}"

        # Try alternative ports in the range
        local port_found=false
        for ((port=start_port+1; port<=end_port; port++)); do
            echo -n "    Trying port $port... "
            if ! docker ps --format '{{.Ports}}' | grep -q ":$port->" && ! lsof -i ":$port" > /dev/null 2>&1; then
                echo -e "${GREEN}OK${NC}"
                has_changes=true
                port_found=true

                # Update the corresponding environment variable in temp file
                case $service in
                    "grpc") sed -i.bak "s/GRPC_PORT=.*/GRPC_PORT=$port/" "$temp_ports" ;;
                    "http") sed -i.bak "s/HTTP_PORT=.*/HTTP_PORT=$port/" "$temp_ports" ;;
                    "localstack") sed -i.bak "s/LOCALSTACK_PORT=.*/LOCALSTACK_PORT=$port/" "$temp_ports" ;;
                    "postgres") sed -i.bak "s/POSTGRES_PORT=.*/POSTGRES_PORT=$port/" "$temp_ports" ;;
                    "redis") sed -i.bak "s/REDIS_PORT=.*/REDIS_PORT=$port/" "$temp_ports" ;;
                esac
                break
            fi
            echo -e "${RED}IN USE${NC}"
        done

        if ! $port_found; then
            echo -e "\n${RED}No available ports found for $service_name in range $start_port-$end_port${NC}"
            echo -e "${YELLOW}Resolution options:${NC}"
            echo "1. Stop conflicting Docker containers:"
            echo "   docker ps"
            echo "   docker stop <container-id>"
            echo "2. Stop local processes using these ports:"
            echo "   lsof -i :<port>"
            echo "   kill <pid>"
            echo "3. Modify port ranges in the setup script"
            rm -f "$temp_ports"
            return 1
        fi
    done < <(echo "$services")

    # Source the temp file to update environment variables
    source "$temp_ports"
    rm -f "$temp_ports"

    # Export the selected ports
    export GRPC_PORT HTTP_PORT LOCALSTACK_PORT POSTGRES_PORT REDIS_PORT

    if $has_changes; then
        echo -e "\n${YELLOW}Using alternative ports:${NC}"
        [ "$GRPC_PORT" != "50051" ] && echo "- gRPC Server: $GRPC_PORT"
        [ "$HTTP_PORT" != "8080" ] && echo "- HTTP Server: $HTTP_PORT"
        [ "$LOCALSTACK_PORT" != "4566" ] && echo "- LocalStack: $LOCALSTACK_PORT"
        [ "$POSTGRES_PORT" != "5432" ] && echo "- PostgreSQL: $POSTGRES_PORT"
        [ "$REDIS_PORT" != "6379" ] && echo "- Redis: $REDIS_PORT"

        # Update docker-compose.yml with new ports
        echo -e "\n${YELLOW}Updating docker-compose.yml with new ports...${NC}"
        local temp_file=$(mktemp)
        while IFS= read -r line; do
            case "$line" in
                *'"50051:50051"'*)
                    echo "      - \"${GRPC_PORT}:50051\"  # gRPC server" >> "$temp_file"
                    ;;
                *'"8080:8080"'*)
                    echo "      - \"${HTTP_PORT}:8080\"    # HTTP server" >> "$temp_file"
                    ;;
                *'"4566:4566"'*)
                    echo "      - \"${LOCALSTACK_PORT}:4566\"  # LocalStack" >> "$temp_file"
                    ;;
                *'"5432:5432"'*)
                    echo "      - \"${POSTGRES_PORT}:5432\"  # PostgreSQL" >> "$temp_file"
                    ;;
                *'"6379:6379"'*)
                    echo "      - \"${REDIS_PORT}:6379\"  # Redis" >> "$temp_file"
                    ;;
                *)
                    echo "$line" >> "$temp_file"
                    ;;
            esac
        done < docker-compose.yml
        mv "$temp_file" docker-compose.yml
    fi

    echo -e "\n${GREEN}All ports are configured and available!${NC}"
}

check_docker() {
    # Check if Docker is installed
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}Docker is not installed.${NC}"
        echo -e "${YELLOW}Please install Docker for your operating system:${NC}"
        case "$OS" in
            "Mac")
                echo "Visit: https://www.docker.com/products/docker-desktop"
                ;;
            "Linux")
                echo "Run: curl -fsSL https://get.docker.com -o get-docker.sh && sudo sh get-docker.sh"
                ;;
            "Windows")
                echo "Visit: https://www.docker.com/products/docker-desktop"
                ;;
            *)
                echo "Visit: https://docs.docker.com/engine/install/"
                ;;
        esac
        exit 1
    fi

    # Check if Docker daemon is running and try to start it
    if ! docker info &> /dev/null; then
        echo -e "${YELLOW}Docker daemon is not running.${NC}"

        case "$OS" in
            "Mac")
                if [ -e "/Applications/Docker.app" ]; then
                    echo "Attempting to start Docker Desktop..."
                    open -a Docker
                fi
                ;;
            "Linux")
                echo "Attempting to start Docker daemon..."
                if command -v systemctl &> /dev/null; then
                    sudo systemctl start docker || true
                elif command -v service &> /dev/null; then
                    sudo service docker start || true
                fi
                ;;
            "Windows")
                if [ -e "/mnt/c/Program Files/Docker/Docker/Docker Desktop.exe" ]; then
                    echo "Attempting to start Docker Desktop..."
                    "/mnt/c/Program Files/Docker/Docker/Docker Desktop.exe" &
                fi
                ;;
        esac

        # Wait for Docker to start
        echo "Waiting for Docker to start..."
        for i in {1..30}; do
            if docker info &> /dev/null; then
                echo -e "${GREEN}Docker is now running!${NC}"
                return 0
            fi
            echo -n "."
            sleep 2
        done

        # If Docker didn't start, provide OS-specific instructions
        echo -e "\n${RED}Could not start Docker automatically.${NC}"
        echo -e "${YELLOW}Please start Docker manually:${NC}"
        case "$OS" in
            "Mac")
                echo "1. Open Docker Desktop from your Applications folder"
                echo "2. Wait for the whale icon to appear in the menu bar"
                ;;
            "Linux")
                echo "Run one of these commands:"
                echo "  sudo systemctl start docker"
                echo "  sudo service docker start"
                ;;
            "Windows")
                echo "1. Open Docker Desktop from the Start menu"
                echo "2. Wait for the whale icon to appear in the system tray"
                ;;
            *)
                echo "Start Docker according to your OS instructions"
                ;;
        esac
        echo "Then run this script again."
        exit 1
    fi
}

check_docker_compose() {
    if ! command -v docker-compose &> /dev/null; then
        # First check if we have Docker Compose V2 (docker compose)
        if docker compose version &> /dev/null; then
            # Create an alias for docker-compose
            docker_compose_cmd="docker compose"
            return 0
        fi

        echo -e "${RED}Docker Compose is not installed.${NC}"
        echo -e "${YELLOW}Installation instructions:${NC}"
        case "$OS" in
            "Mac")
                echo "Docker Compose should be included with Docker Desktop."
                echo "Please reinstall Docker Desktop."
                ;;
            "Linux")
                echo "Install using one of these methods:"
                echo "1. Package manager:"
                echo "   sudo apt-get install docker-compose  # For Ubuntu/Debian"
                echo "   sudo yum install docker-compose      # For RHEL/CentOS"
                echo "2. Manual installation:"
                echo "   sudo curl -L \"https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)\" -o /usr/local/bin/docker-compose"
                echo "   sudo chmod +x /usr/local/bin/docker-compose"
                ;;
            "Windows")
                echo "Docker Compose should be included with Docker Desktop."
                echo "Please reinstall Docker Desktop."
                ;;
            *)
                echo "Visit: https://docs.docker.com/compose/install/"
                ;;
        esac
        exit 1
    fi
}

prompt_slack_credentials() {
    # Use the dynamic HTTP port or default
    local http_port=${HTTP_PORT:-8080}

    echo -e "\n${YELLOW}Slack Configuration${NC}"
    read -p "Enter your Slack Client ID (press Enter to use default): " SLACK_CLIENT_ID
    SLACK_CLIENT_ID=${SLACK_CLIENT_ID:-"7656730043137.8419590983429"}

    read -p "Enter your Slack Client Secret (press Enter to use default): " SLACK_CLIENT_SECRET
    SLACK_CLIENT_SECRET=${SLACK_CLIENT_SECRET:-"374add2fd9704aa7f4e98b262e1e3583"}

    read -p "Enter your Slack Redirect URL (press Enter to use default: https://localhost:${http_port}/oauth/callback): " SLACK_REDIRECT_URL
    SLACK_REDIRECT_URL=${SLACK_REDIRECT_URL:-"https://localhost:${http_port}/oauth/callback"}

    # Export for docker-compose
    export SLACK_CLIENT_ID
    export SLACK_CLIENT_SECRET
    export SLACK_REDIRECT_URL
}

cleanup() {
    echo "Cleaning up..."
    # Stop and remove containers
    docker-compose down -v

    # Get all used ports
    local ports=(
        "${GRPC_PORT:-50051}"
        "${HTTP_PORT:-8080}"
        "${LOCALSTACK_PORT:-4566}"
        "${POSTGRES_PORT:-5432}"
        "${REDIS_PORT:-6379}"
    )

    # Remove any leftover containers using our ports
    for port in "${ports[@]}"; do
        echo -n "Checking port $port... "
        container_id=$(docker ps -q --filter "publish=$port")
        if [ ! -z "$container_id" ]; then
            echo "found container $container_id"
            echo "Stopping container..."
            docker stop "$container_id" >/dev/null 2>&1 || true
        else
            echo "no containers found"
        fi
    done

    # Reset environment variables
    unset GRPC_PORT HTTP_PORT LOCALSTACK_PORT POSTGRES_PORT REDIS_PORT
}

generate_env_file() {
    echo "Generating .env file..."
    cat > .env << EOF
# Server Configuration
GRPC_PORT=50051
HTTP_PORT=8080

# HTTP Server Timeouts
HTTP_READ_HEADER_TIMEOUT_SECONDS=60
HTTP_READ_TIMEOUT_SECONDS=60
HTTP_WRITE_TIMEOUT_SECONDS=60
HTTP_IDLE_TIMEOUT_SECONDS=120

# Circuit Breaker Configuration
CIRCUIT_BREAKER_INTERVAL_SECONDS=60
CIRCUIT_BREAKER_TIMEOUT_SECONDS=30

# OAuth Configuration
OAUTH_STATE_TIMEOUT_MINUTES=15

# Slack Configuration
SLACK_CLIENT_ID=7656730043137.8419590983429
SLACK_CLIENT_SECRET=374add2fd9704aa7f4e98b262e1e3583
SLACK_REDIRECT_URL=https://localhost:\${HTTP_PORT}/oauth/callback
SLACK_BASE_URL=https://slack.com/api
SLACK_SCOPES=chat:write,channels:read,groups:read
SLACK_RETRY_MAX=3
SLACK_RETRY_WAIT_SECONDS=5
SLACK_RATE_LIMIT_RPS=1.0
SLACK_RATE_LIMIT_BURST=3

# Database Configuration
POSTGRES_PORT=5432
POSTGRES_USER=aryon
POSTGRES_PASSWORD=aryon
POSTGRES_DB=aryondb
DB_DSN=postgres://\${POSTGRES_USER}:\${POSTGRES_PASSWORD}@postgres:\${POSTGRES_PORT}/\${POSTGRES_DB}?sslmode=disable
HOST_DB_DSN=postgres://\${POSTGRES_USER}:\${POSTGRES_PASSWORD}@localhost:\${POSTGRES_PORT}/\${POSTGRES_DB}?sslmode=disable
DB_CONN_MAX_LIFETIME_MINUTES=5
DB_MAX_IDLE_CONNS=10
DB_MAX_OPEN_CONNS=100

# Redis Configuration
REDIS_PORT=6379
REDIS_ADDR=redis:\${REDIS_PORT}
HOST_REDIS_ADDR=localhost:\${REDIS_PORT}
REDIS_DB=0

# AWS Configuration
AWS_REGION=us-east-1
LOCALSTACK_PORT=4566
AWS_ENDPOINT=http://localstack:\${LOCALSTACK_PORT}
HOST_AWS_ENDPOINT=http://localhost:\${LOCALSTACK_PORT}
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
AWS_DEFAULT_REGION=us-east-1
AWS_RETRY_MAX_ATTEMPTS=5
AWS_RETRY_MAX_BACKOFF_SECONDS=60

# LocalStack Configuration
SERVICES=secretsmanager
DEBUG=1
DOCKER_HOST=unix:///var/run/docker.sock
LAMBDA_EXECUTOR=docker
PERSISTENCE=1
LS_LOG=trace

# Host Ports (for local development)
HOST_GRPC_PORT=\${GRPC_PORT}
HOST_HTTP_PORT=\${HTTP_PORT}
EOF

    echo "Generated .env file successfully"
}

main() {
    # Set up cleanup trap
    trap cleanup EXIT

    # Check prerequisites
    check_docker
    check_docker_compose
    check_ports

    # Get Slack credentials
    prompt_slack_credentials

    # Add this line near the beginning of the script, before starting services
    if [ ! -f ".env" ]; then
        generate_env_file
    else
        echo ".env file already exists, skipping generation"
        echo "To regenerate .env file, delete the existing one and run this script again"
    fi

    # Start services
    echo "Starting services..."
    docker-compose down -v >/dev/null 2>&1 || true
    docker-compose up --build -d

    # Use default ports if not set by check_ports
    GRPC_PORT=${GRPC_PORT:-50051}
    HTTP_PORT=${HTTP_PORT:-8080}
    LOCALSTACK_PORT=${LOCALSTACK_PORT:-4566}
    POSTGRES_PORT=${POSTGRES_PORT:-5432}
    REDIS_PORT=${REDIS_PORT:-6379}

    echo -e "\n${GREEN}Services are starting up...${NC}"
    echo -e "The following services will be available:"
    echo -e "- gRPC Server: localhost:${GRPC_PORT}"
    echo -e "- HTTP Server: localhost:${HTTP_PORT}"
    echo -e "- LocalStack: localhost:${LOCALSTACK_PORT}"
    echo -e "- PostgreSQL: localhost:${POSTGRES_PORT}"
    echo -e "- Redis: localhost:${REDIS_PORT}"
    echo

    echo -e "\n${YELLOW}Debugging Tips:${NC}"
    echo "1. View logs: docker-compose logs -f app"
    echo "2. Access container shell: docker-compose exec app bash"
    echo "3. Restart services: docker-compose restart"

    echo -e "\nPress Ctrl+C to stop all services"

    # Keep the script running until Ctrl+C
    while true; do
        if ! docker-compose ps --services --filter "status=running" | grep -q "app"; then
            echo -e "${RED}Service stopped unexpectedly${NC}"
            echo -e "${YELLOW}Showing recent logs:${NC}"
            docker-compose logs --tail=50 app
            exit 1
        fi
        sleep 1
    done
}

main