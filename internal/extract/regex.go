package extract

import (
	"bytes"
	"regexp"
	"sort"
)

var (
	// Python
	rePyImport     = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z0-9_\.]+)`)
	rePyFromImport = regexp.MustCompile(`(?m)^\s*from\s+([A-Za-z0-9_\.]+)\s+import\b`)

	// Java
	reJavaImport = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([A-Za-z0-9_\.*]+)\s*;`)
)

func ExtractPythonImportTargets(src []byte) []string {
	var out []string
	out = append(out, findAllSubmatch1(rePyImport, src)...)
	out = append(out, findAllSubmatch1(rePyFromImport, src)...)
	return uniqSorted(out)
}

func ExtractJavaImportTargets(src []byte) []string {
	return uniqSorted(findAllSubmatch1(reJavaImport, src))
}

func findAllSubmatch1(re *regexp.Regexp, src []byte) []string {
	m := re.FindAllSubmatch(src, -1)
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for _, sm := range m {
		if len(sm) < 2 {
			continue
		}
		out = append(out, string(bytes.TrimSpace(sm[1])))
	}
	return out
}

func uniqSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	sort.Strings(in)
	out := in[:0]
	var last string
	for i, s := range in {
		if i == 0 || s != last {
			out = append(out, s)
			last = s
		}
	}
	return out
}
