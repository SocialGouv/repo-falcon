package extract

import (
	"context"
	"fmt"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

// JavaFile is the result of extracting package, imports and symbols from a Java file.
type JavaFile struct {
	PackageName string
	Imports     []string
	Symbols     []Symbol
}

// ExtractJavaFile parses a Java file with tree-sitter and extracts
// the package declaration, imports, and top-level symbol declarations.
func ExtractJavaFile(repoRelPath string, content []byte) (JavaFile, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return JavaFile{}, fmt.Errorf("parse %s: %w", repoRelPath, err)
	}

	root := tree.RootNode()
	var out JavaFile
	var symbols []Symbol

	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "package_declaration":
			out.PackageName = javaExtractPackageDecl(child, content)

		case "import_declaration":
			if imp := javaExtractImport(child, content); imp != "" {
				out.Imports = append(out.Imports, imp)
			}

		case "class_declaration":
			symbols = append(symbols, javaExtractTypeDecl(child, content, "class")...)

		case "interface_declaration":
			symbols = append(symbols, javaExtractTypeDecl(child, content, "interface")...)

		case "enum_declaration":
			symbols = append(symbols, javaExtractTypeDecl(child, content, "enum")...)

		case "record_declaration":
			symbols = append(symbols, javaExtractTypeDecl(child, content, "record")...)

		case "annotation_type_declaration":
			if name := nodeIdentifier(child, "name", content); name != "" {
				symbols = append(symbols, symbolFromNode(child, "annotation", name, "java"))
			}
		}
	}

	out.Imports = uniqSorted(out.Imports)

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

	out.Symbols = symbols
	return out, nil
}

// javaExtractPackageDecl extracts the package name from a package_declaration node.
func javaExtractPackageDecl(node *sitter.Node, content []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "scoped_identifier", "identifier":
			return javaScopedName(child, content)
		}
	}
	return ""
}

// javaExtractImport extracts the import path from an import_declaration node.
// Handles regular, static, and wildcard imports.
func javaExtractImport(node *sitter.Node, content []byte) string {
	var path string
	hasAsterisk := false

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "scoped_identifier", "identifier":
			path = javaScopedName(child, content)
		case "asterisk":
			hasAsterisk = true
		}
	}

	if path == "" {
		return ""
	}
	if hasAsterisk {
		return path + ".*"
	}
	return path
}

// javaExtractTypeDecl extracts symbols from a type declaration (class, interface, enum, record).
// It also descends one level into the body to extract methods and fields.
func javaExtractTypeDecl(node *sitter.Node, content []byte, kind string) []Symbol {
	typeName := nodeIdentifier(node, "name", content)
	if typeName == "" {
		return nil
	}

	var symbols []Symbol
	symbols = append(symbols, symbolFromNode(node, kind, typeName, "java"))

	// Descend into body for members.
	body := node.ChildByFieldName("body")
	if body == nil {
		return symbols
	}

	for i := 0; i < int(body.ChildCount()); i++ {
		member := body.Child(i)
		if member == nil {
			continue
		}

		switch member.Type() {
		case "method_declaration":
			if name := nodeIdentifier(member, "name", content); name != "" {
				sym := symbolFromNode(member, "method", name, "java")
				sym.QualifiedName = typeName + "." + name
				symbols = append(symbols, sym)
			}

		case "constructor_declaration":
			if name := nodeIdentifier(member, "name", content); name != "" {
				sym := symbolFromNode(member, "constructor", name, "java")
				sym.QualifiedName = typeName + "." + name
				symbols = append(symbols, sym)
			}

		case "field_declaration", "constant_declaration":
			symbols = append(symbols, javaExtractFieldDecl(member, content, typeName)...)
		}
	}

	return symbols
}

// javaExtractFieldDecl extracts field/constant names from a field_declaration or
// constant_declaration node.
func javaExtractFieldDecl(node *sitter.Node, content []byte, typeName string) []Symbol {
	var out []Symbol
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || child.Type() != "variable_declarator" {
			continue
		}
		name := nodeIdentifier(child, "name", content)
		if name == "" {
			continue
		}
		sym := symbolFromNode(child, "field", name, "java")
		sym.QualifiedName = typeName + "." + name
		out = append(out, sym)
	}
	return out
}

// javaScopedName recursively builds a dotted name from a scoped_identifier
// or identifier node.
func javaScopedName(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "identifier":
		return string(content[node.StartByte():node.EndByte()])
	case "scoped_identifier":
		scope := node.ChildByFieldName("scope")
		name := node.ChildByFieldName("name")
		scopeStr := javaScopedName(scope, content)
		nameStr := ""
		if name != nil {
			nameStr = string(content[name.StartByte():name.EndByte()])
		}
		if scopeStr != "" {
			return scopeStr + "." + nameStr
		}
		return nameStr
	default:
		// Fallback: use raw text.
		text := string(content[node.StartByte():node.EndByte()])
		return strings.TrimSpace(text)
	}
}
