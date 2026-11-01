package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// goVersionCheck matches a toolchain check of the form
// `go env GOVERSION)" = "goX"` in a shell, Makefile or workflow line.
var goVersionCheck = regexp.MustCompile(`go env GOVERSION\)" = "(go[^"]+)"`)

// goModPin is the toolchain go.mod pins: its toolchain line, else its go
// directive spelled as a GOVERSION. A toolchain line that differs from the go
// directive is drift, since the repository pins one exact version.
func goModPin(goMod string) (string, []string) {
	var directive, toolchain string
	for _, line := range strings.Split(goMod, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			directive = "go" + fields[1]
		}
		if len(fields) == 2 && fields[0] == "toolchain" {
			toolchain = fields[1]
		}
	}
	switch {
	case toolchain != "" && directive != "" && toolchain != directive:
		return toolchain, []string{fmt.Sprintf("go.mod: toolchain %s differs from go directive %s", toolchain, directive)}
	case toolchain != "":
		return toolchain, nil
	case directive != "":
		return directive, nil
	}
	return "", []string{"go.mod: no go directive or toolchain line"}
}

// goVersionPinDrift names every GOVERSION check in files whose version is not
// go.mod's pin, and an AGENTS.md without such a check (ASD-V0-880).
func goVersionPinDrift(goMod string, files map[string]string) []string {
	pin, failures := goModPin(goMod)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		checks := 0
		for number, line := range strings.Split(files[name], "\n") {
			for _, match := range goVersionCheck.FindAllStringSubmatch(line, -1) {
				checks++
				if pin != "" && match[1] != pin {
					failures = append(failures, fmt.Sprintf("%s:%d: pins %s, go.mod pins %s", name, number+1, match[1], pin))
				}
			}
		}
		if name == "AGENTS.md" && checks == 0 {
			failures = append(failures, "AGENTS.md: no go env GOVERSION check in the Verify block")
		}
	}
	return failures
}

// TestASDV0880GoVersionPinsAgree is the go-version-pins-check doc gate
// (V1-3882): every repository toolchain check names go.mod's version.
func TestASDV0880GoVersionPinsAgree(t *testing.T) {
	read := func(path string) string {
		data, err := os.ReadFile(filepath.Join("..", "..", path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	files := map[string]string{}
	for _, path := range []string{"AGENTS.md", "README.md", "Makefile"} {
		files[path] = read(path)
	}
	workflows, err := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.yml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("workflows: %v %v", workflows, err)
	}
	for _, workflow := range workflows {
		path := filepath.ToSlash(strings.TrimPrefix(workflow, filepath.Join("..", "..")+string(filepath.Separator)))
		files[path] = read(path)
	}
	for _, failure := range goVersionPinDrift(read("go.mod"), files) {
		t.Errorf("go-version-pins-check: FAIL %s (fix: make every GOVERSION check name the go.mod version)", failure)
	}
}

// TestASDV0880GoVersionPinDriftNamesEveryMismatch proves the gate fails on
// each drift it exists for and passes agreeing pins.
func TestASDV0880GoVersionPinDriftNamesEveryMismatch(t *testing.T) {
	check := func(version string) string {
		return "test \"$(GOTOOLCHAIN=local go env GOVERSION)\" = \"" + version + "\"\n"
	}
	agree := map[string]string{"AGENTS.md": "```sh\n" + check("go1.27.3") + "```\n", "Makefile": "go-version:\n\t" + strings.ReplaceAll(check("go1.27.3"), "$(", "$$(")}
	if got := goVersionPinDrift("module m\n\ngo 1.27.3\n", agree); len(got) != 0 {
		t.Fatalf("agreeing pins: %v", got)
	}
	cases := map[string]struct {
		goMod string
		files map[string]string
		want  string
	}{
		"stale AGENTS.md":    {"go 1.27.3\n", map[string]string{"AGENTS.md": "x\n" + check("go1.27.2")}, "AGENTS.md:2: pins go1.27.2, go.mod pins go1.27.3"},
		"stale workflow":     {"go 1.27.3\n", map[string]string{"AGENTS.md": check("go1.27.3"), ".github/workflows/ci.yml": "      run: |\n        " + check("go1.27.1")}, ".github/workflows/ci.yml:2: pins go1.27.1, go.mod pins go1.27.3"},
		"toolchain line":     {"go 1.27.3\ntoolchain go1.27.4\n", map[string]string{"AGENTS.md": check("go1.27.4")}, "go.mod: toolchain go1.27.4 differs from go directive go1.27.3"},
		"no AGENTS.md check": {"go 1.27.3\n", map[string]string{"AGENTS.md": "no check\n"}, "AGENTS.md: no go env GOVERSION check in the Verify block"},
		"no go.mod pin":      {"module m\n", map[string]string{"AGENTS.md": check("go1.27.3")}, "go.mod: no go directive or toolchain line"},
	}
	for name, c := range cases {
		got := goVersionPinDrift(c.goMod, c.files)
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: drift = %q, want [%q]", name, got, c.want)
		}
	}
}
