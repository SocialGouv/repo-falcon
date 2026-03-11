package agentsetup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// IsInteractive returns true if stdin is connected to a terminal (TTY).
func IsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// PromptAgentSelection presents an interactive multi-select list of agents.
// When running in a TTY, users navigate with arrow keys and toggle with Space.
// Press Enter to confirm or 'a' to select/deselect all.
// Falls back to comma-separated number input when raw mode is unavailable.
func PromptAgentSelection(w io.Writer, r io.Reader) ([]AgentID, error) {
	// Try interactive mode if w is a file (terminal).
	if f, ok := w.(*os.File); ok {
		fd := int(f.Fd())
		if term.IsTerminal(fd) {
			return promptInteractive(f, fd)
		}
	}
	return promptFallback(w, r)
}

// promptInteractive renders a checkbox multi-select using raw terminal input.
func promptInteractive(f *os.File, fd int) ([]AgentID, error) {
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return promptFallback(f, os.Stdin)
	}
	defer term.Restore(fd, oldState)

	agents := SupportedAgents
	selected := make([]bool, len(agents))
	cursor := 0

	render := func() {
		// Move cursor to start and clear.
		fmt.Fprintf(f, "\r\n")
		fmt.Fprintf(f, "Which coding agents do you use?\r\n")
		fmt.Fprintf(f, "  ↑/↓ navigate · Space toggle · a all · Enter confirm\r\n")
		fmt.Fprintf(f, "\r\n")
		for i, a := range agents {
			check := "  "
			if selected[i] {
				check = "✓ "
			}
			pointer := "  "
			if i == cursor {
				pointer = "▸ "
			}
			fmt.Fprintf(f, "  %s[%s] %s\r\n", pointer, check, a.Label)
		}
	}

	clear := func() {
		// Move up past all rendered lines and clear them.
		lines := len(agents) + 4 // header + instructions + blank + items
		for i := 0; i < lines; i++ {
			fmt.Fprintf(f, "\x1b[A\x1b[2K")
		}
	}

	render()

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}
		if n == 0 {
			continue
		}

		switch {
		// Enter
		case buf[0] == '\r' || buf[0] == '\n':
			clear()
			var ids []AgentID
			for i, a := range agents {
				if selected[i] {
					ids = append(ids, a.ID)
				}
			}
			if len(ids) > 0 {
				names := make([]string, len(ids))
				for i, id := range ids {
					names[i] = string(id)
				}
				fmt.Fprintf(f, "Selected agents: %s\r\n", strings.Join(names, ", "))
			} else {
				fmt.Fprintf(f, "No agents selected.\r\n")
			}
			return ids, nil

		// Ctrl-C / Escape
		case buf[0] == 3 || buf[0] == 27 && n == 1:
			clear()
			fmt.Fprintf(f, "Agent selection cancelled.\r\n")
			return nil, nil

		// Space: toggle
		case buf[0] == ' ':
			selected[cursor] = !selected[cursor]
			clear()
			render()

		// 'a' or 'A': toggle all
		case buf[0] == 'a' || buf[0] == 'A':
			allSelected := true
			for _, s := range selected {
				if !s {
					allSelected = false
					break
				}
			}
			for i := range selected {
				selected[i] = !allSelected
			}
			clear()
			render()

		// Arrow keys (escape sequences: ESC [ A/B)
		case n >= 3 && buf[0] == 27 && buf[1] == '[':
			switch buf[2] {
			case 'A': // Up
				if cursor > 0 {
					cursor--
				} else {
					cursor = len(agents) - 1
				}
				clear()
				render()
			case 'B': // Down
				if cursor < len(agents)-1 {
					cursor++
				} else {
					cursor = 0
				}
				clear()
				render()
			}

		// 'j' down, 'k' up (vim keys)
		case buf[0] == 'j':
			if cursor < len(agents)-1 {
				cursor++
			} else {
				cursor = 0
			}
			clear()
			render()
		case buf[0] == 'k':
			if cursor > 0 {
				cursor--
			} else {
				cursor = len(agents) - 1
			}
			clear()
			render()
		}
	}

	return nil, nil
}

// promptFallback uses line-based input for non-TTY environments.
func promptFallback(w io.Writer, r io.Reader) ([]AgentID, error) {
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
