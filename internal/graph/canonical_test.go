package graph

import "testing"

func TestCanonRepoRelPath(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "a/b/c.go", want: "a/b/c.go"},
		{in: "./a/b/../c.go", want: "a/c.go"},
		{in: "a\\b\\c.go", want: "a/b/c.go"},
		{in: "../x.go", wantErr: true},
		{in: "/abs/x.go", wantErr: true},
		{in: ".", wantErr: true},
	}

	for _, tc := range tcs {
		got, err := CanonRepoRelPath(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("CanonRepoRelPath(%q): expected error, got none", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("CanonRepoRelPath(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("CanonRepoRelPath(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}
