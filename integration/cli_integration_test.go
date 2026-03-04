package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"repofalcon/internal/artifacts"
)

type fixtureCase struct {
	name       string
	fixtureRel string
	// A repo-relative file path to modify between base/head commits.
	changePath string
}

var fixtures = []fixtureCase{
	{name: "go_js", fixtureRel: filepath.ToSlash(filepath.Join("testdata", "tinyrepo_go_js")), changePath: filepath.ToSlash(filepath.Join("pkg", "greeter", "greeter.go"))},
	{name: "go_py", fixtureRel: filepath.ToSlash(filepath.Join("testdata", "tinyrepo_go_py")), changePath: "main.go"},
	{name: "go_java", fixtureRel: filepath.ToSlash(filepath.Join("testdata", "tinyrepo_go_java")), changePath: filepath.ToSlash(filepath.Join("lib", "lib.go"))},
}

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

func falconBin(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		if _, err := exec.LookPath("go"); err != nil {
			buildErr = fmt.Errorf("go not found in PATH: %w", err)
			return
		}
		outDir, err := os.MkdirTemp("", "falcon-bin-")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(outDir, "falcon")

		// Build once; integration tests then execute the binary.
		cmd := exec.Command("go", "build", "-o", builtBin, "./cmd/falcon")
		cmd.Dir = repoRootDir(t)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("go build ./cmd/falcon: %w\n%s", err, buf.String())
			return
		}
	})
	if buildErr != nil {
		t.Fatalf("build falcon binary: %v", buildErr)
	}
	return builtBin
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := findRepoRoot(wd)
	if err != nil {
		t.Fatalf("find repo root from %s: %v", wd, err)
	}
	return root
}

func findRepoRoot(start string) (string, error) {
	d := filepath.Clean(start)
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("go.mod not found")
		}
		d = parent
	}
}

func TestCLI_EndToEnd_Fixtures(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}

	bin := falconBin(t)

	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			repoDir, baseSHA, headSHA := initGitRepoFromFixture(t, fx.fixtureRel, fx.changePath)
			outDir := filepath.Join(t.TempDir(), "artifacts")

			runFalcon(t, bin, "index", "--repo", repoDir, "--out", outDir)
			runFalcon(t, bin, "snapshot", "--in", outDir, "--out", outDir)
			runFalcon(t, bin, "pr-pack", "--repo", repoDir, "--snapshot", outDir, "--base", baseSHA, "--head", headSHA, "--out", outDir)

			mustExist(t, filepath.Join(outDir, "metadata.json"))
			mustExist(t, filepath.Join(outDir, "nodes.parquet"))
			mustExist(t, filepath.Join(outDir, "files.parquet"))
			mustExist(t, filepath.Join(outDir, "packages.parquet"))
			mustExist(t, filepath.Join(outDir, "symbols.parquet"))
			mustExist(t, filepath.Join(outDir, "edges.parquet"))
			mustExist(t, filepath.Join(outDir, "findings.parquet"))
			mustExist(t, filepath.Join(outDir, "pr_context_pack.json"))
			mustExist(t, filepath.Join(outDir, "review_report.md"))
		})
	}
}

func TestCLI_Determinism_IndexSnapshotAndPRPack(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	bin := falconBin(t)

	// Use the mixed Go+JS fixture for determinism: exercises multi-language scanning and import extraction.
	fx := fixtures[0]
	repoDir, baseSHA, headSHA := initGitRepoFromFixture(t, fx.fixtureRel, fx.changePath)

	// Run twice into the same snapshot directory so `snapshot_dir` in JSON is stable.
	outDir := filepath.Join(t.TempDir(), "artifacts")

	ctx := context.Background()

	// Run 1
	runFalcon(t, bin, "index", "--repo", repoDir, "--out", outDir)
	runFalcon(t, bin, "snapshot", "--in", outDir, "--out", outDir)
	runFalcon(t, bin, "pr-pack", "--repo", repoDir, "--snapshot", outDir, "--base", baseSHA, "--head", headSHA, "--out", outDir)

	tables1 := readAllTables(t, ctx, outDir)
	packJSON1 := mustReadFile(t, filepath.Join(outDir, "pr_context_pack.json"))
	report1 := mustReadFile(t, filepath.Join(outDir, "review_report.md"))

	// Clean and run 2
	if err := os.RemoveAll(outDir); err != nil {
		t.Fatalf("remove outDir: %v", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir outDir: %v", err)
	}

	runFalcon(t, bin, "index", "--repo", repoDir, "--out", outDir)
	runFalcon(t, bin, "snapshot", "--in", outDir, "--out", outDir)
	runFalcon(t, bin, "pr-pack", "--repo", repoDir, "--snapshot", outDir, "--base", baseSHA, "--head", headSHA, "--out", outDir)

	tables2 := readAllTables(t, ctx, outDir)
	packJSON2 := mustReadFile(t, filepath.Join(outDir, "pr_context_pack.json"))
	report2 := mustReadFile(t, filepath.Join(outDir, "review_report.md"))

	if !reflect.DeepEqual(tables1, tables2) {
		t.Fatalf("parquet tables differ between runs")
	}
	if !bytes.Equal(packJSON1, packJSON2) {
		t.Fatalf("byte content differs for pr_context_pack.json between runs")
	}
	if !bytes.Equal(report1, report2) {
		t.Fatalf("byte content differs for review_report.md between runs")
	}
}

type allTables struct {
	Files    []artifacts.FileRow
	Packages []artifacts.PackageRow
	Symbols  []artifacts.SymbolRow
	Edges    []artifacts.EdgeRow
	Findings []artifacts.FindingRow
	Nodes    []artifacts.NodeRow
}

func readAllTables(t *testing.T, ctx context.Context, outDir string) allTables {
	t.Helper()

	files, err := artifacts.ReadFilesParquet(ctx, filepath.Join(outDir, "files.parquet"))
	if err != nil {
		t.Fatalf("read files: %v", err)
	}
	pkgs, err := artifacts.ReadPackagesParquet(ctx, filepath.Join(outDir, "packages.parquet"))
	if err != nil {
		t.Fatalf("read packages: %v", err)
	}
	syms, err := artifacts.ReadSymbolsParquet(ctx, filepath.Join(outDir, "symbols.parquet"))
	if err != nil {
		t.Fatalf("read symbols: %v", err)
	}
	edges, err := artifacts.ReadEdgesParquet(ctx, filepath.Join(outDir, "edges.parquet"))
	if err != nil {
		t.Fatalf("read edges: %v", err)
	}
	findings, err := artifacts.ReadFindingsParquet(ctx, filepath.Join(outDir, "findings.parquet"))
	if err != nil {
		t.Fatalf("read findings: %v", err)
	}
	nodes, err := artifacts.ReadNodesParquet(ctx, filepath.Join(outDir, "nodes.parquet"))
	if err != nil {
		t.Fatalf("read nodes: %v", err)
	}

	sort.SliceStable(files, func(i, j int) bool { return files[i].FileID < files[j].FileID })
	sort.SliceStable(pkgs, func(i, j int) bool { return pkgs[i].PackageID < pkgs[j].PackageID })
	sort.SliceStable(syms, func(i, j int) bool { return syms[i].SymbolID < syms[j].SymbolID })
	sort.SliceStable(edges, func(i, j int) bool { return edges[i].EdgeID < edges[j].EdgeID })
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].FindingID < findings[j].FindingID })
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].NodeID < nodes[j].NodeID })

	return allTables{Files: files, Packages: pkgs, Symbols: syms, Edges: edges, Findings: findings, Nodes: nodes}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func compareParquetTables(t *testing.T, ctx context.Context, out1, out2 string) {
	t.Helper()

	files1, err := artifacts.ReadFilesParquet(ctx, filepath.Join(out1, "files.parquet"))
	if err != nil {
		t.Fatalf("read files run1: %v", err)
	}
	files2, err := artifacts.ReadFilesParquet(ctx, filepath.Join(out2, "files.parquet"))
	if err != nil {
		t.Fatalf("read files run2: %v", err)
	}
	sort.SliceStable(files1, func(i, j int) bool { return files1[i].FileID < files1[j].FileID })
	sort.SliceStable(files2, func(i, j int) bool { return files2[i].FileID < files2[j].FileID })
	if !reflect.DeepEqual(files1, files2) {
		t.Fatalf("files.parquet differs between runs")
	}

	pkgs1, err := artifacts.ReadPackagesParquet(ctx, filepath.Join(out1, "packages.parquet"))
	if err != nil {
		t.Fatalf("read packages run1: %v", err)
	}
	pkgs2, err := artifacts.ReadPackagesParquet(ctx, filepath.Join(out2, "packages.parquet"))
	if err != nil {
		t.Fatalf("read packages run2: %v", err)
	}
	sort.SliceStable(pkgs1, func(i, j int) bool { return pkgs1[i].PackageID < pkgs1[j].PackageID })
	sort.SliceStable(pkgs2, func(i, j int) bool { return pkgs2[i].PackageID < pkgs2[j].PackageID })
	if !reflect.DeepEqual(pkgs1, pkgs2) {
		t.Fatalf("packages.parquet differs between runs")
	}

	syms1, err := artifacts.ReadSymbolsParquet(ctx, filepath.Join(out1, "symbols.parquet"))
	if err != nil {
		t.Fatalf("read symbols run1: %v", err)
	}
	syms2, err := artifacts.ReadSymbolsParquet(ctx, filepath.Join(out2, "symbols.parquet"))
	if err != nil {
		t.Fatalf("read symbols run2: %v", err)
	}
	sort.SliceStable(syms1, func(i, j int) bool { return syms1[i].SymbolID < syms1[j].SymbolID })
	sort.SliceStable(syms2, func(i, j int) bool { return syms2[i].SymbolID < syms2[j].SymbolID })
	if !reflect.DeepEqual(syms1, syms2) {
		t.Fatalf("symbols.parquet differs between runs")
	}

	edges1, err := artifacts.ReadEdgesParquet(ctx, filepath.Join(out1, "edges.parquet"))
	if err != nil {
		t.Fatalf("read edges run1: %v", err)
	}
	edges2, err := artifacts.ReadEdgesParquet(ctx, filepath.Join(out2, "edges.parquet"))
	if err != nil {
		t.Fatalf("read edges run2: %v", err)
	}
	sort.SliceStable(edges1, func(i, j int) bool { return edges1[i].EdgeID < edges1[j].EdgeID })
	sort.SliceStable(edges2, func(i, j int) bool { return edges2[i].EdgeID < edges2[j].EdgeID })
	if !reflect.DeepEqual(edges1, edges2) {
		t.Fatalf("edges.parquet differs between runs")
	}

	findings1, err := artifacts.ReadFindingsParquet(ctx, filepath.Join(out1, "findings.parquet"))
	if err != nil {
		t.Fatalf("read findings run1: %v", err)
	}
	findings2, err := artifacts.ReadFindingsParquet(ctx, filepath.Join(out2, "findings.parquet"))
	if err != nil {
		t.Fatalf("read findings run2: %v", err)
	}
	sort.SliceStable(findings1, func(i, j int) bool { return findings1[i].FindingID < findings1[j].FindingID })
	sort.SliceStable(findings2, func(i, j int) bool { return findings2[i].FindingID < findings2[j].FindingID })
	if !reflect.DeepEqual(findings1, findings2) {
		t.Fatalf("findings.parquet differs between runs")
	}

	nodes1, err := artifacts.ReadNodesParquet(ctx, filepath.Join(out1, "nodes.parquet"))
	if err != nil {
		t.Fatalf("read nodes run1: %v", err)
	}
	nodes2, err := artifacts.ReadNodesParquet(ctx, filepath.Join(out2, "nodes.parquet"))
	if err != nil {
		t.Fatalf("read nodes run2: %v", err)
	}
	sort.SliceStable(nodes1, func(i, j int) bool { return nodes1[i].NodeID < nodes1[j].NodeID })
	sort.SliceStable(nodes2, func(i, j int) bool { return nodes2[i].NodeID < nodes2[j].NodeID })
	if !reflect.DeepEqual(nodes1, nodes2) {
		t.Fatalf("nodes.parquet differs between runs")
	}
}

func compareBytes(t *testing.T, a, b string) {
	t.Helper()
	ba, err := os.ReadFile(a)
	if err != nil {
		t.Fatalf("read %s: %v", a, err)
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		t.Fatalf("read %s: %v", b, err)
	}
	if !bytes.Equal(ba, bb) {
		t.Fatalf("byte content differs: %s vs %s", a, b)
	}
}

func runFalcon(t *testing.T, bin string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	// Ensure stable output from tools (and avoid locale surprises).
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		t.Fatalf("falcon %s failed: %v\n%s", strings.Join(args, " "), err, buf.String())
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func initGitRepoFromFixture(t *testing.T, fixtureRel, changePath string) (repoDir, baseSHA, headSHA string) {
	t.Helper()

	// Resolve fixture path relative to module root (not package dir).
	fixtureDir := filepath.Join(repoRootDir(t), filepath.FromSlash(fixtureRel))

	tmp := t.TempDir()
	repoDir = filepath.Join(tmp, "repo")
	if err := copyDir(repoDir, fixtureDir); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	git(t, repoDir, "init")
	git(t, repoDir, "config", "user.email", "ci@example.com")
	git(t, repoDir, "config", "user.name", "CI")

	git(t, repoDir, "add", "-A")
	git(t, repoDir, "commit", "-m", "base")
	baseSHA = strings.TrimSpace(gitOut(t, repoDir, "rev-parse", "HEAD"))

	// Create a deterministic change in a tracked file.
	absChange := filepath.Join(repoDir, filepath.FromSlash(changePath))
	b, err := os.ReadFile(absChange)
	if err != nil {
		t.Fatalf("read change file %s: %v", changePath, err)
	}
	// Append content based on file hash, so the change itself is deterministic.
	sum := sha256.Sum256(b)
	add := fmt.Sprintf("\n// change %s\n", hex.EncodeToString(sum[:4]))
	if err := os.WriteFile(absChange, append(b, []byte(add)...), 0o644); err != nil {
		t.Fatalf("write change file %s: %v", changePath, err)
	}

	git(t, repoDir, "add", "-A")
	git(t, repoDir, "commit", "-m", "head")
	headSHA = strings.TrimSpace(gitOut(t, repoDir, "rev-parse", "HEAD"))

	if baseSHA == "" || headSHA == "" || baseSHA == headSHA {
		t.Fatalf("expected distinct base/head SHAs, got base=%q head=%q", baseSHA, headSHA)
	}
	return repoDir, baseSHA, headSHA
}

func git(t *testing.T, repoDir string, args ...string) {
	t.Helper()
	_ = gitOut(t, repoDir, args...)
}

func gitOut(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, buf.String())
	}
	return buf.String()
}

func copyDir(dst, src string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		// Never copy VCS state.
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		outPath := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(outPath, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return nil
	})
}
