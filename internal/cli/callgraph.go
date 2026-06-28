package cli

import (
	"fmt"
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
	byName      map[string][]symCandidate
	byQualified map[string][]symCandidate
	refs        []pendingRef
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
	recvType    string
	kind        string // "call" | "reference"
	lang        string
	callerPkg   string
	fileID      string
	line        int
}

func newCallGraphBuilder() *callGraphBuilder {
	return &callGraphBuilder{byName: map[string][]symCandidate{}, byQualified: map[string][]symCandidate{}}
}

// addSymbol registers a definition so later use-sites can resolve to it.
func (b *callGraphBuilder) addSymbol(name, qualified, symID, pkg, lang, kind string) {
	if name == "" {
		return
	}
	c := symCandidate{symID: symID, pkg: pkg, lang: lang, kind: kind}
	b.byName[name] = append(b.byName[name], c)
	if qualified != "" && qualified != name {
		b.byQualified[qualified] = append(b.byQualified[qualified], c)
	}
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
			recvType:    r.RecvType,
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

func typeKind(kind string) bool {
	switch kind {
	case "type", "class", "interface", "enum", "record", "struct":
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
		edgeType := graph.EdgeCalls
		if r.kind == "reference" {
			edgeType = graph.EdgeReferences
		}

		for _, res := range b.resolve(r) {
			edgeID := graph.NewEdgeID(r.callerSymID, res.target, edgeType, "")
			if _, dup := seen[edgeID]; dup {
				continue
			}
			seen[edgeID] = struct{}{}

			conf := res.conf
			fileID := r.fileID
			line := int32(r.line)
			props := fmt.Sprintf(`{"confidence_label":%q}`, res.label)
			out = append(out, artifacts.EdgeRow{
				EdgeID:         edgeID,
				EdgeType:       string(edgeType),
				SrcID:          r.callerSymID,
				DstID:          res.target,
				SrcType:        string(graph.NodeTypeSymbol),
				DstType:        string(graph.NodeTypeSymbol),
				SiteFileID:     &fileID,
				SiteStartLine:  &line,
				Confidence:     &conf,
				PropertiesJSON: &props,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].EdgeID < out[j].EdgeID })
	return out
}

// Confidence rubric (mirrors graphify's EXTRACTED / INFERRED / AMBIGUOUS), carried
// on each edge as Confidence + a confidence_label in PropertiesJSON:
//   - EXTRACTED 1.0  — unambiguous: receiver-typed Type.method, or a unique
//     same-package name match (essentially certain in a single package).
//   - INFERRED  0.9  — receiver-typed but resolved repo-wide (unique).
//   - INFERRED  0.75 — unique repo-wide name match (no same-package candidate).
//   - AMBIGUOUS 0.5  — a few plausible targets and no way to choose; emitted to
//     ALL of them (bounded fan-out) and flagged, rather than silently dropped.
const (
	labelExtracted     = "EXTRACTED"
	labelInferred      = "INFERRED"
	labelAmbiguous     = "AMBIGUOUS"
	confExtracted      = 1.0
	confInferredRecv   = 0.9
	confInferredGlobal = 0.75
	confAmbiguous      = 0.5
	ambiguousFanoutMax = 4 // above this a name is too ambiguous to be useful
)

type resolution struct {
	target string
	conf   float32
	label  string
}

// resolve turns one use-site into zero or more edges per the confidence rubric.
func (b *callGraphBuilder) resolve(r pendingRef) []resolution {
	// Receiver-typed method call: resolve directly to RecvType.Callee.
	if r.kind == "call" && r.recvType != "" {
		if res, ok := resolveUnique(b.byQualified[r.recvType+"."+r.callee], r.lang, r.callerSymID, r.callerPkg, confExtracted, confInferredRecv); ok {
			return []resolution{res}
		}
	}

	cands := b.byName[r.callee]
	if len(cands) == 0 {
		return nil
	}
	// Same-language, non-self; prefer the kind matching the relation.
	var preferred, any []symCandidate
	for _, c := range cands {
		if c.lang != r.lang || c.symID == r.callerSymID {
			continue
		}
		any = append(any, c)
		if (r.kind == "reference" && typeKind(c.kind)) || (r.kind == "call" && callableKind(c.kind)) {
			preferred = append(preferred, c)
		}
	}
	pool := preferred
	if len(pool) == 0 {
		pool = any
	}
	if len(pool) == 0 {
		return nil
	}

	if res, ok := resolveUnique(pool, r.lang, r.callerSymID, r.callerPkg, confExtracted, confInferredGlobal); ok {
		return []resolution{res}
	}

	// Ambiguous: a few candidates, no disambiguation. Emit them all, flagged,
	// when the fan-out is small enough to be useful; otherwise drop.
	amb := samePackage(pool, r.callerPkg)
	if len(amb) == 0 {
		amb = pool
	}
	if len(amb) < 2 || len(amb) > ambiguousFanoutMax {
		return nil
	}
	out := make([]resolution, 0, len(amb))
	for _, c := range amb {
		out = append(out, resolution{target: c.symID, conf: confAmbiguous, label: labelAmbiguous})
	}
	return out
}

// resolveUnique returns a single high-confidence resolution: a unique
// same-package match (samePkgConf), else a unique repo-wide match (globalConf).
func resolveUnique(cands []symCandidate, lang, callerSymID, callerPkg string, samePkgConf, globalConf float32) (resolution, bool) {
	var pool []symCandidate
	for _, c := range cands {
		if c.lang == lang && c.symID != callerSymID {
			pool = append(pool, c)
		}
	}
	if len(pool) == 0 {
		return resolution{}, false
	}
	if sp := samePackage(pool, callerPkg); len(sp) == 1 {
		return resolution{target: sp[0].symID, conf: samePkgConf, label: labelExtracted}, true
	} else if len(sp) == 0 && len(pool) == 1 {
		return resolution{target: pool[0].symID, conf: globalConf, label: labelInferred}, true
	}
	return resolution{}, false
}

func samePackage(pool []symCandidate, callerPkg string) []symCandidate {
	var out []symCandidate
	for _, c := range pool {
		if c.pkg == callerPkg {
			out = append(out, c)
		}
	}
	return out
}
