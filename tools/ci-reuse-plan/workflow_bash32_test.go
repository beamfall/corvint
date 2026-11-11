// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// workflowBash32Rules names the Bash 4+ constructs refused in workflow text
// (ASC-V0-780), a subset of script/check-bash32.sh's rules (ASD-V0-750) that
// a line-by-line read of YAML can decide without tracking quote state.
var workflowBash32Rules = []struct {
	label string
	re    *regexp.Regexp
}{
	{"mapfile", regexp.MustCompile(`(^|[^A-Za-z0-9_-])(mapfile|readarray)(\s|$)`)},
	{"associative-array", regexp.MustCompile(`(^|[^A-Za-z0-9_-])(declare|local|typeset|readonly)\s+-[A-Za-z]*A`)},
	{"nameref", regexp.MustCompile(`(^|[^A-Za-z0-9_-])(declare|local|typeset)\s+-[A-Za-z]*n[A-Za-z]*(\s|$)`)},
	{"case-modification", regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*(\[[^\]]*\])?[,^]`)},
	{"parameter-transformation", regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*(\[[^\]]*\])?@[QEPAaUuLK]\}`)},
	{"globstar", regexp.MustCompile(`shopt\s+-s\s+([A-Za-z]+\s+)*globstar`)},
	{"coproc", regexp.MustCompile(`(^|[^A-Za-z0-9_-])coproc(\s|$)`)},
	{"wait-next", regexp.MustCompile(`(^|[^A-Za-z0-9_-])wait\s+-n`)},
	{"append-both", regexp.MustCompile(`&>>`)},
}

func workflowBash32Hits(text string) []string {
	var hits []string
	for i, line := range strings.Split(text, "\n") {
		code := strings.TrimSpace(line)
		if strings.HasPrefix(code, "#") {
			continue
		}
		if at := strings.Index(code, " # "); at >= 0 {
			code = code[:at]
		}
		for _, rule := range workflowBash32Rules {
			if rule.re.MatchString(code) {
				hits = append(hits, rule.label+" line "+strconv.Itoa(i+1))
			}
		}
	}
	return hits
}

// ASC-V0-780 (extends ASD-V0-750): workflow run scripts are executed by macOS
// /bin/bash 3.2.57 as well as by the Linux runners' Bash 5: the macOS jobs run
// them, and this package's tests run step scripts with the host's bash in the
// nightly macOS gate. check-bash32.sh reads only tracked shell scripts, so a
// Bash 4 construct in a run block passed every Linux check and broke the Mac
// (beamfall/corvint#9, mapfile in linecov-miss.yml). This test refuses one on
// any host.
func TestWorkflowRunScriptsAvoidBash4Constructs_ASCV0780(t *testing.T) {
	for _, tc := range []struct{ line, label string }{
		{`          mapfile -t failed < <(find "$d" -type f)`, "mapfile"},
		{`          readarray -t rows < rows.txt`, "mapfile"},
		{`          declare -A seen`, "associative-array"},
		{`          local -n ref=$1`, "nameref"},
		{`          echo "${name,,}"`, "case-modification"},
		{`          echo "${name^^}"`, "case-modification"},
		{`          echo "${name@Q}"`, "parameter-transformation"},
		{`          shopt -s nullglob globstar`, "globstar"},
		{`          coproc worker { cat; }`, "coproc"},
		{`          wait -n "$a" "$b"`, "wait-next"},
		{`          make gate &>> "$log"`, "append-both"},
	} {
		hits := workflowBash32Hits(tc.line)
		if len(hits) != 1 || hits[0] != tc.label+" line 1" {
			t.Errorf("%q: hits %v, want one %s", tc.line, hits, tc.label)
		}
	}
	for _, line := range []string{
		`          # mapfile would be shorter, but macOS has Bash 3.2`,
		`          rows=() # no mapfile here`,
		`          while IFS= read -r f; do failed+=("$f"); done < <(find "$d" -type f)`,
		`          echo "${{ github.event.inputs.sha }}" "${name:-none}" "${#rows[@]}"`,
		`          printf '%s\n' "$line" | tr '[:upper:]' '[:lower:]'`,
		`          local name=$1`,
		`          wait "$pid"`,
	} {
		if hits := workflowBash32Hits(line); len(hits) != 0 {
			t.Errorf("%q: unexpected hits %v", line, hits)
		}
	}

	dir := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	for _, e := range entries {
		if e.IsDir() || !(strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, hit := range workflowBash32Hits(string(b)) {
			t.Errorf(".github/workflows/%s: %s: Bash 4+ construct; macOS /bin/bash is 3.2", e.Name(), hit)
		}
	}
	if scanned == 0 {
		t.Fatal("no workflow was scanned")
	}
}
