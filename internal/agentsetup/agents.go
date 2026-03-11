package agentsetup

import (
	"os"
	"path/filepath"
)

// AgentID identifies a supported coding agent.
type AgentID string

const (
	AgentClaude AgentID = "claude"
	AgentRoo    AgentID = "roo"
	AgentCline  AgentID = "cline"
)

// DetectConfiguredAgents returns the list of agents that already have
// falcon MCP configuration in the given repository root.
func DetectConfiguredAgents(repoRoot string) []AgentID {
	markers := map[AgentID]string{
		AgentClaude: ".mcp.json",
		AgentRoo:    filepath.Join(".roo", "mcp.json"),
		AgentCline:  filepath.Join(".cline", "mcp_settings.json"),
	}
	var found []AgentID
	for _, a := range SupportedAgents {
		p, ok := markers[a.ID]
		if !ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoRoot, p)); err == nil {
			found = append(found, a.ID)
		}
	}
	return found
}

// AgentInfo describes a supported coding agent for display and selection.
type AgentInfo struct {
	ID    AgentID
	Label string
}

// SupportedAgents lists all coding agents that falcon can auto-configure.
var SupportedAgents = []AgentInfo{
	{ID: AgentClaude, Label: "Claude Code"},
	{ID: AgentRoo, Label: "Roo Code"},
	{ID: AgentCline, Label: "Cline"},
}

// ParseAgentIDs parses a comma-separated string into a list of valid AgentIDs.
// Unknown IDs are silently ignored.
func ParseAgentIDs(s string) []AgentID {
	if s == "" {
		return nil
	}
	var result []AgentID
	seen := make(map[AgentID]bool)
	for _, part := range splitAndTrim(s, ',') {
		id := AgentID(part)
		if isValidAgent(id) && !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	return result
}

func isValidAgent(id AgentID) bool {
	for _, a := range SupportedAgents {
		if a.ID == id {
			return true
		}
	}
	return false
}

func splitAndTrim(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			p := trimSpace(s[start:i])
			if p != "" {
				parts = append(parts, p)
			}
			start = i + 1
		}
	}
	p := trimSpace(s[start:])
	if p != "" {
		parts = append(parts, p)
	}
	return parts
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}
