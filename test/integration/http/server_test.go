package http_test

import (
	"context"
	"github.com/connector-recruitment/test/integration/testserver"
	"io"
	"net/http"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	srv := testserver.SetupIntegrationTestServer(t)

	resp, err := http.Get(srv.HTTPAddress + "/health")
	if err != nil {
		t.Fatalf("Failed to call /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestOAuthCallback_Success(t *testing.T) {
	srv := testserver.SetupIntegrationTestServer(t)

	state, err := srv.OAuthManager.GenerateState(context.Background())
	if err != nil {
		t.Fatalf("Failed to generate OAuth state: %v", err)
	}

	url := srv.HTTPAddress + "/oauth/callback?code=dummy-code&state=" + state

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to call /oauth/callback: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 200 OK, got %d. Body: %s", resp.StatusCode, string(body))
	}
}
