package remoteauth

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

// Server is the embedded, single-owner OAuth 2.1 Authorization Server
// required by dadecore/standards/private-mcp. It has no user/session store:
// every code and token is a self-describing encrypted blob (see crypto.go).
// The owner-key gate in Authorize is the mandatory first step of every
// authorization attempt.
type Server struct {
	Cipher      *Cipher
	OwnerKey    string // ENGRAM_OWNER_KEY - required, checked at the gate
	BearerToken string // MCP_BEARER_TOKEN - optional direct CLI auth path
	Issuer      string // public base URL, e.g. https://engram.dadecore.example (no trailing slash)

	AccessTTL  time.Duration
	RefreshTTL time.Duration
	CodeTTL    time.Duration
}

// NewServer applies DadeCore-standard defaults for token lifetimes.
func NewServer(c *Cipher, ownerKey, bearerToken, issuer string) *Server {
	return &Server{
		Cipher:      c,
		OwnerKey:    ownerKey,
		BearerToken: bearerToken,
		Issuer:      strings.TrimRight(issuer, "/"),
		AccessTTL:   time.Hour,
		RefreshTTL:  90 * 24 * time.Hour,
		CodeTTL:     2 * time.Minute,
	}
}

type clientMeta struct {
	RedirectURIs []string `json:"redirect_uris"`
	ClientName   string   `json:"client_name,omitempty"`
	CreatedAt    int64    `json:"created_at"`
}

type authCodePayload struct {
	Type                string `json:"type"` // "code"
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Scope               string `json:"scope"`
	Exp                 int64  `json:"exp"`
}

type tokenPayload struct {
	Type     string `json:"type"` // "access" | "refresh"
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
	Exp      int64  `json:"exp"`
}

// ---- RFC 8414 discovery ----

func (s *Server) AuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                s.Issuer,
		"authorization_endpoint":                s.Issuer + "/authorize",
		"token_endpoint":                        s.Issuer + "/token",
		"registration_endpoint":                 s.Issuer + "/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                      []string{"mcp"},
	})
}

// ProtectedResourceMetadata implements RFC 9728 so clients can discover
// where the authorization server for this MCP resource lives, per the
// 401 + WWW-Authenticate handshake used by Claude and the MCP auth spec.
func (s *Server) ProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"resource":              s.Issuer + "/mcp",
		"authorization_servers": []string{s.Issuer},
	})
}

// ---- RFC 7591 Dynamic Client Registration ----

func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.RedirectURIs) == 0 {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata", "redirect_uris is required")
		return
	}
	meta := clientMeta{RedirectURIs: req.RedirectURIs, ClientName: req.ClientName, CreatedAt: time.Now().Unix()}
	clientID, err := s.Cipher.EncodeJSON(meta)
	if err != nil {
		log.Printf("[remoteauth] register: encode client_id: %v", err)
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "could not register client")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"client_id":                  clientID,
		"redirect_uris":              req.RedirectURIs,
		"client_name":                req.ClientName,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}

// ---- Authorization endpoint: owner-key gate, then issue a code ----

var gateTmpl = template.Must(template.New("gate").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>Engram — Owner Authorization</title>
<style>body{font-family:system-ui,sans-serif;max-width:420px;margin:80px auto;padding:0 16px}
h1{font-size:1.1rem} input{width:100%;padding:8px;margin:8px 0;box-sizing:border-box}
button{padding:8px 16px} .err{color:#b00020;font-weight:600}</style></head>
<body>
<h1>Authorize access to this Engram memory</h1>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<p>Client: {{.ClientName}}</p>
<form method="post" action="{{.Action}}">
<label>Owner key<input type="password" name="owner_key" autofocus required></label>
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
<input type="hidden" name="state" value="{{.State}}">
<input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
<input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
<input type="hidden" name="scope" value="{{.Scope}}">
<button type="submit">Authorize</button>
</form>
</body></html>`))

type gateData struct {
	Error, Action, ClientID, ClientName, RedirectURI, State, CodeChallenge, CodeChallengeMethod, Scope string
}

func (s *Server) Authorize(w http.ResponseWriter, r *http.Request) {
	if s.OwnerKey == "" {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "ENGRAM_OWNER_KEY is not configured on the server")
		return
	}

	q := r.URL.Query()
	if r.Method == http.MethodPost {
		q = r.Form
		if err := r.ParseForm(); err != nil {
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "could not parse form")
			return
		}
		q = r.PostForm
	}

	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	state := q.Get("state")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")
	scope := q.Get("scope")

	var meta clientMeta
	if err := s.Cipher.DecodeJSON(clientID, &meta); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client", "unknown or malformed client_id")
		return
	}
	if !containsExact(meta.RedirectURIs, redirectURI) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "redirect_uri does not match registered client")
		return
	}
	if q.Get("response_type") != "code" && r.Method == http.MethodGet {
		writeOAuthError(w, http.StatusBadRequest, "unsupported_response_type", "only response_type=code is supported")
		return
	}
	if codeChallengeMethod != "S256" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "code_challenge_method must be S256 (PKCE is mandatory)")
		return
	}

	data := gateData{
		Action: s.Issuer + "/authorize", ClientID: clientID, ClientName: meta.ClientName,
		RedirectURI: redirectURI, State: state, CodeChallenge: codeChallenge,
		CodeChallengeMethod: codeChallengeMethod, Scope: scope,
	}

	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		gateTmpl.Execute(w, data)
		return
	}

	// POST: verify the owner key. This is the mandatory first gate — on
	// failure the flow stops here with a visible, explicit error. It never
	// falls through silently and never hangs.
	submitted := q.Get("owner_key")
	if subtle.ConstantTimeCompare([]byte(submitted), []byte(s.OwnerKey)) != 1 {
		data.Error = "Incorrect owner key. Access denied."
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		gateTmpl.Execute(w, data)
		return
	}

	code, err := s.Cipher.EncodeJSON(authCodePayload{
		Type: "code", ClientID: clientID, RedirectURI: redirectURI,
		CodeChallenge: codeChallenge, CodeChallengeMethod: codeChallengeMethod,
		Scope: scope, Exp: time.Now().Add(s.CodeTTL).Unix(),
	})
	if err != nil {
		log.Printf("[remoteauth] authorize: encode code: %v", err)
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "could not issue authorization code")
		return
	}

	dest := redirectURI + "?code=" + template.URLQueryEscaper(code)
	if state != "" {
		dest += "&state=" + template.URLQueryEscaper(state)
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

// ---- Token endpoint ----

func (s *Server) Token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "could not parse form body (expected application/x-www-form-urlencoded)")
		return
	}

	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.tokenFromCode(w, r)
	case "refresh_token":
		s.tokenFromRefresh(w, r)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
	}
}

func (s *Server) tokenFromCode(w http.ResponseWriter, r *http.Request) {
	code := r.PostForm.Get("code")
	verifier := r.PostForm.Get("code_verifier")
	redirectURI := r.PostForm.Get("redirect_uri")
	clientID := r.PostForm.Get("client_id")

	var payload authCodePayload
	if err := s.Cipher.DecodeJSON(code, &payload); err != nil || payload.Type != "code" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "invalid or expired authorization code")
		return
	}
	if time.Now().Unix() > payload.Exp {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authorization code has expired")
		return
	}
	if payload.RedirectURI != redirectURI || (clientID != "" && payload.ClientID != clientID) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri/client_id mismatch")
		return
	}
	if !VerifyPKCES256(verifier, payload.CodeChallenge) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	s.issueTokens(w, payload.ClientID, payload.Scope)
}

func (s *Server) tokenFromRefresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.PostForm.Get("refresh_token")
	var payload tokenPayload
	if err := s.Cipher.DecodeJSON(refreshToken, &payload); err != nil || payload.Type != "refresh" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "invalid refresh token")
		return
	}
	if time.Now().Unix() > payload.Exp {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "refresh token has expired")
		return
	}
	// Rotation: a fresh refresh token is issued on every use, per OAuth 2.1's
	// requirement to rotate refresh tokens for public clients. Because
	// tokens are stateless (no revocation list), the previous refresh token
	// remains cryptographically valid until its own expiry — this is a
	// deliberate, documented tradeoff of the no-storage design, not an
	// oversight. Keep RefreshTTL conservative if this matters to you.
	s.issueTokens(w, payload.ClientID, payload.Scope)
}

func (s *Server) issueTokens(w http.ResponseWriter, clientID, scope string) {
	now := time.Now()
	access, err := s.Cipher.EncodeJSON(tokenPayload{Type: "access", ClientID: clientID, Scope: scope, Exp: now.Add(s.AccessTTL).Unix()})
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "could not issue access token")
		return
	}
	refresh, err := s.Cipher.EncodeJSON(tokenPayload{Type: "refresh", ClientID: clientID, Scope: scope, Exp: now.Add(s.RefreshTTL).Unix()})
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "could not issue refresh token")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(s.AccessTTL.Seconds()),
		"refresh_token": refresh,
		"scope":         scope,
	})
}

// ---- Bearer verification middleware for the /mcp endpoint ----

// RequireBearer accepts either MCP_BEARER_TOKEN (direct CLI path) or a
// valid, unexpired access token minted by Token(). On failure it returns
// 401 with a WWW-Authenticate header pointing at protected-resource
// metadata, per RFC 9728 / the MCP authorization spec, so compliant clients
// (including Claude) can discover this authorization server automatically.
func (s *Server) RequireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(authz, "Bearer ")
		if !ok || token == "" {
			s.unauthorized(w)
			return
		}
		if s.BearerToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.BearerToken)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		var payload tokenPayload
		if err := s.Cipher.DecodeJSON(token, &payload); err != nil || payload.Type != "access" || time.Now().Unix() > payload.Exp {
			s.unauthorized(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata=%q`, s.Issuer+"/.well-known/oauth-protected-resource"))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
}

func writeOAuthError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": desc})
}

func containsExact(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
