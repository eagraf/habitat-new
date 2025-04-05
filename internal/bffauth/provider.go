package bffauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/google/uuid"
)

// BFF Auth - Allow for Habitat node's to authenticate with each other.

type ChallengeRequest struct {
	DID string `json:"did"`
}

type friendStore map[string]*friend

type friend struct {
	DID       string `json:"did"`
	PublicKey crypto.PublicKey
}

type FriendRequest struct {
	DID                string `json:"did"`
	PublicKeyMultibase string `json:"public_key_multibase"`
}

type Provider struct {
	challengePersister ChallengeSessionPersister
	friends            friendStore
	signingKey         []byte
}

func NewProvider(challengePersister ChallengeSessionPersister, signingKey []byte) *Provider {
	return &Provider{
		challengePersister: challengePersister,
		friends:            make(friendStore),
		signingKey:         signingKey,
	}
}

func (p *Provider) GetRoutes() []api.Route {
	return []api.Route{
		api.NewBasicRoute(http.MethodPost, "/node/bff/challenge", p.handleChallenge),
		api.NewBasicRoute(http.MethodPost, "/node/bff/auth", p.handleAuth),
		api.NewBasicRoute(http.MethodPost, "/node/bff/add_friend", p.handleAddFriend),
		api.NewBasicRoute(http.MethodGet, "/node/bff/test", p.handleTest),
	}
}

func (p *Provider) handleChallenge(w http.ResponseWriter, r *http.Request) {
	var req ChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, ok := p.friends[req.DID]
	if !ok {
		http.Error(w, fmt.Sprintf("Friend with DID %s not found", req.DID), http.StatusNotFound)
		return
	}

	challenge, err := GenerateChallenge()
	if err != nil {
		http.Error(w, "Failed to generate challenge", http.StatusInternalServerError)
		return
	}

	// Create session with UUID
	sessionID := uuid.New().String()
	session := &ChallengeSession{
		SessionID: sessionID,
		DID:       req.DID,
		Challenge: challenge,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	err = p.challengePersister.SaveSession(session)
	if err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{
		"challenge": challenge,
		"session":   sessionID,
	})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (p *Provider) handleAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Proof     string `json:"proof"`
	}
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

	friend, ok := p.friends[session.DID]
	if !ok {
		http.Error(w, fmt.Sprintf("Friend with DID %s not found", session.DID), http.StatusNotFound)
		return
	}

	// Verify proof
	valid, err := VerifyProof(session.Challenge, req.Proof, friend.PublicKey)
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
	err = json.NewEncoder(w).Encode(map[string]string{
		"token": token,
	})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (p *Provider) handleAddFriend(w http.ResponseWriter, r *http.Request) {
	var req FriendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	pubKey, err := crypto.ParsePublicMultibase(req.PublicKeyMultibase)
	if err != nil {
		http.Error(w, "Invalid public key", http.StatusBadRequest)
		return
	}

	p.friends[req.DID] = &friend{
		DID:       req.DID,
		PublicKey: pubKey,
	}

	w.WriteHeader(http.StatusCreated)
}

func (p *Provider) handleTest(w http.ResponseWriter, r *http.Request) {
	err := ValidateRequest(r, p.signingKey)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unauthorized: %s", err), http.StatusUnauthorized)
		return
	}

	err = json.NewEncoder(w).Encode(map[string]string{
		"message": "Hello, world!",
	})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
