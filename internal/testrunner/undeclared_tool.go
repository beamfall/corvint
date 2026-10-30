package testrunner

import (
	"regexp"
	"sort"
	"strings"
)

// undeclaredTool matches Go's exec error for a program absent from the runner's PATH, in plain or
// JSON-escaped output (TRE-V0-120).
var undeclaredTool = regexp.MustCompile(`exec: \\?"([^"\\\s]{1,64})\\?": executable file not found in \$PATH`)

// undeclaredTools names the programs a failing test could not start, sorted and deduplicated.
func undeclaredTools(in Input) []string {
	set := map[string]bool{}
	for _, b := range [][]byte{in.Stdout, in.Stderr} {
		for _, m := range undeclaredTool.FindAllSubmatch(b, 16) {
			set[string(m[1])] = true
		}
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// undeclaredToolDetail is the problem detail for a failing test that named missing tools, or "".
func undeclaredToolDetail(in Input) string {
	names := undeclaredTools(in)
	if len(names) == 0 {
		return ""
	}
	return "a failing test could not start " + strings.Join(names, ", ") + " from the runner's PATH; declare it in the request tools (TRE-V0-120)"
}
