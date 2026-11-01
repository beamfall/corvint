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
var goVersionCheck = regexp.MustCompile(`go env GOVERSION[^)]*\)" = "(go[^"]+)"`)

// goVersionAssign matches `NAME=$(... go env GOVERSION ...)`, whose variable a
// later `"$NAME" != "goX"` comparison checks (goVersionCompare).
var goVersionAssign = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)=\$\([^)]*go env GOVERSION[^)]*\)`)

var goVersionCompare = regexp.MustCompile(`"\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?" *(?:!=|==|=) *"(go[^"]+)"`)

// goModPin is the toolchain go.mod pins: its toolchain line, else its go
// directive spelled as a GOVERSION. A toolchain line that differs from the go
// directive is drift, since the repository pins one exact version.
func goModPin(goMod string) (string, []string) {
	var directive, toolchain string
	for _, line := range strings.Split(goMod, "\n") {
		if comment := strings.Index(line, "//"); comment >= 0 {
			line = line[:comment]
		}
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
		pins := func(number int, version string) {
			checks++
			if pin != "" && version != pin {
				failures = append(failures, fmt.Sprintf("%s:%d: pins %s, go.mod pins %s", name, number+1, version, pin))
			}
		}
		// assigned maps a variable holding go env GOVERSION to its line, until
		// a comparison checks it; one left unchecked is a pin the gate cannot read.
		assigned := map[string]int{}
		for number, line := range strings.Split(files[name], "\n") {
			direct := goVersionCheck.FindAllStringSubmatch(line, -1)
			for _, match := range direct {
				pins(number, match[1])
			}
			assignments := goVersionAssign.FindAllStringSubmatch(line, -1)
			for _, match := range assignments {
				assigned[match[1]] = number + 1
			}
			for _, match := range goVersionCompare.FindAllStringSubmatch(line, -1) {
				if _, ok := assigned[match[1]]; ok {
					delete(assigned, match[1])
					pins(number, match[2])
				}
			}
			if strings.Contains(line, "go env GOVERSION") && len(direct) == 0 && len(assignments) == 0 {
				failures = append(failures, fmt.Sprintf("%s:%d: go env GOVERSION check in a form the gate cannot read", name, number+1))
			}
		}
		variables := make([]string, 0, len(assigned))
		for variable := range assigned {
			variables = append(variables, variable)
		}
		sort.Strings(variables)
		for _, variable := range variables {
			failures = append(failures, fmt.Sprintf("%s:%d: %s holds go env GOVERSION but no comparison names a version", name, assigned[variable], variable))
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
	agree[".github/workflows/ci.yml"] = "go_found=$(go env GOVERSION 2> /dev/null || true)\nif [ \"$go_found\" != \"go1.27.3\" ]; then\n"
	if got := goVersionPinDrift("module m\n\ngo 1.27.3 // pin\n", agree); len(got) != 0 {
		t.Fatalf("agreeing pins: %v", got)
	}
	cases := map[string]struct {
		goMod string
		files map[string]string
		want  string
	}{
		"stale AGENTS.md":     {"go 1.27.3\n", map[string]string{"AGENTS.md": "x\n" + check("go1.27.2")}, "AGENTS.md:2: pins go1.27.2, go.mod pins go1.27.3"},
		"stale workflow":      {"go 1.27.3\n", map[string]string{"AGENTS.md": check("go1.27.3"), ".github/workflows/ci.yml": "      run: |\n        " + check("go1.27.1")}, ".github/workflows/ci.yml:2: pins go1.27.1, go.mod pins go1.27.3"},
		"toolchain line":      {"go 1.27.3\ntoolchain go1.27.4\n", map[string]string{"AGENTS.md": check("go1.27.4")}, "go.mod: toolchain go1.27.4 differs from go directive go1.27.3"},
		"no AGENTS.md check":  {"go 1.27.3\n", map[string]string{"AGENTS.md": "no check\n"}, "AGENTS.md: no go env GOVERSION check in the Verify block"},
		"no go.mod pin":       {"module m\n", map[string]string{"AGENTS.md": check("go1.27.3")}, "go.mod: no go directive or toolchain line"},
		"commented go line":   {"go 1.27.3 // pin\n", map[string]string{"AGENTS.md": check("go1.27.2")}, "AGENTS.md:1: pins go1.27.2, go.mod pins go1.27.3"},
		"commented toolchain": {"go 1.27.3\ntoolchain go1.27.4 // pin\n", map[string]string{"AGENTS.md": check("go1.27.4")}, "go.mod: toolchain go1.27.4 differs from go directive go1.27.3"},
		"stale variable pin":  {"go 1.27.3\n", map[string]string{"AGENTS.md": check("go1.27.3"), ".github/workflows/ci.yml": "      run: |\n        " + check("go1.27.3") + "        go_found=$(/usr/local/go/bin/go env GOVERSION 2> /dev/null || true)\n        if [ \"$go_found\" != \"go1.27.2\" ]; then\n"}, ".github/workflows/ci.yml:4: pins go1.27.2, go.mod pins go1.27.3"},
		"unchecked variable":  {"go 1.27.3\n", map[string]string{"AGENTS.md": check("go1.27.3"), ".github/workflows/ci.yml": "found=$(go env GOVERSION)\necho \"$found\"\n"}, ".github/workflows/ci.yml:1: found holds go env GOVERSION but no comparison names a version"},
		"unreadable form":     {"go 1.27.3\n", map[string]string{"AGENTS.md": check("go1.27.3"), ".github/workflows/ci.yml": "[ \"$(go env GOVERSION)\" == go1.27.2 ]\n"}, ".github/workflows/ci.yml:1: go env GOVERSION check in a form the gate cannot read"},
	}
	for name, c := range cases {
		got := goVersionPinDrift(c.goMod, c.files)
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: drift = %q, want [%q]", name, got, c.want)
		}
	}
}
