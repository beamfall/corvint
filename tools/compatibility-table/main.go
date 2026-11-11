// SPDX-License-Identifier: Apache-2.0
// compatibility-table generates docs/COMPATIBILITY.md, the Corvint 1.x compatibility promise, from
// the surface list tools/compatibility-table/surfaces.json, and checks the committed page
// (CMP-V1-001 to CMP-V1-006, docs/specs/compatibility-promise-v1.md).
//
//	compatibility-table [--root DIR] [--write]
//
// Without --write it checks: the list names exactly the CMP-V1-003 and CMP-V1-005 surfaces, each
// row's stability follows from its class and contract tests, every contract test it names exists,
// every format label it names is a string literal in its label source, and the committed page is
// byte-identical to the generated one. With --write it runs the same list checks and then writes
// the page.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	listPath    = "tools/compatibility-table/surfaces.json"
	pagePath    = "docs/COMPATIBILITY.md"
	preview     = "preview"
	stable      = "stable"
	notPromised = "not-promised"

	// Classes of a CMP-V1-003 surface.
	inRepo    = "in-repo"    // ships from this repository today
	notYet    = "not-yet"    // does not exist yet: a named change creates it
	otherRepo = "other-repo" // another repository owns it
)

// List is the surface list: one entry per CMP-V1-003 or CMP-V1-005 surface, in page order.
type List struct {
	Surfaces []Surface `json:"surfaces"`
}

// Surface is one row of the page. Its stability must be the one its class and tests imply
// (wantStability): a CMP-V1-003 surface that ships from this repository is stable exactly when it
// names a contract test (CMP-V1-002), one that does not exist yet or that another repository owns
// is not-promised and names what it waits on (CMP-V1-003), and a CMP-V1-005 surface is preview.
type Surface struct {
	ID          string `json:"id"`
	Surface     string `json:"surface"`
	Kind        string `json:"kind"`
	Class       string `json:"class,omitempty"`
	Stability   string `json:"stability"`
	Label       string `json:"label,omitempty"`
	LabelSource string `json:"labelSource,omitempty"`
	Tests       []Test `json:"tests,omitempty"`
	WaitsOn     string `json:"waitsOn,omitempty"`
	Notes       string `json:"notes"`
}

// Test names one Go test function: the package directory, relative to the repository root and
// spelled "./dir", and the function name.
type Test struct {
	Package string `json:"package"`
	Name    string `json:"name"`
}

// group says which requirement lists a surface.
type group int

const (
	stableSet  group = iota + 1 // CMP-V1-003: stable when a contract test covers it
	previewSet                  // CMP-V1-005: preview by name
)

// catalogue is every surface CMP-V1-003 and CMP-V1-005 name, by list ID. The list must hold
// exactly these IDs; an ID outside it is an unlisted surface.
var catalogue = map[string]group{
	// CMP-V1-003 item 1: the supported-command tables.
	"corvint-verbs":            stableSet,
	"corvint-flags":            stableSet,
	"corvint-exit-codes":       stableSet,
	"corvint-docs-verbs":       stableSet,
	"corvint-docs-flags":       stableSet,
	"corvint-docs-exit-codes":  stableSet,
	"corvint-tasks-verbs":      stableSet,
	"corvint-tasks-flags":      stableSet,
	"corvint-tasks-exit-codes": stableSet,
	// CMP-V1-003 item 2: the documented --json formats.
	"flow-file":            stableSet,
	"plan":                 stableSet,
	"test-run-receipt":     stableSet,
	"run-evidence":         stableSet,
	"ticket-schema-v1":     stableSet,
	"receipt-reference-v1": stableSet,
	// CMP-V1-003 item 3: the documented files.
	"corvint-dir-files": stableSet,
	"tasks-store-files": stableSet,
	// CMP-V1-003 item 4: the action and the agent packages.
	"action-inputs":               stableSet,
	"action-outputs":              stableSet,
	"action-major-tag":            stableSet,
	"agent-package-version-range": stableSet,
	// CMP-V1-003 item 5: the MCP tool names.
	"mcp-tool-names": stableSet,
	// CMP-V1-005: preview by name.
	"feature-map":         previewSet,
	"data-map":            previewSet,
	"change-report":       previewSet,
	"review-check":        previewSet,
	"flow-link-proposals": previewSet,
	"harness-event":       previewSet,
	"mcp-schemas":         previewSet,
	"go-packages":         previewSet,
	"index":               previewSet,
	"caches":              previewSet,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compatibility-table", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	write := flags.Bool("write", false, "write "+pagePath+" instead of checking it")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "usage: compatibility-table [--root DIR] [--write]\n"+
			"Checks (or with --write, regenerates) %s from %s (CMP-V1-001 to CMP-V1-006).\n"+
			"example: go run ./tools/compatibility-table --write\n", pagePath, listPath)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	list, err := readList(*root)
	if err != nil {
		fmt.Fprintf(stderr, "compatibility-check: FAIL %v\n", err)
		return 1
	}
	if problems := validate(*root, list); len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintf(stderr, "compatibility-check: FAIL %s\n", problem)
		}
		return 1
	}
	page := render(list)
	target := filepath.Join(*root, filepath.FromSlash(pagePath))
	if *write {
		if err := os.WriteFile(target, page, 0o644); err != nil {
			fmt.Fprintf(stderr, "compatibility-table: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "compatibility-table: wrote %s (%s)\n", pagePath, counts(list))
		return 0
	}
	committed, err := os.ReadFile(target)
	if err != nil {
		fmt.Fprintf(stderr, "compatibility-check: FAIL %s: %v; fix: make compatibility-table\n", pagePath, err)
		return 1
	}
	if !bytes.Equal(committed, page) {
		fmt.Fprintf(stderr, "compatibility-check: FAIL %s differs from the generated page at line %d; the page is generated, never edited by hand (CMP-V1-002); fix: edit %s, then make compatibility-table\n", pagePath, firstDifference(committed, page), listPath)
		return 1
	}
	fmt.Fprintf(stdout, "compatibility-check: PASS %s (%s)\n", pagePath, counts(list))
	return 0
}

func readList(root string) (List, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(listPath)))
	if err != nil {
		return List{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var list List
	if err := decoder.Decode(&list); err != nil {
		return List{}, fmt.Errorf("%s: %v", listPath, err)
	}
	if decoder.More() {
		return List{}, fmt.Errorf("%s: trailing data after the list", listPath)
	}
	return list, nil
}

// wantStability is the stability a row's group, class and tests imply, or "" when the class is
// not one of the three a CMP-V1-003 row takes.
func wantStability(surface Surface) string {
	switch {
	case catalogue[surface.ID] == previewSet:
		return preview
	case surface.Class == inRepo && len(surface.Tests) > 0:
		return stable
	case surface.Class == inRepo:
		return preview
	case surface.Class == notYet || surface.Class == otherRepo:
		return notPromised
	}
	return ""
}

// validate returns every problem with the list, each naming its row.
func validate(root string, list List) []string {
	var problems []string
	seen := map[string]bool{}
	for _, surface := range list.Surfaces {
		row := fmt.Sprintf("row %q", surface.ID)
		if seen[surface.ID] {
			problems = append(problems, row+": listed twice")
			continue
		}
		seen[surface.ID] = true
		which, known := catalogue[surface.ID]
		switch {
		case !known:
			problems = append(problems, row+": unlisted surface: CMP-V1-003 and CMP-V1-005 name no such surface, so it cannot have a row; fix: amend the spec first, then add the ID to the catalogue")
			continue
		case which == previewSet && len(surface.Tests) > 0:
			problems = append(problems, row+": a CMP-V1-005 surface is preview by name and names no contract test")
		case which == previewSet && (surface.Class != "" || surface.WaitsOn != ""):
			problems = append(problems, row+": a CMP-V1-005 surface takes no class and no waitsOn")
		case which == stableSet && wantStability(surface) == "":
			problems = append(problems, row+": class must be "+inRepo+", "+notYet+" or "+otherRepo)
		case surface.Class == inRepo && surface.WaitsOn != "":
			problems = append(problems, row+": a surface that ships from this repository waits on nothing")
		case (surface.Class == notYet || surface.Class == otherRepo) && len(surface.Tests) > 0:
			problems = append(problems, row+": a not-promised surface names no contract test")
		case (surface.Class == notYet || surface.Class == otherRepo) && strings.TrimSpace(surface.WaitsOn) == "":
			problems = append(problems, row+": a not-promised row names the change or repository it waits on (CMP-V1-003)")
		}
		if want := wantStability(surface); want != "" && surface.Stability != want {
			switch {
			case surface.Stability == stable && surface.Class == inRepo:
				problems = append(problems, row+": stable row names no contract test (CMP-V1-002)")
			default:
				problems = append(problems, fmt.Sprintf("%s: stability %q, but its class and tests make it %q", row, surface.Stability, want))
			}
		}
		if strings.TrimSpace(surface.Surface) == "" || strings.TrimSpace(surface.Kind) == "" || strings.TrimSpace(surface.Notes) == "" {
			problems = append(problems, row+": surface, kind and notes must be non-empty")
		}
		for _, cell := range []string{surface.Surface, surface.Kind, surface.Label, surface.WaitsOn, surface.Notes} {
			if strings.ContainsAny(cell, "|\n\r") {
				problems = append(problems, row+": a cell holds a pipe or a line break")
				break
			}
		}
		for _, test := range surface.Tests {
			if problem := testExists(root, test); problem != "" {
				problems = append(problems, row+": stable row names contract test "+test.Name+" in "+test.Package+", which "+problem+" (CMP-V1-002)")
			}
		}
		if (surface.Label == "") != (surface.LabelSource == "") {
			problems = append(problems, row+": label and labelSource go together")
		} else if surface.Label != "" {
			if problem := labelInSource(root, surface.Label, surface.LabelSource); problem != "" {
				problems = append(problems, row+": label "+surface.Label+" "+problem+" (CMP-V1-003)")
			}
		}
	}
	var missing []string
	for id := range catalogue {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		problems = append(problems, fmt.Sprintf("row %q: missing: every CMP-V1-003 and CMP-V1-005 surface needs a row", id))
	}
	return problems
}

// testExists returns "" when dir holds a _test.go file declaring func name(t *testing.T), or
// what is wrong otherwise.
func testExists(root string, test Test) string {
	dir, ok := packageDir(test.Package)
	if !ok {
		return "is not a clean ./ package path inside the repository"
	}
	if !strings.HasPrefix(test.Name, "Test") {
		return "is not a Test function name"
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return "does not exist: no such package directory"
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(files, filepath.Join(root, filepath.FromSlash(dir), entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return "cannot be confirmed: " + err.Error()
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == test.Name && fn.Type.Params != nil && len(fn.Type.Params.List) == 1 {
				return ""
			}
		}
	}
	return "does not exist"
}

// packageDir returns the slash path of "./dir" when it is a clean relative path inside the root.
func packageDir(spelled string) (string, bool) {
	dir, ok := strings.CutPrefix(spelled, "./")
	if !ok || dir == "" || path.Clean(dir) != dir || dir == ".." || strings.HasPrefix(dir, "../") || strings.Contains(dir, "\\") {
		return "", false
	}
	return dir, true
}

// labelInSource returns "" when the non-test Go file source holds label as a string literal.
func labelInSource(root, label, source string) string {
	dir, ok := packageDir("./" + source)
	if !ok || !strings.HasSuffix(dir, ".go") || strings.HasSuffix(dir, "_test.go") {
		return "names label source " + source + ", which is not a clean non-test .go file path"
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return "names label source " + source + ", which does not exist"
	}
	if !bytes.Contains(data, []byte(strconv.Quote(label))) && !bytes.Contains(data, []byte("`"+label+"`")) {
		return "is not a string literal in " + source
	}
	return ""
}

func counts(list List) string {
	var stableRows, untested, waiting, previews int
	for _, surface := range list.Surfaces {
		switch {
		case catalogue[surface.ID] == previewSet:
			previews++
		case surface.Stability == stable:
			stableRows++
		case surface.Stability == notPromised:
			waiting++
		default:
			untested++
		}
	}
	return fmt.Sprintf("CMP-V1-003 surfaces: %d stable, %d preview without a contract test, %d not-promised; CMP-V1-005 preview surfaces: %d", stableRows, untested, waiting, previews)
}

func firstDifference(a, b []byte) int {
	line := 1
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return line
		}
		if a[i] == '\n' {
			line++
		}
	}
	return line
}

// render returns the page: the rules (CMP-V1-001, CMP-V1-004 to CMP-V1-006) above one table.
func render(list List) []byte {
	var page strings.Builder
	page.WriteString(header)
	page.WriteString("| Surface | Kind | Stability | Contract tests | Waits on | Notes |\n")
	page.WriteString("|---|---|---|---|---|---|\n")
	for _, surface := range list.Surfaces {
		tests, waits := "none", "nothing"
		if len(surface.Tests) > 0 {
			var names []string
			for _, test := range surface.Tests {
				names = append(names, fmt.Sprintf("`%s` in `%s`", test.Name, test.Package))
			}
			tests = strings.Join(names, "; ")
		}
		if surface.WaitsOn != "" {
			waits = surface.WaitsOn
		}
		name := surface.Surface
		if surface.Label != "" {
			name += fmt.Sprintf(" (label `%s`)", surface.Label)
		}
		fmt.Fprintf(&page, "| %s | %s | %s | %s | %s | %s |\n", name, surface.Kind, surface.Stability, tests, waits, surface.Notes)
	}
	return []byte(page.String())
}

const header = `<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Generated by make compatibility-table from tools/compatibility-table/surfaces.json; do not edit. -->
# Compatibility promise for Corvint 1.x

This page is the whole 1.x compatibility promise: one table, one row per surface, and a stability
column that reads ` + "`stable`" + `, ` + "`preview`" + ` or ` + "`not-promised`" + `. It is generated from a list that
maps each surface to the contract tests that protect it; ` + "`make compatibility-check`" + ` fails when
this page differs from the generated one, a named test does not exist or a row's stability does not
follow from its tests. The contract is [Compatibility Promise V1](specs/compatibility-promise-v1.md)
(CMP-V1-001 to CMP-V1-006).

## Stable, preview and not-promised

- A row is ` + "`stable`" + ` only when a contract test covers the surface, and the row names that test.
  A listed surface that ships from this repository but that no contract test covers has a
  ` + "`preview`" + ` row.
- A ` + "`not-promised`" + ` row is a listed surface that does not exist yet or that another repository
  owns. It names the change or repository it waits on, and 1.x promises nothing about it until a
  later release makes the row ` + "`stable`" + `.
- A ` + "`stable`" + ` surface changes only by addition within 1.x. A release that adds a field to a
  stable format keeps every documented field's name, type and meaning.
- Removing or renaming a stable verb, flag, exit code, field, file, input, output or tool name is
  announced one minor release before it happens, and the old name keeps working as an alias, with
  a deprecation notice, during that release: a flag deprecated in 1.3 still works in 1.3 and is
  removed no earlier than 1.4.
- A ` + "`preview`" + ` surface may change in any release; the release notes name the change.
- A surface that has no row is ` + "`preview`" + `.
- These are ` + "`preview`" + ` by name: the formats ` + "`corvint-feature-map/0`" + `, ` + "`corvint-data-map/0`" + `,
  ` + "`corvint-change-report/0`" + `, ` + "`corvint-review-check/0`" + `, ` + "`corvint-flow-link-proposals/0`" + ` and
  ` + "`corvint-harness-event/0`" + `; the MCP input and output schemas; the Go packages; the index; and
  every cache. An MCP tool name can be ` + "`stable`" + ` while its input schema is ` + "`preview`" + `.
- The promise ends with 1.x.

## Labels

No profile label is renamed at 1.0. The digit in a label such as ` + "`name/0`" + ` is part of the name,
not a stability mark: the stability column carries that meaning, so a stable format whose label
ends in ` + "`/0`" + ` is stable. A breaking change to a stable profile takes the next digit: a release
that needs one writes ` + "`name/1`" + ` and still serves ` + "`name/0`" + ` for one minor release.

## Surfaces

`
