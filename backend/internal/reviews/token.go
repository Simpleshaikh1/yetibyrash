// Package reviews collects reviews from real buyers and lets staff publish them.
package reviews

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewToken makes the secret in a review link and the hash stored for it.
func NewToken() (token string, hash []byte) {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token)
}

// HashToken is how a review link's secret is stored and looked up.
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte("review:" + token))
	return h[:]
}
