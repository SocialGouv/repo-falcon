package agentsetup

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	markerBegin = "<!-- BEGIN FALCON -->"
	markerEnd   = "<!-- END FALCON -->"
)

// UpsertSection inserts or replaces a marker-delimited section in a file.
// If the file contains the markers, the content between them is replaced.
// If the file exists but has no markers, the section is appended.
// If the file does not exist, it is created with the section.
// Parent directories are created as needed.
func UpsertSection(filePath, content string) error {
	section := markerBegin + "\n" + content + "\n" + markerEnd

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}

	existing, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(filePath, []byte(section+"\n"), 0o644)
		}
		return err
	}

	body := string(existing)
	beginIdx := strings.Index(body, markerBegin)
	endIdx := strings.Index(body, markerEnd)

	if beginIdx >= 0 && endIdx >= 0 && endIdx > beginIdx {
		// Replace existing section.
		updated := body[:beginIdx] + section + body[endIdx+len(markerEnd):]
		return os.WriteFile(filePath, []byte(updated), 0o644)
	}

	// Append section to end of file.
	if len(body) > 0 && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += "\n" + section + "\n"
	return os.WriteFile(filePath, []byte(body), 0o644)
}
