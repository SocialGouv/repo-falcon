package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigureWindsurf sets up Windsurf integration for the given repository.
// It creates/updates .windsurfrules with instructions and configures
// the MCP server in .windsurf/mcp.json.
func ConfigureWindsurf(repoRoot string) error {
	// 1. Update .windsurfrules with falcon instructions.
	rulesFile := filepath.Join(repoRoot, ".windsurfrules")
	if err := UpsertSection(rulesFile, falconInstructions); err != nil {
		return err
	}

	// 2. Configure MCP server in .windsurf/mcp.json.
	return upsertWindsurfMCP(filepath.Join(repoRoot, ".windsurf", "mcp.json"))
}

func upsertWindsurfMCP(mcpPath string) error {
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
	mcpServers["falcon"] = mcpServerConfig()
	config["mcpServers"] = mcpServers

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(mcpPath, out, 0o644)
}
