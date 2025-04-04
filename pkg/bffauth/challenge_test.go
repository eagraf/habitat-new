package bffauth

import (
	"testing"

	"github.com/bluesky-social/indigo/atproto/crypto"
)

func TestChallengeProofFlow(t *testing.T) {
	// Generate a test key pair
	privateKey, err := crypto.GeneratePrivateKeyP256()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	publicKey, err := privateKey.PublicKey()
	if err != nil {
		t.Fatalf("Failed to generate public key: %v", err)
	}

	// Generate a challenge
	challenge, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("Failed to generate challenge: %v", err)
	}

	// Create proof with private key
	proof, err := GenerateProof(challenge, privateKey)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	// Verify proof with public key
	valid, err := VerifyProof(challenge, proof, publicKey)
	if err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}

	if !valid {
		t.Error("Proof verification failed")
	}
}
