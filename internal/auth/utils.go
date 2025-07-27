package auth

import (
	"encoding/base64"
	"net/url"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/google/uuid"
)

func generateNonce() string {
	return base64.StdEncoding.EncodeToString([]byte(uuid.NewString()))
}

func constructAuthServerWellKnownURL(identity *identity.Identity, path string) (string, error) {
	url, err := url.Parse(identity.PDSEndpoint())
	if err != nil {
		return "", err
	}

	// Exception for PDS running locally without Tailscale
	if url.Host == "localhost:3000" {
		url.Host = "host.docker.internal:5001"
	}

	return url.JoinPath(path).String(), nil
}
