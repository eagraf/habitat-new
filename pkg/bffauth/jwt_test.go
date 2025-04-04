package bffauth

import (
	"testing"
	"time"
)

func TestJWTFlow(t *testing.T) {
	// Create a test signing key
	signingKey := []byte("test-signing-key")

	// Generate JWT with the challenge
	token, err := GenerateJWT(signingKey)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	// Validate the JWT
	claims, err := ValidateJWT(token, signingKey)
	if err != nil {
		t.Fatalf("Failed to validate JWT: %v", err)
	}

	// Verify expiration time is set correctly (15 minutes from now)
	expectedExpiry := time.Now().Add(15 * time.Minute)
	if claims.ExpiresAt.Time.Sub(expectedExpiry) > time.Second {
		t.Errorf("Unexpected expiration time. Got %v, want approximately %v",
			claims.ExpiresAt.Time, expectedExpiry)
	}
}
