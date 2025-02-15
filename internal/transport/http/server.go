package http

import (
	"net/http"

	"github.com/connector-recruitment/internal/app/config"
	"github.com/connector-recruitment/internal/app/connector"
)

func NewHTTPServer(svc *connector.Service, oauthManager *connector.OAuthStateManager, cfg *config.Config) *http.Server {
	handler := NewHandler(svc, oauthManager)
	mux := http.NewServeMux()

	mux.HandleFunc("/oauth/callback", handler.OAuthCallback)
	mux.HandleFunc("/health", handler.Health)

	return &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}
}
