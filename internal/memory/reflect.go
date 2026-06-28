package memory

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// NodeScore is a cited node's aggregated, time-decayed signal.
type NodeScore struct {
	Node        string
	Score       float64
	UsefulCount int // distinct useful results citing it
	NegCount    int // dead_end/corrected results citing it
}

// Lessons is the distilled work memory.
type Lessons struct {
	Preferred   []NodeScore // corroborated useful nodes
	Tentative   []NodeScore // useful once, not yet corroborated
	Contested   []NodeScore // both positive and negative signals
	DeadEnds    []Record    // questions marked dead_end — don't re-derive
	Corrections []Record    // answers the user corrected
}

// Reflect aggregates records into Lessons. Each citation contributes a signed,
// time-decayed value (useful positive; dead_end/corrected negative) with the
// given half-life, so a fresh dead end outweighs a months-old useful. A node is
// "preferred" only once corroborated by minCorroboration distinct useful results.
func Reflect(records []Record, now time.Time, halfLifeDays float64, minCorroboration int) Lessons {
	if halfLifeDays <= 0 {
		halfLifeDays = 30
	}
	if minCorroboration < 1 {
		minCorroboration = 2
	}

	type agg struct {
		score       float64
		usefulCount int
		negCount    int
	}
	nodes := map[string]*agg{}
	var lessons Lessons

	for _, r := range records {
		ageDays := now.Sub(r.Time).Hours() / 24
		if ageDays < 0 {
			ageDays = 0
		}
		weight := math.Pow(0.5, ageDays/halfLifeDays)
		sign := 0.0
		switch r.Outcome {
		case "useful":
			sign = 1
		case "dead_end", "corrected":
			sign = -1
		}
		for _, n := range r.Nodes {
			a := nodes[n]
			if a == nil {
				a = &agg{}
				nodes[n] = a
			}
			a.score += sign * weight
			if r.Outcome == "useful" {
				a.usefulCount++
			} else {
				a.negCount++
			}
		}
		switch r.Outcome {
		case "dead_end":
			lessons.DeadEnds = append(lessons.DeadEnds, r)
		case "corrected":
			lessons.Corrections = append(lessons.Corrections, r)
		}
	}

	for node, a := range nodes {
		ns := NodeScore{Node: node, Score: a.score, UsefulCount: a.usefulCount, NegCount: a.negCount}
		switch {
		case a.usefulCount > 0 && a.negCount > 0:
			lessons.Contested = append(lessons.Contested, ns)
		case a.score > 0 && a.usefulCount >= minCorroboration:
			lessons.Preferred = append(lessons.Preferred, ns)
		case a.score > 0:
			lessons.Tentative = append(lessons.Tentative, ns)
		}
	}

	byScore := func(s []NodeScore) {
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Score != s[j].Score {
				return s[i].Score > s[j].Score
			}
			return s[i].Node < s[j].Node
		})
	}
	byScore(lessons.Preferred)
	byScore(lessons.Tentative)
	byScore(lessons.Contested)
	return lessons
}

// RenderLessons renders LESSONS.md deterministically.
func RenderLessons(l Lessons) string {
	var b strings.Builder
	b.WriteString("# Falcon Work-Memory Lessons\n\n")
	b.WriteString("Distilled from saved query outcomes. Load this at the start of a session.\n\n")

	section := func(title, hint string, ns []NodeScore) {
		fmt.Fprintf(&b, "## %s\n\n", title)
		if hint != "" {
			fmt.Fprintf(&b, "_%s_\n\n", hint)
		}
		if len(ns) == 0 {
			b.WriteString("(none)\n\n")
			return
		}
		for _, n := range ns {
			fmt.Fprintf(&b, "- `%s` (score %.2f, %d useful", n.Node, n.Score, n.UsefulCount)
			if n.NegCount > 0 {
				fmt.Fprintf(&b, ", %d negative", n.NegCount)
			}
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}

	section("Preferred sources", "corroborated by multiple useful results — trust these first", l.Preferred)
	section("Tentative", "useful once, not yet corroborated", l.Tentative)
	section("Contested", "both useful and unhelpful signals — recency-weighted score decides", l.Contested)

	b.WriteString("## Known dead ends\n\n")
	b.WriteString("_questions/areas that led nowhere — don't re-derive_\n\n")
	if len(l.DeadEnds) == 0 {
		b.WriteString("(none)\n\n")
	} else {
		for _, r := range l.DeadEnds {
			fmt.Fprintf(&b, "- %s\n", r.Question)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Corrections\n\n")
	b.WriteString("_answers the user corrected — use the correction_\n\n")
	if len(l.Corrections) == 0 {
		b.WriteString("(none)\n\n")
	} else {
		for _, r := range l.Corrections {
			fmt.Fprintf(&b, "- Q: %s\n  - correction: %s\n", r.Question, r.Correction)
		}
		b.WriteString("\n")
	}
	return b.String()
}
