package artifacts

// Row structs mirror the persisted Parquet schemas.

type NodeRow struct {
	NodeID           string
	NodeType         string
	DisplayName      string
	PrimaryFileID    *string
	PrimaryPackageID *string
	Language         *string
	Key              string
	AttrsJSON        *string
}

type FileRow struct {
	FileID        string
	Path          string
	Language      string
	Extension     string
	SizeBytes     int64
	ContentSHA256 string
	Lines         *int32
	IsGenerated   bool
	IsTest        bool
}

type PackageRow struct {
	PackageID    string
	Ecosystem    string
	Scope        string
	Name         string
	Version      string
	IsInternal   bool
	RootPath     *string
	ManifestPath *string
}

type SymbolRow struct {
	SymbolID          string
	FileID            string
	PackageID         *string
	Language          string
	Kind              string
	Name              string
	QualifiedName     string
	Signature         *string
	SemanticKey       string
	StartLine         int32
	StartCol          int32
	EndLine           int32
	EndCol            int32
	Visibility        *string
	Modifiers         []string
	ContainerSymbolID *string
}

type FindingRow struct {
	FindingID          string
	SourceTool         string
	RuleID             string
	Severity           string
	Message            string
	MessageFingerprint string
	FileID             *string
	SymbolID           *string
	PackageID          *string
	StartLine          *int32
	StartCol           *int32
	EndLine            *int32
	EndCol             *int32
	CWE                []int32
	Tags               []string
	PropertiesJSON     *string
}

type EdgeRow struct {
	EdgeID         string
	EdgeType       string
	SrcID          string
	DstID          string
	SrcType        string
	DstType        string
	SiteFileID     *string
	SiteStartLine  *int32
	SiteStartCol   *int32
	SiteEndLine    *int32
	SiteEndCol     *int32
	Confidence     *float32
	PropertiesJSON *string
}
