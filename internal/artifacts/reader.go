package artifacts

import (
	"context"
	"fmt"
	"os"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// readParquetTable reads a Parquet file into an Arrow table.
func readParquetTable(ctx context.Context, path string) (arrow.Table, error) {
	rdr, err := file.OpenParquetFile(path, true)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()

	fr, err := pqarrow.NewFileReader(rdr, pqarrow.ArrowReadProperties{}, memory.DefaultAllocator)
	if err != nil {
		return nil, fmt.Errorf("create arrow parquet reader: %w", err)
	}

	tbl, err := fr.ReadTable(ctx)
	if err != nil {
		return nil, fmt.Errorf("read parquet table: %w", err)
	}
	return tbl, nil
}

func ReadFilesParquet(ctx context.Context, path string) ([]FileRow, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	tbl, err := readParquetTable(ctx, path)
	if err != nil {
		return nil, err
	}
	defer tbl.Release()

	tr := array.NewTableReader(tbl, 1024*64)
	defer tr.Release()

	var out []FileRow
	for tr.Next() {
		rec := tr.Record()
		// record is owned by table reader; don't Release.
		out = append(out, readFilesFromRecord(rec)...)
	}
	if err := tr.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ReadPackagesParquet(ctx context.Context, path string) ([]PackageRow, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	tbl, err := readParquetTable(ctx, path)
	if err != nil {
		return nil, err
	}
	defer tbl.Release()

	tr := array.NewTableReader(tbl, 1024*64)
	defer tr.Release()

	var out []PackageRow
	for tr.Next() {
		rec := tr.Record()
		out = append(out, readPackagesFromRecord(rec)...)
	}
	if err := tr.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ReadSymbolsParquet(ctx context.Context, path string) ([]SymbolRow, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	tbl, err := readParquetTable(ctx, path)
	if err != nil {
		return nil, err
	}
	defer tbl.Release()

	tr := array.NewTableReader(tbl, 1024*64)
	defer tr.Release()

	var out []SymbolRow
	for tr.Next() {
		rec := tr.Record()
		out = append(out, readSymbolsFromRecord(rec)...)
	}
	if err := tr.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ReadEdgesParquet(ctx context.Context, path string) ([]EdgeRow, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	tbl, err := readParquetTable(ctx, path)
	if err != nil {
		return nil, err
	}
	defer tbl.Release()

	tr := array.NewTableReader(tbl, 1024*64)
	defer tr.Release()

	var out []EdgeRow
	for tr.Next() {
		rec := tr.Record()
		out = append(out, readEdgesFromRecord(rec)...)
	}
	if err := tr.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ReadFindingsParquet(ctx context.Context, path string) ([]FindingRow, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	tbl, err := readParquetTable(ctx, path)
	if err != nil {
		return nil, err
	}
	defer tbl.Release()

	tr := array.NewTableReader(tbl, 1024*64)
	defer tr.Release()

	var out []FindingRow
	for tr.Next() {
		rec := tr.Record()
		out = append(out, readFindingsFromRecord(rec)...)
	}
	if err := tr.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ReadNodesParquet(ctx context.Context, path string) ([]NodeRow, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	tbl, err := readParquetTable(ctx, path)
	if err != nil {
		return nil, err
	}
	defer tbl.Release()

	tr := array.NewTableReader(tbl, 1024*64)
	defer tr.Release()

	var out []NodeRow
	for tr.Next() {
		rec := tr.Record()
		out = append(out, readNodesFromRecord(rec)...)
	}
	if err := tr.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func readFilesFromRecord(rec arrow.Record) []FileRow {
	col := func(i int) arrow.Array { return rec.Column(i) }

	fileID := col(0).(*array.String)
	path := col(1).(*array.String)
	lang := col(2).(*array.String)
	ext := col(3).(*array.String)
	size := col(4).(*array.Int64)
	sha := col(5).(*array.String)
	lines := col(6).(*array.Int32)
	isGen := col(7).(*array.Boolean)
	isTest := col(8).(*array.Boolean)

	out := make([]FileRow, 0, rec.NumRows())
	for i := 0; i < int(rec.NumRows()); i++ {
		var linesPtr *int32
		if !lines.IsNull(i) {
			v := lines.Value(i)
			linesPtr = &v
		}
		out = append(out, FileRow{
			FileID:        fileID.Value(i),
			Path:          path.Value(i),
			Language:      lang.Value(i),
			Extension:     ext.Value(i),
			SizeBytes:     size.Value(i),
			ContentSHA256: sha.Value(i),
			Lines:         linesPtr,
			IsGenerated:   isGen.Value(i),
			IsTest:        isTest.Value(i),
		})
	}
	return out
}

func readPackagesFromRecord(rec arrow.Record) []PackageRow {
	col := func(i int) arrow.Array { return rec.Column(i) }

	pkgID := col(0).(*array.String)
	eco := col(1).(*array.String)
	scope := col(2).(*array.String)
	name := col(3).(*array.String)
	ver := col(4).(*array.String)
	isInternal := col(5).(*array.Boolean)
	rootPath := col(6).(*array.String)
	manifestPath := col(7).(*array.String)

	out := make([]PackageRow, 0, rec.NumRows())
	for i := 0; i < int(rec.NumRows()); i++ {
		var rootPtr *string
		if !rootPath.IsNull(i) {
			v := rootPath.Value(i)
			rootPtr = &v
		}
		var manifestPtr *string
		if !manifestPath.IsNull(i) {
			v := manifestPath.Value(i)
			manifestPtr = &v
		}
		out = append(out, PackageRow{
			PackageID:    pkgID.Value(i),
			Ecosystem:    eco.Value(i),
			Scope:        scope.Value(i),
			Name:         name.Value(i),
			Version:      ver.Value(i),
			IsInternal:   isInternal.Value(i),
			RootPath:     rootPtr,
			ManifestPath: manifestPtr,
		})
	}
	return out
}

func readSymbolsFromRecord(rec arrow.Record) []SymbolRow {
	col := func(i int) arrow.Array { return rec.Column(i) }

	symID := col(0).(*array.String)
	fileID := col(1).(*array.String)
	pkgID := col(2).(*array.String)
	lang := col(3).(*array.String)
	kind := col(4).(*array.String)
	name := col(5).(*array.String)
	qname := col(6).(*array.String)
	sig := col(7).(*array.String)
	semKey := col(8).(*array.String)
	sl := col(9).(*array.Int32)
	sc := col(10).(*array.Int32)
	el := col(11).(*array.Int32)
	ec := col(12).(*array.Int32)
	vis := col(13).(*array.String)
	mods := col(14).(*array.List)
	container := col(15).(*array.String)

	modsVals := mods.ListValues().(*array.String)
	offsets := mods.Offsets()

	out := make([]SymbolRow, 0, rec.NumRows())
	for i := 0; i < int(rec.NumRows()); i++ {
		var pkgPtr *string
		if !pkgID.IsNull(i) {
			v := pkgID.Value(i)
			pkgPtr = &v
		}
		var sigPtr *string
		if !sig.IsNull(i) {
			v := sig.Value(i)
			sigPtr = &v
		}
		var visPtr *string
		if !vis.IsNull(i) {
			v := vis.Value(i)
			visPtr = &v
		}
		var containerPtr *string
		if !container.IsNull(i) {
			v := container.Value(i)
			containerPtr = &v
		}

		// modifiers: nil if the list is NULL, empty slice if list is present but empty.
		var modsSlice []string
		if mods.IsNull(i) {
			modsSlice = nil
		} else {
			start := int(offsets[i])
			end := int(offsets[i+1])
			modsSlice = make([]string, 0, end-start)
			for j := start; j < end; j++ {
				modsSlice = append(modsSlice, modsVals.Value(j))
			}
		}

		out = append(out, SymbolRow{
			SymbolID:          symID.Value(i),
			FileID:            fileID.Value(i),
			PackageID:         pkgPtr,
			Language:          lang.Value(i),
			Kind:              kind.Value(i),
			Name:              name.Value(i),
			QualifiedName:     qname.Value(i),
			Signature:         sigPtr,
			SemanticKey:       semKey.Value(i),
			StartLine:         sl.Value(i),
			StartCol:          sc.Value(i),
			EndLine:           el.Value(i),
			EndCol:            ec.Value(i),
			Visibility:        visPtr,
			Modifiers:         modsSlice,
			ContainerSymbolID: containerPtr,
		})
	}
	return out
}

func readFindingsFromRecord(rec arrow.Record) []FindingRow {
	col := func(i int) arrow.Array { return rec.Column(i) }

	fid := col(0).(*array.String)
	tool := col(1).(*array.String)
	rule := col(2).(*array.String)
	sev := col(3).(*array.String)
	msg := col(4).(*array.String)
	fp := col(5).(*array.String)
	fileID := col(6).(*array.String)
	symID := col(7).(*array.String)
	pkgID := col(8).(*array.String)
	sl := col(9).(*array.Int32)
	sc := col(10).(*array.Int32)
	el := col(11).(*array.Int32)
	ec := col(12).(*array.Int32)
	cwe := col(13).(*array.List)
	tags := col(14).(*array.List)
	props := col(15).(*array.String)

	cweVals := cwe.ListValues().(*array.Int32)
	cweOffsets := cwe.Offsets()
	tagsVals := tags.ListValues().(*array.String)
	tagsOffsets := tags.Offsets()

	out := make([]FindingRow, 0, rec.NumRows())
	for i := 0; i < int(rec.NumRows()); i++ {
		var filePtr *string
		if !fileID.IsNull(i) {
			v := fileID.Value(i)
			filePtr = &v
		}
		var symPtr *string
		if !symID.IsNull(i) {
			v := symID.Value(i)
			symPtr = &v
		}
		var pkgPtr *string
		if !pkgID.IsNull(i) {
			v := pkgID.Value(i)
			pkgPtr = &v
		}
		var slPtr, scPtr, elPtr, ecPtr *int32
		if !sl.IsNull(i) {
			v := sl.Value(i)
			slPtr = &v
		}
		if !sc.IsNull(i) {
			v := sc.Value(i)
			scPtr = &v
		}
		if !el.IsNull(i) {
			v := el.Value(i)
			elPtr = &v
		}
		if !ec.IsNull(i) {
			v := ec.Value(i)
			ecPtr = &v
		}

		var cweSlice []int32
		if cwe.IsNull(i) {
			cweSlice = nil
		} else {
			start := int(cweOffsets[i])
			end := int(cweOffsets[i+1])
			cweSlice = make([]int32, 0, end-start)
			for j := start; j < end; j++ {
				cweSlice = append(cweSlice, cweVals.Value(j))
			}
		}

		var tagsSlice []string
		if tags.IsNull(i) {
			tagsSlice = nil
		} else {
			start := int(tagsOffsets[i])
			end := int(tagsOffsets[i+1])
			tagsSlice = make([]string, 0, end-start)
			for j := start; j < end; j++ {
				tagsSlice = append(tagsSlice, tagsVals.Value(j))
			}
		}

		var propsPtr *string
		if !props.IsNull(i) {
			v := props.Value(i)
			propsPtr = &v
		}

		out = append(out, FindingRow{
			FindingID:          fid.Value(i),
			SourceTool:         tool.Value(i),
			RuleID:             rule.Value(i),
			Severity:           sev.Value(i),
			Message:            msg.Value(i),
			MessageFingerprint: fp.Value(i),
			FileID:             filePtr,
			SymbolID:           symPtr,
			PackageID:          pkgPtr,
			StartLine:          slPtr,
			StartCol:           scPtr,
			EndLine:            elPtr,
			EndCol:             ecPtr,
			CWE:                cweSlice,
			Tags:               tagsSlice,
			PropertiesJSON:     propsPtr,
		})
	}
	return out
}

func readEdgesFromRecord(rec arrow.Record) []EdgeRow {
	col := func(i int) arrow.Array { return rec.Column(i) }

	eid := col(0).(*array.String)
	et := col(1).(*array.String)
	src := col(2).(*array.String)
	dst := col(3).(*array.String)
	srcType := col(4).(*array.String)
	dstType := col(5).(*array.String)
	siteFile := col(6).(*array.String)
	sl := col(7).(*array.Int32)
	sc := col(8).(*array.Int32)
	el := col(9).(*array.Int32)
	ec := col(10).(*array.Int32)
	conf := col(11).(*array.Float32)
	props := col(12).(*array.String)

	out := make([]EdgeRow, 0, rec.NumRows())
	for i := 0; i < int(rec.NumRows()); i++ {
		var siteFilePtr *string
		if !siteFile.IsNull(i) {
			v := siteFile.Value(i)
			siteFilePtr = &v
		}
		var slPtr, scPtr, elPtr, ecPtr *int32
		if !sl.IsNull(i) {
			v := sl.Value(i)
			slPtr = &v
		}
		if !sc.IsNull(i) {
			v := sc.Value(i)
			scPtr = &v
		}
		if !el.IsNull(i) {
			v := el.Value(i)
			elPtr = &v
		}
		if !ec.IsNull(i) {
			v := ec.Value(i)
			ecPtr = &v
		}
		var confPtr *float32
		if !conf.IsNull(i) {
			v := conf.Value(i)
			confPtr = &v
		}
		var propsPtr *string
		if !props.IsNull(i) {
			v := props.Value(i)
			propsPtr = &v
		}

		out = append(out, EdgeRow{
			EdgeID:         eid.Value(i),
			EdgeType:       et.Value(i),
			SrcID:          src.Value(i),
			DstID:          dst.Value(i),
			SrcType:        srcType.Value(i),
			DstType:        dstType.Value(i),
			SiteFileID:     siteFilePtr,
			SiteStartLine:  slPtr,
			SiteStartCol:   scPtr,
			SiteEndLine:    elPtr,
			SiteEndCol:     ecPtr,
			Confidence:     confPtr,
			PropertiesJSON: propsPtr,
		})
	}
	return out
}

func readNodesFromRecord(rec arrow.Record) []NodeRow {
	col := func(i int) arrow.Array { return rec.Column(i) }

	nid := col(0).(*array.String)
	ntype := col(1).(*array.String)
	disp := col(2).(*array.String)
	pf := col(3).(*array.String)
	pp := col(4).(*array.String)
	lang := col(5).(*array.String)
	key := col(6).(*array.String)
	attrs := col(7).(*array.String)

	out := make([]NodeRow, 0, rec.NumRows())
	for i := 0; i < int(rec.NumRows()); i++ {
		var pfPtr, ppPtr, langPtr, attrsPtr *string
		if !pf.IsNull(i) {
			v := pf.Value(i)
			pfPtr = &v
		}
		if !pp.IsNull(i) {
			v := pp.Value(i)
			ppPtr = &v
		}
		if !lang.IsNull(i) {
			v := lang.Value(i)
			langPtr = &v
		}
		if !attrs.IsNull(i) {
			v := attrs.Value(i)
			attrsPtr = &v
		}

		out = append(out, NodeRow{
			NodeID:           nid.Value(i),
			NodeType:         ntype.Value(i),
			DisplayName:      disp.Value(i),
			PrimaryFileID:    pfPtr,
			PrimaryPackageID: ppPtr,
			Language:         langPtr,
			Key:              key.Value(i),
			AttrsJSON:        attrsPtr,
		})
	}
	return out
}
