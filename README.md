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

## Requirements:

## Overview

The goal of this exercise is to implement a gRPC service that handles the lifecycle of "connectors" that integrate with
Slack. A connector is a named entity that connects the system to a third party service such as slack.

You are required to implement a connector service that does the following:

- Stores static Slack tokens in AWS Secrets Manager (mocked via LocalStack).
- Stores connector metadata in a PostgreSQL database.
    - Workspace ID
    - Tenant ID
    - Created At
    - Updated At
    - Default Send Channel ID - Messages will be sent to this channel by default.
- Provides endpoints to create, retrieve, and delete connectors.
- A static Go function outside the service that takes a connector id and a string simple message and sends it to the
  default configured channel. In addition to all other configuration parameters, such as aws config that is connected to localstack.
  - You are free to model the configuration parameters as you see fit.

---

## Features

- **gRPC** service with three methods:
    - `CreateConnector` (You are given static access tokens and the default channel name(which needs to be resolved to its ID). See [Bonus](#bonus) for OAuthV2)
    - `GetConnector`
    - `DeleteConnector`
- **Secrets Manager** integration (LocalStack).
- **Slack integration** to send messages using an already created connector.
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
   +------------+
   | PostgreSQL |
   +------------+
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

## Bonus

- Implement Slack OAuthV2 flow for real token retrieval.
    - Implement gRPC method `GetOAuthV2URL`
- Expand the connector functionality (attachments, threading, etc.).

---