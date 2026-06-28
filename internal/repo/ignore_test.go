package repo

import "testing"

func TestIgnoreMatcher(t *testing.T) {
	m := newIgnoreMatcher([]string{
		"# a comment",
		"",
		"node_modules/", // dir-only, any depth
		".iterion/",
		"*.min.js",      // basename glob, any depth
		"/build",        // anchored to root
		"dist/**/*.map", // anchored, ** span
		"docs/",         // dir-only
		"!docs/keep.md", // negation overrides
		"vendor",        // bare name, file or dir, any depth
	})

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"node_modules", true, true},
		{"pkg/node_modules", true, true},
		{"pkg/node_modules/lib.js", false, true}, // under ignored dir
		{".iterion", true, true},
		{".iterion/runs/x.json", false, true},
		{"src/app.min.js", false, true},
		{"app.js", false, false},
		{"build", true, true},
		{"build/out.o", false, true},
		{"sub/build", true, false}, // anchored: only root-level build
		{"dist/a/b/c.map", false, true},
		{"dist/a/b/c.js", false, false},
		{"docs", true, true},
		{"docs/readme.md", false, true},
		{"docs/keep.md", false, false}, // negated
		{"vendor", true, true},
		{"pkg/vendor/x.go", false, true},
		{"pkg/main.go", false, false},
	}
	for _, c := range cases {
		if got := m.Match(c.path, c.isDir); got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestIgnoreMatcherEmpty(t *testing.T) {
	var m *ignoreMatcher
	if m.Match("anything", false) {
		t.Error("nil matcher must not ignore")
	}
	if got := newIgnoreMatcher(nil).Match("x.go", false); got {
		t.Error("empty matcher must not ignore")
	}
}

func TestIsGeneratedJS(t *testing.T) {
	cases := []struct {
		name  string
		size  int64
		lines int32
		want  bool
	}{
		{"app.min.js", 1000, 1, true},
		{"index-A1b2C3.js", 200 * 1024, 3, true}, // minified bundle signature
		{"bundle.js.map", 10, 1, true},
		{"main.go", 999999, 1, false}, // not js/ts caller-gated, but name alone is false
		{"component.tsx", 8000, 200, false},
		{"small.js", 40 * 1024, 2, false}, // under size threshold
	}
	for _, c := range cases {
		if got := isGeneratedJS(c.name, c.size, c.lines); got != c.want {
			t.Errorf("isGeneratedJS(%q, %d, %d) = %v, want %v", c.name, c.size, c.lines, got, c.want)
		}
	}
}
