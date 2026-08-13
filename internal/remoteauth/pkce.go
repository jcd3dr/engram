package remoteauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// VerifyPKCES256 checks a PKCE code_verifier against the code_challenge
// issued at /authorize time, using the S256 method required by OAuth 2.1
// and the MCP authorization spec. challenge must be the base64url(no
// padding) SHA-256 digest of verifier.
func VerifyPKCES256(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}
