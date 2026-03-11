package integration_test

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"repofalcon/internal/artifacts"
)

// TestExtraction_Python verifies that the Python tree-sitter extractor
// produces correct packages, symbols, edges, and internal/external classification.
func TestExtraction_Python(t *testing.T) {
	bin := falconBin(t)
	fixtureDir := filepath.Join(repoRootDir(t), "testdata", "tinyrepo_go_py")
	outDir := filepath.Join(t.TempDir(), "artifacts")

	runFalcon(t, bin, "index", "--repo", fixtureDir, "--out", outDir)

	ctx := context.Background()
	tables := readAllTables(t, ctx, outDir)

	// ---- Files ----
	pyFiles := filterFiles(tables.Files, "python")
	if len(pyFiles) < 4 {
		t.Fatalf("expected >= 4 Python files, got %d", len(pyFiles))
	}
	assertFileExists(t, pyFiles, "py/app.py")
	assertFileExists(t, pyFiles, "py/util.py")
	assertFileExists(t, pyFiles, "py/__init__.py")
	assertFileExists(t, pyFiles, "py/models/user.py")
	assertFileExists(t, pyFiles, "py/models/__init__.py")

	// ---- Packages ----
	pyPkgs := filterPackages(tables.Packages, "python")
	if len(pyPkgs) == 0 {
		t.Fatal("expected Python packages, got 0")
	}

	// Internal packages: directory-based packages for the py/ tree.
	assertPackageInternal(t, pyPkgs, "py", true)
	assertPackageInternal(t, pyPkgs, "py/models", true)

	// Relative imports must be internal.
	assertPackageInternal(t, pyPkgs, ".util", true)
	assertPackageInternal(t, pyPkgs, ".user", true)

	// External (stdlib) imports must NOT be internal.
	assertPackageInternal(t, pyPkgs, "json", false)
	assertPackageInternal(t, pyPkgs, "math", false)
	assertPackageInternal(t, pyPkgs, "dataclasses", false)

	// ---- Symbols ----
	pySyms := filterSymbols(tables.Symbols, "python")
	if len(pySyms) == 0 {
		t.Fatal("expected Python symbols, got 0")
	}

	// Functions.
	assertSymbolExists(t, pySyms, "func", "handler")
	assertSymbolExists(t, pySyms, "func", "double")
	assertSymbolExists(t, pySyms, "func", "create_user")

	// Classes (decorated or not).
	assertSymbolExists(t, pySyms, "class", "User")
	assertSymbolExists(t, pySyms, "class", "AppConfig")

	// ---- Edges ----
	assertEdgeCount(t, tables.Edges, "CONTAINS", "Package", "File", 5) // at least one per Python file
	assertEdgeCount(t, tables.Edges, "IMPORTS", "File", "Package", 1)  // at least one import edge
	assertEdgeCount(t, tables.Edges, "DEFINES", "File", "Symbol", 1)   // at least one defines edge
	assertEdgeCount(t, tables.Edges, "IN_FILE", "Symbol", "File", 1)   // at least one in_file edge
}

// TestExtraction_Java verifies the Java tree-sitter extractor.
func TestExtraction_Java(t *testing.T) {
	bin := falconBin(t)
	fixtureDir := filepath.Join(repoRootDir(t), "testdata", "tinyrepo_go_java")
	outDir := filepath.Join(t.TempDir(), "artifacts")

	runFalcon(t, bin, "index", "--repo", fixtureDir, "--out", outDir)

	ctx := context.Background()
	tables := readAllTables(t, ctx, outDir)

	// ---- Files ----
	javaFiles := filterFiles(tables.Files, "java")
	if len(javaFiles) != 1 {
		t.Fatalf("expected 1 Java file, got %d", len(javaFiles))
	}
	assertFileExists(t, javaFiles, "java/src/main/java/com/example/App.java")

	// ---- Packages ----
	javaPkgs := filterPackages(tables.Packages, "java")
	if len(javaPkgs) == 0 {
		t.Fatal("expected Java packages, got 0")
	}

	// The com.example package (from package declaration) must be internal.
	assertPackageInternal(t, javaPkgs, "com.example", true)

	// java.util.List and java.util.Collections.emptyList must be external.
	assertPackageInternal(t, javaPkgs, "java.util.List", false)
	assertPackageInternal(t, javaPkgs, "java.util.Collections.emptyList", false)

	// ---- Symbols ----
	javaSyms := filterSymbols(tables.Symbols, "java")
	if len(javaSyms) == 0 {
		t.Fatal("expected Java symbols, got 0")
	}

	assertSymbolExists(t, javaSyms, "class", "App")
	assertSymbolExists(t, javaSyms, "method", "run")

	// Verify qualified name for the method.
	for _, s := range javaSyms {
		if s.Name == "run" && s.QualifiedName != "App.run" {
			t.Errorf("method run: qualifiedName = %q, want %q", s.QualifiedName, "App.run")
		}
	}

	// ---- Edges ----
	assertEdgeCount(t, tables.Edges, "CONTAINS", "Package", "File", 1)
	assertEdgeCount(t, tables.Edges, "IMPORTS", "File", "Package", 1)
	assertEdgeCount(t, tables.Edges, "DEFINES", "File", "Symbol", 1)
}

// TestExtraction_JS verifies the JS/TS tree-sitter extractor.
func TestExtraction_JS(t *testing.T) {
	bin := falconBin(t)
	fixtureDir := filepath.Join(repoRootDir(t), "testdata", "tinyrepo_go_js")
	outDir := filepath.Join(t.TempDir(), "artifacts")

	runFalcon(t, bin, "index", "--repo", fixtureDir, "--out", outDir)

	ctx := context.Background()
	tables := readAllTables(t, ctx, outDir)

	// ---- Files ----
	jsFiles := filterFiles(tables.Files, "js")
	if len(jsFiles) < 2 {
		t.Fatalf("expected >= 2 JS files, got %d", len(jsFiles))
	}
	assertFileExists(t, jsFiles, "web/index.js")
	assertFileExists(t, jsFiles, "web/lib.js")

	// ---- Packages ----
	jsPkgs := filterPackages(tables.Packages, "js")
	if len(jsPkgs) == 0 {
		t.Fatal("expected JS packages, got 0")
	}

	// Directory package must be internal.
	assertPackageInternal(t, jsPkgs, "web", true)

	// Relative import must be internal.
	assertPackageInternal(t, jsPkgs, "./lib.js", true)

	// ---- Symbols ----
	jsSyms := filterSymbols(tables.Symbols, "js")
	if len(jsSyms) == 0 {
		t.Fatal("expected JS symbols, got 0")
	}

	// ---- Edges ----
	assertEdgeCount(t, tables.Edges, "CONTAINS", "Package", "File", 1)
	assertEdgeCount(t, tables.Edges, "IMPORTS", "File", "Package", 1)
}

// TestExtraction_Go verifies the Go AST extractor.
func TestExtraction_Go(t *testing.T) {
	bin := falconBin(t)
	fixtureDir := filepath.Join(repoRootDir(t), "testdata", "tinyrepo_go_js")
	outDir := filepath.Join(t.TempDir(), "artifacts")

	runFalcon(t, bin, "index", "--repo", fixtureDir, "--out", outDir)

	ctx := context.Background()
	tables := readAllTables(t, ctx, outDir)

	// ---- Files ----
	goFiles := filterFiles(tables.Files, "go")
	if len(goFiles) < 2 {
		t.Fatalf("expected >= 2 Go files, got %d", len(goFiles))
	}

	// ---- Packages ----
	goPkgs := filterPackages(tables.Packages, "go")
	if len(goPkgs) == 0 {
		t.Fatal("expected Go packages, got 0")
	}

	// Internal packages: at least one from the repo.
	foundInternal := false
	for _, p := range goPkgs {
		if p.IsInternal && p.Ecosystem == "go" {
			foundInternal = true
			break
		}
	}
	if !foundInternal {
		t.Fatal("expected at least one internal Go package")
	}

	// External imports (fmt) must be external.
	assertPackageInternal(t, goPkgs, "fmt", false)

	// Internal import must be internal.
	assertPackageInternal(t, goPkgs, "example.com/tinyrepo/gojs/pkg/util", true)

	// ---- Symbols ----
	goSyms := filterSymbols(tables.Symbols, "go")
	if len(goSyms) == 0 {
		t.Fatal("expected Go symbols, got 0")
	}

	assertSymbolExists(t, goSyms, "func", "New")
	assertSymbolExists(t, goSyms, "func", "Add")
	assertSymbolExists(t, goSyms, "type", "Greeter")
	assertSymbolExists(t, goSyms, "method", "Greet")

	// ---- Edges ----
	assertEdgeCount(t, tables.Edges, "CONTAINS", "Package", "File", 1)
	assertEdgeCount(t, tables.Edges, "DEFINES", "File", "Symbol", 1)
}

// ---- Helpers ----

func filterFiles(files []artifacts.FileRow, lang string) []artifacts.FileRow {
	var out []artifacts.FileRow
	for _, f := range files {
		if f.Language == lang {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func filterPackages(pkgs []artifacts.PackageRow, ecosystem string) []artifacts.PackageRow {
	var out []artifacts.PackageRow
	for _, p := range pkgs {
		if p.Ecosystem == ecosystem {
			out = append(out, p)
		}
	}
	return out
}

func filterSymbols(syms []artifacts.SymbolRow, lang string) []artifacts.SymbolRow {
	var out []artifacts.SymbolRow
	for _, s := range syms {
		if s.Language == lang {
			out = append(out, s)
		}
	}
	return out
}

func assertFileExists(t *testing.T, files []artifacts.FileRow, path string) {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return
		}
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	t.Errorf("file %q not found in: %v", path, paths)
}

func assertPackageInternal(t *testing.T, pkgs []artifacts.PackageRow, name string, wantInternal bool) {
	t.Helper()
	for _, p := range pkgs {
		if p.Name == name {
			if p.IsInternal != wantInternal {
				t.Errorf("package %q: isInternal = %v, want %v", name, p.IsInternal, wantInternal)
			}
			return
		}
	}
	names := make([]string, len(pkgs))
	for i, p := range pkgs {
		names[i] = p.Name
	}
	t.Errorf("package %q not found in: %v", name, names)
}

func assertSymbolExists(t *testing.T, syms []artifacts.SymbolRow, kind, name string) {
	t.Helper()
	for _, s := range syms {
		if s.Kind == kind && s.Name == name {
			return
		}
	}
	desc := make([]string, len(syms))
	for i, s := range syms {
		desc[i] = s.Kind + ":" + s.Name
	}
	t.Errorf("symbol %s %q not found in: %v", kind, name, desc)
}

// assertEdgeCount checks that at least minCount edges of the given type and
// src/dst node types exist.
func assertEdgeCount(t *testing.T, edges []artifacts.EdgeRow, edgeType, srcType, dstType string, minCount int) {
	t.Helper()
	count := 0
	for _, e := range edges {
		if e.EdgeType == edgeType && e.SrcType == srcType && e.DstType == dstType {
			count++
		}
	}
	if count < minCount {
		t.Errorf("edges %s (%s -> %s): got %d, want >= %d", edgeType, srcType, dstType, count, minCount)
	}
}
