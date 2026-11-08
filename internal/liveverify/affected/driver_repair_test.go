// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-9050: empty conjunction arguments short circuit before expansion;
// unsupported functions remain unknown rather than choosing a single branch.
func TestMakeAndEmptyArguments_AFP_V0_9050(t *testing.T) {
	for _, condition := range []string{
		"$(and x,,y)", "$(and x,   ,y)", "$(and x,\t,y)",
		"$(and ,x)", "$(and x,)", "$(and x,$(future-function x),y)",
	} {
		t.Run(condition, func(t *testing.T) {
			files := boundFiles("check:; go test ./$(if " + condition + ",a,b)\nall: check\n")
			files["a/a.go"] = "package a\n"
			files["b/b.go"] = "package b\n"
			graph := driverGraph(t, files, nil)
			selected := driverSelections(affected.Select(graph, []string{"b/b.go"}))
			wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			if condition == "$(and x,$(future-function x),y)" {
				selected = driverSelections(affected.Select(graph, []string{"a/a.go"}))
				wantDriverSelection(t, selected, []string{"driver:make:Makefile#check", makeAll, jobUnit}, nil)
			}
		})
	}
}

// AFP-V0-9051: certification requires the whole expanded read set to stay
// within recorded edges, including filesystem type and package resolution.
func TestDriverSourcePathCertification_AFP_V0_9051(t *testing.T) {
	for _, tc := range []struct {
		name, command, link, target string
		plain                       bool
	}{
		{"glob-link", "cat data/*.txt", "data/input.txt", "../core/core.go", false},
		{"glob-escaping-link", "cat data/*.txt", "data/input.txt", "../../outside", false},
		{"glob-unknown-form", "cat data/**/input.txt", "", "", false},
		{"glob-parent-traversal", "cat data/../data/*.txt", "", "", false},
		{"glob-directory-match", "cat data/*", "", "", false},
		{"glob-plain", "cat data/*.txt", "", "", true},
		{"package-link", "go test ./alias", "alias", "core", false},
		{"package-recursive-link", "go test ./alias/...", "alias", "core", false},
		{"package-link-parent", "go test ./alias/../core", "alias", "core", false},
		{"package-missing", "go test ./missing", "", "", false},
		{"package-unknown-form", "go test core@future", "", "", false},
		{"package-plain", "go test ./core", "", "", true},
		{"directory-link", "go -C alias test .", "alias", "core", false},
		{"directory-attached-link", "go -C=alias test", "alias", "core", false},
		{"directory-escaping-link", "go -C alias test .", "alias", "../outside", false},
		{"directory-dangling-link", "go -C alias test .", "alias", "missing", false},
		{"directory-unknown-form", "go -C core@future test .", "", "", false},
		{"directory-plain", "go -C core test .", "", "", true},
		{"cwd-link", "cd alias && go test .", "alias", "core", false},
		{"cwd-link-parent", "cd alias/.. && go test ./core", "alias", "core", false},
		{"cwd-unknown-form", "cd core@future && go test .", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			files := boundFiles("check:; $(if yes," + tc.command + "," + tc.command + ")\nall: check\n")
			files["go.mod"] = "module example.test/m\n\ngo 1.27\n"
			files["core/core.go"] = "package core\n"
			files["data/plain.txt"] = "plain\n"
			files["data/sub/INPUT"] = "nested\n"
			for name, body := range files {
				file := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.link != "" {
				if err := os.Symlink(tc.target, filepath.Join(root, filepath.FromSlash(tc.link))); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			graph, err := affected.Build(t.Context(), root, golang.Language{}, affected.Driver{})
			if err != nil {
				t.Fatal(err)
			}
			if got := computedSite(t, graph, "driver:make:Makefile#check"); got != !tc.plain {
				t.Fatalf("computed site %t; want %t", got, !tc.plain)
			}
			units := []string{"driver:make:Makefile#check", makeAll, jobUnit}
			if !tc.plain {
				for _, dirty := range []string{"core/core.go", sealedCEM} {
					selected := driverSelections(affected.Select(graph, []string{dirty}))
					wantDriverSelection(t, selected, units, nil)
				}
			} else {
				selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
				wantDriverSelection(t, selected, nil, units)
				dirty := "core/core.go"
				if tc.name == "glob-plain" {
					dirty = "data/plain.txt"
				}
				selected = driverSelections(affected.Select(graph, []string{dirty}))
				wantDriverSelection(t, selected, units, nil)
			}
		})
	}
}
