package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const claudeCodeSlimPluginFloor = "0.1.1"
const claudeCodePluginListTimeout = 2 * time.Second // bounds only the read-only Claude plugin capability probe

type claudeCodePlugin struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Enabled     *bool  `json:"enabled"`
	Marketplace string `json:"marketplace"`
}

func ClaudeCodeSettingsPath() string {
	home, _ := userHomeDir()
	return filepath.Join(claudeCodeConfigRoot(home), "settings.json")
}

func claudeCodeConfigRoot(home string) string {
	dir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	if dir == "" {
		return filepath.Join(home, ".claude")
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

func ClaudeCodeUserMCPPath() string {
	home, _ := userHomeDir()
	if strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")) == "" {
		return filepath.Join(home, ".claude.json")
	}
	return filepath.Join(claudeCodeConfigRoot(home), ".claude.json")
}

func VerifyClaudeCodeSlimCapability() error {
	claudeBin, err := lookPathFn("claude")
	if err != nil {
		return fmt.Errorf("locate claude: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), claudeCodePluginListTimeout)
	defer cancel()
	out, err := runCommandWithContext(ctx, claudeBin, "plugin", "list", "--json")
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("claude plugin list --json timed out")
		}
		return fmt.Errorf("claude plugin list --json: %w", err)
	}

	plugins, err := parseClaudeCodePluginList(out)
	if err != nil {
		return err
	}

	matches := make([]claudeCodePlugin, 0, 1)
	for _, plugin := range plugins {
		if isEngramClaudeCodePlugin(plugin) {
			matches = append(matches, plugin)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("no Engram plugin from the expected marketplace is listed")
	}
	if len(matches) != 1 {
		return fmt.Errorf("multiple Engram plugins from the expected marketplace are listed")
	}

	plugin := matches[0]
	if plugin.Enabled == nil || !*plugin.Enabled {
		return fmt.Errorf("the listed Engram plugin is disabled")
	}
	if !meetsClaudeCodeSlimPluginFloor(plugin.Version) {
		return fmt.Errorf("plugin version %q does not meet the required %s floor", plugin.Version, claudeCodeSlimPluginFloor)
	}
	return nil
}

func parseClaudeCodePluginList(raw []byte) ([]claudeCodePlugin, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, fmt.Errorf("Claude plugin list returned empty JSON")
	}
	if raw[0] == '[' {
		var plugins []claudeCodePlugin
		if err := json.Unmarshal(raw, &plugins); err != nil {
			return nil, fmt.Errorf("parse Claude plugin list JSON: %w", err)
		}
		return plugins, nil
	}
	if raw[0] == '{' {
		var response struct {
			Plugins json.RawMessage `json:"plugins"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, fmt.Errorf("parse Claude plugin list JSON: %w", err)
		}
		if len(response.Plugins) == 0 {
			return nil, fmt.Errorf("Claude plugin list JSON has no plugins array")
		}
		var plugins []claudeCodePlugin
		if err := json.Unmarshal(response.Plugins, &plugins); err != nil {
			return nil, fmt.Errorf("parse Claude plugin list plugins: %w", err)
		}
		return plugins, nil
	}
	return nil, fmt.Errorf("unrecognized Claude plugin list JSON shape")
}

func isEngramClaudeCodePlugin(plugin claudeCodePlugin) bool {
	name := strings.ToLower(strings.TrimSpace(plugin.Name))
	if name != "engram" && name != "engram@engram" {
		return false
	}
	marketplace := strings.TrimSpace(plugin.Marketplace)
	return strings.EqualFold(marketplace, "engram") || strings.EqualFold(marketplace, claudeCodeMarketplace)
}

func meetsClaudeCodeSlimPluginFloor(version string) bool {
	values, prerelease, ok := parseClaudeCodePluginVersion(version)
	if !ok {
		return false
	}
	if values == [3]int{0, 1, 1} && prerelease {
		return false
	}
	return values[0] > 0 || (values[0] == 0 && (values[1] > 1 || (values[1] == 1 && values[2] >= 1)))
}

// EnsureClaudeCodeUserMCP registers Engram's remote MCP server in Claude Code's
// user configuration. The entry is an HTTP transport; no local binary is used.
func EnsureClaudeCodeUserMCP() error {
	return writeClaudeCodeUserMCP()
}

// StaleMCPCommand identifies a generic client registration pointing to a missing binary.
type StaleMCPCommand struct {
	Slug    string `json:"slug"`
	Command string `json:"command"`
}

// StaleMCPCommands reports legacy local-binary registrations. This installer
// only writes remote HTTP entries, so there is nothing local to report.
func StaleMCPCommands() ([]StaleMCPCommand, error) {
	return []StaleMCPCommand{}, nil
}

func parseClaudeCodePluginVersion(version string) ([3]int, bool, bool) {
	var values [3]int
	if version == "" || version != strings.TrimSpace(version) {
		return values, false, false
	}
	version, build, hasBuild := strings.Cut(version, "+")
	if hasBuild && !validSemverIdentifiers(build, false) {
		return values, false, false
	}
	if strings.Contains(build, "+") {
		return values, false, false
	}
	core, prerelease, hasPrerelease := strings.Cut(version, "-")
	if hasPrerelease && !validSemverIdentifiers(prerelease, true) {
		return values, false, false
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return values, false, false
	}
	for i, part := range parts {
		if !validSemverNumber(part) {
			return values, false, false
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return values, false, false
		}
		values[i] = value
	}
	return values, hasPrerelease, true
}
