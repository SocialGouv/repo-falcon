package secure

import (
	"path/filepath"
	"testing"
)

func TestValidGitRef(t *testing.T) {
	ok := []string{"main", "v1.2.3", "feature/x", "a1b2c3d", "origin/main", "HEAD~3", "a..b"}
	bad := []string{"", "-x", "--upload-pack=evil", "a b", "a\tb", "a\nb", "\x00"}
	for _, r := range ok {
		if !ValidGitRef(r) {
			t.Errorf("ValidGitRef(%q) = false, want true", r)
		}
	}
	for _, r := range bad {
		if ValidGitRef(r) {
			t.Errorf("ValidGitRef(%q) = true, want false", r)
		}
	}
}

func TestSanitizeLabel(t *testing.T) {
	cases := map[string]string{
		"  Auth & Sessions \n": "Auth & Sessions",
		"`DSL Compiler`":       "DSL Compiler",
		"a\x00b\tc":            "a b c",
	}
	for in, want := range cases {
		if got := SanitizeLabel(in); got != want {
			t.Errorf("SanitizeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWithinBase(t *testing.T) {
	base := "/tmp/base"
	if _, err := WithinBase(base, filepath.Join(base, "sub", "x.json")); err != nil {
		t.Errorf("inside path should be allowed: %v", err)
	}
	if _, err := WithinBase(base, "/tmp/other/x"); err == nil {
		t.Error("path outside base should be rejected")
	}
	if _, err := WithinBase(base, filepath.Join(base, "..", "escape")); err == nil {
		t.Error("traversal should be rejected")
	}
}
