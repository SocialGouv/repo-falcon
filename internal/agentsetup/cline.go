package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigureCline sets up Cline integration for the given repository.
// It creates/updates .clinerules with falcon instructions and configures
// the MCP server in .cline/mcp_settings.json.
func ConfigureCline(repoRoot, falconBin string) error {
	// 1. Update .clinerules with falcon instructions.
	clinerules := filepath.Join(repoRoot, ".clinerules")
	if err := UpsertSection(clinerules, falconInstructions); err != nil {
		return err
	}

	// 2. Configure MCP server in .cline/mcp_settings.json.
	snapshotDir := filepath.Join(repoRoot, ".falcon", "artifacts")
	return upsertClineMCP(filepath.Join(repoRoot, ".cline", "mcp_settings.json"), falconBin, snapshotDir)
}

func upsertClineMCP(mcpPath, falconBin, snapshotDir string) error {
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
	mcpServers["falcon"] = mcpServerConfig(falconBin, snapshotDir)
	config["mcpServers"] = mcpServers

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(mcpPath, out, 0o644)
}
