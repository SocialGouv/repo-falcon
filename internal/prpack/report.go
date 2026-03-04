package prpack

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type ReportOptions struct {
	TopFiles   int
	TopSymbols int
}

func WriteReviewReportMarkdown(path string, pack ContextPack, opts ReportOptions) error {
	topFiles := opts.TopFiles
	if topFiles <= 0 {
		topFiles = 20
	}
	topSyms := opts.TopSymbols
	if topSyms <= 0 {
		topSyms = 20
	}

	var b strings.Builder
	b.Grow(8 * 1024)

	f := func(format string, args ...any) { b.WriteString(fmt.Sprintf(format, args...)) }

	f("# PR Review Report\n\n")
	f("- Base: `%s`\n", pack.Base)
	f("- Head: `%s`\n", pack.Head)
	f("- Changed files: %d\n", len(pack.ChangedFiles))
	f("- Impacted files: %d\n", len(pack.ImpactedFiles))
	f("- Impacted symbols: %d\n", len(pack.ImpactedSymbols))
	f("- Impacted packages: %d\n", len(pack.ImpactedPackages))
	f("- Findings: %d\n\n", len(pack.Findings))

	if len(pack.ChangedFiles) > 0 {
		b.WriteString("## Changed Files\n\n")
		for _, c := range pack.ChangedFiles {
			if c.OldPath != "" {
				f("- `%s` %s (from `%s`)\n", c.Path, c.Status, c.OldPath)
			} else {
				f("- `%s` %s\n", c.Path, c.Status)
			}
		}
		b.WriteString("\n")
	}

	if len(pack.ImpactedFiles) > 0 {
		b.WriteString("## Impacted Files (top)\n\n")
		lim := topFiles
		if lim > len(pack.ImpactedFiles) {
			lim = len(pack.ImpactedFiles)
		}
		for i := 0; i < lim; i++ {
			it := pack.ImpactedFiles[i]
			flag := ""
			if !it.InSnapshot {
				flag = " (not in snapshot)"
			}
			f("- `%s`%s\n", it.Path, flag)
		}
		b.WriteString("\n")
	}

	if len(pack.ImpactedSymbols) > 0 {
		b.WriteString("## Impacted Symbols (top)\n\n")
		lim := topSyms
		if lim > len(pack.ImpactedSymbols) {
			lim = len(pack.ImpactedSymbols)
		}
		for i := 0; i < lim; i++ {
			s := pack.ImpactedSymbols[i]
			f("- `%s` (%s) in `%s`\n", s.QualifiedName, s.Kind, s.FilePath)
		}
		b.WriteString("\n")
	}

	if len(pack.Findings) > 0 {
		b.WriteString("## Findings\n\n")
		sevSet := map[string]bool{}
		for _, fd := range pack.Findings {
			sevSet[fd.Severity] = true
		}
		sevs := make([]string, 0, len(sevSet))
		for s := range sevSet {
			sevs = append(sevs, s)
		}
		sort.Strings(sevs)
		for _, sev := range sevs {
			f("### %s\n\n", sev)
			for _, fd := range pack.Findings {
				if fd.Severity != sev {
					continue
				}
				loc := ""
				if fd.FilePath != nil {
					loc = *fd.FilePath
				}
				line := ""
				if fd.StartLine != nil {
					line = fmt.Sprintf(":%d", *fd.StartLine)
				}
				if loc != "" {
					loc = "`" + loc + "`" + line
				}
				if loc != "" {
					f("- **%s** `%s`: %s (%s)\n", fd.SourceTool, fd.RuleID, fd.Message, loc)
				} else {
					f("- **%s** `%s`: %s\n", fd.SourceTool, fd.RuleID, fd.Message)
				}
			}
			b.WriteString("\n")
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}
