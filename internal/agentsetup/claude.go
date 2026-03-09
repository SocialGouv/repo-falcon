package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigureClaude sets up Claude Code integration for the given repository.
// It creates/updates CLAUDE.md with falcon instructions and configures
// the MCP server in .claude/settings.json.
func ConfigureClaude(repoRoot, falconBin string) error {
	// 1. Update CLAUDE.md with falcon instructions.
	claudeMD := filepath.Join(repoRoot, "CLAUDE.md")
	if err := UpsertSection(claudeMD, falconInstructions); err != nil {
		return err
	}

	// 2. Configure MCP server in .claude/settings.json.
	settingsPath := filepath.Join(repoRoot, ".claude", "settings.json")
	snapshotDir := filepath.Join(repoRoot, ".falcon", "artifacts")
	return upsertClaudeSettings(settingsPath, falconBin, snapshotDir)
}

func upsertClaudeSettings(settingsPath, falconBin, snapshotDir string) error {
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
	mcpServers["falcon"] = mcpServerConfig(falconBin, snapshotDir)
	settings["mcpServers"] = mcpServers

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(settingsPath, out, 0o644)
}
