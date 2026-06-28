package extract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
)

type GoFile struct {
	PackageName string
	Imports     []string
	Symbols     []Symbol
	References  []Reference
}

func ExtractGoFile(repoRelPath string, src []byte) (GoFile, error) {
	fset := token.NewFileSet()
	// parser.SkipObjectResolution keeps it fast and deterministic.
	f, err := parser.ParseFile(fset, repoRelPath, src, parser.SkipObjectResolution)
	if err != nil {
		return GoFile{}, err
	}

	out := GoFile{PackageName: f.Name.Name}

	// imports
	seen := make(map[string]struct{}, len(f.Imports))
	for _, is := range f.Imports {
		p, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out.Imports = append(out.Imports, p)
	}
	sort.Strings(out.Imports)

	// top-level symbols
	for _, d := range f.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			kind := "func"
			q := dd.Name.Name
			if dd.Recv != nil && len(dd.Recv.List) > 0 {
				kind = "method"
				q = recvTypeName(dd.Recv.List[0].Type) + "." + dd.Name.Name
			}
			out.Symbols = append(out.Symbols, symFromPositions(fset, dd.Name.Pos(), dd.End(), kind, dd.Name.Name, q))
			out.References = append(out.References, goFuncRefs(fset, q, dd)...)
		case *ast.GenDecl:
			switch dd.Tok {
			case token.TYPE:
				for _, sp := range dd.Specs {
					ts, ok := sp.(*ast.TypeSpec)
					if !ok {
						continue
					}
					out.Symbols = append(out.Symbols, symFromPositions(fset, ts.Name.Pos(), ts.End(), "type", ts.Name.Name, ts.Name.Name))
				}
			case token.VAR:
				for _, sp := range dd.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, n := range vs.Names {
						out.Symbols = append(out.Symbols, symFromPositions(fset, n.Pos(), vs.End(), "var", n.Name, n.Name))
					}
				}
			case token.CONST:
				for _, sp := range dd.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, n := range vs.Names {
						out.Symbols = append(out.Symbols, symFromPositions(fset, n.Pos(), vs.End(), "const", n.Name, n.Name))
					}
				}
			}
		}
	}

	sort.SliceStable(out.Symbols, func(i, j int) bool {
		a, b := out.Symbols[i], out.Symbols[j]
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

	for i := range out.Symbols {
		out.Symbols[i].Language = "go"
	}

	return out, nil
}

func symFromPositions(fset *token.FileSet, startPos, endPos token.Pos, kind, name, qualified string) Symbol {
	sp := fset.PositionFor(startPos, true)
	ep := fset.PositionFor(endPos, true)
	return Symbol{
		Kind:          kind,
		Name:          name,
		QualifiedName: qualified,
		StartLine:     sp.Line,
		StartCol:      sp.Column,
		EndLine:       ep.Line,
		EndCol:        ep.Column,
	}
}

// goFuncRefs records, for one function/method, both its call sites (CALLS) and
// its named-type uses (REFERENCES), attributed to the enclosing symbol
// (fromQualified).
//
// Method calls get receiver typing: a lightweight intra-function pass infers the
// type of locals (receiver, params, `x := T{}` / `&T{}` / `new(T)`, `var x T`)
// so `x.M()` resolves to `T.M` instead of guessing by bare method name. This
// turns many otherwise-ambiguous (and therefore dropped) method calls into
// precise edges.
func goFuncRefs(fset *token.FileSet, fromQualified string, fn *ast.FuncDecl) []Reference {
	varType := map[string]string{}
	typeRefs := map[string]token.Pos{} // type name -> first-seen position

	// Receiver, params, results contribute both var types and type references.
	for _, fl := range fieldLists(fn) {
		if fl == nil {
			continue
		}
		for _, f := range fl.List {
			tn := goTypeLeaf(f.Type)
			if tn != "" {
				if _, seen := typeRefs[tn]; !seen {
					typeRefs[tn] = f.Type.Pos()
				}
			}
			for _, nm := range f.Names {
				if tn != "" {
					varType[nm.Name] = tn
				}
			}
		}
	}

	if fn.Body == nil {
		return goEmitRefs(fset, fromQualified, nil, typeRefs)
	}

	// Pass 1: infer local variable types from declarations.
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if s.Tok == token.DEFINE {
				for i, lhs := range s.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok || i >= len(s.Rhs) {
						continue
					}
					if tn := goExprType(s.Rhs[i]); tn != "" {
						varType[id.Name] = tn
					}
				}
			}
		case *ast.DeclStmt:
			gd, ok := s.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, sp := range gd.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok || vs.Type == nil {
					continue
				}
				tn := goTypeLeaf(vs.Type)
				for _, nm := range vs.Names {
					if tn != "" {
						varType[nm.Name] = tn
					}
				}
			}
		}
		return true
	})

	// Pass 2: collect call sites and composite-literal type references.
	var calls []Reference
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.CallExpr:
			name, qualifier, recv := "", "", ""
			switch f := e.Fun.(type) {
			case *ast.Ident:
				name = f.Name
			case *ast.SelectorExpr:
				name = f.Sel.Name
				if x, ok := f.X.(*ast.Ident); ok {
					qualifier = x.Name
					if t, known := varType[x.Name]; known {
						recv = t
					}
				}
			}
			if name != "" {
				pos := fset.PositionFor(e.Pos(), true)
				calls = append(calls, Reference{
					FromQualified: fromQualified,
					Callee:        name,
					Qualifier:     qualifier,
					RecvType:      recv,
					Kind:          "call",
					Line:          pos.Line,
					Col:           pos.Column,
				})
			}
		case *ast.CompositeLit:
			if tn := goTypeLeaf(e.Type); tn != "" {
				if _, seen := typeRefs[tn]; !seen {
					typeRefs[tn] = e.Pos()
				}
			}
		}
		return true
	})

	return goEmitRefs(fset, fromQualified, calls, typeRefs)
}

func goEmitRefs(fset *token.FileSet, fromQualified string, calls []Reference, typeRefs map[string]token.Pos) []Reference {
	refs := calls
	for tn, pos := range typeRefs {
		if isGoBuiltinType(tn) {
			continue
		}
		p := fset.PositionFor(pos, true)
		refs = append(refs, Reference{
			FromQualified: fromQualified,
			Callee:        tn,
			Kind:          "reference",
			Line:          p.Line,
			Col:           p.Column,
		})
	}
	return refs
}

func fieldLists(fn *ast.FuncDecl) []*ast.FieldList {
	var out []*ast.FieldList
	out = append(out, fn.Recv)
	if fn.Type != nil {
		out = append(out, fn.Type.Params, fn.Type.Results)
	}
	return out
}

// goExprType infers a simple type name from an initializer expression, for the
// common forms `T{}`, `&T{}`, `new(T)`.
func goExprType(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.CompositeLit:
		return goTypeLeaf(x.Type)
	case *ast.UnaryExpr:
		if cl, ok := x.X.(*ast.CompositeLit); ok {
			return goTypeLeaf(cl.Type)
		}
	case *ast.CallExpr:
		if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "new" && len(x.Args) == 1 {
			return goTypeLeaf(x.Args[0])
		}
	}
	return ""
}

// goTypeLeaf returns the trailing type name of a (pointer/array/slice/map/
// generic/qualified) type expression — best-effort, stable for common cases.
func goTypeLeaf(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return goTypeLeaf(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name // pkg.Type -> Type
	case *ast.ArrayType:
		return goTypeLeaf(t.Elt)
	case *ast.IndexExpr:
		return goTypeLeaf(t.X)
	case *ast.IndexListExpr:
		return goTypeLeaf(t.X)
	}
	return ""
}

func isGoBuiltinType(name string) bool {
	switch name {
	case "string", "bool", "byte", "rune", "error", "any",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128":
		return true
	}
	return false
}

func recvTypeName(expr ast.Expr) string {
	// Best-effort receiver type name, stable for common cases.
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	case *ast.SelectorExpr:
		// pkg.Type
		return recvTypeName(t.X) + "." + t.Sel.Name
	default:
		return "<recv>"
	}
}
