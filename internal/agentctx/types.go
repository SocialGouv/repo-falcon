package agentctx

// ArchSummary is the top-level summary of a code knowledge graph,
// ready to be rendered as markdown or JSON for consumption by coding agents.
type ArchSummary struct {
	// LangStats maps language name to file count.
	LangStats map[string]int

	// TotalFiles is the total number of files in the graph.
	TotalFiles int

	// SymbolKinds maps symbol kind to count.
	SymbolKinds map[string]int

	// TotalSymbols is the total symbol count.
	TotalSymbols int

	// InternalPkgs are packages owned by this repo.
	InternalPkgs []PackageSummary

	// ExternalDeps are external dependencies.
	ExternalDeps []ExternalDep

	// DepGraph holds internal package dependency edges: from → []to.
	DepGraph map[string][]string

	// ReverseDepGraph holds reverse dependency edges: to → []from.
	ReverseDepGraph map[string][]string
}

// PackageSummary describes an internal package and its contents.
type PackageSummary struct {
	Name      string
	Ecosystem string
	Files     []string
	Symbols   []SymbolBrief
	Imports   []string   // packages this package's files import
	ImportedBy []string  // packages whose files import this package
}

// SymbolBrief is a compact symbol descriptor.
type SymbolBrief struct {
	Name          string
	QualifiedName string
	Kind          string
	FilePath      string
	Line          int32
}

// ExternalDep is an external dependency with its ecosystem and which
// internal packages use it.
type ExternalDep struct {
	Name      string
	Ecosystem string
	UsedBy    []string // internal package names
}
