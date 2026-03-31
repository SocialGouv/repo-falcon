package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigureRoo sets up Roo Code integration for the given repository.
// It creates .roo/rules/falcon.md with instructions and .roo/mcp.json
// with the MCP server configuration.
func ConfigureRoo(repoRoot string) error {
	// 1. Create .roo/rules/falcon.md with instructions.
	rulesDir := filepath.Join(repoRoot, ".roo", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return err
	}
	falconMD := filepath.Join(rulesDir, "falcon.md")
	if err := os.WriteFile(falconMD, []byte(falconInstructions+"\n"), 0o644); err != nil {
		return err
	}

	// 2. Configure MCP server in .roo/mcp.json.
	return upsertRooMCP(filepath.Join(repoRoot, ".roo", "mcp.json"))
}

func upsertRooMCP(mcpPath string) error {
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		return err
	}

	// Read existing config or start fresh.
	config := make(map[string]any)
	data, err := os.ReadFile(mcpPath)
	if err == nil {
		_ = json.Unmarshal(data, &config)
	}

	// Merge mcpServers.falcon entry.
	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
	}
	mcpServers["falcon"] = mcpServerConfig()
	config["mcpServers"] = mcpServers

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(mcpPath, out, 0o644)
}
