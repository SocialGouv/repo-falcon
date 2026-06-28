package extract

import sitter "github.com/smacker/go-tree-sitter"

// callSite is a raw call/construction use-site discovered in a tree-sitter tree,
// before it is attributed to an enclosing symbol.
type callSite struct {
	callee string
	recv   string // receiver expression text: "this"/"self", a variable name, or ""
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

// collectRefs walks a tree-sitter tree and produces call/reference edges with
// receiver typing: a method call `x.m()` is tagged with the inferred type of
// `x` (RecvType) so the index resolver can pin it to `Type.m` instead of
// guessing by bare method name. `this`/`self` resolve to the enclosing class.
func collectRefs(root *sitter.Node, content []byte, lang tsLangKind, symbols []Symbol) []Reference {
	sites := collectCallSites(root, content, lang)
	varTypes := buildVarTypes(root, content, lang)

	var refs []Reference
	for _, s := range sites {
		enc, ok := innermostEnclosing(symbols, s.line, s.col)
		if !ok {
			continue
		}
		recvType := ""
		switch s.recv {
		case "":
			// no receiver
		case "this", "self":
			if cls, ok := innermostEnclosingClass(symbols, s.line, s.col); ok {
				recvType = cls
			}
		default:
			recvType = varTypes[s.recv]
		}
		refs = append(refs, Reference{
			FromQualified: enc.QualifiedName,
			Callee:        s.callee,
			RecvType:      recvType,
			Kind:          "call",
			Line:          s.line,
			Col:           s.col,
		})
	}
	return refs
}

// collectCallSites walks the whole tree and records every call/construction
// site with its callee simple name and receiver expression.
func collectCallSites(root *sitter.Node, content []byte, lang tsLangKind) []callSite {
	var sites []callSite
	walkTree(root, func(n *sitter.Node) {
		name, recv, ok := calleeAndRecv(n, content, lang)
		if !ok || name == "" {
			return
		}
		sp := n.StartPoint()
		sites = append(sites, callSite{callee: name, recv: recv, line: int(sp.Row) + 1, col: int(sp.Column) + 1})
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

// calleeAndRecv returns the simple name being called/constructed at node n and
// its receiver expression text, if n is a call site for the given language.
func calleeAndRecv(n *sitter.Node, content []byte, lang tsLangKind) (string, string, bool) {
	switch lang {
	case tsJS:
		switch n.Type() {
		case "call_expression":
			name, recv := exprLeafAndRecv(n.ChildByFieldName("function"), content, "property", "object")
			return name, recv, true
		case "new_expression":
			if c := n.ChildByFieldName("constructor"); c != nil {
				return exprLeafName(c, content, "property"), "", true
			}
		}
		return "", "", false
	case tsPython:
		if n.Type() != "call" {
			return "", "", false
		}
		name, recv := exprLeafAndRecv(n.ChildByFieldName("function"), content, "attribute", "object")
		return name, recv, true
	case tsJava:
		switch n.Type() {
		case "method_invocation":
			name := nodeText(n.ChildByFieldName("name"), content)
			recv := ""
			if obj := n.ChildByFieldName("object"); obj != nil {
				recv = nodeText(obj, content)
			}
			return name, recv, true
		case "object_creation_expression":
			if t := n.ChildByFieldName("type"); t != nil {
				return simpleTypeName(t, content), "", true
			}
		}
		return "", "", false
	}
	return "", "", false
}

// exprLeafAndRecv returns (calleeName, receiver) for a member/attribute callee:
// `foo` -> (foo, ""), `a.foo` -> (foo, "a"), `this.foo` -> (foo, "this").
func exprLeafAndRecv(fn *sitter.Node, content []byte, memberField, objectField string) (string, string) {
	if fn == nil {
		return "", ""
	}
	switch fn.Type() {
	case "identifier":
		return nodeText(fn, content), ""
	case "member_expression", "attribute":
		name := ""
		if p := fn.ChildByFieldName(memberField); p != nil {
			name = nodeText(p, content)
		}
		recv := ""
		if o := fn.ChildByFieldName(objectField); o != nil {
			switch o.Type() {
			case "identifier", "this":
				recv = nodeText(o, content)
			}
		}
		return name, recv
	}
	return "", ""
}

// exprLeafName returns just the trailing simple name of a callee expression.
func exprLeafName(fn *sitter.Node, content []byte, memberField string) string {
	name, _ := exprLeafAndRecv(fn, content, memberField, "object")
	return name
}

// buildVarTypes infers a file-level map of variable name -> simple type name
// from typed parameters, local declarations and fields. It is best-effort: name
// collisions across scopes are rare and the resolver's same-package preference
// plus qualified matching keep the result precise.
func buildVarTypes(root *sitter.Node, content []byte, lang tsLangKind) map[string]string {
	vt := map[string]string{}
	walkTree(root, func(n *sitter.Node) {
		switch lang {
		case tsJava:
			switch n.Type() {
			case "formal_parameter", "field_declaration", "local_variable_declaration":
				t := n.ChildByFieldName("type")
				if t == nil {
					return
				}
				tn := simpleTypeName(t, content)
				if tn == "" {
					return
				}
				for _, name := range declaredNames(n, content) {
					vt[name] = tn
				}
			}
		case tsJS:
			switch n.Type() {
			case "required_parameter", "optional_parameter":
				if name, tn := tsParamType(n, content); name != "" && tn != "" {
					vt[name] = tn
				}
			case "variable_declarator", "public_field_definition":
				name := nodeText(n.ChildByFieldName("name"), content)
				if name == "" {
					return
				}
				if t := n.ChildByFieldName("type"); t != nil {
					if tn := tsTypeLeaf(typeAnnotationInner(t), content); tn != "" {
						vt[name] = tn
					}
				}
				if v := n.ChildByFieldName("value"); v != nil && v.Type() == "new_expression" {
					if tn := exprLeafName(v.ChildByFieldName("constructor"), content, "property"); tn != "" {
						vt[name] = tn
					}
				}
			}
		case tsPython:
			switch n.Type() {
			case "typed_parameter":
				name := ""
				if n.ChildCount() > 0 {
					name = nodeText(n.Child(0), content)
				}
				if t := n.ChildByFieldName("type"); t != nil && name != "" {
					vt[name] = pyTypeLeaf(t, content)
				}
			case "assignment":
				left := n.ChildByFieldName("left")
				if left == nil || left.Type() != "identifier" {
					return
				}
				name := nodeText(left, content)
				if t := n.ChildByFieldName("type"); t != nil { // x: T = ...
					vt[name] = pyTypeLeaf(t, content)
				} else if r := n.ChildByFieldName("right"); r != nil && r.Type() == "call" {
					// x = T(...) where T looks like a class (Capitalized).
					if cn := exprLeafName(r.ChildByFieldName("function"), content, "attribute"); cn != "" && isCapitalized(cn) {
						vt[name] = cn
					}
				}
			}
		}
	})
	return vt
}

// declaredNames returns the variable/parameter names declared by a Java
// parameter/field/local node.
func declaredNames(n *sitter.Node, content []byte) []string {
	if name := n.ChildByFieldName("name"); name != nil {
		return []string{nodeText(name, content)}
	}
	var out []string
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		if c != nil && c.Type() == "variable_declarator" {
			if name := c.ChildByFieldName("name"); name != nil {
				out = append(out, nodeText(name, content))
			}
		}
	}
	return out
}

func tsParamType(n *sitter.Node, content []byte) (string, string) {
	pat := n.ChildByFieldName("pattern")
	name := ""
	if pat != nil && pat.Type() == "identifier" {
		name = nodeText(pat, content)
	}
	tn := ""
	if t := n.ChildByFieldName("type"); t != nil {
		tn = tsTypeLeaf(typeAnnotationInner(t), content)
	}
	return name, tn
}

// typeAnnotationInner unwraps a `: T` type_annotation to its inner type node.
func typeAnnotationInner(t *sitter.Node) *sitter.Node {
	if t == nil {
		return nil
	}
	if t.Type() == "type_annotation" {
		for i := 0; i < int(t.ChildCount()); i++ {
			c := t.Child(i)
			if c != nil && c.Type() != ":" {
				return c
			}
		}
		return nil
	}
	return t
}

func tsTypeLeaf(t *sitter.Node, content []byte) string {
	if t == nil {
		return ""
	}
	switch t.Type() {
	case "type_identifier", "identifier", "predefined_type":
		return nodeText(t, content)
	case "generic_type":
		if t.ChildCount() > 0 {
			return tsTypeLeaf(t.Child(0), content)
		}
	case "nested_type_identifier":
		if name := t.ChildByFieldName("name"); name != nil {
			return nodeText(name, content)
		}
	}
	return ""
}

func pyTypeLeaf(t *sitter.Node, content []byte) string {
	switch t.Type() {
	case "identifier":
		return nodeText(t, content)
	case "subscript": // List[T], Optional[T] -> base
		if v := t.ChildByFieldName("value"); v != nil {
			return nodeText(v, content)
		}
	case "attribute":
		if a := t.ChildByFieldName("attribute"); a != nil {
			return nodeText(a, content)
		}
	}
	return ""
}

func isCapitalized(s string) bool {
	return s != "" && s[0] >= 'A' && s[0] <= 'Z'
}

// simpleTypeName returns the trailing identifier of a (generic/qualified) Java
// type node, e.g. java.util.List<X> -> List.
func simpleTypeName(t *sitter.Node, content []byte) string {
	switch t.Type() {
	case "type_identifier", "identifier":
		return nodeText(t, content)
	case "scoped_type_identifier":
		if name := t.ChildByFieldName("name"); name != nil {
			return nodeText(name, content)
		}
	case "generic_type":
		if t.ChildCount() > 0 {
			return simpleTypeName(t.Child(0), content)
		}
	case "array_type":
		if el := t.ChildByFieldName("element"); el != nil {
			return simpleTypeName(el, content)
		}
	}
	return nodeText(t, content)
}

func nodeText(n *sitter.Node, content []byte) string {
	if n == nil {
		return ""
	}
	return string(content[n.StartByte():n.EndByte()])
}

// innermostEnclosing returns the smallest-span symbol whose range contains
// (line, col).
func innermostEnclosing(symbols []Symbol, line, col int) (Symbol, bool) {
	return innermostMatching(symbols, line, col, func(Symbol) bool { return true })
}

// innermostEnclosingClass returns the smallest class/type symbol containing the
// position — used to type `this`/`self`.
func innermostEnclosingClass(symbols []Symbol, line, col int) (string, bool) {
	s, ok := innermostMatching(symbols, line, col, func(s Symbol) bool {
		switch s.Kind {
		case "class", "interface", "enum", "record", "struct", "type":
			return true
		}
		return false
	})
	if !ok {
		return "", false
	}
	return s.Name, true
}

func innermostMatching(symbols []Symbol, line, col int, pred func(Symbol) bool) (Symbol, bool) {
	best := -1
	bestSpan := 1 << 62
	for i, s := range symbols {
		if !pred(s) || !posInRange(line, col, s) {
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
