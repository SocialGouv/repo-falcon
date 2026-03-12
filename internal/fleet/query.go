package fleet

import (
	"fmt"
	"os/exec"
	"strings"
)

// RunQuery executes ad-hoc SQL across all fleet Parquet files using the DuckDB CLI.
//
// It creates DuckDB views for each repo's tables, then runs the user's SQL.
// Per-repo views: <reponame>_files, <reponame>_packages, etc.
// Union views: all_files, all_packages, all_symbols, all_edges, all_findings.
// Each view includes a _repo column for provenance.
func RunQuery(m *Manifest, sql, format string) (string, error) {
	duckdbBin, err := exec.LookPath("duckdb")
	if err != nil {
		return "", fmt.Errorf("duckdb CLI not found in PATH: %w\n\nInstall: https://duckdb.org/docs/installation/", err)
	}

	tables := []string{"files", "packages", "symbols", "edges", "findings"}

	var preamble strings.Builder
	for _, repo := range m.Repos {
		name := SanitizeSQLIdent(repo.EffectiveName())
		artDir := repo.EffectiveArtifactsDir()
		for _, table := range tables {
			parquetPath := fmt.Sprintf("%s/%s.parquet", artDir, table)
			fmt.Fprintf(&preamble,
				"CREATE OR REPLACE VIEW %s_%s AS SELECT '%s' AS _repo, * FROM read_parquet('%s');\n",
				name, table, repo.EffectiveName(), parquetPath)
		}
	}

	// Union views across all repos.
	for _, table := range tables {
		var parts []string
		for _, repo := range m.Repos {
			name := SanitizeSQLIdent(repo.EffectiveName())
			parts = append(parts, fmt.Sprintf("SELECT * FROM %s_%s", name, table))
		}
		fmt.Fprintf(&preamble,
			"CREATE OR REPLACE VIEW all_%s AS %s;\n",
			table, strings.Join(parts, " UNION ALL "))
	}

	fullSQL := preamble.String() + "\n" + sql + ";\n"

	mode := "-table"
	switch strings.ToLower(format) {
	case "json":
		mode = "-json"
	case "csv":
		mode = "-csv"
	case "table":
		mode = "-table"
	}

	cmd := exec.Command(duckdbBin, mode)
	cmd.Stdin = strings.NewReader(fullSQL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("duckdb query failed: %w\n%s", err, string(out))
	}
	return string(out), nil
}

// SanitizeSQLIdent makes a repo name safe for use as a SQL identifier.
func SanitizeSQLIdent(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
			b.WriteRune(c)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
