package mcp

import "os"

// selfExecutable returns the path to the current process's executable.
func selfExecutable() (string, error) {
	return os.Executable()
}
