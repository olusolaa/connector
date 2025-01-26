# Connector Service (Go + Buf + LocalStack)

- **Go** (for the service)
- **Buf** (for Protobuf and gRPC code generation)
- **LocalStack** (to mock AWS Secrets Manager)
- **PostgreSQL** (optional, for additional data storage)

## Table of Contents

1. [Overview](#overview)
2. [Features](#features)
3. [Architecture](#architecture)
4. [Prerequisites](#prerequisites)
5. [Setup](#setup)
6. [Buf Installation](#buf-installation)
7. [Protobuf Generation](#protobuf-generation)
8. [Docker Compose](#docker-compose)
9. [Running the Service](#running-the-service)
10. [Endpoints](#endpoints)

---

## Overview

This gRPC service handles the lifecycle of "connectors" that integrate with Slack. It:

- Stores Slack tokens in AWS Secrets Manager (mocked via LocalStack).
- Provides endpoints to create, retrieve, and delete connectors.
- Demonstrates sending a message to Slack using the stored token (no persistent connection required).

---

## Features

- **gRPC** service with three methods:
    - `CreateConnector`
    - `GetConnector`
    - `DeleteConnector`
- **Secrets Manager** integration (LocalStack).
- **Slack integration** to send messages.
- **Optional PostgreSQL** usage for tracking connector metadata.

---

## Architecture

```
  +-----------------+         +--------------------+
  | gRPC Client     | ----->  | Slack Connector    |
  | (e.g., grpcurl) |         | Service (Go + Buf) |
  +-----------------+         +--------------------+
        |                                 |
        | (AWS SDK)                       | (Slack API)
        v                                 v
  LocalStack (Secrets Manager)       Slack (Real or Mock)
        |
   +-----------+
   |PostgreSQL |
   | (optional)|
   +-----------+
```

---

## Prerequisites

- **Go** (>= 1.18 recommended)
- **Docker**
- **Buf** (for protobuf generation)
- **Slack token** (real or mocked)

---

## Setup

### Buf Installation

Follow the [official Buf installation guide](https://docs.buf.build/installation) for your OS.

### Protobuf Generation

From the repository root, run:

```bash
buf generate
```

This will generate Go code into the configured output folder (e.g., `gen/`).

### Docker Compose

Spin up LocalStack and PostgreSQL using the provided docker-compose.yml like this:

```bash
docker-compose up -d
```

---

## Running the Service

1. Start LocalStack (and Postgres if needed):

   ```bash
   docker-compose up -d
   ```

2. Generate protobuf stubs (if you haven’t yet):

   ```bash
   buf generate
   ```

3. Run the server:

   ```bash
   go run cmd/server/main.go
   ```

The service should listen on `:50051` (or another configured port).

---

## Endpoints

The gRPC service typically exposes these methods:

1. **CreateConnector**

- Accepts a connector name and Slack token.
- Stores the token in Secrets Manager and optionally a database record.

2. **GetConnector**

- Retrieves a previously stored connector’s Slack token or metadata.

3. **DeleteConnector**

- Removes the connector’s Slack token from Secrets Manager and clears any DB records.

---

## Bonus

- Implement Slack OAuth flow for real token retrieval.
- Expand the connector functionality (attachments, threading, etc.).

---