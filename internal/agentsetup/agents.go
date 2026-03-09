package agentsetup

// AgentID identifies a supported coding agent.
type AgentID string

const (
	AgentClaude AgentID = "claude"
	AgentRoo    AgentID = "roo"
	AgentCline  AgentID = "cline"
)

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
