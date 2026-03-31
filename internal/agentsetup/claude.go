package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ConfigureClaude sets up Claude Code integration for the given repository.
// It creates/updates CLAUDE.md with falcon instructions and configures
// the MCP server in .mcp.json.
func ConfigureClaude(repoRoot string) error {
	// 1. Update CLAUDE.md with falcon instructions.
	claudeMD := filepath.Join(repoRoot, "CLAUDE.md")
	if err := UpsertSection(claudeMD, falconInstructions); err != nil {
		return err
	}

	// 2. Configure MCP server in .mcp.json.
	settingsPath := filepath.Join(repoRoot, ".mcp.json")
	if err := upsertClaudeSettings(settingsPath); err != nil {
		return err
	}

	// 3. Configure SessionStart hook in .claude/settings.json.
	hooksPath := filepath.Join(repoRoot, ".claude", "settings.json")
	return upsertClaudeHooks(hooksPath)
}

func upsertClaudeSettings(settingsPath string) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	// Read existing settings or start fresh.
	settings := make(map[string]any)
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	// Merge mcpServers.falcon entry.
	mcpServers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
	}
	mcpServers["falcon"] = mcpServerConfig()
	settings["mcpServers"] = mcpServers

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(settingsPath, out, 0o644)
}

func upsertClaudeHooks(settingsPath string) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	settings := make(map[string]any)
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	hookCmd := "npx -y repo-falcon sync --agents none"

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	startHooks, _ := hooks["SessionStart"].([]any)

	if !containsFalconHook(startHooks, hookCmd) {
		hookEntry := map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": hookCmd,
					"async":   true,
					"timeout": 300,
				},
			},
		}
		startHooks = append(startHooks, hookEntry)
	}

	hooks["SessionStart"] = startHooks
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(settingsPath, out, 0o644)
}

// containsFalconHook checks whether any hook group already contains a
// falcon sync command. If found with a different binary path, it updates
// the command in place.
func containsFalconHook(hookGroups []any, hookCmd string) bool {
	for _, group := range hookGroups {
		groupMap, ok := group.(map[string]any)
		if !ok {
			continue
		}
		innerHooks, ok := groupMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range innerHooks {
			hMap, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := hMap["command"].(string); ok && strings.Contains(cmd, "falcon sync") {
				hMap["command"] = hookCmd
				hMap["async"] = true
				hMap["timeout"] = float64(300)
				return true
			}
		}
	}
	return false
}
