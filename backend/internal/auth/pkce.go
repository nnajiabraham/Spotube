package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const verifierLength = 64

func GenerateCodeVerifier() (string, error) {
	buf := make([]byte, verifierLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func CodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
