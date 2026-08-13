package main

// Remote MCP over Streamable HTTP — DadeCore addition.
//
// This file adds a second, additive runtime mode to `engram mcp`
// (`--transport=http`). It builds the exact same *server.MCPServer that
// stdio uses (via the existing mcp.NewServerWithConfig seam) and serves it
// with mark3labs/mcp-go's StreamableHTTPServer instead of ServeStdio.
// internal/mcp/mcp.go and internal/store are not touched by this file.
//
// Auth follows dadecore/standards/private-mcp: a single-owner OAuth 2.1
// Authorization Server embedded here (internal/remoteauth), with an
// owner-key gate as the mandatory first step, plus MCP_BEARER_TOKEN as a
// direct alternate path for CLI-style clients. See internal/remoteauth for
// the implementation; this file only wires it onto an http.ServeMux.
//
// This is deliberately independent of `engram cloud serve`
// (internal/cloud/*), which is an unrelated Postgres-backed sync/dashboard
// REST API, not MCP.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Gentleman-Programming/engram/internal/mcp"
	"github.com/Gentleman-Programming/engram/internal/remoteauth"
	"github.com/Gentleman-Programming/engram/internal/store"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func cmdMCPHTTP(cfg store.Config) {
	toolsFilter := ""
	projectOverride := strings.TrimSpace(os.Getenv("ENGRAM_PROJECT"))
	addr := strings.TrimSpace(os.Getenv("ENGRAM_HTTP_ADDR"))
	if addr == "" {
		port := strings.TrimSpace(os.Getenv("ENGRAM_MCP_PORT"))
		if port == "" {
			port = "8080"
		}
		addr = "0.0.0.0:" + port
	}

	for i := 2; i < len(os.Args); i++ {
		switch {
		case strings.HasPrefix(os.Args[i], "--tools="):
			toolsFilter = strings.TrimPrefix(os.Args[i], "--tools=")
		case os.Args[i] == "--tools" && i+1 < len(os.Args):
			toolsFilter = os.Args[i+1]
			i++
		case strings.HasPrefix(os.Args[i], "--project="):
			projectOverride = strings.TrimSpace(strings.TrimPrefix(os.Args[i], "--project="))
		case os.Args[i] == "--project" && i+1 < len(os.Args):
			projectOverride = strings.TrimSpace(os.Args[i+1])
			i++
		case strings.HasPrefix(os.Args[i], "--addr="):
			addr = strings.TrimPrefix(os.Args[i], "--addr=")
		case os.Args[i] == "--transport", strings.HasPrefix(os.Args[i], "--transport="):
			// consumed by the dispatcher in main(); ignore here.
			if os.Args[i] == "--transport" {
				i++
			}
		}
	}

	// --- Required DadeCore private-MCP auth config ---
	ownerKey := strings.TrimSpace(os.Getenv("ENGRAM_OWNER_KEY"))
	if ownerKey == "" {
		fatal(fmt.Errorf("ENGRAM_OWNER_KEY is required for --transport=http (single-owner Authorization Server gate; see dadecore/standards/private-mcp)"))
	}
	encKeyRaw := strings.TrimSpace(os.Getenv("ENCRYPTION_KEY"))
	if encKeyRaw == "" {
		fatal(fmt.Errorf("ENCRYPTION_KEY is required for --transport=http (32-byte key, base64 or hex, used for stateless AES-256-GCM tokens)"))
	}
	encKey, err := remoteauth.DecodeEncryptionKey(encKeyRaw)
	if err != nil {
		fatal(err)
	}
	cipher, err := remoteauth.NewCipher(encKey)
	if err != nil {
		fatal(err)
	}
	bearerToken := strings.TrimSpace(os.Getenv("MCP_BEARER_TOKEN"))
	issuer := strings.TrimSpace(os.Getenv("ENGRAM_PUBLIC_URL"))
	if issuer == "" {
		fatal(fmt.Errorf("ENGRAM_PUBLIC_URL is required for --transport=http (public HTTPS base URL of this deployment, e.g. https://engram.example.com — used to build OAuth discovery/issuer URLs)"))
	}
	authSrv := remoteauth.NewServer(cipher, ownerKey, bearerToken, issuer)

	// --- Same store + MCP server construction as stdio mode ---
	s, err := storeNew(cfg)
	if err != nil {
		fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	_, mgrStop := tryStartAutosync(ctx, s, cfg)
	stopped := false
	stopAutosync := func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		if mgrStop != nil {
			mgrStop()
		}
	}
	defer stopAutosync()

	mcpCfg := mcp.MCPConfig{DefaultProject: projectOverride}
	allowlist := resolveMCPTools(toolsFilter)
	mcpSrv := newMCPServerWithConfig(s, mcpCfg, allowlist)

	streamableSrv := mcpserver.NewStreamableHTTPServer(mcpSrv, mcpserver.WithEndpointPath("/mcp"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "engram-remote-mcp"})
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", authSrv.AuthorizationServerMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", authSrv.ProtectedResourceMetadata)
	mux.HandleFunc("/register", authSrv.Register)
	mux.HandleFunc("/authorize", authSrv.Authorize)
	mux.HandleFunc("/token", authSrv.Token)
	mux.Handle("/mcp", authSrv.RequireBearer(streamableSrv))

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if projectOverride == "" {
		log.Printf("[engram-remote-mcp] WARNING: no --project/ENGRAM_PROJECT set; remote tool calls resolve project via the same cwd/.engram/config.json/git heuristics as stdio mode, which has no meaningful cwd for a long-running server process. Set ENGRAM_PROJECT to pin a deterministic project (e.g. dadecore-standards).")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("[engram-remote-mcp] shutting down...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		stopAutosync()
		_ = streamableSrv.Shutdown(shutdownCtx)
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	log.Printf("[engram-remote-mcp] listening on %s (mcp=/mcp health=/health issuer=%s project=%q)", addr, issuer, projectOverride)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
}

// mcpTransportIsHTTP reports whether `engram mcp` was invoked with
// --transport=http (or --transport http), selecting the additive
// Streamable HTTP runtime mode instead of the default stdio transport.
func mcpTransportIsHTTP(args []string) bool {
	for i, a := range args {
		if a == "--transport=http" || a == "--transport=streamable-http" {
			return true
		}
		if (a == "--transport") && i+1 < len(args) {
			v := args[i+1]
			if v == "http" || v == "streamable-http" {
				return true
			}
		}
	}
	return false
}
