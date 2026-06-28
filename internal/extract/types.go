package extract

// FileExtract is a minimal per-file extraction result.
// Currently only Go emits PackageName and Symbols.
type FileExtract struct {
	Language    string
	RepoRelPath string

	PackageName string   // e.g. "main" for Go
	Imports     []string // module/package strings
	Symbols     []Symbol
}

// Symbol is a minimal, deterministic symbol representation.
// QualifiedName is language-specific but should be stable.
type Symbol struct {
	Language      string
	Kind          string
	Name          string
	QualifiedName string
	StartLine     int
	StartCol      int
	EndLine       int
	EndCol        int
}

// Reference is a use-site of one symbol from inside another (a function call or
// an identifier reference). It is name-based: the index resolver maps Callee
// (and optional Qualifier) to a concrete target symbol id, producing CALLS /
// REFERENCES edges. FromQualified is the QualifiedName of the enclosing
// (caller) symbol, used to locate the source symbol id.
type Reference struct {
	FromQualified string // qualified name of the enclosing symbol
	Callee        string // simple name being called/referenced
	Qualifier     string // optional receiver/module qualifier (e.g. pkg alias)
	RecvType      string // resolved receiver type for a method call (e.g. "T" in x.M() when x: T)
	Kind          string // "call" or "reference"
	Line          int
	Col           int
}
