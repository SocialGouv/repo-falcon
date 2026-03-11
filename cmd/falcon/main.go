package main

import (
	"fmt"
	"os"
	"strings"

	"repofalcon/internal/cli"
)

func main() {
	cmd := cli.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		msg := err.Error()
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
		if strings.HasPrefix(msg, "unknown command") {
			fmt.Fprintln(os.Stderr)
			_ = cmd.Usage()
		}
		os.Exit(1)
	}
}
