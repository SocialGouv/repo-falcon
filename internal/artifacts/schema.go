package artifacts

import (
	"github.com/apache/arrow-go/v18/arrow"
)

func NodesSchema() *arrow.Schema {
	return arrow.NewSchema([]arrow.Field{
		{Name: "node_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "node_type", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "display_name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "primary_file_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "primary_package_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "language", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "key", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "attrs_json", Type: arrow.BinaryTypes.String, Nullable: true},
	}, nil)
}

func FilesSchema() *arrow.Schema {
	return arrow.NewSchema([]arrow.Field{
		{Name: "file_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "path", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "language", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "extension", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "size_bytes", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "content_sha256", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "lines", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "is_generated", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
		{Name: "is_test", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
	}, nil)
}

func PackagesSchema() *arrow.Schema {
	return arrow.NewSchema([]arrow.Field{
		{Name: "package_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "ecosystem", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "scope", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "version", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "is_internal", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
		{Name: "root_path", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "manifest_path", Type: arrow.BinaryTypes.String, Nullable: true},
	}, nil)
}

func SymbolsSchema() *arrow.Schema {
	listStr := arrow.ListOf(arrow.BinaryTypes.String)
	return arrow.NewSchema([]arrow.Field{
		{Name: "symbol_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "file_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "package_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "language", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "kind", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "qualified_name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "signature", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "semantic_key", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "start_line", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "start_col", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "end_line", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "end_col", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "visibility", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "modifiers", Type: listStr, Nullable: true},
		{Name: "container_symbol_id", Type: arrow.BinaryTypes.String, Nullable: true},
	}, nil)
}

func FindingsSchema() *arrow.Schema {
	listInt32 := arrow.ListOf(arrow.PrimitiveTypes.Int32)
	listStr := arrow.ListOf(arrow.BinaryTypes.String)
	return arrow.NewSchema([]arrow.Field{
		{Name: "finding_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "source_tool", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "rule_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "severity", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "message", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "message_fingerprint", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "file_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "symbol_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "package_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "start_line", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "start_col", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "end_line", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "end_col", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "cwe", Type: listInt32, Nullable: true},
		{Name: "tags", Type: listStr, Nullable: true},
		{Name: "properties_json", Type: arrow.BinaryTypes.String, Nullable: true},
	}, nil)
}

func EdgesSchema() *arrow.Schema {
	return arrow.NewSchema([]arrow.Field{
		{Name: "edge_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "edge_type", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "src_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "dst_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "src_type", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "dst_type", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "site_file_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "site_start_line", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "site_start_col", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "site_end_line", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "site_end_col", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
		{Name: "confidence", Type: arrow.PrimitiveTypes.Float32, Nullable: true},
		{Name: "properties_json", Type: arrow.BinaryTypes.String, Nullable: true},
	}, nil)
}
