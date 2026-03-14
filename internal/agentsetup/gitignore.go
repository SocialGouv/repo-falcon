package agentsetup

import (
	"os"
	"path/filepath"
	"strings"
)

// EnsureGitignore ensures that the .falcon/ entry is present in the
// repository's .gitignore file. It creates the file if it doesn't exist.
// Idempotent: does nothing if .falcon or .falcon/ is already listed.
func EnsureGitignore(repoRoot string) error {
	gitignorePath := filepath.Join(repoRoot, ".gitignore")

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(gitignorePath, []byte(".falcon/\n"), 0o644)
		}
		return err
	}

	body := string(data)
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case ".falcon", ".falcon/", "/.falcon", "/.falcon/":
			return nil // already present
		}
	}

	// Append .falcon/ entry.
	if len(body) > 0 && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += ".falcon/\n"
	return os.WriteFile(gitignorePath, []byte(body), 0o644)
}
