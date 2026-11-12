// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// unitFacts are the observed facts of one shell file that a hold-only change
// may grow but never shrink.
type unitFacts struct {
	Imports   []string `json:"imports"`
	Tokens    []string `json:"tokens"`
	Globs     []string `json:"globs"`
	Bounded   bool     `json:"bounded"`
	Unbounded string   `json:"unbounded"`
	Frontier  []string `json:"frontier"`
	Names     []string `json:"names"`
	Tests     []string `json:"tests"`
}

// TestShellUnitDifferential_AFPV09970 is the reproducible base-versus-head
// check of lane shellwalk (AFP-V0-9970 to AFP-V0-9977). It is skipped unless
// CORVINT_SHELL_DIFF_FILES names a file listing repository shell paths, one
// per line, read under CORVINT_SHELL_DIFF_ROOT. It writes every file's facts
// to CORVINT_SHELL_DIFF_OUT and, when CORVINT_SHELL_DIFF_BASE names facts an
// earlier plugin wrote for the same files, fails on any lost import, glob or
// path token (unless the head unit is PathTokensBounded) and logs the gains.
//
// Procedure (docs/build-log/2026-11-11-shellwalk.md), from the head checkout:
//
//	git ls-files '*.sh' '*.bash' '*.bats' '.github/*.sh' > "$D/files"
//	git archive BASE | tar -x -C "$D/base"
//	cp internal/liveverify/affected/shell/differential_test.go "$D/base/internal/liveverify/affected/shell/"
//	(cd "$D/base" && CORVINT_SHELL_DIFF_ROOT=$PWD_HEAD CORVINT_SHELL_DIFF_FILES=$D/files \
//	  CORVINT_SHELL_DIFF_OUT=$D/base.json go test -count=1 -run TestShellUnitDifferential ./internal/liveverify/affected/shell/)
//	CORVINT_SHELL_DIFF_ROOT=$PWD CORVINT_SHELL_DIFF_FILES=$D/files CORVINT_SHELL_DIFF_OUT=$D/head.json \
//	  CORVINT_SHELL_DIFF_BASE=$D/base.json go test -count=1 -run TestShellUnitDifferential -v ./internal/liveverify/affected/shell/
func TestShellUnitDifferential_AFPV09970(t *testing.T) {
	list := os.Getenv("CORVINT_SHELL_DIFF_FILES")
	if list == "" {
		t.Skip("CORVINT_SHELL_DIFF_FILES is not set")
	}
	root := os.Getenv("CORVINT_SHELL_DIFF_ROOT")
	body, err := os.ReadFile(list)
	if err != nil {
		t.Fatal(err)
	}
	files := strings.Fields(string(body))
	known := map[string]bool{}
	for _, file := range files {
		known[file] = true
	}
	facts := map[string]unitFacts{}
	for _, file := range files {
		text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			continue
		}
		unit, names, _ := observe(file, string(text), known)
		facts[file] = unitFacts{
			Imports: unit.Imports, Tokens: unit.PathTokens, Globs: unit.PathGlobs, Bounded: unit.PathTokensBounded,
			Unbounded: unit.UnboundedReads, Frontier: unit.Frontier, Names: names, Tests: unit.Tests,
		}
	}
	if out := os.Getenv("CORVINT_SHELL_DIFF_OUT"); out != "" {
		data, err := json.Marshal(facts)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	basePath := os.Getenv("CORVINT_SHELL_DIFF_BASE")
	if basePath == "" {
		return
	}
	data, err := os.ReadFile(basePath)
	if err != nil {
		t.Fatal(err)
	}
	var base map[string]unitFacts
	if err := json.Unmarshal(data, &base); err != nil {
		t.Fatal(err)
	}
	lost := func(before, after []string) []string {
		var missing []string
		for _, item := range before {
			if !slices.Contains(after, item) {
				missing = append(missing, item)
			}
		}
		return missing
	}
	gainedTokens, gainedImports, unbounded, other := 0, 0, 0, 0
	for _, file := range files {
		before, after := base[file], facts[file]
		if missing := lost(before.Imports, after.Imports); len(missing) != 0 {
			t.Errorf("%s lost imports %v", file, missing)
		}
		if missing := lost(before.Globs, after.Globs); len(missing) != 0 {
			t.Errorf("%s lost globs %v", file, missing)
		}
		if missing := lost(before.Tokens, after.Tokens); len(missing) != 0 && !after.Bounded {
			t.Errorf("%s lost path tokens %v", file, missing)
		}
		if len(lost(after.Tokens, before.Tokens)) != 0 {
			gainedTokens++
		}
		if len(lost(after.Imports, before.Imports)) != 0 {
			gainedImports++
		}
		if before.Unbounded != after.Unbounded {
			unbounded++
			t.Logf("%s: unbounded %q -> %q", file, before.Unbounded, after.Unbounded)
		}
		if !slices.Equal(before.Frontier, after.Frontier) || !slices.Equal(before.Names, after.Names) || !slices.Equal(before.Tests, after.Tests) || before.Bounded != after.Bounded {
			other++
			t.Logf("%s: frontier %v -> %v, names %v -> %v, tests %v -> %v, bounded %v -> %v", file, before.Frontier, after.Frontier, before.Names, after.Names, before.Tests, after.Tests, before.Bounded, after.Bounded)
		}
	}
	t.Logf("differential: %d units, %d gain tokens, %d gain imports, %d change unbounded reads, %d change frontier, names, tests or bound", len(files), gainedTokens, gainedImports, unbounded, other)
}
