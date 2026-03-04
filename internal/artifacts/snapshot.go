package artifacts

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"repofalcon/internal/graph"
)

type SnapshotCounts struct {
	Files    int
	Packages int
	Symbols  int
	Findings int
	Edges    int
	Nodes    int
}

// BuildSnapshot reads index artifacts from inDir and writes snapshot artifacts to outDir.
//
// Snapshot artifacts include:
//   - nodes.parquet: union of all node types (File/Package/Symbol/Finding), deduped by node_id
//   - edges.parquet: optionally re-written with dedupe + stable ordering by edge_id
//
// Additionally, any missing edge endpoints will be synthesized as minimal nodes.
func BuildSnapshot(ctx context.Context, inDir, outDir string) (SnapshotCounts, error) {
	files, err := ReadFilesParquet(ctx, filepath.Join(inDir, "files.parquet"))
	if err != nil {
		return SnapshotCounts{}, fmt.Errorf("read files: %w", err)
	}
	packages, err := ReadPackagesParquet(ctx, filepath.Join(inDir, "packages.parquet"))
	if err != nil {
		return SnapshotCounts{}, fmt.Errorf("read packages: %w", err)
	}
	symbols, err := ReadSymbolsParquet(ctx, filepath.Join(inDir, "symbols.parquet"))
	if err != nil {
		return SnapshotCounts{}, fmt.Errorf("read symbols: %w", err)
	}
	findings, err := ReadFindingsParquet(ctx, filepath.Join(inDir, "findings.parquet"))
	if err != nil {
		return SnapshotCounts{}, fmt.Errorf("read findings: %w", err)
	}
	edges, err := ReadEdgesParquet(ctx, filepath.Join(inDir, "edges.parquet"))
	if err != nil {
		return SnapshotCounts{}, fmt.Errorf("read edges: %w", err)
	}

	// Node materialization.
	nodeByID := map[string]NodeRow{}
	addNode := func(n NodeRow) {
		if n.NodeID == "" {
			return
		}
		if _, exists := nodeByID[n.NodeID]; exists {
			return
		}
		nodeByID[n.NodeID] = n
	}

	for _, fr := range files {
		lang := strings.TrimSpace(fr.Language)
		langPtr := &lang
		attrs := stableAttrsJSON(map[string]any{
			"path":           fr.Path,
			"language":       fr.Language,
			"extension":      fr.Extension,
			"size_bytes":     fr.SizeBytes,
			"content_sha256": fr.ContentSHA256,
			"lines":          fr.Lines,
			"is_generated":   fr.IsGenerated,
			"is_test":        fr.IsTest,
		})
		addNode(NodeRow{
			NodeID:      fr.FileID,
			NodeType:    string(graph.NodeTypeFile),
			Key:         graph.FileKey(fr.Path),
			DisplayName: fr.Path,
			Language:    langPtr,
			AttrsJSON:   &attrs,
		})
	}

	for _, pr := range packages {
		eco := graph.CanonicalLanguage(pr.Ecosystem)
		ecoPtr := &eco
		attrs := stableAttrsJSON(map[string]any{
			"ecosystem":     pr.Ecosystem,
			"scope":         pr.Scope,
			"name":          pr.Name,
			"version":       pr.Version,
			"is_internal":   pr.IsInternal,
			"root_path":     pr.RootPath,
			"manifest_path": pr.ManifestPath,
		})
		display := pr.Name
		if strings.TrimSpace(pr.Version) != "" {
			display = display + "@" + pr.Version
		}
		addNode(NodeRow{
			NodeID:      pr.PackageID,
			NodeType:    string(graph.NodeTypePackage),
			Key:         graph.PackageKey(pr.Ecosystem, pr.Name),
			DisplayName: display,
			Language:    ecoPtr,
			AttrsJSON:   &attrs,
		})
	}

	for _, sr := range symbols {
		lang := graph.CanonicalLanguage(sr.Language)
		langPtr := &lang
		attrs := stableAttrsJSON(map[string]any{
			"file_id":             sr.FileID,
			"package_id":          sr.PackageID,
			"language":            sr.Language,
			"kind":                sr.Kind,
			"name":                sr.Name,
			"qualified_name":      sr.QualifiedName,
			"signature":           sr.Signature,
			"semantic_key":        sr.SemanticKey,
			"start_line":          sr.StartLine,
			"start_col":           sr.StartCol,
			"end_line":            sr.EndLine,
			"end_col":             sr.EndCol,
			"visibility":          sr.Visibility,
			"modifiers":           sr.Modifiers,
			"container_symbol_id": sr.ContainerSymbolID,
		})
		fileID := sr.FileID
		pkgID := sr.PackageID
		addNode(NodeRow{
			NodeID:           sr.SymbolID,
			NodeType:         string(graph.NodeTypeSymbol),
			Key:              strings.TrimSpace(sr.SemanticKey),
			DisplayName:      sr.QualifiedName,
			PrimaryFileID:    &fileID,
			PrimaryPackageID: pkgID,
			Language:         langPtr,
			AttrsJSON:        &attrs,
		})
	}

	for _, fr := range findings {
		attrs := stableAttrsJSON(map[string]any{
			"source_tool":         fr.SourceTool,
			"rule_id":             fr.RuleID,
			"severity":            fr.Severity,
			"message":             fr.Message,
			"message_fingerprint": fr.MessageFingerprint,
			"file_id":             fr.FileID,
			"symbol_id":           fr.SymbolID,
			"package_id":          fr.PackageID,
			"start_line":          fr.StartLine,
			"start_col":           fr.StartCol,
			"end_line":            fr.EndLine,
			"end_col":             fr.EndCol,
			"cwe":                 fr.CWE,
			"tags":                fr.Tags,
			"properties_json":     fr.PropertiesJSON,
		})
		addNode(NodeRow{
			NodeID:      fr.FindingID,
			NodeType:    string(graph.NodeTypeFinding),
			Key:         graph.FindingKey(fr.SourceTool, fr.RuleID, derefString(fr.FileID), int(derefInt32(fr.StartLine)), int(derefInt32(fr.StartCol)), fr.Message),
			DisplayName: fr.RuleID,
			AttrsJSON:   &attrs,
		})
	}

	// Ensure all edge endpoints exist.
	for _, e := range edges {
		if _, ok := nodeByID[e.SrcID]; !ok {
			addNode(synthNodeForEdgeEndpoint(e.SrcID, e.SrcType))
		}
		if _, ok := nodeByID[e.DstID]; !ok {
			addNode(synthNodeForEdgeEndpoint(e.DstID, e.DstType))
		}
	}

	// Materialize nodes slice (sorted/deduped in writer too, but keep stable here).
	nodes := make([]NodeRow, 0, len(nodeByID))
	for _, n := range nodeByID {
		nodes = append(nodes, n)
	}
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].NodeID < nodes[j].NodeID })

	// Stable ordering + dedupe edges by edge_id.
	sort.SliceStable(edges, func(i, j int) bool { return edges[i].EdgeID < edges[j].EdgeID })
	edges = dedupeByKey(edges, func(r EdgeRow) string { return r.EdgeID })

	if err := EnsureDir(outDir); err != nil {
		return SnapshotCounts{}, err
	}
	if err := WriteNodesParquet(filepath.Join(outDir, "nodes.parquet"), nodes); err != nil {
		return SnapshotCounts{}, err
	}
	if err := WriteEdgesParquetByEdgeID(filepath.Join(outDir, "edges.parquet"), edges); err != nil {
		return SnapshotCounts{}, err
	}

	return SnapshotCounts{
		Files:    len(files),
		Packages: len(packages),
		Symbols:  len(symbols),
		Findings: len(findings),
		Edges:    len(edges),
		Nodes:    len(nodes),
	}, nil
}

func synthNodeForEdgeEndpoint(nodeID, nodeType string) NodeRow {
	nt := strings.TrimSpace(nodeType)
	if nt == "" {
		nt = "Unknown"
	}
	// Minimal node: key is node_id (fallback). display_name is node_id.
	return NodeRow{
		NodeID:      nodeID,
		NodeType:    nt,
		Key:         nodeID,
		DisplayName: nodeID,
		AttrsJSON:   nil,
	}
}

// stableAttrsJSON encodes a JSON object with deterministic key ordering.
// It omits nil values from the top-level map.
func stableAttrsJSON(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v == nil {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build JSON object manually to preserve key order.
	var b strings.Builder
	b.Grow(256)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		vb := mustMarshalStableJSON(m[k])
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.String()
}

// mustMarshalStableJSON marshals v to JSON deterministically for common Go types.
//
// It provides stable ordering for maps with string keys, recursively.
// Errors are intentionally ignored to keep snapshotting best-effort (attrs are optional).
func mustMarshalStableJSON(v any) []byte {
	b, err := marshalStableJSON(v)
	if err != nil {
		// Fall back to encoding/json for unknown/unhandled types.
		bb, _ := json.Marshal(v)
		return bb
	}
	return b
}

func marshalStableJSON(v any) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}

	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return []byte("null"), nil
		}
		rv = rv.Elem()
		v = rv.Interface()
	}

	switch rv.Kind() {
	case reflect.Map:
		// Only support string-key maps (the only kind we expect in attrs).
		if rv.Type().Key().Kind() != reflect.String {
			return json.Marshal(v)
		}
		keys := rv.MapKeys()
		ks := make([]string, 0, len(keys))
		for _, k := range keys {
			ks = append(ks, k.String())
		}
		sort.Strings(ks)
		var b strings.Builder
		b.Grow(256)
		b.WriteByte('{')
		for i, k := range ks {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			b.Write(kb)
			b.WriteByte(':')
			vv := rv.MapIndex(reflect.ValueOf(k))
			if !vv.IsValid() {
				b.WriteString("null")
				continue
			}
			vb, err := marshalStableJSON(vv.Interface())
			if err != nil {
				return nil, err
			}
			b.Write(vb)
		}
		b.WriteByte('}')
		return []byte(b.String()), nil

	case reflect.Slice, reflect.Array:
		// Special-case []byte to keep base64 behavior of encoding/json.
		if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
			return json.Marshal(v)
		}
		var b strings.Builder
		b.Grow(256)
		b.WriteByte('[')
		n := rv.Len()
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			vb, err := marshalStableJSON(rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			b.Write(vb)
		}
		b.WriteByte(']')
		return []byte(b.String()), nil

	default:
		// Scalars / structs / other types: defer to encoding/json.
		return json.Marshal(v)
	}
}
