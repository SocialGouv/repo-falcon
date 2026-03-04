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
