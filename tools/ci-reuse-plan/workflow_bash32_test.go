// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	{"associative-array", regexp.MustCompile(`(^|[^A-Za-z0-9_-])(declare|local|typeset|readonly)(\s+-[A-Za-z]+)*\s+-[A-Za-z]*A`)},
	{"nameref", regexp.MustCompile(`(^|[^A-Za-z0-9_-])(declare|local|typeset)(\s+-[A-Za-z]+)*\s+-[A-Za-z]*n[A-Za-z]*(\s|$)`)},
	{"case-modification", regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*(\[[^\]]*\])?[,^]`)},
	{"parameter-transformation", regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*(\[[^\]]*\])?@[QEPAaUuLK]\}`)},
	{"globstar", regexp.MustCompile(`shopt\s+-s\s+([A-Za-z]+\s+)*globstar`)},
	{"coproc", regexp.MustCompile(`(^|[^A-Za-z0-9_-])coproc(\s|$)`)},
	{"wait-next", regexp.MustCompile(`(^|[^A-Za-z0-9_-])wait\s+-n`)},
	{"append-both", regexp.MustCompile(`&>>`)},
}

// workflowBash32Hits reports each rule a command breaks, by the line the
// command starts on. A command continued with a trailing backslash is read as
// one: its lines are joined both with and without a space, because the shell
// drops the backslash and the newline and keeps only the indentation YAML
// leaves, so either spelling can be the one Bash runs.
func workflowBash32Hits(text string) []string {
	var hits, parts []string
	lines := strings.Split(text, "\n")
	first := 0
	for i, line := range lines {
		code := strings.TrimSpace(line)
		if len(parts) == 0 {
			first = i
		}
		if strings.HasPrefix(code, "#") {
			code = ""
		}
		// A trailing comment is cut only where the quotes before it balance, so
		// a " # " inside a string cannot hide a construct later on the line.
		for at := strings.Index(code, " # "); at >= 0; {
			if before := code[:at]; strings.Count(before, `"`)%2 == 0 && strings.Count(before, `'`)%2 == 0 {
				code = before
				break
			}
			next := strings.Index(code[at+1:], " # ")
			if next < 0 {
				break
			}
			at += 1 + next
		}
		// An odd run of trailing backslashes continues the command; an even
		// run is escaped backslashes.
		if body := strings.TrimRight(code, `\`); (len(code)-len(body))%2 == 1 && i+1 < len(lines) {
			parts = append(parts, code[:len(code)-1])
			continue
		}
		parts = append(parts, code)
		tight, spaced := strings.Join(parts, ""), strings.Join(parts, " ")
		parts = parts[:0]
		for _, rule := range workflowBash32Rules {
			if rule.re.MatchString(tight) || rule.re.MatchString(spaced) {
				hits = append(hits, rule.label+" line "+strconv.Itoa(first+1))
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
		{`          declare -r -A seen`, "associative-array"},
		{`          local -n ref=$1`, "nameref"},
		{`          local -i -n ref=$1`, "nameref"},
		{`          echo "build # 1"; mapfile -t rows < rows.txt`, "mapfile"},
		{`          echo 'it # is' "so # so"; mapfile -t rows < rows.txt # read them`, "mapfile"},
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
		`          local -r name=$1`,
		`          echo "build # 1" # mapfile is not used`,
		`          wait "$pid"`,
	} {
		if hits := workflowBash32Hits(line); len(hits) != 0 {
			t.Errorf("%q: unexpected hits %v", line, hits)
		}
	}

	// A continued command is one command, reported at its first line.
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"          shopt -s \\\n            globstar", []string{"globstar line 1"}},
		{"          declare \\\n            -A seen", []string{"associative-array line 1"}},
		{"          declare\\\n            -A seen", []string{"associative-array line 1"}},
		{"          read\\\n          array -t rows < rows.txt", []string{"mapfile line 1"}},
		{"          echo one\n          wait \\\n            -n \\\n            \"$a\"\n          mapfile -t rows", []string{"wait-next line 2", "mapfile line 5"}},
		{"          echo one \\\n            two\n          echo three", nil},
		{"          echo C:\\\\\n          mapfile -t rows", []string{"mapfile line 2"}},
		{"          shopt -s nullglob \\", nil},
	} {
		if hits := workflowBash32Hits(tc.text); !slices.Equal(hits, tc.want) {
			t.Errorf("%q: hits %v, want %v", tc.text, hits, tc.want)
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
