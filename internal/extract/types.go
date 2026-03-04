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
