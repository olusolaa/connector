package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/connector-recruitment/internal/app/connector"
	"github.com/connector-recruitment/pkg/logger"
)

type Handler struct {
	svc          *connector.Service
	oauthManager *connector.OAuthStateManager
}

func NewHandler(svc *connector.Service, oauthManager *connector.OAuthStateManager) *Handler {
	return &Handler{
		svc:          svc,
		oauthManager: oauthManager,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	logger.Info().Msg("Received /oauth/callback request")

	ctx := r.Context()
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}
	if state == "" {
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}

	if err := h.oauthManager.ValidateState(ctx, state); err != nil {
		logger.Warn().Err(err).Msg("Invalid OAuth state parameter")
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	token, err := h.svc.ExchangeOAuthCode(ctx, code)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to exchange code for token")
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}

	parts := strings.Split(token, "-")
	workspaceID := ""
	if len(parts) > 1 {
		workspaceID = parts[1]
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<html>
<body style="font-family: Arial, sans-serif; max-width: 800px; margin: 40px auto; padding: 0 20px;">
	<h1 style="color: #1a73e8;">OAuth Flow Successful! 🎉</h1>
	<p>Your access token has been received. You can now create a connector using this token.</p>
	<div style="background-color: #f8f9fa; padding: 20px; border-radius: 8px; margin: 20px 0;">
		<h3>Your Configuration Values:</h3>
		<ul>
			<li><strong>Workspace ID:</strong> %s (automatically extracted from your token)</li>
			<li><strong>Tenant ID:</strong> You can use any unique identifier for your organization (e.g., "my-company" or "team-1")</li>
		</ul>
	</div>
	<p>To create a connector, use this command:</p>
	<pre style="background-color: #f8f9fa; padding: 20px; border-radius: 8px; overflow-x: auto;">
grpcurl -plaintext -d '{
  "workspace_id": "%s",
  "tenant_id": "your-organization-id",
  "token": "%s",
  "default_channel_name": "all-moneta"
}' localhost:50051 connector.v1.ConnectorService/CreateConnector</pre>
	<p style="color: #666;">Note: Replace "your-organization-id" with your desired tenant identifier.</p>
</body>
</html>
`, workspaceID, workspaceID, token)
}
