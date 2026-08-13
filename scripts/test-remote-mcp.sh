#!/bin/sh
# Smoke test for Engram's Remote MCP (Streamable HTTP) transport.
# Exercises the full DadeCore private-MCP auth flow (DCR -> owner-key gate
# -> PKCE token exchange) and core memory tools end-to-end against a
# running instance. Intended to be run from a SEPARATE machine/container
# from the Engram server itself, to prove real network reachability.
#
# Usage:
#   BASE_URL=https://engram.example.com \
#   OWNER_KEY=... \
#   ./scripts/test-remote-mcp.sh
#
# Or, for the simpler MCP_BEARER_TOKEN CLI path only:
#   BASE_URL=https://engram.example.com BEARER_TOKEN=... \
#   ./scripts/test-remote-mcp.sh --bearer-only
set -eu

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
OWNER_KEY="${OWNER_KEY:-}"
BEARER_TOKEN="${BEARER_TOKEN:-}"
BEARER_ONLY=0
[ "${1:-}" = "--bearer-only" ] && BEARER_ONLY=1

echo "== GET /health (no auth) =="
curl -sf "$BASE_URL/health"; echo

if [ "$BEARER_ONLY" = "1" ]; then
  [ -n "$BEARER_TOKEN" ] || { echo "BEARER_TOKEN is required for --bearer-only" >&2; exit 1; }
  AUTH_HEADER="Authorization: Bearer $BEARER_TOKEN"
else
  [ -n "$OWNER_KEY" ] || { echo "OWNER_KEY is required (or pass --bearer-only with BEARER_TOKEN)" >&2; exit 1; }

  echo "== GET /.well-known/oauth-authorization-server =="
  curl -sf "$BASE_URL/.well-known/oauth-authorization-server"; echo

  echo "== POST /register (DCR) =="
  REG=$(curl -sf -X POST "$BASE_URL/register" -H "Content-Type: application/json" \
    -d '{"redirect_uris":["http://localhost:9999/callback"],"client_name":"smoke-test"}')
  CLIENT_ID=$(echo "$REG" | python3 -c "import sys,json;print(json.load(sys.stdin)['client_id'])")

  VERIFIER="smoke-test-verifier-0123456789abcdefghijklmnopqrstuvwxyz"
  CHALLENGE=$(python3 -c "
import hashlib, base64
print(base64.urlsafe_b64encode(hashlib.sha256(b'$VERIFIER').digest()).decode().rstrip('='))")

  echo "== POST /authorize with WRONG owner key (must show explicit error) =="
  curl -sf -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/authorize" \
    --data-urlencode "client_id=$CLIENT_ID" \
    --data-urlencode "redirect_uri=http://localhost:9999/callback" \
    --data-urlencode "code_challenge=$CHALLENGE" \
    --data-urlencode "code_challenge_method=S256" \
    --data-urlencode "scope=mcp" \
    --data-urlencode "owner_key=definitely-wrong" || true

  echo "== POST /authorize with correct owner key =="
  LOC=$(curl -sf -D - -o /dev/null -X POST "$BASE_URL/authorize" \
    --data-urlencode "client_id=$CLIENT_ID" \
    --data-urlencode "redirect_uri=http://localhost:9999/callback" \
    --data-urlencode "code_challenge=$CHALLENGE" \
    --data-urlencode "code_challenge_method=S256" \
    --data-urlencode "scope=mcp" \
    --data-urlencode "owner_key=$OWNER_KEY" | grep -i "^Location:")
  CODE=$(echo "$LOC" | sed -n 's/.*code=\([^&]*\).*/\1/p' | tr -d '\r')

  echo "== POST /token (authorization_code + PKCE) =="
  TOKEN_RESP=$(curl -sf -X POST "$BASE_URL/token" \
    --data-urlencode "grant_type=authorization_code" \
    --data-urlencode "code=$CODE" \
    --data-urlencode "redirect_uri=http://localhost:9999/callback" \
    --data-urlencode "client_id=$CLIENT_ID" \
    --data-urlencode "code_verifier=$VERIFIER")
  ACCESS_TOKEN=$(echo "$TOKEN_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin)['access_token'])")
  AUTH_HEADER="Authorization: Bearer $ACCESS_TOKEN"
fi

MCP="$BASE_URL/mcp"
CT="Content-Type: application/json"
ACCEPT="Accept: application/json, text/event-stream"

echo "== MCP initialize =="
INIT_HEADERS=$(curl -sf -D - -o /tmp/_init_body.json -X POST "$MCP" -H "$AUTH_HEADER" -H "$CT" -H "$ACCEPT" -d '{
  "jsonrpc":"2.0","id":1,"method":"initialize",
  "params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke-test","version":"0.0.1"}}
}')
SESSION_ID=$(echo "$INIT_HEADERS" | grep -i "^Mcp-Session-Id:" | sed 's/.*: //I' | tr -d '\r')
cat /tmp/_init_body.json; echo
curl -sf -X POST "$MCP" -H "$AUTH_HEADER" -H "$CT" -H "$ACCEPT" -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' > /dev/null

call() { curl -sf -X POST "$MCP" -H "$AUTH_HEADER" -H "$CT" -H "$ACCEPT" -H "Mcp-Session-Id: $SESSION_ID" -d "$1"; }

echo "== tools/list =="
call '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | python3 -c "import sys,json; d=json.load(sys.stdin); print([t['name'] for t in d['result']['tools']])"

echo "== mem_save =="
call '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"mem_save","arguments":{"title":"Smoke test memory","type":"discovery","content":"Created by scripts/test-remote-mcp.sh"}}}'; echo

echo "== mem_search =="
call '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"mem_search","arguments":{"query":"Smoke test memory"}}}'; echo

echo "All checks passed."
