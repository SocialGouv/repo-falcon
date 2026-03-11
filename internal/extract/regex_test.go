package extract

import (
	"reflect"
	"testing"
)

func TestUniqSorted(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil", nil, nil},
		{"empty", []string{}, nil},
		{"single", []string{"a"}, []string{"a"}},
		{"sorted_unique", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"unsorted", []string{"c", "a", "b"}, []string{"a", "b", "c"}},
		{"duplicates", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Copy input to avoid mutating test data.
			var in []string
			if tt.in != nil {
				in = make([]string, len(tt.in))
				copy(in, tt.in)
			}
			got := uniqSorted(in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("uniqSorted(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
