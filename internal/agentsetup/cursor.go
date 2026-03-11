package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigureCursor sets up Cursor integration for the given repository.
// It creates .cursor/rules/falcon.mdc with instructions and configures
// the MCP server in .cursor/mcp.json.
func ConfigureCursor(repoRoot, falconBin string) error {
	// 1. Create .cursor/rules/falcon.mdc with instructions.
	rulesDir := filepath.Join(repoRoot, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return err
	}
	mdcContent := `---
description: RepoFalcon Code Knowledge Graph
alwaysApply: true
---

` + falconInstructions + "\n"
	falconMDC := filepath.Join(rulesDir, "falcon.mdc")
	if err := os.WriteFile(falconMDC, []byte(mdcContent), 0o644); err != nil {
		return err
	}

	// 2. Configure MCP server in .cursor/mcp.json.
	return upsertCursorMCP(filepath.Join(repoRoot, ".cursor", "mcp.json"), falconBin)
}

func upsertCursorMCP(mcpPath, falconBin string) error {
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		return err
	}

	config := make(map[string]any)
	data, err := os.ReadFile(mcpPath)
	if err == nil {
		_ = json.Unmarshal(data, &config)
	}

	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
	}
	mcpServers["falcon"] = mcpServerConfig(falconBin)
	config["mcpServers"] = mcpServers

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(mcpPath, out, 0o644)
}
