package grpc_test

import (
	"context"
	"testing"

	"github.com/connector-recruitment/test/integration/testserver"
	_ "github.com/lib/pq"

	connV1 "github.com/connector-recruitment/proto/gen/connector/v1"
)

func TestConnectorServiceGRPC(t *testing.T) {
	server := testserver.SetupIntegrationTestServer(t)

	ctx := context.Background()
	client := connV1.NewConnectorServiceClient(server.GrpcConn)

	createReq := &connV1.CreateConnectorRequest{
		WorkspaceId:        "workspace-1",
		TenantId:           "tenant-1",
		Token:              "test-token",
		DefaultChannelName: "general",
	}
	createResp, err := client.CreateConnector(ctx, createReq)
	if err != nil {
		t.Fatalf("CreateConnector failed: %v", err)
	}
	connProto := createResp.Connector
	if connProto.WorkspaceId != createReq.WorkspaceId {
		t.Errorf("expected workspace id %q, got %q", createReq.WorkspaceId, connProto.WorkspaceId)
	}
	if connProto.TenantId != createReq.TenantId {
		t.Errorf("expected tenant id %q, got %q", createReq.TenantId, connProto.TenantId)
	}
	if connProto.DefaultChannelId != "C1234567890" {
		t.Errorf("expected default channel id %q, got %q", "C1234567890", connProto.DefaultChannelId)
	}
	if connProto.SecretVersion != "v1" {
		t.Errorf("expected secret version %q, got %q", "v1", connProto.SecretVersion)
	}
	createdID := connProto.Id

	getReq := &connV1.GetConnectorRequest{Id: createdID}
	getResp, err := client.GetConnector(ctx, getReq)
	if err != nil {
		t.Fatalf("GetConnector failed: %v", err)
	}
	if getResp.Connector.Id != createdID {
		t.Errorf("expected connector id %q, got %q", createdID, getResp.Connector.Id)
	}

	oauthURLReq := &connV1.GetOAuthV2URLRequest{RedirectUri: "https://example.com/callback"}
	oauthURLResp, err := client.GetOAuthV2URL(ctx, oauthURLReq)
	if err != nil {
		t.Fatalf("GetOAuthV2URL failed: %v", err)
	}
	expectedURL := "https://example.com/oauth?state="
	if len(oauthURLResp.Url) <= len(expectedURL) {
		t.Errorf("expected OAuth URL to start with %q, got %q", expectedURL, oauthURLResp.Url)
	}

	exchangeReq := &connV1.ExchangeOAuthCodeRequest{Code: "dummy-code"}
	exchangeResp, err := client.ExchangeOAuthCode(ctx, exchangeReq)
	if err != nil {
		t.Fatalf("ExchangeOAuthCode failed: %v", err)
	}
	if exchangeResp.AccessToken != "exchanged-token" {
		t.Errorf("expected access token %q, got %q", "exchanged-token", exchangeResp.AccessToken)
	}

	deleteReq := &connV1.DeleteConnectorRequest{
		Id:          createdID,
		WorkspaceId: "workspace-1",
		TenantId:    "tenant-1",
	}
	deleteResp, err := client.DeleteConnector(ctx, deleteReq)
	if err != nil {
		t.Fatalf("DeleteConnector failed: %v", err)
	}
	expectedMsg := "Connector deleted successfully"
	if deleteResp.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, deleteResp.Message)
	}

	_, err = client.GetConnector(ctx, getReq)
	if err == nil {
		t.Error("expected error when getting a deleted connector, but got none")
	}
}
