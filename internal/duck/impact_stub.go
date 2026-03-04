//go:build !duckdb

package duck

import "errors"

// Available reports whether DuckDB-backed computations are compiled in.
func Available() bool { return false }

var ErrNotBuiltWithDuckDB = errors.New("duckdb support not built (compile with -tags duckdb)")
