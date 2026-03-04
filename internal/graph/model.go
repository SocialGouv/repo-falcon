package graph

// NodeType is the high-level type name persisted in `nodes.parquet`.
type NodeType string

const (
	NodeTypeFile    NodeType = "File"
	NodeTypeSymbol  NodeType = "Symbol"
	NodeTypePackage NodeType = "Package"
	NodeTypeFinding NodeType = "Finding"
)

// EdgeType is the canonical edge type string persisted in `edges.parquet`.
type EdgeType string

const (
	EdgeContains   EdgeType = "CONTAINS"
	EdgeDefines    EdgeType = "DEFINES"
	EdgeInFile     EdgeType = "IN_FILE"
	EdgeImports    EdgeType = "IMPORTS"
	EdgeDependsOn  EdgeType = "DEPENDS_ON"
	EdgeReferences EdgeType = "REFERENCES"
	EdgeCalls      EdgeType = "CALLS"
	EdgeAbout      EdgeType = "ABOUT"
)

// Minimal model structs used across the pipeline.
// These intentionally omit fields not needed by this scaffold.

type File struct {
	FileID   string
	Path     string
	Language string
}

type Package struct {
	PackageID string
	Language  string
	Name      string
}

type Symbol struct {
	SymbolID      string
	Language      string
	Package       string
	QualifiedName string
	RepoRelPath   string
	StartLine     int
	StartCol      int
	EndLine       int
	EndCol        int
}

type Finding struct {
	FindingID   string
	Tool        string
	RuleID      string
	RepoRelPath string
	StartLine   int
	StartCol    int
	Message     string
}
