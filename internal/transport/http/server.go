package http

import (
	"net/http"
	"time"

	"github.com/connector-recruitment/internal/app/connector"
)

func NewHTTPServer(svc *connector.Service, oauthManager *connector.OAuthStateManager) *http.Server {
	handler := NewHandler(svc, oauthManager)
	mux := http.NewServeMux()

	mux.HandleFunc("/oauth/callback", handler.OAuthCallback)
	mux.HandleFunc("/health", handler.Health)

	return &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 60 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}
