package controller

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/pkg/bffauth"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type AuthenticationMiddleware struct {
	nodeController NodeController
	useTLS         bool
	rootUserCert   *x509.Certificate
}

func NewAuthenticationMiddleware(ctrl NodeController, useTLS bool, rootUserCert *x509.Certificate) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{
		nodeController: ctrl,
		useTLS:         useTLS,
		rootUserCert:   rootUserCert,
	}
}

func (amw *AuthenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		var username string
		var userID string

		// Only authenticate if TLS is enabled. This is temporary.
		if amw.useTLS {
			if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.VerifiedChains[0]) == 0 {
				http.Error(w, "No client certificate found", http.StatusUnauthorized)
				return
			}

			// TODO we probably need to loop through all verified chains in the future
			reqCert := r.TLS.VerifiedChains[0][0]
			username := reqCert.Subject.CommonName

			var storedCert *x509.Certificate
			if username == constants.RootUsername {
				storedCert = amw.rootUserCert
				username = constants.RootUsername
				userID = constants.RootUserID
			} else {
				userCert, user, err := getUserCertAndInfo(amw.nodeController, username)
				if err != nil {
					http.Error(w, fmt.Sprintf("Error getting user certificate: %s", err), http.StatusInternalServerError)
					return
				}
				userID = user.ID
				storedCert = userCert
			}

			if !reqCert.Equal(storedCert) {
				http.Error(w, fmt.Sprintf("Certificate in request did not match certificate for username %s", username), http.StatusInternalServerError)
				return
			}
		} else {
			// TODO @eagraf - figure out a better way to make authenticating in dev mode more expedient
			username = "root"
			userID = "0"
		}

		log.Info().Msgf("Authenticated user: %s", username)

		ctx := r.Context()
		newCtx := context.WithValue(ctx, constants.ContextKeyUserID, userID)

		// Pass down the request to the next middleware (or final handler)
		next.ServeHTTP(w, r.WithContext(newCtx))
	})
}

func getUserCertAndInfo(controller NodeController, username string) (*x509.Certificate, *node.User, error) {
	// Look up the user in the node's user list
	user, err := controller.GetUserByUsername(username)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting user: %s", err)
	}

	block, _ := pem.Decode([]byte(user.Certificate))
	if block == nil {
		return nil, nil, errors.New("got nil block after decoding PEM")
	}

	if block.Type != "CERTIFICATE" {
		return nil, nil, errors.New("expected CERTIFICATE PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return cert, user, nil
}

// BFF Auth - Allow for Habitat node's to authenticate with each other.

type BFFChallengeRequest struct {
	DID string `json:"did"`
}

// BFFChallengeRoute returns a challenge string to the client, along with a challenge session ID.
type BFFChallengeRoute struct {
	challengePersister bffauth.ChallengeSessionPersister
	friends            FriendStore
}

func NewBFFChallengeRoute(challengePersister bffauth.ChallengeSessionPersister, friends FriendStore) *BFFChallengeRoute {
	return &BFFChallengeRoute{
		challengePersister: challengePersister,
		friends:            friends,
	}
}

func (h *BFFChallengeRoute) Pattern() string {
	return "/node/bff/challenge"
}

func (h *BFFChallengeRoute) Method() string {
	return http.MethodPost
}

func (h *BFFChallengeRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BFFChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO @eagraf - validate DID's public key

	_, ok := h.friends[req.DID]
	if !ok {
		http.Error(w, fmt.Sprintf("Friend with DID %s not found", req.DID), http.StatusNotFound)
		return
	}

	challenge, err := bffauth.GenerateChallenge()
	if err != nil {
		http.Error(w, "Failed to generate challenge", http.StatusInternalServerError)
		return
	}

	// Create session with UUID
	sessionID := uuid.New().String()
	session := &bffauth.ChallengeSession{
		SessionID: sessionID,
		DID:       req.DID,
		Challenge: challenge,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	err = h.challengePersister.SaveSession(session)
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

// BFFAuthRoute validates a proof of possession of a challenge string, and returns a JWT.
type BFFAuthRoute struct {
	challengePersister bffauth.ChallengeSessionPersister
	friends            FriendStore
	signingKey         []byte
}

func NewBFFAuthRoute(challengePersister bffauth.ChallengeSessionPersister, friends FriendStore, signingKey []byte) *BFFAuthRoute {
	return &BFFAuthRoute{
		challengePersister: challengePersister,
		friends:            friends,
		signingKey:         signingKey,
	}
}

func (h *BFFAuthRoute) Pattern() string {
	return "/node/bff/auth"
}

func (h *BFFAuthRoute) Method() string {
	return http.MethodPost
}

func (h *BFFAuthRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Proof     string `json:"proof"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := h.challengePersister.GetSession(req.SessionID)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusBadRequest)
		return
	}

	err = h.challengePersister.DeleteSession(req.SessionID)
	if err != nil {
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	// TODO @eagraf - get public key from the DNS / DID:plc
	friend, ok := h.friends[session.DID]
	if !ok {
		http.Error(w, fmt.Sprintf("Friend with DID %s not found", session.DID), http.StatusNotFound)
		return
	}

	// Verify proof
	valid, err := bffauth.VerifyProof(session.Challenge, req.Proof, friend.PublicKey)
	if err != nil || !valid {
		http.Error(w, "Invalid proof", http.StatusUnauthorized)
		return
	}
	log.Info().Msgf("Signing key while signing: %s", h.signingKey)

	// Generate JWT
	token, err := bffauth.GenerateJWT(h.signingKey)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Sanity check by validating the token
	_, err = bffauth.ValidateJWT(token, h.signingKey)
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

type FriendStore map[string]*Friend

type Friend struct {
	DID       string `json:"did"`
	PublicKey crypto.PublicKey
}

type BFFAddFriendRoute struct {
	friends FriendStore
}

type FriendRequest struct {
	DID                string `json:"did"`
	PublicKeyMultibase string `json:"public_key_multibase"`
}

func NewBFFAddFriendRoute(friends FriendStore) *BFFAddFriendRoute {
	return &BFFAddFriendRoute{friends: friends}
}

func (h *BFFAddFriendRoute) Pattern() string {
	return "/node/bff/add_friend"
}

func (h *BFFAddFriendRoute) Method() string {
	return http.MethodPost
}

func (h *BFFAddFriendRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	h.friends[req.DID] = &Friend{
		DID:       req.DID,
		PublicKey: pubKey,
	}

	w.WriteHeader(http.StatusCreated)
}

type BFFTestRoute struct {
	challengePersister bffauth.ChallengeSessionPersister
	signingKey         []byte
}

func NewBFFTestRoute(challengePersister bffauth.ChallengeSessionPersister, signingKey []byte) *BFFTestRoute {
	return &BFFTestRoute{
		challengePersister: challengePersister,
		signingKey:         signingKey,
	}
}

func (h *BFFTestRoute) Pattern() string {
	return "/node/bff/test"
}

func (h *BFFTestRoute) Method() string {
	return http.MethodGet
}

func (h *BFFTestRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	log.Info().Msgf("Authorization header: %s", authHeader)
	log.Info().Msgf("Signing key when validating: %s", h.signingKey)

	err := bffauth.ValidateRequest(r, h.signingKey)
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

func GetBFFRoutes(
	challengePersister bffauth.ChallengeSessionPersister,
	friends FriendStore,
	signingKey []byte,
) []api.Route {
	return []api.Route{
		NewBFFChallengeRoute(challengePersister, friends),
		NewBFFAuthRoute(challengePersister, friends, signingKey),
		NewBFFAddFriendRoute(friends),
		NewBFFTestRoute(challengePersister, signingKey),
	}
}
