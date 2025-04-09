package bffauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/eagraf/habitat-new/internal/resolvers"
	"github.com/pkg/errors"
)

type ExternalHabitatUser struct {
	did             string
	publicKey       crypto.PublicKey
	host            string
	bffEndpointBase string
}

type Client interface {
	GetToken(targetDID string) (string, error)
}

type client struct {
	did string

	privateKey crypto.PrivateKey

	activeTokens map[string]string

	habitatHostResolver resolvers.HabitatHostResolver
	publicKeyResolver   resolvers.PublicKeyResolver
	scheme              string
}

func NewClient(clientDID string, privateKey crypto.PrivateKey, habitatHostResolver resolvers.HabitatHostResolver, publicKeyResolver resolvers.PublicKeyResolver) Client {
	return &client{
		did:                 clientDID,
		privateKey:          privateKey,
		activeTokens:        make(map[string]string),
		habitatHostResolver: habitatHostResolver,
		publicKeyResolver:   publicKeyResolver,
	}
}

// GetToken returns a token for the remote Habitat user
// If the token is already in the cache, it returns the cached token
// Otherwise, it queries the remote Habitat user's bff/auth endpoint and caches the token
func (c *client) GetToken(targetDID string) (string, error) {
	token, ok := c.activeTokens[targetDID]
	if ok {
		return token, nil
	}

	targetUser, err := c.resolveTargetUser(targetDID)
	if err != nil {
		return "", err
	}

	token, err = c.getToken(targetUser)
	if err != nil {
		return "", err
	}

	c.activeTokens[targetDID] = token
	return token, nil
}

func (c *client) resolveTargetUser(targetDID string) (*ExternalHabitatUser, error) {
	habitatHost, scheme, err := c.habitatHostResolver(targetDID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve habitat host")
	}

	endpoint := &url.URL{
		Scheme: scheme,
		Host:   habitatHost,
		Path:   "/habitat/api/node/bff",
	}

	publicKey, err := c.publicKeyResolver(targetDID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve public key")
	}

	return &ExternalHabitatUser{
		did:             targetDID,
		publicKey:       publicKey,
		host:            habitatHost,
		bffEndpointBase: endpoint.String(),
	}, nil
}

// getChallenge queries the remote Habitat user's bff/challenge endpoint
func (c *client) getChallenge(remoteHabitatUser *ExternalHabitatUser) (string, string, error) {
	endpoint := remoteHabitatUser.bffEndpointBase + "/challenge"

	pubKeyMultibase := remoteHabitatUser.publicKey.Multibase()

	reqBody := ChallengeRequest{
		DID:                remoteHabitatUser.did,
		PublicKeyMultibase: pubKeyMultibase,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	httpClient := &http.Client{}
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return "", "", err
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to get challenge with status: %d, body: %s", resp.StatusCode, string(respBytes))
	}

	var respBody ChallengeResponse
	err = json.Unmarshal(respBytes, &respBody)
	if err != nil {
		return "", "", err
	}

	return respBody.Challenge, respBody.Session, nil
}

// queryAuthEndpoint queries the remote Habitat user's bff/auth endpoint
func (c *client) queryAuthEndpoint(remoteHabitatUser *ExternalHabitatUser, sessionID string, proof string) (string, error) {
	endpoint := remoteHabitatUser.bffEndpointBase + "/auth"

	reqBody := AuthRequest{
		SessionID: sessionID,
		Proof:     proof,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	httpClient := &http.Client{}

	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return "", err
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get token with status: %d, body: %s", resp.StatusCode, string(respBytes))
	}

	var respBody AuthResponse
	err = json.Unmarshal(respBytes, &respBody)
	if err != nil {
		return "", err
	}

	return respBody.Token, nil
}

func (c *client) getToken(targetUser *ExternalHabitatUser) (string, error) {

	challenge, sessionID, err := c.getChallenge(targetUser)
	if err != nil {
		return "", err
	}

	proof, err := GenerateProof(challenge, c.privateKey)
	if err != nil {
		return "", err
	}

	token, err := c.queryAuthEndpoint(targetUser, sessionID, proof)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (c *client) constructRemoteHabitatEndpoint(remoteHabitatUser *ExternalHabitatUser, path string) *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   remoteHabitatUser.host,
		Path:   path,
	}
}
