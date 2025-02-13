# ------------------------------------------------------------
# --- Stage 1: Builder ---
# ------------------------------------------------------------
FROM golang:1.22 AS builder

WORKDIR /app


RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
        postgresql-client \
        protobuf-compiler \
        unzip \
        curl \
        less \
    && apt-get clean -y \
    && rm -rf /var/lib/apt/lists/*

RUN curl -sSL "https://github.com/fullstorydev/grpcurl/releases/download/v1.8.9/grpcurl_1.8.9_linux_x86_64.tar.gz" \
    | tar -xz -C /usr/local/bin

RUN arch=$(uname -m) && \
    if [ "${arch}" = "aarch64" ]; then \
        curl "https://awscli.amazonaws.com/awscli-exe-linux-aarch64.zip" -o "awscliv2.zip"; \
    else \
        curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"; \
    fi && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf aws awscliv2.zip

RUN curl -sSL "https://github.com/bufbuild/buf/releases/download/v1.28.1/buf-Linux-x86_64" -o /usr/local/bin/buf \
    && chmod +x /usr/local/bin/buf

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /app/myapp ./go-server/cmd/server

COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh


# ------------------------------------------------------------
# --- Stage 2: Final (Runtime) Image ---
# ------------------------------------------------------------
FROM debian:bookworm-slim AS final

LABEL maintainer="olusolaa <olusolae@gmail.com" \
      version="1.0.0"

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates netcat-openbsd curl \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m appuser

COPY --from=builder /app/myapp /app/myapp
COPY --from=builder /app/start.sh /app/start.sh
COPY --from=builder /usr/local/bin/aws /usr/local/bin/aws
COPY --from=builder /usr/local/aws-cli/ /usr/local/aws-cli/
COPY --from=builder /usr/local/bin/grpcurl /usr/local/bin/grpcurl

HEALTHCHECK --interval=30s --timeout=10s --retries=3 --start-period=5s \
  CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/start.sh"]
CMD []
