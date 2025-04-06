package bffauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bluesky-social/indigo/atproto/crypto"
)

type ExternalHabitatUser struct {
	DID       string
	PublicKey crypto.PublicKey
	Host      string
}

func getTempStores() (map[string]crypto.PrivateKey, map[string]*ExternalHabitatUser, error) {
	testKey, err := crypto.GeneratePrivateKeyP256()
	if err != nil {
		return nil, nil, err
	}

	testPubKey, err := testKey.PublicKey()
	if err != nil {
		return nil, nil, err
	}

	testUser := &ExternalHabitatUser{
		DID:       "did:plc:avkzrsj7u4kmoq33gx6v4lh4",
		PublicKey: testPubKey,
		Host:      "beacon-dev.tail07d32.ts.net",
	}

	return map[string]crypto.PrivateKey{
			"did:plc:avkzrsj7u4kmoq33gx6v4lh4": testKey,
		}, map[string]*ExternalHabitatUser{
			"did:plc:avkzrsj7u4kmoq33gx6v4lh4": testUser,
		}, nil

}

type Client struct {
	// This is the store of temporary friends that are added to the BFF.
	// Helps us avoid implementing full public key resolution from DIDs with TailScale / localhost  setups
	tempFriendStore map[string]*ExternalHabitatUser

	clientDID string

	privateKey crypto.PrivateKey

	activeTokens map[string]string
}

func NewClient(clientDID string) (*Client, error) {
	privKeyStore, friendStore, err := getTempStores()
	if err != nil {
		return nil, err
	}

	privateKey, ok := privKeyStore[clientDID]
	if !ok {
		return nil, fmt.Errorf("no private key found for client: %s", clientDID)
	}

	return &Client{
		privateKey:      privateKey,
		tempFriendStore: friendStore,
		activeTokens:    make(map[string]string),
	}, nil
}

// GetToken returns a token for the remote Habitat user
// If the token is already in the cache, it returns the cached token
// Otherwise, it queries the remote Habitat user's bff/auth endpoint and caches the token
func (c *Client) GetToken(targetDID string) (string, error) {
	token, ok := c.activeTokens[targetDID]
	if ok {
		return token, nil
	}

	friend, ok := c.tempFriendStore[targetDID]
	if !ok {
		return "", fmt.Errorf("no friend entry found for target: %s", targetDID)
	}

	token, err := c.getToken(friend)
	if err != nil {
		return "", err
	}

	c.activeTokens[targetDID] = token
	return token, nil
}

// getChallenge queries the remote Habitat user's bff/challenge endpoint
func (c *Client) getChallenge(remoteHabitatUser *ExternalHabitatUser) (string, string, error) {
	endpoint := constructRemoteHabitatEndpoint(remoteHabitatUser, "/habitat/api/node/bff/challenge")

	pubKeyMultibase := remoteHabitatUser.PublicKey.Multibase()

	reqBody := ChallengeRequest{
		DID:                remoteHabitatUser.DID,
		PublicKeyMultibase: pubKeyMultibase,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	httpClient := &http.Client{}
	resp, err := httpClient.Post(endpoint.String(), "application/json", bytes.NewBuffer(reqBytes))
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
func (c *Client) queryAuthEndpoint(remoteHabitatUser *ExternalHabitatUser, sessionID string, proof string) (string, error) {
	endpoint := constructRemoteHabitatEndpoint(remoteHabitatUser, "/habitat/api/node/bff/auth")

	reqBody := AuthRequest{
		SessionID: sessionID,
		Proof:     proof,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	httpClient := &http.Client{}

	resp, err := httpClient.Post(endpoint.String(), "application/json", bytes.NewBuffer(reqBytes))
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

func (c *Client) getToken(remoteHabitatUser *ExternalHabitatUser) (string, error) {

	challenge, sessionID, err := c.getChallenge(remoteHabitatUser)
	if err != nil {
		return "", err
	}

	proof, err := GenerateProof(challenge, c.privateKey)
	if err != nil {
		return "", err
	}

	token, err := c.queryAuthEndpoint(remoteHabitatUser, sessionID, proof)
	if err != nil {
		return "", err
	}

	return token, nil
}

func constructRemoteHabitatEndpoint(remoteHabitatUser *ExternalHabitatUser, path string) *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   remoteHabitatUser.Host,
		Path:   path,
	}
}
