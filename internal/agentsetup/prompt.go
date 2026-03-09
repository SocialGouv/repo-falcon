package agentsetup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// IsInteractive returns true if stdin is connected to a terminal (TTY).
func IsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// PromptAgentSelection presents a numbered list of agents and reads the user's
// selection from r. It writes the prompt to w. The user can enter comma-separated
// numbers (e.g. "1,2,3") or "all". Returns the selected AgentIDs.
func PromptAgentSelection(w io.Writer, r io.Reader) ([]AgentID, error) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Which coding agents do you use? (select numbers, comma-separated)")
	fmt.Fprintln(w)
	for i, a := range SupportedAgents {
		fmt.Fprintf(w, "  %d) %s\n", i+1, a.Label)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Enter selection (e.g. 1,2 or 'all'), or press Enter to skip: ")

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return nil, nil
	}
	line := strings.TrimSpace(scanner.Text())
	if line == "" {
		return nil, nil
	}

	if strings.EqualFold(line, "all") {
		ids := make([]AgentID, len(SupportedAgents))
		for i, a := range SupportedAgents {
			ids[i] = a.ID
		}
		return ids, nil
	}

	var ids []AgentID
	seen := make(map[AgentID]bool)
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(SupportedAgents) {
			fmt.Fprintf(w, "  (skipping invalid selection: %q)\n", part)
			continue
		}
		id := SupportedAgents[n-1].ID
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}

	return ids, nil
}
