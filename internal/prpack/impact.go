package prpack

import (
	"fmt"
	"sort"
	"strings"

	"repofalcon/internal/artifacts"
	"repofalcon/internal/graph"
)

// ComputeImpact deterministically computes impacted files/symbols/packages/findings.
//
// The fallback algorithm is pure in-memory and always available.
func ComputeImpact(tables SnapshotTables, changedPaths []string, opts ImpactOptions) (ImpactResult, error) {
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 2
	}

	// Canonicalize changed paths (deterministic).
	canonChanged := make([]string, 0, len(changedPaths))
	for _, p := range changedPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cp, err := graph.CanonRepoRelPath(p)
		if err != nil {
			return ImpactResult{}, fmt.Errorf("canonicalize changed path %q: %w", p, err)
		}
		canonChanged = append(canonChanged, cp)
	}
	sort.Strings(canonChanged)
	canonChanged = dedupeStrings(canonChanged)

	fileByPath := map[string]artifacts.FileRow{}
	filePathByID := map[string]string{}
	for _, fr := range tables.Files {
		p, err := graph.CanonRepoRelPath(fr.Path)
		if err != nil {
			return ImpactResult{}, fmt.Errorf("canonicalize snapshot file path %q: %w", fr.Path, err)
		}
		fr.Path = p
		fileByPath[p] = fr
		filePathByID[fr.FileID] = p
	}

	symByID := map[string]artifacts.SymbolRow{}
	symsByFile := map[string][]string{}
	for _, sr := range tables.Symbols {
		symByID[sr.SymbolID] = sr
		symsByFile[sr.FileID] = append(symsByFile[sr.FileID], sr.SymbolID)
	}
	for fid, ids := range symsByFile {
		sort.Strings(ids)
		symsByFile[fid] = dedupeStrings(ids)
	}

	pkgByID := map[string]artifacts.PackageRow{}
	for _, pr := range tables.Packages {
		pkgByID[pr.PackageID] = pr
	}

	// Build adjacency for selected edge types; treat as undirected for bounded BFS.
	allowed := map[string]bool{
		string(graph.EdgeDefines):  true,
		string(graph.EdgeInFile):   true,
		string(graph.EdgeImports):  true,
		string(graph.EdgeContains): true,
	}

	adj := map[string][]string{}
	addEdge := func(a, b string) {
		if a == "" || b == "" {
			return
		}
		adj[a] = append(adj[a], b)
	}

	// Track ABOUT edges to attach findings if present.
	aboutByFinding := map[string][]string{}
	aboutFindingIDs := map[string]bool{}
	for _, e := range tables.Edges {
		if e.EdgeType == string(graph.EdgeAbout) {
			if e.SrcType == string(graph.NodeTypeFinding) {
				aboutByFinding[e.SrcID] = append(aboutByFinding[e.SrcID], e.DstID)
				aboutFindingIDs[e.SrcID] = true
			}
			if e.DstType == string(graph.NodeTypeFinding) {
				aboutByFinding[e.DstID] = append(aboutByFinding[e.DstID], e.SrcID)
				aboutFindingIDs[e.DstID] = true
			}
			continue
		}
		if !allowed[e.EdgeType] {
			continue
		}
		addEdge(e.SrcID, e.DstID)
		addEdge(e.DstID, e.SrcID)
	}
	for k, vs := range adj {
		sort.Strings(vs)
		adj[k] = dedupeStrings(vs)
	}
	for k, vs := range aboutByFinding {
		sort.Strings(vs)
		aboutByFinding[k] = dedupeStrings(vs)
	}

	// Seeds are file node IDs for changed paths.
	seedFileIDToPath := map[string]string{}
	seeds := make([]string, 0, len(canonChanged))
	for _, p := range canonChanged {
		fr, ok := fileByPath[p]
		fid := ""
		if ok {
			fid = fr.FileID
		} else {
			fid = graph.NewFileID(p)
		}
		seedFileIDToPath[fid] = p
		seeds = append(seeds, fid)
	}
	sort.Strings(seeds)
	seeds = dedupeStrings(seeds)

	impFileIDs := map[string]bool{}
	impSymIDs := map[string]bool{}
	impPkgIDs := map[string]bool{}

	for _, fid := range seeds {
		impFileIDs[fid] = true
	}

	// Bounded BFS on node IDs.
	type qitem struct {
		id    string
		depth int
	}
	visited := map[string]bool{}
	q := make([]qitem, 0, len(seeds))
	for _, s := range seeds {
		visited[s] = true
		q = append(q, qitem{id: s, depth: 0})
	}
	for qi := 0; qi < len(q); qi++ {
		cur := q[qi]
		if cur.depth >= maxDepth {
			continue
		}
		for _, nb := range adj[cur.id] {
			if visited[nb] {
				continue
			}
			visited[nb] = true
			nDepth := cur.depth + 1
			q = append(q, qitem{id: nb, depth: nDepth})

			if _, ok := filePathByID[nb]; ok {
				impFileIDs[nb] = true
			}
			if sr, ok := symByID[nb]; ok {
				impSymIDs[nb] = true
				impFileIDs[sr.FileID] = true
				if sr.PackageID != nil {
					impPkgIDs[*sr.PackageID] = true
				}
			}
			if _, ok := pkgByID[nb]; ok {
				impPkgIDs[nb] = true
			}
		}
	}

	// Expand symbols for impacted files.
	impFileIDsList := mapKeys(impFileIDs)
	sort.Strings(impFileIDsList)
	for _, fid := range impFileIDsList {
		for _, sid := range symsByFile[fid] {
			impSymIDs[sid] = true
		}
	}

	// Ensure packages connected to impacted files (via IMPORTS/CONTAINS) are included.
	for _, e := range tables.Edges {
		if e.EdgeType != string(graph.EdgeImports) && e.EdgeType != string(graph.EdgeContains) {
			continue
		}
		if impFileIDs[e.SrcID] {
			if _, ok := pkgByID[e.DstID]; ok {
				impPkgIDs[e.DstID] = true
			}
		}
		if impFileIDs[e.DstID] {
			if _, ok := pkgByID[e.SrcID]; ok {
				impPkgIDs[e.SrcID] = true
			}
		}
	}

	// Materialize impacted files.
	impFiles := make([]ImpactedFile, 0, len(impFileIDs))
	for fid := range impFileIDs {
		path := filePathByID[fid]
		if path == "" {
			path = seedFileIDToPath[fid]
		}
		if path == "" {
			path = "unknown:" + fid
		}
		if fr, ok := fileByPath[path]; ok {
			lang := fr.Language
			ext := fr.Extension
			sz := fr.SizeBytes
			sha := fr.ContentSHA256
			isGen := fr.IsGenerated
			isTest := fr.IsTest
			impFiles = append(impFiles, ImpactedFile{
				Path:          path,
				FileID:        fr.FileID,
				InSnapshot:    true,
				Language:      &lang,
				Extension:     &ext,
				SizeBytes:     &sz,
				ContentSHA256: &sha,
				Lines:         fr.Lines,
				IsGenerated:   &isGen,
				IsTest:        &isTest,
			})
			continue
		}
		impFiles = append(impFiles, ImpactedFile{Path: path, FileID: fid, InSnapshot: false})
	}
	sort.SliceStable(impFiles, func(i, j int) bool {
		if impFiles[i].Path != impFiles[j].Path {
			return impFiles[i].Path < impFiles[j].Path
		}
		return impFiles[i].FileID < impFiles[j].FileID
	})

	// Materialize impacted symbols.
	impSyms := make([]ImpactedSymbol, 0, len(impSymIDs))
	for sid := range impSymIDs {
		sr, ok := symByID[sid]
		if !ok {
			continue
		}
		filePath := filePathByID[sr.FileID]
		if filePath == "" {
			filePath = seedFileIDToPath[sr.FileID]
		}
		if filePath == "" {
			filePath = "unknown:" + sr.FileID
		}
		impSyms = append(impSyms, ImpactedSymbol{
			SymbolID:      sr.SymbolID,
			QualifiedName: sr.QualifiedName,
			Kind:          sr.Kind,
			Language:      sr.Language,
			FileID:        sr.FileID,
			FilePath:      filePath,
			PackageID:     sr.PackageID,
		})
	}
	sort.SliceStable(impSyms, func(i, j int) bool {
		if impSyms[i].QualifiedName != impSyms[j].QualifiedName {
			return impSyms[i].QualifiedName < impSyms[j].QualifiedName
		}
		return impSyms[i].SymbolID < impSyms[j].SymbolID
	})

	// Materialize impacted packages.
	impPkgs := make([]ImpactedPackage, 0, len(impPkgIDs))
	for pid := range impPkgIDs {
		pr, ok := pkgByID[pid]
		if !ok {
			continue
		}
		impPkgs = append(impPkgs, ImpactedPackage{
			PackageID:    pr.PackageID,
			Ecosystem:    pr.Ecosystem,
			Scope:        pr.Scope,
			Name:         pr.Name,
			Version:      pr.Version,
			IsInternal:   pr.IsInternal,
			RootPath:     pr.RootPath,
			ManifestPath: pr.ManifestPath,
		})
	}
	sort.SliceStable(impPkgs, func(i, j int) bool {
		a, b := impPkgs[i], impPkgs[j]
		if a.Ecosystem != b.Ecosystem {
			return a.Ecosystem < b.Ecosystem
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Version != b.Version {
			return a.Version < b.Version
		}
		return a.PackageID < b.PackageID
	})

	// Attach findings.
	impFindings := make([]ImpactedFinding, 0)
	for _, fr := range tables.Findings {
		include := false
		if fr.FileID != nil && impFileIDs[*fr.FileID] {
			include = true
		}
		if !include && fr.SymbolID != nil && impSymIDs[*fr.SymbolID] {
			include = true
		}
		if !include && fr.PackageID != nil && impPkgIDs[*fr.PackageID] {
			include = true
		}
		if !include && aboutFindingIDs[fr.FindingID] {
			for _, about := range aboutByFinding[fr.FindingID] {
				if impFileIDs[about] || impSymIDs[about] || impPkgIDs[about] {
					include = true
					break
				}
			}
		}
		if !include {
			continue
		}

		var filePathPtr *string
		if fr.FileID != nil {
			if p := filePathByID[*fr.FileID]; p != "" {
				pp := p
				filePathPtr = &pp
			}
		}
		if filePathPtr == nil && fr.SymbolID != nil {
			if sr, ok := symByID[*fr.SymbolID]; ok {
				if p := filePathByID[sr.FileID]; p != "" {
					pp := p
					filePathPtr = &pp
				}
			}
		}

		impFindings = append(impFindings, ImpactedFinding{
			FindingID:          fr.FindingID,
			SourceTool:         fr.SourceTool,
			RuleID:             fr.RuleID,
			Severity:           fr.Severity,
			Message:            fr.Message,
			MessageFingerprint: fr.MessageFingerprint,
			FileID:             fr.FileID,
			FilePath:           filePathPtr,
			SymbolID:           fr.SymbolID,
			PackageID:          fr.PackageID,
			StartLine:          fr.StartLine,
			StartCol:           fr.StartCol,
			EndLine:            fr.EndLine,
			EndCol:             fr.EndCol,
			CWE:                fr.CWE,
			Tags:               fr.Tags,
			PropertiesJSON:     fr.PropertiesJSON,
		})
	}
	sort.SliceStable(impFindings, func(i, j int) bool {
		a, b := impFindings[i], impFindings[j]
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		if a.SourceTool != b.SourceTool {
			return a.SourceTool < b.SourceTool
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		ap := ""
		bp := ""
		if a.FilePath != nil {
			ap = *a.FilePath
		}
		if b.FilePath != nil {
			bp = *b.FilePath
		}
		if ap != bp {
			return ap < bp
		}
		asl := int32(0)
		bsl := int32(0)
		if a.StartLine != nil {
			asl = *a.StartLine
		}
		if b.StartLine != nil {
			bsl = *b.StartLine
		}
		if asl != bsl {
			return asl < bsl
		}
		return a.MessageFingerprint < b.MessageFingerprint
	})

	return ImpactResult{
		ChangedPaths:     canonChanged,
		ImpactedFiles:    impFiles,
		ImpactedSymbols:  impSyms,
		ImpactedPackages: impPkgs,
		AttachedFindings: impFindings,
	}, nil
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func dedupeStrings(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	out := make([]string, 0, len(in))
	prev := in[0]
	out = append(out, prev)
	for i := 1; i < len(in); i++ {
		if in[i] == prev {
			continue
		}
		prev = in[i]
		out = append(out, prev)
	}
	return out
}
