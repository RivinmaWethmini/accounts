package helpers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GenerateCodeVerifier creates a high-entropy cryptographically random string
// conforming to RFC 7636 Section 4.1 (43-128 characters, unreserved URL characters).
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for code verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ComputeCodeChallenge computes the SHA-256 S256 code challenge from a code verifier
// as defined in RFC 7636 Section 4.2.
func ComputeCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GenerateStateToken creates a high-entropy cryptographically random state parameter
// used to prevent Cross-Site Request Forgery (CSRF) in OAuth flows.
func GenerateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for state token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
