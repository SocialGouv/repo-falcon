package extract

import sitter "github.com/smacker/go-tree-sitter"

// callSite is a raw call/construction use-site discovered in a tree-sitter tree,
// before it is attributed to an enclosing symbol.
type callSite struct {
	callee string
	line   int
	col    int
}

// tsLangKind selects the grammar-specific call-node shapes.
type tsLangKind int

const (
	tsJS tsLangKind = iota
	tsPython
	tsJava
)

// collectCallSites walks the whole tree and records every call/construction
// site with its callee simple name.
func collectCallSites(root *sitter.Node, content []byte, lang tsLangKind) []callSite {
	var sites []callSite
	walkTree(root, func(n *sitter.Node) {
		name, ok := calleeName(n, content, lang)
		if !ok || name == "" {
			return
		}
		sp := n.StartPoint()
		sites = append(sites, callSite{callee: name, line: int(sp.Row) + 1, col: int(sp.Column) + 1})
	})
	return sites
}

func walkTree(n *sitter.Node, fn func(*sitter.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for i := 0; i < int(n.ChildCount()); i++ {
		walkTree(n.Child(i), fn)
	}
}

// calleeName returns the simple name being called/constructed at node n, if n is
// a call site for the given language.
func calleeName(n *sitter.Node, content []byte, lang tsLangKind) (string, bool) {
	switch lang {
	case tsJS:
		if n.Type() != "call_expression" {
			return "", false
		}
		return exprLeafName(n.ChildByFieldName("function"), content, "property"), true
	case tsPython:
		if n.Type() != "call" {
			return "", false
		}
		return exprLeafName(n.ChildByFieldName("function"), content, "attribute"), true
	case tsJava:
		switch n.Type() {
		case "method_invocation":
			if name := n.ChildByFieldName("name"); name != nil {
				return nodeText(name, content), true
			}
		case "object_creation_expression":
			if t := n.ChildByFieldName("type"); t != nil {
				return simpleTypeName(t, content), true
			}
		}
		return "", false
	}
	return "", false
}

// exprLeafName extracts the rightmost simple name from a callee expression:
// `foo` -> foo, `a.b.foo` -> foo (member/attribute access). memberField is the
// grammar's field name for the trailing property ("property" in JS,
// "attribute" in Python).
func exprLeafName(fn *sitter.Node, content []byte, memberField string) string {
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		return nodeText(fn, content)
	case "member_expression", "attribute":
		if p := fn.ChildByFieldName(memberField); p != nil {
			return nodeText(p, content)
		}
	}
	return ""
}

// simpleTypeName returns the trailing identifier of a (possibly generic or
// qualified) Java type node, e.g. java.util.List<X> -> List.
func simpleTypeName(t *sitter.Node, content []byte) string {
	switch t.Type() {
	case "type_identifier", "identifier":
		return nodeText(t, content)
	case "scoped_type_identifier":
		if name := t.ChildByFieldName("name"); name != nil {
			return nodeText(name, content)
		}
	case "generic_type":
		// First child is the base type.
		if t.ChildCount() > 0 {
			return simpleTypeName(t.Child(0), content)
		}
	}
	// Fallback: last identifier-ish token.
	return nodeText(t, content)
}

func nodeText(n *sitter.Node, content []byte) string {
	if n == nil {
		return ""
	}
	return string(content[n.StartByte():n.EndByte()])
}

// attributeRefs maps each call site to the innermost enclosing symbol (by source
// range), producing call References. Sites not inside any captured symbol are
// dropped.
func attributeRefs(symbols []Symbol, sites []callSite) []Reference {
	if len(symbols) == 0 || len(sites) == 0 {
		return nil
	}
	var refs []Reference
	for _, s := range sites {
		enc, ok := innermostEnclosing(symbols, s.line, s.col)
		if !ok {
			continue
		}
		refs = append(refs, Reference{
			FromQualified: enc.QualifiedName,
			Callee:        s.callee,
			Kind:          "call",
			Line:          s.line,
			Col:           s.col,
		})
	}
	return refs
}

// innermostEnclosing returns the smallest-span symbol whose range contains
// (line, col).
func innermostEnclosing(symbols []Symbol, line, col int) (Symbol, bool) {
	best := -1
	bestSpan := 1 << 62
	for i, s := range symbols {
		if !posInRange(line, col, s) {
			continue
		}
		span := (s.EndLine-s.StartLine)*100000 + (s.EndCol - s.StartCol)
		if span < bestSpan {
			bestSpan = span
			best = i
		}
	}
	if best < 0 {
		return Symbol{}, false
	}
	return symbols[best], true
}

func posInRange(line, col int, s Symbol) bool {
	if line < s.StartLine || line > s.EndLine {
		return false
	}
	if line == s.StartLine && col < s.StartCol {
		return false
	}
	if line == s.EndLine && col > s.EndCol {
		return false
	}
	return true
}
