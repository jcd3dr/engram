package setup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func resetSetupSeams(t *testing.T) {
	t.Helper()
	oldRuntimeGOOS := runtimeGOOS
	oldUserHomeDir := userHomeDir
	oldLookPathFn := lookPathFn
	oldRunCommand := runCommand
	oldStatFn := statFn
	oldOpenCodeReadFile := openCodeReadFile
	oldOpenCodeWriteFileFn := openCodeWriteFileFn
	oldReadFileFn := readFileFn
	oldWriteFileFn := writeFileFn
	oldJSONMarshalFn := jsonMarshalFn
	oldJSONMarshalIndentFn := jsonMarshalIndentFn
	oldInjectOpenCodeMCPFn := injectOpenCodeMCPFn
	oldInjectOpenCodeTUIPluginFn := injectOpenCodeTUIPluginFn
	oldInjectGeminiMCPFn := injectGeminiMCPFn
	oldWriteGeminiSystemPromptFn := writeGeminiSystemPromptFn
	oldWriteCodexMemoryInstructionFilesFn := writeCodexMemoryInstructionFilesFn
	oldInjectCodexMCPFn := injectCodexMCPFn
	oldInjectCodexMemoryConfigFn := injectCodexMemoryConfigFn
	oldAddClaudeCodeAllowlistFn := addClaudeCodeAllowlistFn
	oldOsExecutable := osExecutable
	oldWriteClaudeCodeUserMCPFn := writeClaudeCodeUserMCPFn
	oldResolveMiseNodeVersionFn := resolveMiseNodeVersionFn

	t.Cleanup(func() {
		runtimeGOOS = oldRuntimeGOOS
		userHomeDir = oldUserHomeDir
		lookPathFn = oldLookPathFn
		runCommand = oldRunCommand
		statFn = oldStatFn
		openCodeReadFile = oldOpenCodeReadFile
		openCodeWriteFileFn = oldOpenCodeWriteFileFn
		readFileFn = oldReadFileFn
		writeFileFn = oldWriteFileFn
		jsonMarshalFn = oldJSONMarshalFn
		jsonMarshalIndentFn = oldJSONMarshalIndentFn
		injectOpenCodeMCPFn = oldInjectOpenCodeMCPFn
		injectOpenCodeTUIPluginFn = oldInjectOpenCodeTUIPluginFn
		injectGeminiMCPFn = oldInjectGeminiMCPFn
		writeGeminiSystemPromptFn = oldWriteGeminiSystemPromptFn
		writeCodexMemoryInstructionFilesFn = oldWriteCodexMemoryInstructionFilesFn
		injectCodexMCPFn = oldInjectCodexMCPFn
		injectCodexMemoryConfigFn = oldInjectCodexMemoryConfigFn
		addClaudeCodeAllowlistFn = oldAddClaudeCodeAllowlistFn
		osExecutable = oldOsExecutable
		writeClaudeCodeUserMCPFn = oldWriteClaudeCodeUserMCPFn
		resolveMiseNodeVersionFn = oldResolveMiseNodeVersionFn
	})
}

func useTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	userHomeDir = func() (string, error) { return home, nil }
	return home
}

func TestSupportedAgentsIncludesGeminiAndCodex(t *testing.T) {
	agents := SupportedAgents()

	var hasGemini bool
	var hasCodex bool
	for _, agent := range agents {
		if agent.Name == "gemini-cli" {
			hasGemini = true
		}
		if agent.Name == "codex" {
			hasCodex = true
		}
	}

	if !hasGemini {
		t.Fatalf("expected gemini-cli in supported agents")
	}
	if !hasCodex {
		t.Fatalf("expected codex in supported agents")
	}
}

func TestInstallGeminiCLIInjectsMCPConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	original := `{"theme":"dark","mcpServers":{"other":{"command":"foo","args":["bar"]}}}`
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	result, err := Install("gemini-cli")
	if err != nil {
		t.Fatalf("install gemini-cli: %v", err)
	}

	if result.Agent != "gemini-cli" {
		t.Fatalf("unexpected agent in result: %q", result.Agent)
	}

	if result.Files != 2 {
		t.Fatalf("expected 2 files written, got %d", result.Files)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse settings: %v", err)
	}

	mcpServers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("expected mcpServers object")
	}

	engram, ok := mcpServers["engram"].(map[string]any)
	if !ok {
		t.Fatalf("expected mcpServers.engram object")
	}

	if engram["httpUrl"] != engramRemoteMCPURL {
		t.Fatalf("expected httpUrl %q, got %#v", engramRemoteMCPURL, engram["httpUrl"])
	}
	if _, hasCommand := engram["command"]; hasCommand {
		t.Fatalf("remote Gemini entry must not carry a local command")
	}

	if _, ok := mcpServers["other"]; !ok {
		t.Fatalf("expected existing mcp server to be preserved")
	}

	systemPath := filepath.Join(home, ".gemini", "system.md")
	systemRaw, err := os.ReadFile(systemPath)
	if err != nil {
		t.Fatalf("read system prompt: %v", err)
	}
	systemText := string(systemRaw)
	if !strings.Contains(systemText, "### AFTER COMPACTION") {
		t.Fatalf("expected AFTER COMPACTION section in system prompt")
	}
	if !strings.Contains(systemText, "FIRST ACTION REQUIRED") {
		t.Fatalf("expected FIRST ACTION REQUIRED guidance in system prompt")
	}

	// GEMINI_SYSTEM_MD should NOT be set (it breaks Gemini outside $HOME)
	envPath := filepath.Join(home, ".gemini", ".env")
	if _, err := os.Stat(envPath); err == nil {
		envRaw, _ := os.ReadFile(envPath)
		if strings.Contains(string(envRaw), "GEMINI_SYSTEM_MD") {
			t.Fatalf("GEMINI_SYSTEM_MD should not be present in .env, got:\n%s", string(envRaw))
		}
	}

	if _, err := Install("gemini-cli"); err != nil {
		t.Fatalf("second install should be idempotent: %v", err)
	}
}

func TestInstallCodexInjectsTOMLAndIsIdempotent(t *testing.T) {
	resetSetupSeams(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	lookPathFn = func(string) (string, error) { return "", errors.New("codex not installed") }

	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	original := strings.Join([]string{
		"[profile]",
		"name = \"dev\"",
		"",
		"[mcp_servers.existing]",
		"command = \"existing\"",
		"args = [\"x\"]",
		"",
		"[mcp_servers.engram]",
		"command = \"wrong\"",
		"args = [\"wrong\"]",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	result, err := Install("codex")
	if err != nil {
		t.Fatalf("install codex: %v", err)
	}

	if result.Agent != "codex" {
		t.Fatalf("unexpected agent in result: %q", result.Agent)
	}

	if result.Files != 3 {
		t.Fatalf("expected 3 files written, got %d", result.Files)
	}

	readAndAssert := func() string {
		t.Helper()
		raw, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read codex config: %v", err)
		}
		text := string(raw)

		if !strings.Contains(text, "[profile]") {
			t.Fatalf("expected existing profile section to be preserved")
		}
		if !strings.Contains(text, "[mcp_servers.existing]") {
			t.Fatalf("expected existing mcp server section to be preserved")
		}
		if strings.Count(text, "[mcp_servers.engram]") != 1 {
			t.Fatalf("expected exactly one engram section, got:\n%s", text)
		}
		// The Codex entry is a remote url, never a local command.
		if !strings.Contains(text, `url = "`+engramRemoteMCPURL+`"`) {
			t.Fatalf("expected engram remote url in config, got:\n%s", text)
		}
		if section := codexEngramSection(text); strings.Contains(section, "command = ") {
			t.Fatalf("remote Codex entry must not carry a local command, got:\n%s", text)
		}
		instructionsPath := filepath.Join(home, ".codex", "engram-instructions.md")
		if !strings.Contains(text, "model_instructions_file = \""+instructionsPath+"\"") {
			t.Fatalf("expected model_instructions_file in config, got:\n%s", text)
		}
		compactPromptPath := filepath.Join(home, ".codex", "engram-compact-prompt.md")
		if !strings.Contains(text, "experimental_compact_prompt_file = \""+compactPromptPath+"\"") {
			t.Fatalf("expected compact prompt file key in config, got:\n%s", text)
		}
		firstSection := strings.Index(text, "[profile]")
		if firstSection == -1 {
			t.Fatalf("expected [profile] section in config")
		}
		if idx := strings.Index(text, "model_instructions_file"); idx == -1 || idx > firstSection {
			t.Fatalf("expected model_instructions_file to be top-level before sections, got:\n%s", text)
		}
		if idx := strings.Index(text, "experimental_compact_prompt_file"); idx == -1 || idx > firstSection {
			t.Fatalf("expected compact prompt key to be top-level before sections, got:\n%s", text)
		}
		return text
	}

	first := readAndAssert()

	if _, err := Install("codex"); err != nil {
		t.Fatalf("second install should be idempotent: %v", err)
	}

	second := readAndAssert()
	if first != second {
		t.Fatalf("expected no changes on second install")
	}

	instructionsRaw, err := os.ReadFile(filepath.Join(home, ".codex", "engram-instructions.md"))
	if err != nil {
		t.Fatalf("read codex instructions: %v", err)
	}
	if !strings.Contains(string(instructionsRaw), "### AFTER COMPACTION") {
		t.Fatalf("expected AFTER COMPACTION section in codex instructions")
	}

	compactRaw, err := os.ReadFile(filepath.Join(home, ".codex", "engram-compact-prompt.md"))
	if err != nil {
		t.Fatalf("read codex compact prompt: %v", err)
	}
	if !strings.Contains(string(compactRaw), "FIRST ACTION REQUIRED") {
		t.Fatalf("expected FIRST ACTION REQUIRED text in compact prompt")
	}
}

// TestInstallCodexPluginCLIPresent verifies that when the codex CLI is in PATH,
// installCodex() runs marketplace add + plugin add with the correct arguments.
func TestInstallCodexPluginCLIPresent(t *testing.T) {
	resetSetupSeams(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	var commands [][]string
	lookPathFn = func(file string) (string, error) {
		if file == "codex" {
			return "/usr/local/bin/codex", nil
		}
		return "", errors.New("not found")
	}
	runCommand = func(name string, args ...string) ([]byte, error) {
		commands = append(commands, append([]string{name}, args...))
		return []byte("ok"), nil
	}

	result, err := Install("codex")
	if err != nil {
		t.Fatalf("Install(codex) failed: %v", err)
	}
	if result.Agent != "codex" {
		t.Fatalf("unexpected agent: %q", result.Agent)
	}
	if result.Files != 3 {
		t.Fatalf("expected 3 files written, got %d", result.Files)
	}

	// Verify marketplace add was called with the right args.
	var foundMarketplace bool
	for _, cmd := range commands {
		if len(cmd) >= 7 &&
			cmd[0] == "/usr/local/bin/codex" &&
			cmd[1] == "plugin" && cmd[2] == "marketplace" && cmd[3] == "add" &&
			cmd[4] == codexMarketplace &&
			cmd[5] == "--ref" && cmd[6] == "main" {
			foundMarketplace = true
		}
	}
	if !foundMarketplace {
		t.Fatalf("expected 'codex plugin marketplace add %s --ref main' to be invoked, got: %v", codexMarketplace, commands)
	}

	// Verify plugin add was called with the right args.
	var foundPluginAdd bool
	for _, cmd := range commands {
		if len(cmd) >= 4 &&
			cmd[0] == "/usr/local/bin/codex" &&
			cmd[1] == "plugin" && cmd[2] == "add" && cmd[3] == "engram@engram" {
			foundPluginAdd = true
		}
	}
	if !foundPluginAdd {
		t.Fatalf("expected 'codex plugin add engram@engram' to be invoked, got: %v", commands)
	}
}

// TestInstallCodexPluginCLIAbsent verifies that when the codex CLI is not in
// PATH, installCodex() does not fail — MCP config is still written and Files==3.
func TestInstallCodexPluginCLIAbsent(t *testing.T) {
	resetSetupSeams(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	lookPathFn = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	runCommand = func(name string, args ...string) ([]byte, error) {
		t.Fatalf("runCommand should not be called when codex CLI is absent, got: %s %v", name, args)
		return nil, nil
	}

	result, err := Install("codex")
	if err != nil {
		t.Fatalf("Install(codex) should succeed even without codex CLI, got: %v", err)
	}
	if result.Agent != "codex" {
		t.Fatalf("unexpected agent: %q", result.Agent)
	}
	if result.Files != 3 {
		t.Fatalf("expected 3 files written, got %d", result.Files)
	}

	// Verify the TOML config was still written.
	configPath := filepath.Join(home, ".codex", "config.toml")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected config.toml to be written: %v", err)
	}
	if !strings.Contains(string(raw), "[mcp_servers.engram]") {
		t.Fatalf("expected [mcp_servers.engram] in config, got:\n%s", raw)
	}
}

// TestInstallCodexPluginIdempotentAlreadyInOutput verifies that when
// marketplace add or plugin add returns an error whose output contains "already",
// the install is still treated as successful (idempotent).
func TestInstallCodexPluginIdempotentAlreadyInOutput(t *testing.T) {
	resetSetupSeams(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	lookPathFn = func(file string) (string, error) {
		if file == "codex" {
			return "/usr/local/bin/codex", nil
		}
		return "", errors.New("not found")
	}
	calls := 0
	runCommand = func(name string, args ...string) ([]byte, error) {
		calls++
		if calls == 1 {
			// marketplace add returns "already exists" with a non-zero exit
			return []byte("marketplace already added"), errors.New("exit 1")
		}
		if calls == 2 {
			// plugin add returns "already installed" with a non-zero exit
			return []byte("plugin already installed"), errors.New("exit 1")
		}
		return []byte("ok"), nil
	}

	result, err := Install("codex")
	if err != nil {
		t.Fatalf("Install(codex) should succeed on already-installed outputs, got: %v", err)
	}
	if result.Files != 3 {
		t.Fatalf("expected 3 files written, got %d", result.Files)
	}
	if calls < 2 {
		t.Fatalf("expected at least 2 codex CLI calls, got %d", calls)
	}
}

func TestInstallPiInstallsPackagesAndWritesConfig(t *testing.T) {
	resetSetupSeams(t)
	agentDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	osExecutable = func() (string, error) { return "/opt/engram/bin/engram", nil }

	var commands []string
	runCommand = func(name string, args ...string) ([]byte, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return []byte("ok"), nil
	}

	result, err := Install("pi")
	if err != nil {
		t.Fatalf("Install(pi) failed: %v", err)
	}
	if result.Agent != "pi" || result.Destination != agentDir || result.Files != 2 {
		t.Fatalf("unexpected install result: %#v", result)
	}
	wantCommands := []string{"pi install npm:gentle-engram@0.1.8", "pi install npm:pi-mcp-adapter"}
	if !reflect.DeepEqual(commands, wantCommands) {
		t.Fatalf("unexpected pi install commands: got %#v want %#v", commands, wantCommands)
	}

	settingsRaw, err := os.ReadFile(filepath.Join(agentDir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(settingsRaw, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	for _, pkg := range []string{"npm:gentle-engram@0.1.8", "npm:pi-mcp-adapter"} {
		if !slices.Contains(settings.Packages, pkg) {
			t.Fatalf("expected settings packages to include %q, got %#v", pkg, settings.Packages)
		}
	}

	mcpRaw, err := os.ReadFile(filepath.Join(agentDir, "mcp.json"))
	if err != nil {
		t.Fatalf("read mcp: %v", err)
	}
	var mcpConfig struct {
		MCPServers map[string]struct {
			URL string `json:"url"`

			Lifecycle   string `json:"lifecycle"`
			DirectTools bool   `json:"directTools"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(mcpRaw, &mcpConfig); err != nil {
		t.Fatalf("parse mcp: %v", err)
	}
	server, ok := mcpConfig.MCPServers["engram"]
	if !ok {
		t.Fatalf("expected mcpServers.engram in %#v", mcpConfig.MCPServers)
	}
	if server.URL != engramRemoteMCPURL || server.Lifecycle != "lazy" || server.DirectTools {
		t.Fatalf("unexpected engram MCP server: %#v", server)
	}
}

func TestInstallPiPreservesExistingEngramMCPServer(t *testing.T) {
	resetSetupSeams(t)
	agentDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	runCommand = func(string, ...string) ([]byte, error) { return []byte("ok"), nil }

	settingsPath := filepath.Join(agentDir, "settings.json")
	mcpPath := filepath.Join(agentDir, "mcp.json")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"packages":["npm:existing"]}`), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	originalMCP := `{"mcpServers":{"engram":{"command":"custom-engram","args":["mcp"],"lifecycle":"eager"},"other":{"command":"other"}}}`
	if err := os.WriteFile(mcpPath, []byte(originalMCP), 0644); err != nil {
		t.Fatalf("write mcp: %v", err)
	}

	result, err := Install("pi")
	if err != nil {
		t.Fatalf("Install(pi) failed: %v", err)
	}
	if result.Files != 1 {
		t.Fatalf("expected only settings to change, got files=%d", result.Files)
	}
	mcpAfter, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("read mcp after install: %v", err)
	}
	if string(mcpAfter) != originalMCP {
		t.Fatalf("expected existing mcp server to be preserved, got %s", mcpAfter)
	}

	settingsRaw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings after install: %v", err)
	}
	if !strings.Contains(string(settingsRaw), "npm:existing") || !strings.Contains(string(settingsRaw), "npm:gentle-engram@0.1.8") || !strings.Contains(string(settingsRaw), "npm:pi-mcp-adapter") {
		t.Fatalf("expected settings packages to be preserved and extended, got %s", settingsRaw)
	}
}

func TestInstallPiCommandFailure(t *testing.T) {
	resetSetupSeams(t)
	runCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("boom"), errors.New("exit 1")
	}
	_, err := Install("pi")
	if err == nil || !strings.Contains(err.Error(), "install npm:gentle-engram@0.1.8") {
		t.Fatalf("expected pi install error, got %v", err)
	}
}

// TestEnsurePiNpmCommandWritesMiseCommand verifies that when mise is detected
// and no npmCommand exists in settings.json, a stable mise-pinned command is written.
func TestEnsurePiNpmCommandWritesMiseCommand(t *testing.T) {
	resetSetupSeams(t)
	agentDir := t.TempDir()
	settingsPath := filepath.Join(agentDir, "settings.json")

	// mise is found in PATH
	lookPathFn = func(file string) (string, error) {
		if file == "mise" {
			return "/usr/local/bin/mise", nil
		}
		return "", errors.New("not found")
	}
	// mise current node returns a version
	resolveMiseNodeVersionFn = func() string { return "node@22.12.0" }

	changed, err := ensurePiNpmCommand(settingsPath)
	if err != nil {
		t.Fatalf("ensurePiNpmCommand failed: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true when mise detected and no npmCommand set")
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	npmCmdRaw, ok := settings["npmCommand"]
	if !ok {
		t.Fatalf("expected npmCommand in settings, got %s", raw)
	}
	var npmCmd []string
	if err := json.Unmarshal(npmCmdRaw, &npmCmd); err != nil {
		t.Fatalf("parse npmCommand: %v", err)
	}
	want := []string{"mise", "exec", "node@22.12.0", "--", "npm"}
	if !reflect.DeepEqual(npmCmd, want) {
		t.Fatalf("expected npmCommand %v, got %v", want, npmCmd)
	}
}

// TestEnsurePiNpmCommandPreservesExisting verifies that an existing npmCommand
// in settings.json is never overwritten (idempotent / user-override safe).
func TestEnsurePiNpmCommandPreservesExisting(t *testing.T) {
	resetSetupSeams(t)
	agentDir := t.TempDir()
	settingsPath := filepath.Join(agentDir, "settings.json")

	existing := `{"npmCommand":["mise","exec","node@20.0.0","--","npm"]}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	// mise is found — but should NOT overwrite the user's existing command
	lookPathFn = func(file string) (string, error) {
		if file == "mise" {
			return "/usr/local/bin/mise", nil
		}
		return "", errors.New("not found")
	}
	resolveMiseNodeVersionFn = func() string { return "node@25.9.0" }

	changed, err := ensurePiNpmCommand(settingsPath)
	if err != nil {
		t.Fatalf("ensurePiNpmCommand failed: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false when npmCommand already set")
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if string(raw) != existing {
		t.Fatalf("expected settings to be unchanged, got %s", raw)
	}
}

// TestEnsurePiNpmCommandNoMise verifies that when mise is not found,
// no npmCommand is written.
func TestEnsurePiNpmCommandNoMise(t *testing.T) {
	resetSetupSeams(t)
	agentDir := t.TempDir()
	settingsPath := filepath.Join(agentDir, "settings.json")

	// mise not found
	lookPathFn = func(file string) (string, error) {
		return "", errors.New("not found")
	}

	changed, err := ensurePiNpmCommand(settingsPath)
	if err != nil {
		t.Fatalf("ensurePiNpmCommand failed: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false when mise is not detected")
	}

	// settings.json should not have been created
	if _, err := os.Stat(settingsPath); err == nil {
		t.Fatalf("expected settings.json not to be created when mise is absent")
	}
}

// TestEnsurePiNpmCommandMiseVersionFallback verifies that when mise is found
// but the version cannot be resolved, "node" is used as the spec fallback.
func TestEnsurePiNpmCommandMiseVersionFallback(t *testing.T) {
	resetSetupSeams(t)
	agentDir := t.TempDir()
	settingsPath := filepath.Join(agentDir, "settings.json")

	lookPathFn = func(file string) (string, error) {
		if file == "mise" {
			return "/usr/local/bin/mise", nil
		}
		return "", errors.New("not found")
	}
	// version resolution fails — returns empty string
	resolveMiseNodeVersionFn = func() string { return "" }

	changed, err := ensurePiNpmCommand(settingsPath)
	if err != nil {
		t.Fatalf("ensurePiNpmCommand failed: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true even when version resolution fails")
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	var npmCmd []string
	if err := json.Unmarshal(settings["npmCommand"], &npmCmd); err != nil {
		t.Fatalf("parse npmCommand: %v", err)
	}
	// Fallback: mise exec node -- npm (no version specifier)
	want := []string{"mise", "exec", "node", "--", "npm"}
	if !reflect.DeepEqual(npmCmd, want) {
		t.Fatalf("expected fallback npmCommand %v, got %v", want, npmCmd)
	}
}

// TestInstallPiWritesNpmCommandWhenMiseDetected verifies that the full
// installPi() flow writes npmCommand to settings.json when mise is available.
func TestInstallPiWritesNpmCommandWhenMiseDetected(t *testing.T) {
	resetSetupSeams(t)
	agentDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	osExecutable = func() (string, error) { return "/opt/engram/bin/engram", nil }
	runCommand = func(string, ...string) ([]byte, error) { return []byte("ok"), nil }

	lookPathFn = func(file string) (string, error) {
		if file == "mise" {
			return "/usr/local/bin/mise", nil
		}
		return "", errors.New("not found")
	}
	resolveMiseNodeVersionFn = func() string { return "node@25.9.0" }

	result, err := Install("pi")
	if err != nil {
		t.Fatalf("Install(pi) failed: %v", err)
	}
	// settings.json changed (packages + npmCommand) + mcp.json
	if result.Files != 2 {
		t.Fatalf("expected 2 files written, got %d", result.Files)
	}

	raw, err := os.ReadFile(filepath.Join(agentDir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	var npmCmd []string
	if err := json.Unmarshal(settings["npmCommand"], &npmCmd); err != nil {
		t.Fatalf("parse npmCommand: %v", err)
	}
	want := []string{"mise", "exec", "node@25.9.0", "--", "npm"}
	if !reflect.DeepEqual(npmCmd, want) {
		t.Fatalf("expected npmCommand %v, got %v", want, npmCmd)
	}
}

// TestInstallPiNoNpmCommandWhenNoMise verifies that installPi() does not write
// npmCommand when mise is absent.
func TestInstallPiNoNpmCommandWhenNoMise(t *testing.T) {
	resetSetupSeams(t)
	agentDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	osExecutable = func() (string, error) { return "/opt/engram/bin/engram", nil }
	runCommand = func(string, ...string) ([]byte, error) { return []byte("ok"), nil }

	lookPathFn = func(file string) (string, error) {
		return "", errors.New("not found")
	}

	_, err := Install("pi")
	if err != nil {
		t.Fatalf("Install(pi) failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(agentDir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	if _, ok := settings["npmCommand"]; ok {
		t.Fatalf("expected no npmCommand when mise is absent, got %s", raw)
	}
}

func TestInstallUnknownAgent(t *testing.T) {
	resetSetupSeams(t)
	_, err := Install("unknown")
	if err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Fatalf("expected unknown agent error, got %v", err)
	}
}

func TestInstallOpenCodeSuccessAndMCPRegistered(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	result, err := installOpenCode()
	if err != nil {
		t.Fatalf("installOpenCode failed: %v", err)
	}
	if result.Files != 3 {
		t.Fatalf("expected 3 files after MCP + TUI registration, got %d", result.Files)
	}

	pluginPath := filepath.Join(xdg, "opencode", "plugins", "engram.ts")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Fatalf("expected plugin file to exist: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(xdg, "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse opencode config: %v", err)
	}
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("expected mcp object in opencode.json")
	}
	if _, ok := mcp["engram"]; !ok {
		t.Fatalf("expected mcp.engram registration")
	}

	tuiRaw, err := os.ReadFile(filepath.Join(xdg, "opencode", "tui.json"))
	if err != nil {
		t.Fatalf("read opencode tui config: %v", err)
	}
	var tuiCfg map[string]any
	if err := json.Unmarshal(tuiRaw, &tuiCfg); err != nil {
		t.Fatalf("parse opencode tui config: %v", err)
	}
	plugins, ok := tuiCfg["plugin"].([]any)
	if !ok {
		t.Fatalf("expected plugin array in tui.json")
	}
	var found bool
	for _, plugin := range plugins {
		if plugin == openCodeSubagentStatuslinePlugin {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q plugin registration", openCodeSubagentStatuslinePlugin)
	}
}

func TestInstallOpenCodeReadEmbeddedError(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	openCodeReadFile = func(string) ([]byte, error) {
		return nil, errors.New("boom")
	}

	_, err := installOpenCode()
	if err == nil || !strings.Contains(err.Error(), "read embedded engram.ts") {
		t.Fatalf("expected read embedded error, got %v", err)
	}
}

func TestInstallOpenCodeWriteError(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	openCodeWriteFileFn = func(string, []byte, os.FileMode) error {
		return errors.New("write boom")
	}

	_, err := installOpenCode()
	if err == nil || !strings.Contains(err.Error(), "write ") {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestInstallOpenCodeMCPInjectionFailureIsNonFatal(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	injectOpenCodeMCPFn = func() error {
		return errors.New("cannot write config")
	}

	result, err := installOpenCode()
	if err != nil {
		t.Fatalf("expected non-fatal MCP injection failure, got %v", err)
	}
	if result.Files != 2 {
		t.Fatalf("expected plugin file + TUI config when MCP injection fails, got %d", result.Files)
	}
}

func TestInstallOpenCodeTUIInjectionFailureIsNonFatal(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	injectOpenCodeTUIPluginFn = func() error {
		return errors.New("cannot write tui config")
	}

	result, err := installOpenCode()
	if err != nil {
		t.Fatalf("expected non-fatal TUI injection failure, got %v", err)
	}
	if result.Files != 2 {
		t.Fatalf("expected plugin file + MCP config when TUI injection fails, got %d", result.Files)
	}
}

func TestInjectOpenCodeMCPPreservesExistingAndIsIdempotent(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	configPath := filepath.Join(xdg, "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	initial := `{"theme":"kanagawa","mcp":{"other":{"type":"local","command":["foo"]}}}`
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	if err := injectOpenCodeMCP(); err != nil {
		t.Fatalf("injectOpenCodeMCP failed: %v", err)
	}
	if err := injectOpenCodeMCP(); err != nil {
		t.Fatalf("injectOpenCodeMCP should be idempotent: %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse updated config: %v", err)
	}
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("expected mcp object")
	}
	if _, ok := mcp["other"]; !ok {
		t.Fatalf("expected existing mcp entry to be preserved")
	}
	engram, ok := mcp["engram"].(map[string]any)
	if !ok {
		t.Fatalf("expected engram object")
	}
	if engram["enabled"] != true {
		t.Fatalf("expected engram.enabled=true")
	}
}

func TestInjectOpenCodeTUIPluginPreservesExistingAndIsIdempotent(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	configPath := filepath.Join(xdg, "opencode", "tui.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	initial := `{"$schema":"https://opencode.ai/tui.json","plugin":["existing-plugin"]}`
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	if err := injectOpenCodeTUIPlugin(); err != nil {
		t.Fatalf("injectOpenCodeTUIPlugin failed: %v", err)
	}
	if err := injectOpenCodeTUIPlugin(); err != nil {
		t.Fatalf("injectOpenCodeTUIPlugin should be idempotent: %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse updated config: %v", err)
	}
	plugins, ok := cfg["plugin"].([]any)
	if !ok {
		t.Fatalf("expected plugin array")
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %v", plugins)
	}
	if plugins[0] != "existing-plugin" {
		t.Fatalf("expected existing plugin to be preserved, got %v", plugins)
	}
	if plugins[1] != openCodeSubagentStatuslinePlugin {
		t.Fatalf("expected %q to be appended, got %v", openCodeSubagentStatuslinePlugin, plugins)
	}
}

func TestInjectOpenCodeTUIPluginConfigErrors(t *testing.T) {
	t.Run("invalid root json", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		xdg := filepath.Join(home, "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)

		configPath := filepath.Join(xdg, "opencode", "tui.json")
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			t.Fatalf("mkdir config dir: %v", err)
		}
		if err := os.WriteFile(configPath, []byte("{"), 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		err := injectOpenCodeTUIPlugin()
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("expected parse config error, got %v", err)
		}
	})

	t.Run("invalid plugin block", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		xdg := filepath.Join(home, "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)

		configPath := filepath.Join(xdg, "opencode", "tui.json")
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			t.Fatalf("mkdir config dir: %v", err)
		}
		if err := os.WriteFile(configPath, []byte(`{"plugin":{"bad":true}}`), 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		err := injectOpenCodeTUIPlugin()
		if err == nil || !strings.Contains(err.Error(), "parse plugin block") {
			t.Fatalf("expected parse plugin block error, got %v", err)
		}
	})
}

func TestInjectOpenCodeMCPConfigErrors(t *testing.T) {
	t.Run("invalid root json", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		xdg := filepath.Join(home, "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)

		configPath := filepath.Join(xdg, "opencode", "opencode.json")
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			t.Fatalf("mkdir config dir: %v", err)
		}
		if err := os.WriteFile(configPath, []byte("{"), 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		err := injectOpenCodeMCP()
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("expected parse config error, got %v", err)
		}
	})

	t.Run("invalid mcp block", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		xdg := filepath.Join(home, "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)

		configPath := filepath.Join(xdg, "opencode", "opencode.json")
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			t.Fatalf("mkdir config dir: %v", err)
		}
		if err := os.WriteFile(configPath, []byte(`{"mcp":"nope"}`), 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		err := injectOpenCodeMCP()
		if err == nil || !strings.Contains(err.Error(), "parse mcp block") {
			t.Fatalf("expected parse mcp block error, got %v", err)
		}
	})

	t.Run("read error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		xdg := filepath.Join(home, "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)

		configPath := filepath.Join(xdg, "opencode", "opencode.json")
		if err := os.MkdirAll(configPath, 0755); err != nil {
			t.Fatalf("create directory at config path: %v", err)
		}

		err := injectOpenCodeMCP()
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("expected read config error, got %v", err)
		}
	})

	t.Run("marshal engram entry error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		xdg := filepath.Join(home, "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)

		jsonMarshalFn = func(any) ([]byte, error) {
			return nil, errors.New("marshal entry boom")
		}

		err := injectOpenCodeMCP()
		if err == nil || !strings.Contains(err.Error(), "marshal engram entry") {
			t.Fatalf("expected marshal engram entry error, got %v", err)
		}
	})

	t.Run("marshal mcp block error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		xdg := filepath.Join(home, "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)

		calls := 0
		jsonMarshalFn = func(v any) ([]byte, error) {
			calls++
			if calls == 2 {
				return nil, errors.New("marshal mcp boom")
			}
			return json.Marshal(v)
		}

		err := injectOpenCodeMCP()
		if err == nil || !strings.Contains(err.Error(), "marshal mcp block") {
			t.Fatalf("expected marshal mcp block error, got %v", err)
		}
	})

	t.Run("marshal config error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		xdg := filepath.Join(home, "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)

		jsonMarshalIndentFn = func(any, string, string) ([]byte, error) {
			return nil, errors.New("marshal config boom")
		}

		err := injectOpenCodeMCP()
		if err == nil || !strings.Contains(err.Error(), "marshal config") {
			t.Fatalf("expected marshal config error, got %v", err)
		}
	})
}

func TestDefaultRunCommandExecutes(t *testing.T) {
	resetSetupSeams(t)
	out, err := runCommand("sh", "-c", "printf ok")
	if err != nil {
		t.Fatalf("expected default runCommand to execute, got %v", err)
	}
	if string(out) != "ok" {
		t.Fatalf("unexpected output: %q", string(out))
	}
}

func TestInstallClaudeCodeBranches(t *testing.T) {
	t.Run("cli missing", func(t *testing.T) {
		resetSetupSeams(t)
		lookPathFn = func(string) (string, error) {
			return "", errors.New("not found")
		}

		_, err := installClaudeCode()
		if err == nil || !strings.Contains(err.Error(), "claude CLI not found") {
			t.Fatalf("expected not found error, got %v", err)
		}
	})

	t.Run("marketplace add hard failure", func(t *testing.T) {
		resetSetupSeams(t)
		lookPathFn = func(string) (string, error) { return "claude", nil }
		runCommand = func(string, ...string) ([]byte, error) {
			return []byte("permission denied"), errors.New("exit 1")
		}

		_, err := installClaudeCode()
		if err == nil || !strings.Contains(err.Error(), "marketplace add failed") {
			t.Fatalf("expected marketplace add failure, got %v", err)
		}
	})

	t.Run("marketplace already then install success", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		lookPathFn = func(string) (string, error) { return "claude", nil }
		writeClaudeCodeUserMCPFn = func() error { return nil }
		calls := 0
		runCommand = func(_ string, args ...string) ([]byte, error) {
			calls++
			if calls == 1 {
				if strings.Join(args, " ") != "plugin marketplace add "+claudeCodeMarketplace {
					t.Fatalf("unexpected first command args: %q", strings.Join(args, " "))
				}
				return []byte("already added"), errors.New("exit 1")
			}
			if strings.Join(args, " ") != "plugin install engram" {
				t.Fatalf("unexpected second command args: %q", strings.Join(args, " "))
			}
			return []byte("installed"), nil
		}

		result, err := installClaudeCode()
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if result.Agent != "claude-code" {
			t.Fatalf("unexpected agent: %q", result.Agent)
		}
		// When writeClaudeCodeUserMCP succeeds, files == 1
		if result.Files != 1 {
			t.Fatalf("expected 1 file when user MCP write succeeds, got %d", result.Files)
		}
		// Destination should point to the .claude/mcp dir, not be empty
		expectedDir := filepath.Join(home, ".claude", "mcp")
		if result.Destination != expectedDir {
			t.Fatalf("expected destination %q, got %q", expectedDir, result.Destination)
		}
	})

	t.Run("install hard failure", func(t *testing.T) {
		resetSetupSeams(t)
		lookPathFn = func(string) (string, error) { return "claude", nil }
		writeClaudeCodeUserMCPFn = func() error { return nil }
		calls := 0
		runCommand = func(string, ...string) ([]byte, error) {
			calls++
			if calls == 1 {
				return []byte("ok"), nil
			}
			return []byte("network failure"), errors.New("exit 1")
		}

		_, err := installClaudeCode()
		if err == nil || !strings.Contains(err.Error(), "plugin install failed") {
			t.Fatalf("expected plugin install failure, got %v", err)
		}
	})

	t.Run("install already is success", func(t *testing.T) {
		resetSetupSeams(t)
		useTestHome(t)
		lookPathFn = func(string) (string, error) { return "claude", nil }
		writeClaudeCodeUserMCPFn = func() error { return nil }
		calls := 0
		runCommand = func(string, ...string) ([]byte, error) {
			calls++
			if calls == 1 {
				return []byte("ok"), nil
			}
			return []byte("already installed"), errors.New("exit 1")
		}

		if _, err := installClaudeCode(); err != nil {
			t.Fatalf("expected already-installed branch to succeed, got %v", err)
		}
	})

	t.Run("user mcp write failure is non-fatal", func(t *testing.T) {
		resetSetupSeams(t)
		useTestHome(t)
		lookPathFn = func(string) (string, error) { return "claude", nil }
		runCommand = func(string, ...string) ([]byte, error) { return []byte("ok"), nil }
		writeClaudeCodeUserMCPFn = func() error { return errors.New("disk full") }

		result, err := installClaudeCode()
		if err != nil {
			t.Fatalf("user MCP write failure should be non-fatal, got %v", err)
		}
		// files == 0 when writeClaudeCodeUserMCP fails
		if result.Files != 0 {
			t.Fatalf("expected 0 files when user MCP write fails, got %d", result.Files)
		}
	})
}

// ─── Issue #100: Windows PATH fix ────────────────────────────────────────────

func TestWriteClaudeCodeUserMCP(t *testing.T) {
	t.Run("writes json with absolute binary path", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		osExecutable = func() (string, error) { return "/usr/local/bin/engram", nil }

		if err := writeClaudeCodeUserMCP(); err != nil {
			t.Fatalf("writeClaudeCodeUserMCP failed: %v", err)
		}

		mcpPath := filepath.Join(home, ".claude", "mcp", "engram.json")
		raw, err := os.ReadFile(mcpPath)
		if err != nil {
			t.Fatalf("read mcp config: %v", err)
		}

		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("parse mcp config: %v", err)
		}

		if cfg["url"] != engramRemoteMCPURL || cfg["type"] != "http" {
			t.Fatalf("expected remote http entry, got %#v", cfg)
		}
		if _, hasCommand := cfg["command"]; hasCommand {
			t.Fatalf("remote Claude Code entry must not carry a local command")
		}
	})

	t.Run("overwrites existing (idempotent — always refreshes path)", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		osExecutable = func() (string, error) { return "/new/path/engram", nil }

		mcpDir := filepath.Join(home, ".claude", "mcp")
		if err := os.MkdirAll(mcpDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(mcpDir, "engram.json"), []byte(`{"command":"old"}`), 0644); err != nil {
			t.Fatalf("write old config: %v", err)
		}

		if err := writeClaudeCodeUserMCP(); err != nil {
			t.Fatalf("writeClaudeCodeUserMCP failed: %v", err)
		}

		raw, err := os.ReadFile(filepath.Join(mcpDir, "engram.json"))
		if err != nil {
			t.Fatalf("read updated config: %v", err)
		}
		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("parse config: %v", err)
		}
		if cfg["url"] != engramRemoteMCPURL {
			t.Fatalf("expected remote url, got %#v", cfg["url"])
		}
	})

	t.Run("marshal error returns error", func(t *testing.T) {
		resetSetupSeams(t)
		useTestHome(t)
		osExecutable = func() (string, error) { return "/bin/engram", nil }
		jsonMarshalIndentFn = func(any, string, string) ([]byte, error) {
			return nil, errors.New("marshal boom")
		}

		err := writeClaudeCodeUserMCP()
		if err == nil || !strings.Contains(err.Error(), "marshal mcp config") {
			t.Fatalf("expected marshal mcp config error, got %v", err)
		}
	})

	t.Run("write error returns error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		osExecutable = func() (string, error) { return "/bin/engram", nil }
		// Make ~/.claude/mcp/engram.json a directory so write fails
		mcpDir := filepath.Join(home, ".claude", "mcp")
		if err := os.MkdirAll(mcpDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(mcpDir, "engram.json"), 0755); err != nil {
			t.Fatalf("create dir as file: %v", err)
		}

		err := writeClaudeCodeUserMCP()
		if err == nil || !strings.Contains(err.Error(), "write mcp config") {
			t.Fatalf("expected write mcp config error, got %v", err)
		}
	})

	t.Run("create dir error returns error", func(t *testing.T) {
		resetSetupSeams(t)
		// Block ~/.claude/mcp creation by making .claude a file
		blocked := t.TempDir()
		if err := os.WriteFile(filepath.Join(blocked, ".claude"), []byte("x"), 0644); err != nil {
			t.Fatalf("write blocking file: %v", err)
		}
		userHomeDir = func() (string, error) { return blocked, nil }
		osExecutable = func() (string, error) { return "/bin/engram", nil }

		err := writeClaudeCodeUserMCP()
		if err == nil || !strings.Contains(err.Error(), "create mcp dir") {
			t.Fatalf("expected create mcp dir error, got %v", err)
		}
	})
}

func TestClaudeCodeMCPDirPaths(t *testing.T) {
	resetSetupSeams(t)
	userHomeDir = func() (string, error) { return "/home/tester", nil }

	expectedDir := filepath.Join("/home/tester", ".claude", "mcp")
	if got := claudeCodeMCPDir(); got != expectedDir {
		t.Fatalf("expected %s, got %s", expectedDir, got)
	}

	expectedPath := filepath.Join("/home/tester", ".claude", "mcp", "engram.json")
	if got := claudeCodeUserMCPPath(); got != expectedPath {
		t.Fatalf("expected %s, got %s", expectedPath, got)
	}
}

func TestPathHelpersAcrossOSVariants(t *testing.T) {
	resetSetupSeams(t)
	userHomeDir = func() (string, error) { return "/home/tester", nil }

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", "")

	runtimeGOOS = "linux"
	if got := openCodeConfigPath(); got != filepath.Join("/home/tester", ".config", "opencode", "opencode.json") {
		t.Fatalf("unexpected linux openCodeConfigPath: %s", got)
	}
	if got := openCodeTUIConfigPath(); got != filepath.Join("/home/tester", ".config", "opencode", "tui.json") {
		t.Fatalf("unexpected linux openCodeTUIConfigPath: %s", got)
	}
	if got := openCodePluginDir(); got != filepath.Join("/home/tester", ".config", "opencode", "plugins") {
		t.Fatalf("unexpected linux openCodePluginDir: %s", got)
	}
	if got := geminiConfigPath(); got != filepath.Join("/home/tester", ".gemini", "settings.json") {
		t.Fatalf("unexpected linux geminiConfigPath: %s", got)
	}
	if got := codexConfigPath(); got != filepath.Join("/home/tester", ".codex", "config.toml") {
		t.Fatalf("unexpected linux codexConfigPath: %s", got)
	}

	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := openCodeConfigPath(); got != filepath.Join("/xdg", "opencode", "opencode.json") {
		t.Fatalf("unexpected linux xdg openCodeConfigPath: %s", got)
	}
	if got := openCodeTUIConfigPath(); got != filepath.Join("/xdg", "opencode", "tui.json") {
		t.Fatalf("unexpected linux xdg openCodeTUIConfigPath: %s", got)
	}
	if got := openCodePluginDir(); got != filepath.Join("/xdg", "opencode", "plugins") {
		t.Fatalf("unexpected linux xdg openCodePluginDir: %s", got)
	}

	runtimeGOOS = "windows"
	t.Setenv("APPDATA", "C:/AppData/Roaming")
	t.Setenv("XDG_CONFIG_HOME", "")
	// OpenCode uses ~/.config/opencode/ on ALL platforms, ignoring %APPDATA%
	if got := openCodeConfigPath(); got != filepath.Join("/home/tester", ".config", "opencode", "opencode.json") {
		t.Fatalf("unexpected windows openCodeConfigPath: %s", got)
	}
	if got := openCodeTUIConfigPath(); got != filepath.Join("/home/tester", ".config", "opencode", "tui.json") {
		t.Fatalf("unexpected windows openCodeTUIConfigPath: %s", got)
	}
	if got := openCodePluginDir(); got != filepath.Join("/home/tester", ".config", "opencode", "plugins") {
		t.Fatalf("unexpected windows openCodePluginDir: %s", got)
	}
	if got := geminiConfigPath(); got != filepath.Join("C:/AppData/Roaming", "gemini", "settings.json") {
		t.Fatalf("unexpected windows geminiConfigPath: %s", got)
	}
	if got := codexConfigPath(); got != filepath.Join("C:/AppData/Roaming", "codex", "config.toml") {
		t.Fatalf("unexpected windows codexConfigPath: %s", got)
	}

	t.Setenv("APPDATA", "")
	// OpenCode still uses ~/.config/opencode/ even without APPDATA
	if got := openCodeConfigPath(); got != filepath.Join("/home/tester", ".config", "opencode", "opencode.json") {
		t.Fatalf("unexpected windows fallback openCodeConfigPath: %s", got)
	}
	if got := openCodeTUIConfigPath(); got != filepath.Join("/home/tester", ".config", "opencode", "tui.json") {
		t.Fatalf("unexpected windows fallback openCodeTUIConfigPath: %s", got)
	}
	if got := openCodePluginDir(); got != filepath.Join("/home/tester", ".config", "opencode", "plugins") {
		t.Fatalf("unexpected windows fallback openCodePluginDir: %s", got)
	}
	if got := geminiConfigPath(); got != filepath.Join("/home/tester", "AppData", "Roaming", "gemini", "settings.json") {
		t.Fatalf("unexpected windows fallback geminiConfigPath: %s", got)
	}
	if got := codexConfigPath(); got != filepath.Join("/home/tester", "AppData", "Roaming", "codex", "config.toml") {
		t.Fatalf("unexpected windows fallback codexConfigPath: %s", got)
	}

	runtimeGOOS = "plan9"
	if got := openCodeConfigPath(); got != filepath.Join("/home/tester", ".config", "opencode", "opencode.json") {
		t.Fatalf("unexpected default openCodeConfigPath: %s", got)
	}
	if got := openCodeTUIConfigPath(); got != filepath.Join("/home/tester", ".config", "opencode", "tui.json") {
		t.Fatalf("unexpected default openCodeTUIConfigPath: %s", got)
	}
	if got := openCodePluginDir(); got != filepath.Join("/home/tester", ".config", "opencode", "plugins") {
		t.Fatalf("unexpected default openCodePluginDir: %s", got)
	}

	if got := geminiSystemPromptPath(); got != filepath.Join(filepath.Dir(geminiConfigPath()), "system.md") {
		t.Fatalf("unexpected gemini system prompt path: %s", got)
	}
	if got := geminiEnvPath(); got != filepath.Join(filepath.Dir(geminiConfigPath()), ".env") {
		t.Fatalf("unexpected gemini env path: %s", got)
	}
	if got := codexInstructionsPath(); got != filepath.Join(filepath.Dir(codexConfigPath()), "engram-instructions.md") {
		t.Fatalf("unexpected codex instructions path: %s", got)
	}
	if got := codexCompactPromptPath(); got != filepath.Join(filepath.Dir(codexConfigPath()), "engram-compact-prompt.md") {
		t.Fatalf("unexpected codex compact prompt path: %s", got)
	}
}

func TestInstallGeminiCLIErrorPropagation(t *testing.T) {
	t.Run("inject mcp fails", func(t *testing.T) {
		resetSetupSeams(t)
		injectGeminiMCPFn = func(string) error { return errors.New("inject failed") }

		_, err := installGeminiCLI()
		if err == nil || !strings.Contains(err.Error(), "inject failed") {
			t.Fatalf("expected inject failure, got %v", err)
		}
	})

	t.Run("write system prompt fails", func(t *testing.T) {
		resetSetupSeams(t)
		injectGeminiMCPFn = func(string) error { return nil }
		writeGeminiSystemPromptFn = func() error { return errors.New("prompt failed") }

		_, err := installGeminiCLI()
		if err == nil || !strings.Contains(err.Error(), "prompt failed") {
			t.Fatalf("expected system prompt failure, got %v", err)
		}
	})

}

func TestInstallCodexErrorPropagation(t *testing.T) {
	t.Run("write instruction files fails", func(t *testing.T) {
		resetSetupSeams(t)
		writeCodexMemoryInstructionFilesFn = func() (string, error) {
			return "", errors.New("instructions failed")
		}

		_, err := installCodex()
		if err == nil || !strings.Contains(err.Error(), "instructions failed") {
			t.Fatalf("expected instructions failure, got %v", err)
		}
	})

	t.Run("inject mcp fails", func(t *testing.T) {
		resetSetupSeams(t)
		writeCodexMemoryInstructionFilesFn = func() (string, error) { return "/tmp/instructions", nil }
		injectCodexMCPFn = func(string) error { return errors.New("mcp failed") }

		_, err := installCodex()
		if err == nil || !strings.Contains(err.Error(), "mcp failed") {
			t.Fatalf("expected mcp failure, got %v", err)
		}
	})

	t.Run("inject memory config fails", func(t *testing.T) {
		resetSetupSeams(t)
		writeCodexMemoryInstructionFilesFn = func() (string, error) { return "/tmp/instructions", nil }
		injectCodexMCPFn = func(string) error { return nil }
		injectCodexMemoryConfigFn = func(string, string, string) error { return errors.New("memory config failed") }

		_, err := installCodex()
		if err == nil || !strings.Contains(err.Error(), "memory config failed") {
			t.Fatalf("expected memory config failure, got %v", err)
		}
	})
}

func TestGeminiAndCodexHelpersErrorPaths(t *testing.T) {
	t.Run("injectGeminiMCP creates file from missing config", func(t *testing.T) {
		resetSetupSeams(t)
		// Force a known absolute path so the test is deterministic.
		osExecutable = func() (string, error) { return "/usr/local/bin/engram", nil }
		configPath := filepath.Join(t.TempDir(), "settings.json")

		if err := injectGeminiMCP(configPath); err != nil {
			t.Fatalf("injectGeminiMCP failed: %v", err)
		}

		raw, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}

		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("parse config: %v", err)
		}

		mcpServers, ok := cfg["mcpServers"].(map[string]any)
		if !ok {
			t.Fatalf("expected mcpServers object")
		}
		engram, ok := mcpServers["engram"].(map[string]any)
		if !ok {
			t.Fatalf("expected engram server object")
		}
		// The Gemini entry is the remote httpUrl, never a local command.
		if engram["httpUrl"] != engramRemoteMCPURL {
			t.Fatalf("expected httpUrl %q, got %#v", engramRemoteMCPURL, engram["httpUrl"])
		}
	})

	t.Run("injectGeminiMCP marshal entry error", func(t *testing.T) {
		resetSetupSeams(t)
		configPath := filepath.Join(t.TempDir(), "settings.json")
		jsonMarshalFn = func(any) ([]byte, error) {
			return nil, errors.New("marshal boom")
		}

		err := injectGeminiMCP(configPath)
		if err == nil || !strings.Contains(err.Error(), "marshal engram entry") {
			t.Fatalf("expected marshal engram entry error, got %v", err)
		}
	})

	t.Run("injectGeminiMCP marshal indent error", func(t *testing.T) {
		resetSetupSeams(t)
		configPath := filepath.Join(t.TempDir(), "settings.json")
		jsonMarshalIndentFn = func(any, string, string) ([]byte, error) {
			return nil, errors.New("indent boom")
		}

		err := injectGeminiMCP(configPath)
		if err == nil || !strings.Contains(err.Error(), "marshal config") {
			t.Fatalf("expected marshal config error, got %v", err)
		}
	})

	t.Run("injectGeminiMCP marshal mcpServers error", func(t *testing.T) {
		resetSetupSeams(t)
		configPath := filepath.Join(t.TempDir(), "settings.json")
		calls := 0
		jsonMarshalFn = func(v any) ([]byte, error) {
			calls++
			if calls == 2 {
				return nil, errors.New("mcp marshal boom")
			}
			return json.Marshal(v)
		}

		err := injectGeminiMCP(configPath)
		if err == nil || !strings.Contains(err.Error(), "marshal mcpServers block") {
			t.Fatalf("expected marshal mcpServers block error, got %v", err)
		}
	})

	t.Run("injectGeminiMCP write error", func(t *testing.T) {
		resetSetupSeams(t)
		configPath := filepath.Join(t.TempDir(), "settings.json")
		writeFileFn = func(string, []byte, os.FileMode) error {
			return errors.New("write boom")
		}

		err := injectGeminiMCP(configPath)
		if err == nil || !strings.Contains(err.Error(), "write config") {
			t.Fatalf("expected write config error, got %v", err)
		}
	})

	t.Run("injectGeminiMCP parse error", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(configPath, []byte("{"), 0644); err != nil {
			t.Fatalf("write invalid json: %v", err)
		}
		err := injectGeminiMCP(configPath)
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("expected parse config error, got %v", err)
		}
	})

	t.Run("injectGeminiMCP parse mcpServers error", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(configPath, []byte(`{"mcpServers":"bad"}`), 0644); err != nil {
			t.Fatalf("write invalid mcpServers: %v", err)
		}
		err := injectGeminiMCP(configPath)
		if err == nil || !strings.Contains(err.Error(), "parse mcpServers block") {
			t.Fatalf("expected parse mcpServers error, got %v", err)
		}
	})

	t.Run("injectGeminiMCP create config dir error", func(t *testing.T) {
		base := t.TempDir()
		parent := filepath.Join(base, "blocked")
		if err := os.WriteFile(parent, []byte("x"), 0644); err != nil {
			t.Fatalf("write blocking file: %v", err)
		}
		err := injectGeminiMCP(filepath.Join(parent, "settings.json"))
		if err == nil || !strings.Contains(err.Error(), "create config dir") {
			t.Fatalf("expected create config dir error, got %v", err)
		}
	})

	t.Run("removeGeminiEnvOverride strips GEMINI_SYSTEM_MD line", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"

		envPath := filepath.Join(home, ".gemini", ".env")
		if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
			t.Fatalf("mkdir env dir: %v", err)
		}
		if err := os.WriteFile(envPath, []byte("OTHER=1\r\nGEMINI_SYSTEM_MD=1\r\n"), 0644); err != nil {
			t.Fatalf("write env file: %v", err)
		}

		removeGeminiEnvOverride()

		raw, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("read env file: %v", err)
		}
		text := string(raw)
		if strings.Contains(text, "GEMINI_SYSTEM_MD") {
			t.Fatalf("expected GEMINI_SYSTEM_MD removed, got:\n%s", text)
		}
		if !strings.Contains(text, "OTHER=1") {
			t.Fatalf("expected OTHER=1 preserved, got:\n%s", text)
		}
	})

	t.Run("removeGeminiEnvOverride deletes empty env file", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"

		envPath := filepath.Join(home, ".gemini", ".env")
		if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
			t.Fatalf("mkdir env dir: %v", err)
		}
		if err := os.WriteFile(envPath, []byte("GEMINI_SYSTEM_MD=1\n"), 0644); err != nil {
			t.Fatalf("write env file: %v", err)
		}

		removeGeminiEnvOverride()

		if _, err := os.Stat(envPath); !os.IsNotExist(err) {
			t.Fatalf("expected env file deleted when only GEMINI_SYSTEM_MD was present")
		}
	})

	t.Run("removeGeminiEnvOverride no-op when file missing", func(t *testing.T) {
		resetSetupSeams(t)
		_ = useTestHome(t)
		runtimeGOOS = "linux"

		// should not panic or error
		removeGeminiEnvOverride()
	})

	t.Run("writeGeminiSystemPrompt create dir error", func(t *testing.T) {
		resetSetupSeams(t)
		blocked := filepath.Join(t.TempDir(), "home-as-file")
		if err := os.WriteFile(blocked, []byte("x"), 0644); err != nil {
			t.Fatalf("write home file: %v", err)
		}
		userHomeDir = func() (string, error) { return blocked, nil }
		runtimeGOOS = "linux"

		err := writeGeminiSystemPrompt()
		if err == nil || !strings.Contains(err.Error(), "create gemini system prompt dir") {
			t.Fatalf("expected create dir error, got %v", err)
		}
	})

	t.Run("injectCodexMCP read error", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.toml")
		if err := os.MkdirAll(configPath, 0755); err != nil {
			t.Fatalf("make config path directory: %v", err)
		}

		err := injectCodexMCP(configPath)
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("expected read config error, got %v", err)
		}
	})

	t.Run("injectCodexMemoryConfig read error", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.toml")
		if err := os.MkdirAll(configPath, 0755); err != nil {
			t.Fatalf("make config path directory: %v", err)
		}

		err := injectCodexMemoryConfig(configPath, "/tmp/instructions.md", "/tmp/compact.md")
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("expected read config error, got %v", err)
		}
	})

	t.Run("injectCodexMemoryConfig creates missing config", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.toml")

		err := injectCodexMemoryConfig(configPath, "/tmp/instructions.md", "/tmp/compact.md")
		if err != nil {
			t.Fatalf("injectCodexMemoryConfig failed: %v", err)
		}

		raw, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		text := string(raw)
		if !strings.Contains(text, "model_instructions_file = \"/tmp/instructions.md\"") {
			t.Fatalf("expected model_instructions_file in config, got:\n%s", text)
		}
		if !strings.Contains(text, "experimental_compact_prompt_file = \"/tmp/compact.md\"") {
			t.Fatalf("expected compact prompt file in config, got:\n%s", text)
		}
	})

	t.Run("injectCodexMemoryConfig write error", func(t *testing.T) {
		resetSetupSeams(t)
		configPath := filepath.Join(t.TempDir(), "config.toml")
		writeFileFn = func(string, []byte, os.FileMode) error {
			return errors.New("write config boom")
		}

		err := injectCodexMemoryConfig(configPath, "/tmp/instructions.md", "/tmp/compact.md")
		if err == nil || !strings.Contains(err.Error(), "write config") {
			t.Fatalf("expected write config error, got %v", err)
		}
	})

	t.Run("upsertCodexEngramBlock replaces section before another section", func(t *testing.T) {
		input := strings.Join([]string{
			"[mcp_servers.engram]",
			"command = \"wrong\"",
			"args = [\"wrong\"]",
			"",
			"[mcp_servers.other]",
			"command = \"other\"",
		}, "\n")

		output := upsertCodexEngramBlock(input)
		if strings.Count(output, "[mcp_servers.engram]") != 1 {
			t.Fatalf("expected one engram block, got:\n%s", output)
		}
		if !strings.Contains(output, "[mcp_servers.other]") {
			t.Fatalf("expected other section preserved, got:\n%s", output)
		}
	})

	t.Run("upsertCodexEngramBlock from empty content", func(t *testing.T) {
		resetSetupSeams(t)
		// Force fallback path so output matches the constant.
		osExecutable = func() (string, error) { return "", errors.New("no executable") }

		output := upsertCodexEngramBlock("\n\n")
		if output != codexEngramBlock+"\n" {
			t.Fatalf("unexpected output for empty content:\n%s", output)
		}
	})
}

func TestInstallRoutesForOpenCodeAndClaude(t *testing.T) {
	t.Run("opencode route", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))

		result, err := Install("opencode")
		if err != nil {
			t.Fatalf("Install(opencode) failed: %v", err)
		}
		if result.Agent != "opencode" {
			t.Fatalf("expected opencode result, got %#v", result)
		}
	})

	t.Run("claude-code route", func(t *testing.T) {
		resetSetupSeams(t)
		useTestHome(t)
		lookPathFn = func(string) (string, error) { return "claude", nil }
		runCommand = func(string, ...string) ([]byte, error) { return []byte("ok"), nil }
		writeClaudeCodeUserMCPFn = func() error { return nil }

		result, err := Install("claude-code")
		if err != nil {
			t.Fatalf("Install(claude-code) failed: %v", err)
		}
		if result.Agent != "claude-code" {
			t.Fatalf("expected claude-code result, got %#v", result)
		}
	})
}

func TestAdditionalHelperBranches(t *testing.T) {
	t.Run("installOpenCode mkdir error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"

		blocked := filepath.Join(home, "xdg-block")
		if err := os.WriteFile(blocked, []byte("x"), 0644); err != nil {
			t.Fatalf("write blocker file: %v", err)
		}
		t.Setenv("XDG_CONFIG_HOME", blocked)

		_, err := installOpenCode()
		if err == nil || !strings.Contains(err.Error(), "create plugin dir") {
			t.Fatalf("expected create plugin dir error, got %v", err)
		}
	})

	t.Run("injectOpenCodeMCP write error when parent missing", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))

		err := injectOpenCodeMCP()
		if err == nil || !strings.Contains(err.Error(), "write config") {
			t.Fatalf("expected write config error, got %v", err)
		}
	})

	t.Run("injectCodexMCP create config dir error", func(t *testing.T) {
		base := t.TempDir()
		blocked := filepath.Join(base, "blocked")
		if err := os.WriteFile(blocked, []byte("x"), 0644); err != nil {
			t.Fatalf("write blocker: %v", err)
		}

		err := injectCodexMCP(filepath.Join(blocked, "config.toml"))
		if err == nil || !strings.Contains(err.Error(), "create config dir") {
			t.Fatalf("expected create config dir error, got %v", err)
		}
	})

	t.Run("injectCodexMCP write error", func(t *testing.T) {
		resetSetupSeams(t)
		configPath := filepath.Join(t.TempDir(), "codex", "config.toml")
		writeFileFn = func(string, []byte, os.FileMode) error {
			return errors.New("write codex boom")
		}

		err := injectCodexMCP(configPath)
		if err == nil || !strings.Contains(err.Error(), "write config") {
			t.Fatalf("expected write config error, got %v", err)
		}
	})

	t.Run("writeCodexMemoryInstructionFiles instructions write error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"

		instructionsPath := filepath.Join(home, ".codex", "engram-instructions.md")
		if err := os.MkdirAll(instructionsPath, 0755); err != nil {
			t.Fatalf("create instructions path as dir: %v", err)
		}

		_, err := writeCodexMemoryInstructionFiles()
		if err == nil || !strings.Contains(err.Error(), "write codex instructions") {
			t.Fatalf("expected instructions write error, got %v", err)
		}
	})

	t.Run("writeCodexMemoryInstructionFiles compact write error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"

		compactPath := filepath.Join(home, ".codex", "engram-compact-prompt.md")
		if err := os.MkdirAll(compactPath, 0755); err != nil {
			t.Fatalf("create compact path as dir: %v", err)
		}

		_, err := writeCodexMemoryInstructionFiles()
		if err == nil || !strings.Contains(err.Error(), "write codex compact prompt") {
			t.Fatalf("expected compact prompt write error, got %v", err)
		}
	})

	t.Run("injectGeminiMCP read error", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "settings.json")
		if err := os.MkdirAll(configPath, 0755); err != nil {
			t.Fatalf("create config path as dir: %v", err)
		}

		err := injectGeminiMCP(configPath)
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("expected read config error, got %v", err)
		}
	})

	t.Run("writeGeminiSystemPrompt write error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)
		runtimeGOOS = "linux"

		systemPath := filepath.Join(home, ".gemini", "system.md")
		if err := os.MkdirAll(systemPath, 0755); err != nil {
			t.Fatalf("create system path as dir: %v", err)
		}

		err := writeGeminiSystemPrompt()
		if err == nil || !strings.Contains(err.Error(), "write gemini system prompt") {
			t.Fatalf("expected write system prompt error, got %v", err)
		}
	})

	t.Run("writeCodexMemoryInstructionFiles create dir error", func(t *testing.T) {
		resetSetupSeams(t)
		blocked := filepath.Join(t.TempDir(), "home-as-file")
		if err := os.WriteFile(blocked, []byte("x"), 0644); err != nil {
			t.Fatalf("write home file: %v", err)
		}
		userHomeDir = func() (string, error) { return blocked, nil }
		runtimeGOOS = "linux"

		_, err := writeCodexMemoryInstructionFiles()
		if err == nil || !strings.Contains(err.Error(), "create codex instructions dir") {
			t.Fatalf("expected create instructions dir error, got %v", err)
		}
	})
}

func TestClaudeCodePermissionTools(t *testing.T) {
	tools := claudeCodePermissionTools(map[string]bool{
		"mem_search":          true,
		"mem_current_project": true,
		"mem_stats":           false,
	})

	want := []string{
		"mcp__engram__mem_current_project",
		"mcp__engram__mem_search",
		"mcp__plugin_engram_engram__mem_current_project",
		"mcp__plugin_engram_engram__mem_search",
	}
	if !reflect.DeepEqual(tools, want) {
		t.Fatalf("unexpected permissions:\nwant %#v\n got %#v", want, tools)
	}

	for _, tool := range []string{
		"mcp__engram__mem_current_project",
		"mcp__engram__mem_judge",
		"mcp__plugin_engram_engram__mem_current_project",
		"mcp__plugin_engram_engram__mem_judge",
	} {
		if !slices.Contains(claudeCodeMCPTools, tool) {
			t.Fatalf("claudeCodeMCPTools missing current agent permission %q", tool)
		}
	}
}

func TestClaudeCodeMemorySkillDoesNotHardcodePluginScopedToolSearch(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "plugin", "claude-code", "skills", "memory", "SKILL.md"))
	if err != nil {
		t.Fatalf("read memory skill: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "select:mcp__plugin_engram_engram__") {
		t.Fatalf("memory skill must not hardcode plugin-scoped ToolSearch names")
	}
	if !strings.Contains(text, "Pass the `project` argument explicitly") {
		t.Fatalf("memory skill must require an explicit project on every mem_* call (multi-project remote server)")
	}
}

func TestAddClaudeCodeAllowlist(t *testing.T) {
	t.Run("creates file from scratch", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)

		if err := AddClaudeCodeAllowlist(); err != nil {
			t.Fatalf("AddClaudeCodeAllowlist() failed: %v", err)
		}

		settingsPath := filepath.Join(home, ".claude", "settings.json")
		raw, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("read settings: %v", err)
		}

		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("parse settings: %v", err)
		}

		perms, ok := cfg["permissions"].(map[string]any)
		if !ok {
			t.Fatalf("expected permissions object")
		}

		allowRaw, ok := perms["allow"].([]any)
		if !ok {
			t.Fatalf("expected allow array")
		}

		if len(allowRaw) != len(claudeCodeMCPTools) {
			t.Fatalf("expected %d tools, got %d", len(claudeCodeMCPTools), len(allowRaw))
		}

		for i, tool := range claudeCodeMCPTools {
			if allowRaw[i] != tool {
				t.Fatalf("expected tool %q at index %d, got %q", tool, i, allowRaw[i])
			}
		}
	})

	t.Run("preserves existing entries", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)

		settingsPath := filepath.Join(home, ".claude", "settings.json")
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		existing := `{"attribution":{"commit":""},"permissions":{"allow":["Read","Write","Glob"],"deny":["Read(.env)"]}}`
		if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
			t.Fatalf("write initial settings: %v", err)
		}

		if err := AddClaudeCodeAllowlist(); err != nil {
			t.Fatalf("AddClaudeCodeAllowlist() failed: %v", err)
		}

		raw, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("read settings: %v", err)
		}

		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("parse settings: %v", err)
		}

		// Check attribution preserved
		if _, ok := cfg["attribution"]; !ok {
			t.Fatalf("expected attribution key to be preserved")
		}

		perms := cfg["permissions"].(map[string]any)

		// Check deny preserved
		deny, ok := perms["deny"].([]any)
		if !ok || len(deny) != 1 || deny[0] != "Read(.env)" {
			t.Fatalf("expected deny list preserved, got %#v", perms["deny"])
		}

		// Check allow has original + new entries
		allow := perms["allow"].([]any)
		expectedLen := 3 + len(claudeCodeMCPTools)
		if len(allow) != expectedLen {
			t.Fatalf("expected %d allow entries, got %d", expectedLen, len(allow))
		}

		// First 3 should be original
		if allow[0] != "Read" || allow[1] != "Write" || allow[2] != "Glob" {
			t.Fatalf("expected original entries preserved at start, got %v %v %v", allow[0], allow[1], allow[2])
		}
	})

	t.Run("idempotent when all tools present", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)

		settingsPath := filepath.Join(home, ".claude", "settings.json")
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		// Write settings with all tools already present
		allowJSON, _ := json.Marshal(claudeCodeMCPTools)
		initial := `{"permissions":{"allow":` + string(allowJSON) + `}}`
		if err := os.WriteFile(settingsPath, []byte(initial), 0644); err != nil {
			t.Fatalf("write initial settings: %v", err)
		}

		beforeRaw, _ := os.ReadFile(settingsPath)

		if err := AddClaudeCodeAllowlist(); err != nil {
			t.Fatalf("AddClaudeCodeAllowlist() failed: %v", err)
		}

		afterRaw, _ := os.ReadFile(settingsPath)

		// File should not have been rewritten (early return)
		if string(afterRaw) != string(beforeRaw) {
			t.Fatalf("expected file unchanged when all tools present")
		}
	})

	t.Run("partial existing adds only missing", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)

		settingsPath := filepath.Join(home, ".claude", "settings.json")
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		// Include 3 tools and verify only the missing permissions are appended.
		partial := []string{
			claudeCodeMCPTools[0],
			claudeCodeMCPTools[3],
			claudeCodeMCPTools[7],
		}
		allowJSON, _ := json.Marshal(partial)
		initial := `{"permissions":{"allow":` + string(allowJSON) + `}}`
		if err := os.WriteFile(settingsPath, []byte(initial), 0644); err != nil {
			t.Fatalf("write initial settings: %v", err)
		}

		if err := AddClaudeCodeAllowlist(); err != nil {
			t.Fatalf("AddClaudeCodeAllowlist() failed: %v", err)
		}

		raw, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("read settings: %v", err)
		}

		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("parse settings: %v", err)
		}

		allow := cfg["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != len(claudeCodeMCPTools) {
			t.Fatalf("expected %d tools (no duplicates), got %d", len(claudeCodeMCPTools), len(allow))
		}

		// Verify no duplicates
		seen := make(map[string]int)
		for _, entry := range allow {
			seen[entry.(string)]++
		}
		for tool, count := range seen {
			if count > 1 {
				t.Fatalf("duplicate tool entry: %q (count %d)", tool, count)
			}
		}
	})

	t.Run("read error returns error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)

		settingsPath := filepath.Join(home, ".claude", "settings.json")
		if err := os.MkdirAll(settingsPath, 0755); err != nil {
			t.Fatalf("mkdir as file: %v", err)
		}

		err := AddClaudeCodeAllowlist()
		if err == nil || !strings.Contains(err.Error(), "read settings") {
			t.Fatalf("expected read settings error, got %v", err)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)

		settingsPath := filepath.Join(home, ".claude", "settings.json")
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(settingsPath, []byte("{broken"), 0644); err != nil {
			t.Fatalf("write invalid json: %v", err)
		}

		err := AddClaudeCodeAllowlist()
		if err == nil || !strings.Contains(err.Error(), "parse settings") {
			t.Fatalf("expected parse settings error, got %v", err)
		}
	})

	t.Run("invalid permissions returns error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)

		settingsPath := filepath.Join(home, ".claude", "settings.json")
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(settingsPath, []byte(`{"permissions":"bad"}`), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}

		err := AddClaudeCodeAllowlist()
		if err == nil || !strings.Contains(err.Error(), "parse permissions") {
			t.Fatalf("expected parse permissions error, got %v", err)
		}
	})

	t.Run("invalid allow list returns error", func(t *testing.T) {
		resetSetupSeams(t)
		home := useTestHome(t)

		settingsPath := filepath.Join(home, ".claude", "settings.json")
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(settingsPath, []byte(`{"permissions":{"allow":"bad"}}`), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}

		err := AddClaudeCodeAllowlist()
		if err == nil || !strings.Contains(err.Error(), "parse allow list") {
			t.Fatalf("expected parse allow list error, got %v", err)
		}
	})

	t.Run("marshal allow list error", func(t *testing.T) {
		resetSetupSeams(t)
		useTestHome(t)

		jsonMarshalFn = func(any) ([]byte, error) {
			return nil, errors.New("marshal boom")
		}

		err := AddClaudeCodeAllowlist()
		if err == nil || !strings.Contains(err.Error(), "marshal allow list") {
			t.Fatalf("expected marshal allow list error, got %v", err)
		}
	})

	t.Run("marshal permissions error", func(t *testing.T) {
		resetSetupSeams(t)
		useTestHome(t)

		calls := 0
		jsonMarshalFn = func(v any) ([]byte, error) {
			calls++
			if calls == 2 {
				return nil, errors.New("marshal perms boom")
			}
			return json.Marshal(v)
		}

		err := AddClaudeCodeAllowlist()
		if err == nil || !strings.Contains(err.Error(), "marshal permissions") {
			t.Fatalf("expected marshal permissions error, got %v", err)
		}
	})

	t.Run("marshal settings error", func(t *testing.T) {
		resetSetupSeams(t)
		useTestHome(t)

		jsonMarshalIndentFn = func(any, string, string) ([]byte, error) {
			return nil, errors.New("indent boom")
		}

		err := AddClaudeCodeAllowlist()
		if err == nil || !strings.Contains(err.Error(), "marshal settings") {
			t.Fatalf("expected marshal settings error, got %v", err)
		}
	})

	t.Run("write error returns error", func(t *testing.T) {
		resetSetupSeams(t)
		useTestHome(t)

		writeFileFn = func(string, []byte, os.FileMode) error {
			return errors.New("write boom")
		}

		err := AddClaudeCodeAllowlist()
		if err == nil || !strings.Contains(err.Error(), "write settings") {
			t.Fatalf("expected write settings error, got %v", err)
		}
	})

	t.Run("claudeCodeSettingsPath uses home dir", func(t *testing.T) {
		resetSetupSeams(t)
		userHomeDir = func() (string, error) { return "/test/home", nil }

		got := claudeCodeSettingsPath()
		expected := filepath.Join("/test/home", ".claude", "settings.json")
		if got != expected {
			t.Fatalf("expected %q, got %q", expected, got)
		}
	})
}

// ─── Issue #18: opencode.jsonc regression tests ─────────────────────────────

func TestStripJSONC(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no comments", `{"key":"value"}`, `{"key":"value"}`},
		{"single line comment", "{\n// comment\n\"key\":\"value\"}", "{\n\n\"key\":\"value\"}"},
		{"multi line comment", "{/* block */\"key\":\"value\"}", "{\"key\":\"value\"}"},
		{"comment inside string preserved", `{"key":"val // not a comment"}`, `{"key":"val // not a comment"}`},
		{"escaped quote in string", `{"key":"val\"ue"}`, `{"key":"val\"ue"}`},
		{"trailing single-line comment", "{\"key\":\"value\" // inline\n}", "{\"key\":\"value\" \n}"},
		{"empty input", "", ""},
		{"only comments", "// nothing here\n/* also nothing */", "\n"},
		{"comment at EOF without newline", "{\"a\":1}// trailing", "{\"a\":1}"},
		{"unterminated multi-line comment", "{\"a\":1}/* never closed", "{\"a\":1}"},
		{"block comment with stars", "{/* ** fancy ** */\"a\":1}", "{\"a\":1}"},
		{"multi-line block comment preserves newlines", "{\n/* line1\nline2 */\n\"a\":1}", "{\n\n\"a\":1}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripJSONC([]byte(tt.input)))
			if got != tt.want {
				t.Fatalf("stripJSONC(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOpenCodeConfigPathPrefersJSONC(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", "")

	// When .jsonc exists, return .jsonc path
	statFn = func(name string) (os.FileInfo, error) {
		if strings.HasSuffix(name, "opencode.jsonc") {
			return nil, nil // exists
		}
		return nil, os.ErrNotExist
	}

	got := openCodeConfigPath()
	expected := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestOpenCodeConfigPathFallsBackToJSON(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", "")

	// When .jsonc does NOT exist, return .json path
	statFn = func(name string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	got := openCodeConfigPath()
	expected := filepath.Join(home, ".config", "opencode", "opencode.json")
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestOpenCodeTUIConfigPathPrefersJSONC(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", "")

	statFn = func(name string) (os.FileInfo, error) {
		if strings.HasSuffix(name, "tui.jsonc") {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}

	got := openCodeTUIConfigPath()
	expected := filepath.Join(home, ".config", "opencode", "tui.jsonc")
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestOpenCodeTUIConfigPathFallsBackToJSON(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", "")

	statFn = func(name string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	got := openCodeTUIConfigPath()
	expected := filepath.Join(home, ".config", "opencode", "tui.json")
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestOpenCodeConfigPathXDGWithJSONC(t *testing.T) {
	resetSetupSeams(t)
	_ = useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")

	statFn = func(name string) (os.FileInfo, error) {
		if strings.HasSuffix(name, "opencode.jsonc") {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}

	got := openCodeConfigPath()
	expected := filepath.Join("/custom/xdg", "opencode", "opencode.jsonc")
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestOpenCodeConfigPathWindowsWithJSONC(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "windows"
	t.Setenv("APPDATA", "C:/Users/test/AppData/Roaming")
	t.Setenv("XDG_CONFIG_HOME", "")

	statFn = func(name string) (os.FileInfo, error) {
		if strings.HasSuffix(name, "opencode.jsonc") {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}

	got := openCodeConfigPath()
	// OpenCode uses ~/.config/opencode/ on all platforms, not %APPDATA%
	expected := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestInjectOpenCodeMCPHandlesJSONC(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", "")

	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a .jsonc file with comments
	jsoncPath := filepath.Join(configDir, "opencode.jsonc")
	content := `{
  // This is a comment
  "theme": "kanagawa",
  "mcp": {
    /* existing server */
    "other": {"type": "local", "command": ["foo"]}
  }
}`
	if err := os.WriteFile(jsoncPath, []byte(content), 0644); err != nil {
		t.Fatalf("write jsonc: %v", err)
	}

	// statFn should find the .jsonc file
	statFn = os.Stat

	if err := injectOpenCodeMCP(); err != nil {
		t.Fatalf("injectOpenCodeMCP with JSONC failed: %v", err)
	}

	// Verify engram was added to the .jsonc file
	raw, err := os.ReadFile(jsoncPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("result should be valid JSON: %v", err)
	}
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("expected mcp object in result")
	}
	if _, ok := mcp["engram"]; !ok {
		t.Fatalf("expected engram to be registered")
	}
	if _, ok := mcp["other"]; !ok {
		t.Fatalf("expected existing 'other' entry to be preserved")
	}
}

func TestInjectOpenCodeTUIPluginHandlesJSONC(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	runtimeGOOS = "linux"
	t.Setenv("XDG_CONFIG_HOME", "")

	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	jsoncPath := filepath.Join(configDir, "tui.jsonc")
	content := `{
  // Keep existing plugins
  "$schema": "https://opencode.ai/tui.json",
  "plugin": [
    "existing-plugin"
  ]
}`
	if err := os.WriteFile(jsoncPath, []byte(content), 0644); err != nil {
		t.Fatalf("write jsonc: %v", err)
	}

	statFn = os.Stat

	if err := injectOpenCodeTUIPlugin(); err != nil {
		t.Fatalf("injectOpenCodeTUIPlugin with JSONC failed: %v", err)
	}

	raw, err := os.ReadFile(jsoncPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("result should be valid JSON: %v", err)
	}
	plugins, ok := cfg["plugin"].([]any)
	if !ok {
		t.Fatalf("expected plugin array in result")
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins in result, got %v", plugins)
	}
	if plugins[0] != "existing-plugin" || plugins[1] != openCodeSubagentStatuslinePlugin {
		t.Fatalf("unexpected plugins after injection: %v", plugins)
	}
}

// ─── Issue #112: OpenCode MCP absolute-path config ───────────────────────────

// ─── Issue #113: OpenCode plugin ENGRAM_BIN bake-in ─────────────────────────

// ─── Issue #116: Sub-agent session inflation fix ─────────────────────────────

func TestGeminiInjectUsesRemoteURL(t *testing.T) {
	resetSetupSeams(t)
	configPath := filepath.Join(t.TempDir(), "settings.json")
	if err := injectGeminiMCP(configPath); err != nil {
		t.Fatalf("injectGeminiMCP failed: %v", err)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	entry := cfg.MCPServers["engram"]
	if entry["httpUrl"] != engramRemoteMCPURL {
		t.Fatalf("expected httpUrl %q, got %#v", engramRemoteMCPURL, entry)
	}
	if _, hasCommand := entry["command"]; hasCommand {
		t.Fatalf("remote Gemini entry must not carry a local command")
	}
}

func TestCodexBlockUsesRemoteURL(t *testing.T) {
	want := "[mcp_servers.engram]\nurl = \"" + engramRemoteMCPURL + "\""
	if got := codexEngramBlockStr(); got != want {
		t.Fatalf("codex block = %q, want %q", got, want)
	}
}

func TestInjectOpenCodeMCPUsesRemoteURL(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	if err := os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := injectOpenCodeMCP(); err != nil {
		t.Fatalf("injectOpenCodeMCP failed: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	var cfg struct {
		MCP map[string]map[string]any `json:"mcp"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse opencode.json: %v", err)
	}
	entry := cfg.MCP["engram"]
	if entry["type"] != "remote" || entry["url"] != engramRemoteMCPURL {
		t.Fatalf("expected remote engram entry, got %#v", entry)
	}
	if _, hasCommand := entry["command"]; hasCommand {
		t.Fatalf("remote entry must not carry a local command")
	}
}

// codexEngramSection returns the [mcp_servers.engram] table from a Codex config,
// so unrelated servers that legitimately use a command are not inspected.
func codexEngramSection(text string) string {
	start := strings.Index(text, "[mcp_servers.engram]")
	if start == -1 {
		return ""
	}
	section := text[start+1:]
	if next := strings.Index(section, "\n["); next != -1 {
		section = section[:next]
	}
	return section
}

func TestInstallOpenCodePluginIsRemoteOnly(t *testing.T) {
	resetSetupSeams(t)
	home := useTestHome(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))

	if _, err := installOpenCode(); err != nil {
		t.Fatalf("installOpenCode failed: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(home, "xdg", "opencode", "plugins", "engram.ts"))
	if err != nil {
		t.Fatalf("read installed plugin: %v", err)
	}
	content := string(raw)
	for _, forbidden := range []string{"ENGRAM_BIN", "engram serve", "127.0.0.1", "ENGRAM_PORT"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("installed OpenCode plugin must not reference a local engram (%q)", forbidden)
		}
	}
	if !strings.Contains(content, "PROJECT RULE") {
		t.Fatalf("installed OpenCode plugin must carry the project rule")
	}
}
