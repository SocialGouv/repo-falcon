package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigureCopilot sets up GitHub Copilot integration for the given repository.
// It creates/updates .github/copilot-instructions.md with instructions and
// configures the MCP server in .vscode/mcp.json.
func ConfigureCopilot(repoRoot, falconBin string) error {
	// 1. Update .github/copilot-instructions.md with falcon instructions.
	ghDir := filepath.Join(repoRoot, ".github")
	if err := os.MkdirAll(ghDir, 0o755); err != nil {
		return err
	}
	instructionsFile := filepath.Join(ghDir, "copilot-instructions.md")
	if err := UpsertSection(instructionsFile, falconInstructions); err != nil {
		return err
	}

	// 2. Configure MCP server in .vscode/mcp.json.
	return upsertCopilotMCP(filepath.Join(repoRoot, ".vscode", "mcp.json"), falconBin)
}

func upsertCopilotMCP(mcpPath, falconBin string) error {
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		return err
	}

	config := make(map[string]any)
	data, err := os.ReadFile(mcpPath)
	if err == nil {
		_ = json.Unmarshal(data, &config)
	}

	// VS Code / Copilot uses "servers" key, not "mcpServers".
	servers, ok := config["servers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers["falcon"] = mcpServerConfig(falconBin)
	config["servers"] = servers

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(mcpPath, out, 0o644)
}
