package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTTestHelper manages JWT authentication test resources.
type JWTTestHelper struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	JWKS       string
	Token      string
	KID        string
}

// NewJWTTestHelper creates a new JWT test helper with generated keys.
func NewJWTTestHelper(kid string) (*JWTTestHelper, error) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	helper := &JWTTestHelper{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		KID:        kid,
	}

	// Generate JWKS
	jwks, err := helper.generateJWKS()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWKS: %w", err)
	}
	helper.JWKS = jwks

	// Generate JWT token with far future expiration
	token, err := helper.generateToken(map[string]interface{}{
		"sub":  "test-user",
		"name": "Test User",
		"iat":  time.Now().Unix(),
		"exp":  9999999999,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT token: %w", err)
	}
	helper.Token = token

	return helper, nil
}

// generateJWKS creates a JWKS JSON string from the public key.
func (h *JWTTestHelper) generateJWKS() (string, error) {
	// Convert the modulus (n) to base64url
	n := h.PublicKey.N.Bytes()
	nBase64 := base64.RawURLEncoding.EncodeToString(n)

	// Convert the exponent (e) to base64url
	e := big.NewInt(int64(h.PublicKey.E))
	eBytes := e.Bytes()
	eBase64 := base64.RawURLEncoding.EncodeToString(eBytes)

	// Create JWKS structure
	jwks := map[string]interface{}{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"kid": h.KID,
				"use": "sig",
				"alg": "RS256",
				"n":   nBase64,
				"e":   eBase64,
			},
		},
	}

	jwksBytes, err := json.Marshal(jwks)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWKS: %w", err)
	}

	return string(jwksBytes), nil
}

// generateToken creates a JWT token with the given claims.
func (h *JWTTestHelper) generateToken(claims map[string]interface{}) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	token.Header["kid"] = h.KID

	tokenString, err := token.SignedString(h.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// Cleanup removes any resources (currently no-op, but included for completeness).
func (h *JWTTestHelper) Cleanup() {
	// Zero out sensitive data
	if h.PrivateKey != nil {
		h.PrivateKey = nil
	}
	h.Token = ""
}
