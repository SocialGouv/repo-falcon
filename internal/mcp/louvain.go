package mcp

import "sort"

// louvain runs deterministic multi-level Louvain modularity optimization on an
// undirected weighted graph and returns a map of node -> community id (an int).
//
// Determinism: nodes are processed in sorted order and ties are broken by
// smallest community representative, so the same graph always yields the same
// partition. Modularity optimization (unlike label propagation) penalizes
// merging everything into one community, so dense hubs no longer collapse the
// whole graph into a single giant cluster.
//
// adj must be symmetric (adj[u][v] == adj[v][u]); self-loops (adj[u][u]) are
// allowed and represent intra-cluster weight in aggregated levels.
func louvain(adj map[string]map[string]float64) map[string]int {
	nodes := make([]string, 0, len(adj))
	for n := range adj {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	// orig2super maps each original node to its current super-node label.
	orig2super := make(map[string]string, len(nodes))
	for _, n := range nodes {
		orig2super[n] = n
	}

	cur := adj
	for {
		comm, changed := louvainLevel(cur)
		if !changed {
			break
		}
		// Compose mapping: original -> super -> new community representative.
		for orig, super := range orig2super {
			orig2super[orig] = comm[super]
		}
		// Aggregate the graph by community.
		next := aggregate(cur, comm)
		if len(next) == len(cur) {
			break // no further coarsening
		}
		cur = next
	}

	// Assign stable integer ids by sorted community representative.
	reps := map[string]bool{}
	for _, r := range orig2super {
		reps[r] = true
	}
	repList := make([]string, 0, len(reps))
	for r := range reps {
		repList = append(repList, r)
	}
	sort.Strings(repList)
	repID := make(map[string]int, len(repList))
	for i, r := range repList {
		repID[r] = i
	}
	out := make(map[string]int, len(orig2super))
	for orig, super := range orig2super {
		out[orig] = repID[super]
	}
	return out
}

// louvainLevel runs one phase of local moving and returns, for each node, the
// representative (smallest member id) of its community, plus whether any node
// moved.
func louvainLevel(adj map[string]map[string]float64) (map[string]string, bool) {
	nodes := make([]string, 0, len(adj))
	var twoM float64
	ki := make(map[string]float64, len(adj))
	for n, nbrs := range adj {
		nodes = append(nodes, n)
		var k float64
		for _, w := range nbrs {
			k += w
		}
		k += adj[n][n] // self-loop counts twice
		ki[n] = k
		twoM += k
	}
	sort.Strings(nodes)
	if twoM == 0 {
		// No edges: each node its own community.
		comm := make(map[string]string, len(nodes))
		for _, n := range nodes {
			comm[n] = n
		}
		return comm, false
	}

	n2c := make(map[string]string, len(nodes)) // node -> community key
	members := make(map[string][]string)       // community -> nodes
	sigmaTot := make(map[string]float64)       // community -> sum ki
	for _, n := range nodes {
		n2c[n] = n
		members[n] = []string{n}
		sigmaTot[n] = ki[n]
	}

	moved := false
	for improved := true; improved; {
		improved = false
		for _, u := range nodes {
			cu := n2c[u]
			// Weight from u to each neighbouring community (excluding self).
			wTo := map[string]float64{}
			for v, w := range adj[u] {
				if v == u {
					continue
				}
				wTo[n2c[v]] += w
			}
			// Remove u from its community.
			sigmaTot[cu] -= ki[u]

			bestC, bestGain := cu, wTo[cu]-sigmaTot[cu]*ki[u]/twoM
			// Deterministic iteration over candidate communities.
			cands := make([]string, 0, len(wTo))
			for c := range wTo {
				cands = append(cands, c)
			}
			sort.Strings(cands)
			for _, c := range cands {
				gain := wTo[c] - sigmaTot[c]*ki[u]/twoM
				if gain > bestGain || (gain == bestGain && c < bestC) {
					bestC, bestGain = c, gain
				}
			}
			// Place u into bestC.
			sigmaTot[bestC] += ki[u]
			if bestC != cu {
				n2c[u] = bestC
				moved = true
				improved = true
			}
		}
	}

	// Canonicalize each community to its smallest member id.
	byComm := map[string][]string{}
	for _, n := range nodes {
		byComm[n2c[n]] = append(byComm[n2c[n]], n)
	}
	canon := map[string]string{}
	for c, ms := range byComm {
		rep := ms[0]
		for _, m := range ms[1:] {
			if m < rep {
				rep = m
			}
		}
		canon[c] = rep
	}
	comm := make(map[string]string, len(nodes))
	for _, n := range nodes {
		comm[n] = canon[n2c[n]]
	}
	return comm, moved
}

// aggregate collapses each community into a single super-node, summing edge
// weights between communities and accumulating intra-community weight as a
// self-loop.
func aggregate(adj map[string]map[string]float64, comm map[string]string) map[string]map[string]float64 {
	next := map[string]map[string]float64{}
	ensure := func(c string) {
		if next[c] == nil {
			next[c] = map[string]float64{}
		}
	}
	for u, nbrs := range adj {
		cu := comm[u]
		ensure(cu)
		for v, w := range nbrs {
			cv := comm[v]
			next[cu][cv] += w
		}
	}
	return next
}
