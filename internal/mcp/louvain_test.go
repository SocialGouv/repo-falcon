package mcp

import "testing"

func sym(adj map[string]map[string]float64, a, b string) {
	if adj[a] == nil {
		adj[a] = map[string]float64{}
	}
	if adj[b] == nil {
		adj[b] = map[string]float64{}
	}
	adj[a][b]++
	adj[b][a]++
}

func TestLouvainTwoClusters(t *testing.T) {
	// Two triangles {a,b,c} and {d,e,f} joined by a single bridge c-d.
	adj := map[string]map[string]float64{}
	sym(adj, "a", "b")
	sym(adj, "b", "c")
	sym(adj, "a", "c")
	sym(adj, "d", "e")
	sym(adj, "e", "f")
	sym(adj, "d", "f")
	sym(adj, "c", "d") // bridge

	comm := louvain(adj)
	if comm["a"] != comm["b"] || comm["b"] != comm["c"] {
		t.Errorf("a,b,c should share a community: %v", comm)
	}
	if comm["d"] != comm["e"] || comm["e"] != comm["f"] {
		t.Errorf("d,e,f should share a community: %v", comm)
	}
	if comm["a"] == comm["d"] {
		t.Errorf("the two triangles should be different communities: %v", comm)
	}
}

func TestLouvainDeterministic(t *testing.T) {
	build := func() map[string]map[string]float64 {
		adj := map[string]map[string]float64{}
		sym(adj, "x1", "x2")
		sym(adj, "x2", "x3")
		sym(adj, "x1", "x3")
		sym(adj, "y1", "y2")
		sym(adj, "y2", "y3")
		sym(adj, "x3", "y1")
		return adj
	}
	a := louvain(build())
	b := louvain(build())
	for k, v := range a {
		if b[k] != v {
			t.Fatalf("non-deterministic: node %s got %d then %d", k, v, b[k])
		}
	}
}
