package extract

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// JSFile is the result of extracting imports and symbols from a JS/TS file.
type JSFile struct {
	Imports []string
	Symbols []Symbol
}

// ExtractJSFile parses a JS/TS/JSX/TSX file with tree-sitter and extracts
// imports and top-level symbol declarations.
func ExtractJSFile(repoRelPath string, content []byte, langTag string) (JSFile, error) {
	parser := sitter.NewParser()

	lang, err := jsLanguage(repoRelPath, langTag)
	if err != nil {
		return JSFile{}, err
	}
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return JSFile{}, fmt.Errorf("parse %s: %w", repoRelPath, err)
	}

	root := tree.RootNode()

	var imports []string
	var symbols []Symbol

	// Walk top-level statements.
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child == nil {
			continue
		}
		kind := child.Type()

		switch kind {
		case "import_statement":
			if src := nodeStringField(child, "source", content); src != "" {
				imports = append(imports, src)
			}

		case "export_statement":
			// Re-export: export { ... } from '...' or export * from '...'
			if src := nodeStringField(child, "source", content); src != "" {
				imports = append(imports, src)
			}
			// Exported declaration: export function foo() {}
			if decl := child.ChildByFieldName("declaration"); decl != nil {
				symbols = append(symbols, extractDecl(decl, content, langTag)...)
			}

		case "function_declaration", "generator_function_declaration":
			symbols = append(symbols, extractDecl(child, content, langTag)...)

		case "class_declaration":
			symbols = append(symbols, extractDecl(child, content, langTag)...)

		case "lexical_declaration", "variable_declaration":
			symbols = append(symbols, extractDecl(child, content, langTag)...)
		}
	}

	// Recursive scan for require() calls since CommonJS require can appear
	// anywhere (top-level, inside functions, conditionally, etc.).
	collectRequires(root, content, &imports)

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

	return JSFile{Imports: imports, Symbols: symbols}, nil
}

// jsLanguage returns the tree-sitter language for the given file.
func jsLanguage(repoRelPath, langTag string) (*sitter.Language, error) {
	switch langTag {
	case "js":
		// The JavaScript grammar handles JSX syntax as well.
		return javascript.GetLanguage(), nil
	case "ts":
		ext := strings.ToLower(filepath.Ext(repoRelPath))
		if ext == ".tsx" {
			return tsx.GetLanguage(), nil
		}
		return typescript.GetLanguage(), nil
	default:
		return nil, fmt.Errorf("unsupported JS/TS language tag: %q", langTag)
	}
}

// nodeStringField extracts the text content of a string-valued field on a node.
// For example, import_statement has a "source" field that is a string node.
func nodeStringField(node *sitter.Node, fieldName string, content []byte) string {
	field := node.ChildByFieldName(fieldName)
	if field == nil {
		return ""
	}
	return extractStringContent(field, content)
}

// extractStringContent extracts the text from a string node.
// It looks for a string_fragment child which contains the actual text
// (without quotes).
func extractStringContent(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "string_fragment" {
			return string(content[child.StartByte():child.EndByte()])
		}
	}
	// Fallback: strip quotes from the node text.
	text := string(content[node.StartByte():node.EndByte()])
	text = strings.Trim(text, "\"'`")
	return text
}

// extractDecl extracts symbols from a declaration node.
func extractDecl(node *sitter.Node, content []byte, langTag string) []Symbol {
	kind := node.Type()
	switch kind {
	case "function_declaration", "generator_function_declaration":
		name := nodeIdentifier(node, "name", content)
		if name == "" {
			return nil
		}
		return []Symbol{symbolFromNode(node, "function", name, langTag)}

	case "class_declaration":
		name := nodeIdentifier(node, "name", content)
		if name == "" {
			return nil
		}
		return []Symbol{symbolFromNode(node, "class", name, langTag)}

	case "lexical_declaration":
		return extractVariableDeclarators(node, content, langTag)

	case "variable_declaration":
		return extractVariableDeclarators(node, content, langTag)

	default:
		return nil
	}
}

// extractVariableDeclarators extracts symbols from variable_declarator children.
func extractVariableDeclarators(node *sitter.Node, content []byte, langTag string) []Symbol {
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
		// Determine kind from the parent declaration keyword.
		declKind := "var"
		if node.Type() == "lexical_declaration" {
			firstText := string(content[node.StartByte():node.EndByte()])
			if strings.HasPrefix(strings.TrimSpace(firstText), "const") {
				declKind = "const"
			} else {
				declKind = "let"
			}
		}
		out = append(out, symbolFromNode(child, declKind, name, langTag))
	}
	return out
}

// nodeIdentifier extracts the text of an identifier field on a node.
func nodeIdentifier(node *sitter.Node, fieldName string, content []byte) string {
	id := node.ChildByFieldName(fieldName)
	if id == nil {
		return ""
	}
	return string(content[id.StartByte():id.EndByte()])
}

// symbolFromNode creates a Symbol from a tree-sitter node position.
func symbolFromNode(node *sitter.Node, kind, name, langTag string) Symbol {
	sp := node.StartPoint()
	ep := node.EndPoint()
	return Symbol{
		Language:      langTag,
		Kind:          kind,
		Name:          name,
		QualifiedName: name,
		StartLine:     int(sp.Row) + 1, // tree-sitter is 0-indexed, we use 1-indexed
		StartCol:      int(sp.Column) + 1,
		EndLine:       int(ep.Row) + 1,
		EndCol:        int(ep.Column) + 1,
	}
}

// collectRequires recursively walks a subtree and collects require() call
// targets. It appends to the imports slice.
func collectRequires(node *sitter.Node, content []byte, imports *[]string) {
	if node == nil {
		return
	}
	if node.Type() == "call_expression" {
		fn := node.ChildByFieldName("function")
		if fn != nil && fn.Type() == "identifier" {
			fnName := string(content[fn.StartByte():fn.EndByte()])
			if fnName == "require" {
				args := node.ChildByFieldName("arguments")
				if args != nil {
					for i := 0; i < int(args.ChildCount()); i++ {
						arg := args.Child(i)
						if arg != nil && arg.Type() == "string" {
							src := extractStringContent(arg, content)
							if src != "" {
								*imports = append(*imports, src)
							}
							break
						}
					}
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		collectRequires(node.Child(i), content, imports)
	}
}
