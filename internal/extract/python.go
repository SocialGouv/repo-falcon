package extract

import (
	"context"
	"fmt"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// PythonFile is the result of extracting imports and symbols from a Python file.
type PythonFile struct {
	Imports []string
	Symbols []Symbol
}

// ExtractPythonFile parses a Python file with tree-sitter and extracts
// imports and top-level symbol declarations.
func ExtractPythonFile(repoRelPath string, content []byte) (PythonFile, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return PythonFile{}, fmt.Errorf("parse %s: %w", repoRelPath, err)
	}

	root := tree.RootNode()

	var imports []string
	var symbols []Symbol

	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "import_statement":
			imports = append(imports, pyExtractImportStatement(child, content)...)

		case "import_from_statement":
			if mod := pyExtractFromModule(child, content); mod != "" {
				imports = append(imports, mod)
			}

		case "future_import_statement":
			imports = append(imports, "__future__")

		case "function_definition":
			if name := nodeIdentifier(child, "name", content); name != "" {
				symbols = append(symbols, symbolFromNode(child, "func", name, "python"))
			}

		case "class_definition":
			if name := nodeIdentifier(child, "name", content); name != "" {
				symbols = append(symbols, symbolFromNode(child, "class", name, "python"))
			}

		case "decorated_definition":
			defn := child.ChildByFieldName("definition")
			if defn == nil {
				continue
			}
			switch defn.Type() {
			case "function_definition":
				if name := nodeIdentifier(defn, "name", content); name != "" {
					// Use the decorated_definition span so decorators are included.
					sym := symbolFromNode(child, "func", name, "python")
					symbols = append(symbols, sym)
				}
			case "class_definition":
				if name := nodeIdentifier(defn, "name", content); name != "" {
					sym := symbolFromNode(child, "class", name, "python")
					symbols = append(symbols, sym)
				}
			}
		}
	}

	imports = uniqSorted(imports)

	sort.SliceStable(symbols, func(i, j int) bool {
		a, b := symbols[i], symbols[j]
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

	return PythonFile{Imports: imports, Symbols: symbols}, nil
}

// pyExtractImportStatement handles:
//
//	import os
//	import os.path
//	import a, b, c
//	import a.b as c
func pyExtractImportStatement(node *sitter.Node, content []byte) []string {
	var out []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "dotted_name":
			out = append(out, string(content[child.StartByte():child.EndByte()]))
		case "aliased_import":
			// import a.b as c → extract "a.b"
			name := child.ChildByFieldName("name")
			if name != nil {
				out = append(out, string(content[name.StartByte():name.EndByte()]))
			}
		}
	}
	return out
}

// pyExtractFromModule extracts the module name from an import_from_statement.
// Handles:
//
//	from os.path import join      → "os.path"
//	from . import util            → "."
//	from .util import double      → ".util"
//	from ...pkg.sub import thing  → "...pkg.sub"
func pyExtractFromModule(node *sitter.Node, content []byte) string {
	modNode := node.ChildByFieldName("module_name")
	if modNode == nil {
		return ""
	}

	switch modNode.Type() {
	case "dotted_name":
		return string(content[modNode.StartByte():modNode.EndByte()])

	case "relative_import":
		return pyExtractRelativeImport(modNode, content)

	default:
		// Fallback: use node text.
		return string(content[modNode.StartByte():modNode.EndByte()])
	}
}

// pyExtractRelativeImport builds a relative import string from a
// relative_import node. The node has an import_prefix child (the dots)
// and an optional dotted_name child.
func pyExtractRelativeImport(node *sitter.Node, content []byte) string {
	var prefix string
	var name string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "import_prefix":
			prefix = string(content[child.StartByte():child.EndByte()])
			prefix = strings.TrimSpace(prefix)
		case "dotted_name":
			name = string(content[child.StartByte():child.EndByte()])
		}
	}

	return prefix + name
}
