// Package remoteauth implements the DadeCore private-MCP authentication
// standard (dadecore/standards/private-mcp) for Engram's Remote MCP
// (Streamable HTTP) transport: an embedded single-owner OAuth 2.1
// Authorization Server with an owner-key gate as the mandatory first step,
// PKCE S256, RFC 8414 discovery, RFC 7591 Dynamic Client Registration, and
// stateless AES-256-GCM encrypted tokens (no session/token storage).
//
// This package is purely additive: it does not modify internal/mcp or
// internal/store. It is only wired in by cmd/engram's HTTP transport mode.
package remoteauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidToken is returned for any malformed, tampered, or undecryptable
// token blob. Callers must not distinguish "malformed" from "wrong key" in
// responses (avoids oracle behavior); this is one bucket by design.
var ErrInvalidToken = errors.New("remoteauth: invalid token")

// Cipher encrypts/decrypts stateless token blobs with AES-256-GCM.
// The key is provided via ENCRYPTION_KEY (32 raw bytes, base64-std or hex
// encoded). No token or session state is ever persisted to disk or SQLite;
// every issued token is fully self-describing and verified only by
// decrypting it with this key.
type Cipher struct {
	gcm cipher.AEAD
}

// NewCipher builds a Cipher from a 32-byte AES-256 key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("remoteauth: ENCRYPTION_KEY must decode to exactly 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("remoteauth: building AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("remoteauth: building GCM: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

// EncodeJSON encrypts the JSON encoding of v and returns a URL-safe,
// unpadded base64 string suitable for use as an OAuth token/code value.
func (c *Cipher) EncodeJSON(v any) (string, error) {
	plaintext, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("remoteauth: marshal payload: %w", err)
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("remoteauth: generating nonce: %w", err)
	}
	sealed := c.gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// DecodeJSON decrypts a token produced by EncodeJSON into v. It returns
// ErrInvalidToken (wrapped) for any decoding, authentication, or unmarshal
// failure — callers should treat all of these identically as "reject".
func (c *Cipher) DecodeJSON(token string, v any) error {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return fmt.Errorf("%w: base64 decode: %v", ErrInvalidToken, err)
	}
	ns := c.gcm.NonceSize()
	if len(raw) < ns {
		return fmt.Errorf("%w: ciphertext too short", ErrInvalidToken)
	}
	nonce, ciphertext := raw[:ns], raw[ns:]
	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("%w: decrypt: %v", ErrInvalidToken, err)
	}
	if err := json.Unmarshal(plaintext, v); err != nil {
		return fmt.Errorf("%w: unmarshal: %v", ErrInvalidToken, err)
	}
	return nil
}

// DecodeEncryptionKey accepts ENCRYPTION_KEY as either 44-char standard
// base64 (32 raw bytes) or 64-char hex (32 raw bytes) and returns the raw
// key bytes.
func DecodeEncryptionKey(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := decodeHex(s); err == nil && len(b) == 32 {
		return b, nil
	}
	return nil, fmt.Errorf("remoteauth: ENCRYPTION_KEY must be 32 bytes as base64 or hex (got %d chars)", len(s))
}

func decodeHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd length hex string")
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi, err := hexNibble(s[i*2])
		if err != nil {
			return nil, err
		}
		lo, err := hexNibble(s[i*2+1])
		if err != nil {
			return nil, err
		}
		out[i] = hi<<4 | lo
	}
	return out, nil
}

func hexNibble(b byte) (byte, error) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', nil
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, nil
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, nil
	default:
		return 0, fmt.Errorf("invalid hex char %q", b)
	}
}
