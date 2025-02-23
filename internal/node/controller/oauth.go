package controller

import (
	"fmt"
	"net/http"

	"github.com/eagraf/go-atproto-oauth/oauth"
	"github.com/eagraf/habitat-new/internal/node/api"
)

type HabitatOAuthProxy struct {
	oauthConfig oauth.Config
	persister   oauth.Persister
}

func NewHabitatOAuthProxy(domain string, authPersister oauth.Persister) *HabitatOAuthProxy {

	// In Habitat, the domain for this tailnet happens to be the same as the PDS URL.
	pdsURL := fmt.Sprintf("https://%s", domain)

	oauthConfig := oauth.Config{
		Protocol:     "https",
		Host:         domain,
		SecretJWK:    `{"use":"sig","kty":"EC","kid":"demo-1737595555","crv":"P-256","alg":"ES256","x":"1hF99bxkcHj-aTWnEYzPJvrb8p0d2mfr7OI1NsTYHV4","y":"QWXmL4UwsdtLnKNJKppnr3RPf6NsZx7-IGWRuuySJh4","d":"pcSEJr8TirUbxaCbQ_L17x_00_SDvB1jZpZmNsRra_Y"}`,
		PDSURL:       pdsURL,
		EndpointPath: "/habitat/oauth",
	}
	return &HabitatOAuthProxy{
		oauthConfig: oauthConfig,
		persister:   authPersister,
	}
}

func (h *HabitatOAuthProxy) loginHandler(w http.ResponseWriter, r *http.Request) {
	handler := oauth.LoginHandler(h.oauthConfig, h.persister)
	handler.ServeHTTP(w, r)
}

func (h *HabitatOAuthProxy) callbackHandler(w http.ResponseWriter, r *http.Request) {
	handler := oauth.CallbackHandler(h.oauthConfig, h.persister)
	handler.ServeHTTP(w, r)
}

func (h *HabitatOAuthProxy) jwksHandler(w http.ResponseWriter, r *http.Request) {
	handler := oauth.JWKSHandler(h.oauthConfig)
	handler.ServeHTTP(w, r)
}

func (h *HabitatOAuthProxy) clientMetadataHandler(w http.ResponseWriter, r *http.Request) {
	handler := oauth.ClientMetadataHandler(h.oauthConfig)
	handler.ServeHTTP(w, r)
}

func (h *HabitatOAuthProxy) OAuthRoutes() []api.Route {
	return []api.Route{
		newRoute(http.MethodPost, "/login", h.loginHandler),
		newRoute(http.MethodGet, "/callback", h.callbackHandler),
		newRoute(http.MethodGet, "/jwks.json", h.jwksHandler),
		newRoute(http.MethodGet, "/client_metadata.json", h.clientMetadataHandler),
	}
}
