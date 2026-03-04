package prpack

import "repofalcon/internal/artifacts"

// SnapshotTables are the minimum inputs needed for deterministic impact computation.
// These are typically loaded from a snapshot artifacts directory.
type SnapshotTables struct {
	Files    []artifacts.FileRow
	Packages []artifacts.PackageRow
	Symbols  []artifacts.SymbolRow
	Edges    []artifacts.EdgeRow
	Findings []artifacts.FindingRow
}

type ImpactOptions struct {
	// MaxDepth bounds BFS expansions on the graph induced by edges.
	// If zero, a default of 2 is used.
	MaxDepth int
}

type ImpactResult struct {
	ChangedPaths     []string
	ImpactedFiles    []ImpactedFile
	ImpactedSymbols  []ImpactedSymbol
	ImpactedPackages []ImpactedPackage
	AttachedFindings []ImpactedFinding
}

type ImpactedFile struct {
	Path          string  `json:"path"`
	FileID        string  `json:"file_id"`
	InSnapshot    bool    `json:"in_snapshot"`
	Language      *string `json:"language,omitempty"`
	Extension     *string `json:"extension,omitempty"`
	SizeBytes     *int64  `json:"size_bytes,omitempty"`
	ContentSHA256 *string `json:"content_sha256,omitempty"`
	Lines         *int32  `json:"lines,omitempty"`
	IsGenerated   *bool   `json:"is_generated,omitempty"`
	IsTest        *bool   `json:"is_test,omitempty"`
}

type ImpactedSymbol struct {
	SymbolID      string  `json:"symbol_id"`
	QualifiedName string  `json:"qualified_name"`
	Kind          string  `json:"kind"`
	Language      string  `json:"language"`
	FileID        string  `json:"file_id"`
	FilePath      string  `json:"file_path"`
	PackageID     *string `json:"package_id,omitempty"`
}

type ImpactedPackage struct {
	PackageID    string  `json:"package_id"`
	Ecosystem    string  `json:"ecosystem"`
	Scope        string  `json:"scope"`
	Name         string  `json:"name"`
	Version      string  `json:"version"`
	IsInternal   bool    `json:"is_internal"`
	RootPath     *string `json:"root_path,omitempty"`
	ManifestPath *string `json:"manifest_path,omitempty"`
}

type ImpactedFinding struct {
	FindingID          string   `json:"finding_id"`
	SourceTool         string   `json:"source_tool"`
	RuleID             string   `json:"rule_id"`
	Severity           string   `json:"severity"`
	Message            string   `json:"message"`
	MessageFingerprint string   `json:"message_fingerprint"`
	FileID             *string  `json:"file_id,omitempty"`
	FilePath           *string  `json:"file_path,omitempty"`
	SymbolID           *string  `json:"symbol_id,omitempty"`
	PackageID          *string  `json:"package_id,omitempty"`
	StartLine          *int32   `json:"start_line,omitempty"`
	StartCol           *int32   `json:"start_col,omitempty"`
	EndLine            *int32   `json:"end_line,omitempty"`
	EndCol             *int32   `json:"end_col,omitempty"`
	CWE                []int32  `json:"cwe,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	PropertiesJSON     *string  `json:"properties_json,omitempty"`
}
