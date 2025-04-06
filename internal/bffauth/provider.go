package bffauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// BFF Auth - Allow for Habitat node's to authenticate with each other.
type Provider struct {
	challengePersister ChallengeSessionPersister
	signingKey         []byte
}

func NewProvider(challengePersister ChallengeSessionPersister, signingKey []byte) *Provider {
	return &Provider{
		challengePersister: challengePersister,
		signingKey:         signingKey,
	}
}

func (p *Provider) GetRoutes() []api.Route {
	return []api.Route{
		api.NewBasicRoute(http.MethodPost, "/node/bff/challenge", p.handleChallenge),
		api.NewBasicRoute(http.MethodPost, "/node/bff/auth", p.handleAuth),
		api.NewBasicRoute(http.MethodGet, "/node/bff/test", p.handleTest),
		api.NewBasicRoute(http.MethodPost, "/node/bff/test-client", p.handleTestClient),
	}
}

// p.handleChallenge takes the following http request
type ChallengeRequest struct {
	DID                string `json:"did"`
	PublicKeyMultibase string `json:"public_key_multibase"`
}

// p.handleChallenge responds with the following response
type ChallengeResponse struct {
	Challenge string `json:"challenge"`
	Session   string `json:"session"`
}

func (p *Provider) handleChallenge(w http.ResponseWriter, r *http.Request) {
	var req ChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	challenge, err := GenerateChallenge()
	if err != nil {
		http.Error(w, "Failed to generate challenge", http.StatusInternalServerError)
		return
	}

	key, err := crypto.ParsePublicMultibase(req.PublicKeyMultibase)
	if err != nil {
		http.Error(w, "Failed to parse public key", http.StatusInternalServerError)
		return
	}

	// Create session with UUID
	sessionID := uuid.New().String()
	session := &ChallengeSession{
		SessionID: sessionID,
		DID:       req.DID,
		PublicKey: key,
		Challenge: challenge,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	err = p.challengePersister.SaveSession(session)
	if err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp, err := json.Marshal(ChallengeResponse{
		Challenge: challenge,
		Session:   sessionID,
	})
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(resp); err != nil {
		log.Err(err).Msgf("error sending response in handleChallenge")
	}
}

type AuthRequest struct {
	SessionID string `json:"session_id"`
	Proof     string `json:"proof"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

func (p *Provider) handleAuth(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := p.challengePersister.GetSession(req.SessionID)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusBadRequest)
		return
	}

	err = p.challengePersister.DeleteSession(req.SessionID)
	if err != nil {
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	// Verify proof
	valid, err := VerifyProof(session.Challenge, req.Proof, session.PublicKey)
	if err != nil || !valid {
		http.Error(w, "Invalid proof", http.StatusUnauthorized)
		return
	}

	// Generate JWT
	token, err := GenerateJWT(p.signingKey)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Sanity check by validating the token
	_, err = ValidateJWT(token, p.signingKey)
	if err != nil {
		http.Error(w, "Failed to validate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp, err := json.Marshal(AuthResponse{
		Token: token,
	})
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(resp); err != nil {
		log.Err(err).Msgf("error sending response in handleAuth")
	}
}

func (p *Provider) handleTest(w http.ResponseWriter, r *http.Request) {
	err := ValidateRequest(r, p.signingKey)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unauthorized: %s", err), http.StatusUnauthorized)
		return
	}

	bytes, err := json.Marshal(map[string]string{
		"message": "Hello, world!",
	})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(bytes)
	if err != nil {
		log.Err(err).Msgf("error sending response in handleTest")
	}

}

type TestClientRequest struct {
	ClientDID string `json:"client_did"`
	TargetDID string `json:"target_did"`
}

type TestClientResponse struct {
	Token string `json:"token"`
}

func (p *Provider) handleTestClient(w http.ResponseWriter, r *http.Request) {
	reqBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %s", err), http.StatusInternalServerError)
		return
	}

	var reqBody TestClientRequest
	err = json.Unmarshal(reqBytes, &reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to unmarshal request body: %s", err), http.StatusInternalServerError)
		return
	}

	client, err := NewClient(reqBody.ClientDID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create client: %s", err), http.StatusInternalServerError)
		return
	}

	token, err := client.GetToken(reqBody.TargetDID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get token: %s", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp, err := json.Marshal(TestClientResponse{
		Token: token,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal response: %s", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(resp); err != nil {
		log.Err(err).Msgf("error sending response in handleTestClient")
	}
}
