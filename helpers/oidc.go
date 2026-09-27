package helpers

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	GoogleTokenURL = "https://oauth2.googleapis.com/token"
	GoogleCertsURL = "https://www.googleapis.com/oauth2/v3/certs"
)

type GoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	IDToken     string `json:"id_token"`
}

type GoogleIDTokenClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwksCache struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

var googleJWKSCache = &jwksCache{
	keys: make(map[string]*rsa.PublicKey),
}

func getGooglePublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	googleJWKSCache.mu.RLock()
	if time.Now().Before(googleJWKSCache.expiresAt) {
		if key, ok := googleJWKSCache.keys[kid]; ok {
			googleJWKSCache.mu.RUnlock()
			return key, nil
		}
	}
	googleJWKSCache.mu.RUnlock()

	googleJWKSCache.mu.Lock()
	defer googleJWKSCache.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Now().Before(googleJWKSCache.expiresAt) {
		if key, ok := googleJWKSCache.keys[kid]; ok {
			return key, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GoogleCertsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for Google JWKS: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Google JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Google JWKS response: %w", err)
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("failed to parse Google JWKS response: %w", err)
	}

	parsedKeys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pubKey, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		parsedKeys[k.Kid] = pubKey
	}

	googleJWKSCache.keys = parsedKeys
	googleJWKSCache.expiresAt = time.Now().Add(1 * time.Hour)

	key, ok := parsedKeys[kid]
	if !ok {
		return nil, fmt.Errorf("Google public key with kid '%s' not found", kid)
	}
	return key, nil
}

func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("invalid modulus encoding: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("invalid exponent encoding: %w", err)
	}

	var eInt int
	if len(eBytes) < 4 {
		padded := make([]byte, 4)
		copy(padded[4-len(eBytes):], eBytes)
		eInt = int(binary.BigEndian.Uint32(padded))
	} else {
		eInt = int(binary.BigEndian.Uint32(eBytes))
	}

	nInt := new(big.Int).SetBytes(nBytes)
	return &rsa.PublicKey{
		N: nInt,
		E: eInt,
	}, nil
}

// VerifyGoogleIDToken validates the JWT signature against Google's JWKS and asserts
// standard OIDC claims (iss, aud, exp, and email_verified).
func VerifyGoogleIDToken(ctx context.Context, idTokenStr, expectedClientID string) (*GoogleIDTokenClaims, error) {
	claims := &GoogleIDTokenClaims{}

	token, err := jwt.ParseWithClaims(idTokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing algorithm in id_token: %v", token.Header["alg"])
		}
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, errors.New("missing kid header in id_token")
		}
		return getGooglePublicKey(ctx, kid)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to verify id_token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid id_token signature or claims")
	}

	// Validate Issuer
	validIssuers := []string{"https://accounts.google.com", "accounts.google.com"}
	isValidIssuer := false
	for _, validIss := range validIssuers {
		if claims.Issuer == validIss {
			isValidIssuer = true
			break
		}
	}
	if !isValidIssuer {
		return nil, fmt.Errorf("invalid token issuer: %s", claims.Issuer)
	}

	// Validate Audience (if configured)
	if expectedClientID != "" {
		audMatched := false
		for _, aud := range claims.Audience {
			if aud == expectedClientID {
				audMatched = true
				break
			}
		}
		if !audMatched {
			return nil, fmt.Errorf("token audience does not match configured client_id")
		}
	}

	// Validate Expiration
	if claims.ExpiresAt == nil || time.Now().After(claims.ExpiresAt.Time) {
		return nil, errors.New("id_token has expired")
	}

	// Validate Email Verification to prevent account takeover
	if !claims.EmailVerified {
		return nil, errors.New("google account email is not verified")
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return nil, errors.New("missing sub claim in id_token")
	}

	return claims, nil
}

// ExchangeGoogleCode performs a server-to-server TLS POST request to exchange the authorization code
// and code_verifier for tokens at the Google OAuth 2.0 token endpoint.
func ExchangeGoogleCode(ctx context.Context, code, verifier, clientID, clientSecret, redirectURI string) (*GoogleTokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", clientID)
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")
	data.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		GoogleTokenURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token exchange request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token exchange response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google token endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp GoogleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse Google token response: %w", err)
	}

	if tokenResp.IDToken == "" {
		return nil, errors.New("missing id_token in Google token response")
	}

	return &tokenResp, nil
}
