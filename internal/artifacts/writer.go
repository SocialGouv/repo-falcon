package artifacts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// WriteEmptyTables writes all required tables with zero rows.
func WriteEmptyTables(outDir string) error {
	if err := EnsureDir(outDir); err != nil {
		return err
	}
	if err := WriteNodesParquet(filepath.Join(outDir, "nodes.parquet"), nil); err != nil {
		return err
	}
	if err := WriteFilesParquet(filepath.Join(outDir, "files.parquet"), nil); err != nil {
		return err
	}
	if err := WritePackagesParquet(filepath.Join(outDir, "packages.parquet"), nil); err != nil {
		return err
	}
	if err := WriteSymbolsParquet(filepath.Join(outDir, "symbols.parquet"), nil); err != nil {
		return err
	}
	if err := WriteEdgesParquet(filepath.Join(outDir, "edges.parquet"), nil); err != nil {
		return err
	}
	if err := WriteFindingsParquet(filepath.Join(outDir, "findings.parquet"), nil); err != nil {
		return err
	}
	return nil
}

func WriteNodesParquet(path string, rows []NodeRow) error {
	schema := NodesSchema()
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].NodeID < rows[j].NodeID })
	rows = dedupeByKey(rows, func(r NodeRow) string { return r.NodeID })

	rb := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer rb.Release()

	for _, r := range rows {
		rb.Field(0).(*array.StringBuilder).Append(r.NodeID)
		rb.Field(1).(*array.StringBuilder).Append(r.NodeType)
		rb.Field(2).(*array.StringBuilder).Append(r.DisplayName)
		appendOptString(rb.Field(3).(*array.StringBuilder), r.PrimaryFileID)
		appendOptString(rb.Field(4).(*array.StringBuilder), r.PrimaryPackageID)
		appendOptString(rb.Field(5).(*array.StringBuilder), r.Language)
		rb.Field(6).(*array.StringBuilder).Append(r.Key)
		appendOptString(rb.Field(7).(*array.StringBuilder), r.AttrsJSON)
	}

	return writeParquetRecord(path, schema, rb.NewRecord())
}

func WriteFilesParquet(path string, rows []FileRow) error {
	schema := FilesSchema()
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	rows = dedupeByKey(rows, func(r FileRow) string { return r.FileID })

	rb := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer rb.Release()

	for _, r := range rows {
		rb.Field(0).(*array.StringBuilder).Append(r.FileID)
		rb.Field(1).(*array.StringBuilder).Append(r.Path)
		rb.Field(2).(*array.StringBuilder).Append(r.Language)
		rb.Field(3).(*array.StringBuilder).Append(r.Extension)
		rb.Field(4).(*array.Int64Builder).Append(r.SizeBytes)
		rb.Field(5).(*array.StringBuilder).Append(r.ContentSHA256)
		appendOptInt32(rb.Field(6).(*array.Int32Builder), r.Lines)
		rb.Field(7).(*array.BooleanBuilder).Append(r.IsGenerated)
		rb.Field(8).(*array.BooleanBuilder).Append(r.IsTest)
	}

	return writeParquetRecord(path, schema, rb.NewRecord())
}

func WritePackagesParquet(path string, rows []PackageRow) error {
	schema := PackagesSchema()
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Ecosystem != b.Ecosystem {
			return a.Ecosystem < b.Ecosystem
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Version < b.Version
	})
	rows = dedupeByKey(rows, func(r PackageRow) string { return r.PackageID })

	rb := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer rb.Release()

	for _, r := range rows {
		rb.Field(0).(*array.StringBuilder).Append(r.PackageID)
		rb.Field(1).(*array.StringBuilder).Append(r.Ecosystem)
		rb.Field(2).(*array.StringBuilder).Append(r.Scope)
		rb.Field(3).(*array.StringBuilder).Append(r.Name)
		rb.Field(4).(*array.StringBuilder).Append(r.Version)
		rb.Field(5).(*array.BooleanBuilder).Append(r.IsInternal)
		appendOptString(rb.Field(6).(*array.StringBuilder), r.RootPath)
		appendOptString(rb.Field(7).(*array.StringBuilder), r.ManifestPath)
	}

	return writeParquetRecord(path, schema, rb.NewRecord())
}

func WriteSymbolsParquet(path string, rows []SymbolRow) error {
	schema := SymbolsSchema()
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.FileID != b.FileID {
			return a.FileID < b.FileID
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		if a.StartCol != b.StartCol {
			return a.StartCol < b.StartCol
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.QualifiedName < b.QualifiedName
	})
	rows = dedupeByKey(rows, func(r SymbolRow) string { return r.SymbolID })

	rb := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer rb.Release()

	modsBuilder := rb.Field(14).(*array.ListBuilder)
	modsValBuilder := modsBuilder.ValueBuilder().(*array.StringBuilder)

	for _, r := range rows {
		rb.Field(0).(*array.StringBuilder).Append(r.SymbolID)
		rb.Field(1).(*array.StringBuilder).Append(r.FileID)
		appendOptString(rb.Field(2).(*array.StringBuilder), r.PackageID)
		rb.Field(3).(*array.StringBuilder).Append(r.Language)
		rb.Field(4).(*array.StringBuilder).Append(r.Kind)
		rb.Field(5).(*array.StringBuilder).Append(r.Name)
		rb.Field(6).(*array.StringBuilder).Append(r.QualifiedName)
		appendOptString(rb.Field(7).(*array.StringBuilder), r.Signature)
		rb.Field(8).(*array.StringBuilder).Append(r.SemanticKey)
		rb.Field(9).(*array.Int32Builder).Append(r.StartLine)
		rb.Field(10).(*array.Int32Builder).Append(r.StartCol)
		rb.Field(11).(*array.Int32Builder).Append(r.EndLine)
		rb.Field(12).(*array.Int32Builder).Append(r.EndCol)
		appendOptString(rb.Field(13).(*array.StringBuilder), r.Visibility)

		// modifiers: NULL if absent (nil slice), empty list if explicitly empty.
		if r.Modifiers == nil {
			modsBuilder.AppendNull()
		} else {
			modsBuilder.Append(true)
			for _, m := range r.Modifiers {
				modsValBuilder.Append(m)
			}
		}

		appendOptString(rb.Field(15).(*array.StringBuilder), r.ContainerSymbolID)
	}

	return writeParquetRecord(path, schema, rb.NewRecord())
}

func WriteFindingsParquet(path string, rows []FindingRow) error {
	schema := FindingsSchema()
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.SourceTool != b.SourceTool {
			return a.SourceTool < b.SourceTool
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		ai, bi := derefString(a.FileID), derefString(b.FileID)
		if ai != bi {
			return ai < bi
		}
		asl, bsl := derefInt32(a.StartLine), derefInt32(b.StartLine)
		if asl != bsl {
			return asl < bsl
		}
		asc, bsc := derefInt32(a.StartCol), derefInt32(b.StartCol)
		if asc != bsc {
			return asc < bsc
		}
		return a.MessageFingerprint < b.MessageFingerprint
	})
	rows = dedupeByKey(rows, func(r FindingRow) string { return r.FindingID })

	rb := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer rb.Release()

	cweBuilder := rb.Field(13).(*array.ListBuilder)
	cweValBuilder := cweBuilder.ValueBuilder().(*array.Int32Builder)
	tagsBuilder := rb.Field(14).(*array.ListBuilder)
	tagsValBuilder := tagsBuilder.ValueBuilder().(*array.StringBuilder)

	for _, r := range rows {
		rb.Field(0).(*array.StringBuilder).Append(r.FindingID)
		rb.Field(1).(*array.StringBuilder).Append(r.SourceTool)
		rb.Field(2).(*array.StringBuilder).Append(r.RuleID)
		rb.Field(3).(*array.StringBuilder).Append(r.Severity)
		rb.Field(4).(*array.StringBuilder).Append(r.Message)
		rb.Field(5).(*array.StringBuilder).Append(r.MessageFingerprint)
		appendOptString(rb.Field(6).(*array.StringBuilder), r.FileID)
		appendOptString(rb.Field(7).(*array.StringBuilder), r.SymbolID)
		appendOptString(rb.Field(8).(*array.StringBuilder), r.PackageID)
		appendOptInt32(rb.Field(9).(*array.Int32Builder), r.StartLine)
		appendOptInt32(rb.Field(10).(*array.Int32Builder), r.StartCol)
		appendOptInt32(rb.Field(11).(*array.Int32Builder), r.EndLine)
		appendOptInt32(rb.Field(12).(*array.Int32Builder), r.EndCol)

		if r.CWE == nil {
			cweBuilder.AppendNull()
		} else {
			cweBuilder.Append(true)
			for _, v := range r.CWE {
				cweValBuilder.Append(v)
			}
		}

		if r.Tags == nil {
			tagsBuilder.AppendNull()
		} else {
			tagsBuilder.Append(true)
			for _, v := range r.Tags {
				tagsValBuilder.Append(v)
			}
		}

		appendOptString(rb.Field(15).(*array.StringBuilder), r.PropertiesJSON)
	}

	return writeParquetRecord(path, schema, rb.NewRecord())
}

func WriteEdgesParquet(path string, rows []EdgeRow) error {
	schema := EdgesSchema()
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.EdgeType != b.EdgeType {
			return a.EdgeType < b.EdgeType
		}
		if a.SrcID != b.SrcID {
			return a.SrcID < b.SrcID
		}
		if a.DstID != b.DstID {
			return a.DstID < b.DstID
		}
		asf, bsf := derefString(a.SiteFileID), derefString(b.SiteFileID)
		if asf != bsf {
			return asf < bsf
		}
		asl, bsl := derefInt32(a.SiteStartLine), derefInt32(b.SiteStartLine)
		if asl != bsl {
			return asl < bsl
		}
		asc, bsc := derefInt32(a.SiteStartCol), derefInt32(b.SiteStartCol)
		if asc != bsc {
			return asc < bsc
		}
		return a.EdgeID < b.EdgeID
	})
	rows = dedupeByKey(rows, func(r EdgeRow) string { return r.EdgeID })

	rb := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer rb.Release()

	for _, r := range rows {
		rb.Field(0).(*array.StringBuilder).Append(r.EdgeID)
		rb.Field(1).(*array.StringBuilder).Append(r.EdgeType)
		rb.Field(2).(*array.StringBuilder).Append(r.SrcID)
		rb.Field(3).(*array.StringBuilder).Append(r.DstID)
		rb.Field(4).(*array.StringBuilder).Append(r.SrcType)
		rb.Field(5).(*array.StringBuilder).Append(r.DstType)
		appendOptString(rb.Field(6).(*array.StringBuilder), r.SiteFileID)
		appendOptInt32(rb.Field(7).(*array.Int32Builder), r.SiteStartLine)
		appendOptInt32(rb.Field(8).(*array.Int32Builder), r.SiteStartCol)
		appendOptInt32(rb.Field(9).(*array.Int32Builder), r.SiteEndLine)
		appendOptInt32(rb.Field(10).(*array.Int32Builder), r.SiteEndCol)
		appendOptFloat32(rb.Field(11).(*array.Float32Builder), r.Confidence)
		appendOptString(rb.Field(12).(*array.StringBuilder), r.PropertiesJSON)
	}

	return writeParquetRecord(path, schema, rb.NewRecord())
}

// WriteEdgesParquetByEdgeID is a deterministic edge writer which orders rows by edge_id
// and dedupes by edge_id.
//
// This is intended for snapshot materialization where strict ordering matters.
func WriteEdgesParquetByEdgeID(path string, rows []EdgeRow) error {
	schema := EdgesSchema()
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].EdgeID < rows[j].EdgeID })
	rows = dedupeByKey(rows, func(r EdgeRow) string { return r.EdgeID })

	rb := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer rb.Release()

	for _, r := range rows {
		rb.Field(0).(*array.StringBuilder).Append(r.EdgeID)
		rb.Field(1).(*array.StringBuilder).Append(r.EdgeType)
		rb.Field(2).(*array.StringBuilder).Append(r.SrcID)
		rb.Field(3).(*array.StringBuilder).Append(r.DstID)
		rb.Field(4).(*array.StringBuilder).Append(r.SrcType)
		rb.Field(5).(*array.StringBuilder).Append(r.DstType)
		appendOptString(rb.Field(6).(*array.StringBuilder), r.SiteFileID)
		appendOptInt32(rb.Field(7).(*array.Int32Builder), r.SiteStartLine)
		appendOptInt32(rb.Field(8).(*array.Int32Builder), r.SiteStartCol)
		appendOptInt32(rb.Field(9).(*array.Int32Builder), r.SiteEndLine)
		appendOptInt32(rb.Field(10).(*array.Int32Builder), r.SiteEndCol)
		appendOptFloat32(rb.Field(11).(*array.Float32Builder), r.Confidence)
		appendOptString(rb.Field(12).(*array.StringBuilder), r.PropertiesJSON)
	}

	return writeParquetRecord(path, schema, rb.NewRecord())
}

func writeParquetRecord(path string, schema *arrow.Schema, rec arrow.Record) error {
	defer rec.Release()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	props := parquet.NewWriterProperties(
		parquet.WithCompression(compress.Codecs.Zstd),
		parquet.WithDictionaryDefault(true),
	)
	arrProps := pqarrow.NewArrowWriterProperties(pqarrow.WithStoreSchema())

	w, err := pqarrow.NewFileWriter(schema, f, props, arrProps)
	if err != nil {
		return fmt.Errorf("create parquet writer: %w", err)
	}
	defer w.Close()

	if err := w.Write(rec); err != nil {
		return fmt.Errorf("write parquet record: %w", err)
	}

	return nil
}

func appendOptString(b *array.StringBuilder, v *string) {
	if v == nil {
		b.AppendNull()
		return
	}
	b.Append(*v)
}

func appendOptInt32(b *array.Int32Builder, v *int32) {
	if v == nil {
		b.AppendNull()
		return
	}
	b.Append(*v)
}

func appendOptFloat32(b *array.Float32Builder, v *float32) {
	if v == nil {
		b.AppendNull()
		return
	}
	b.Append(*v)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func dedupeByKey[T any](rows []T, keyFn func(T) string) []T {
	if len(rows) == 0 {
		return rows
	}
	out := rows[:0]
	var lastKey string
	var hasLast bool
	for _, r := range rows {
		k := keyFn(r)
		if hasLast && k == lastKey {
			continue
		}
		hasLast = true
		lastKey = k
		out = append(out, r)
	}
	return out
}
