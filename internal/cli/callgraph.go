package cli

import (
	"sort"

	"repofalcon/internal/artifacts"
	"repofalcon/internal/extract"
	"repofalcon/internal/graph"
)

// callGraphBuilder accumulates symbol definitions and call/reference sites
// across all files, then resolves each use-site to a concrete target symbol by
// name, emitting CALLS / REFERENCES edges (symbol -> symbol).
//
// Resolution is name-based and deterministic, with a confidence score:
//   - a unique same-package match wins (confidence 1.0),
//   - else a unique repo-wide match wins (confidence 0.7),
//   - ambiguous names are dropped (precision over recall, to avoid graph noise).
//
// This mirrors how tree-sitter-class indexers (e.g. graphify) build a call
// graph without full type resolution: good enough to answer "who calls X" and
// "what does X call" while staying honest about confidence.
type callGraphBuilder struct {
	byName map[string][]symCandidate
	refs   []pendingRef
}

type symCandidate struct {
	symID string
	pkg   string
	lang  string
	kind  string
}

type pendingRef struct {
	callerSymID string
	callee      string
	kind        string // "call" | "reference"
	lang        string
	callerPkg   string
	fileID      string
	line        int
}

func newCallGraphBuilder() *callGraphBuilder {
	return &callGraphBuilder{byName: map[string][]symCandidate{}}
}

// addSymbol registers a definition so later use-sites can resolve to it.
func (b *callGraphBuilder) addSymbol(name, symID, pkg, lang, kind string) {
	if name == "" {
		return
	}
	b.byName[name] = append(b.byName[name], symCandidate{symID: symID, pkg: pkg, lang: lang, kind: kind})
}

// addRefs records a file's use-sites. qualToSymID maps an enclosing symbol's
// QualifiedName to its id so a site can be attributed to its caller symbol.
func (b *callGraphBuilder) addRefs(lang, callerPkg, fileID string, qualToSymID map[string]string, refs []extract.Reference) {
	for _, r := range refs {
		caller := qualToSymID[r.FromQualified]
		if caller == "" {
			continue // call sits outside any captured symbol; skip
		}
		b.refs = append(b.refs, pendingRef{
			callerSymID: caller,
			callee:      r.Callee,
			kind:        r.Kind,
			lang:        lang,
			callerPkg:   callerPkg,
			fileID:      fileID,
			line:        r.Line,
		})
	}
}

func callableKind(kind string) bool {
	switch kind {
	case "func", "method", "function", "class", "constructor":
		return true
	default:
		return false
	}
}

// resolveEdges turns recorded use-sites into deduped symbol->symbol edges.
func (b *callGraphBuilder) resolveEdges() []artifacts.EdgeRow {
	seen := map[string]struct{}{}
	var out []artifacts.EdgeRow

	for _, r := range b.refs {
		cands := b.byName[r.callee]
		if len(cands) == 0 {
			continue
		}

		// Filter to same-language, callable, non-self candidates.
		var callable, any []symCandidate
		for _, c := range cands {
			if c.lang != r.lang || c.symID == r.callerSymID {
				continue
			}
			any = append(any, c)
			if callableKind(c.kind) {
				callable = append(callable, c)
			}
		}
		pool := callable
		if len(pool) == 0 {
			pool = any
		}
		if len(pool) == 0 {
			continue
		}

		target, confidence, ok := pickTarget(pool, r.callerPkg)
		if !ok {
			continue
		}

		edgeType := graph.EdgeCalls
		if r.kind == "reference" {
			edgeType = graph.EdgeReferences
		}
		edgeID := graph.NewEdgeID(r.callerSymID, target, edgeType, "")
		if _, dup := seen[edgeID]; dup {
			continue
		}
		seen[edgeID] = struct{}{}

		conf := confidence
		fileID := r.fileID
		line := int32(r.line)
		out = append(out, artifacts.EdgeRow{
			EdgeID:        edgeID,
			EdgeType:      string(edgeType),
			SrcID:         r.callerSymID,
			DstID:         target,
			SrcType:       string(graph.NodeTypeSymbol),
			DstType:       string(graph.NodeTypeSymbol),
			SiteFileID:    &fileID,
			SiteStartLine: &line,
			Confidence:    &conf,
		})
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].EdgeID < out[j].EdgeID })
	return out
}

// pickTarget selects the best resolution: unique same-package (1.0), else unique
// repo-wide (0.7); ambiguous → no edge.
func pickTarget(pool []symCandidate, callerPkg string) (string, float32, bool) {
	var samePkg []symCandidate
	for _, c := range pool {
		if c.pkg == callerPkg {
			samePkg = append(samePkg, c)
		}
	}
	if len(samePkg) == 1 {
		return samePkg[0].symID, 1.0, true
	}
	if len(samePkg) == 0 && len(pool) == 1 {
		return pool[0].symID, 0.7, true
	}
	return "", 0, false
}
