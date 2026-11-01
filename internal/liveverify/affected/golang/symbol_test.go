package golang_test

import (
	"bufio"
	"bytes"
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// symbolHub is the base tree of the symbol-level fixture: hub's exported
// Shout reaches the unexported helper, Counter reaches the unexported
// counter type and its bump method through wrap, and Quiet and Size reach
// neither. Each importer names one of them; leaf and quietleaf import an
// importer.
const symbolHub = `package hub

import "strings"

func helper(s string) string { return strings.ToUpper(s) }

// Shout reaches helper.
func Shout(s string) string { return helper(s) }

// Quiet reaches nothing that changes.
func Quiet(s string) string { return strings.ToLower(s) }

type counter struct{ n int }

func (c *counter) bump() int { c.n++; return c.n }

type wrap struct{ c *counter }

func (w *wrap) Bump() int { return w.c.bump() }

// Counter hands out a counter behind an interface.
func Counter() interface{ Bump() int } { return &wrap{c: &counter{}} }

// Size is a constant.
const Size = 4
`

var symbolBase = map[string]string{
	"go.mod":                      "module example.test/m\n\ngo 1.21\n",
	"hub/hub.go":                  symbolHub,
	"hub/hub_test.go":             "package hub\n\nimport \"testing\"\n\nfunc TestHub(t *testing.T) { _ = Shout(\"a\") }\n",
	"shout/shout.go":              "package shout\n\nimport \"example.test/m/hub\"\n\nfunc Loud() string { return hub.Shout(\"a\") }\n",
	"shout/shout_test.go":         "package shout\n\nimport \"testing\"\n\nfunc TestLoud(t *testing.T) { _ = Loud() }\n",
	"quiet/quiet.go":              "package quiet\n\nimport \"example.test/m/hub\"\n\nfunc Soft() string { return hub.Quiet(\"A\") }\n",
	"quiet/quiet_test.go":         "package quiet\n\nimport \"testing\"\n\nfunc TestSoft(t *testing.T) { _ = Soft() }\n",
	"count/count.go":              "package count\n\nimport \"example.test/m/hub\"\n\nfunc Twice() int { c := hub.Counter(); c.Bump(); return c.Bump() }\n",
	"count/count_test.go":         "package count\n\nimport \"testing\"\n\nfunc TestTwice(t *testing.T) { _ = Twice() }\n",
	"size/size.go":                "package size\n\nimport \"example.test/m/hub\"\n\nvar Buffer [hub.Size]byte\n",
	"size/size_test.go":           "package size\n\nimport \"testing\"\n\nfunc TestBuffer(t *testing.T) { _ = len(Buffer) }\n",
	"leaf/leaf.go":                "package leaf\n\nimport \"example.test/m/shout\"\n\nfunc Echo() string { return shout.Loud() }\n",
	"leaf/leaf_test.go":           "package leaf\n\nimport \"testing\"\n\nfunc TestEcho(t *testing.T) { _ = Echo() }\n",
	"quietleaf/quietleaf.go":      "package quietleaf\n\nimport \"example.test/m/quiet\"\n\nfunc Hush() string { return quiet.Soft() }\n",
	"quietleaf/quietleaf_test.go": "package quietleaf\n\nimport \"testing\"\n\nfunc TestHush(t *testing.T) { _ = Hush() }\n",
}

// symbolMutations are body edits to one hub declaration each, with the
// linked symbol a test binary must hold to run it (empty for a constant) and
// the selection the symbol-level rules give.
var symbolMutations = []struct {
	name, from, to, symbol, selected string
}{
	{name: "unexported func body", from: "strings.ToUpper(s)", to: "strings.ToUpper(s + \"!\")", symbol: "example.test/m/hub.helper", selected: "hub leaf shout"},
	{name: "unexported method body", from: "c.n++; return c.n", to: "c.n += 2; return c.n", symbol: "example.test/m/hub.(*counter).bump", selected: "count hub"},
	{name: "exported func body", from: "strings.ToLower(s)", to: "strings.ToLower(s + \"?\")", symbol: "example.test/m/hub.Quiet", selected: "hub quiet quietleaf"},
	{name: "constant value", from: "Size = 4", to: "Size = 8", selected: "hub size"},
}

// symbolRun writes base with head's edits applied over the tree, builds the
// graph and returns the records for dirty with the plan they give.
func symbolRun(t *testing.T, base, head map[string]string, dirty ...string) (affected.GoAdditive, affected.Plan) {
	t.Helper()
	root := t.TempDir()
	files := maps.Clone(base)
	baseFS := fstest.MapFS{}
	for relative, body := range base {
		baseFS[relative] = &fstest.MapFile{Data: []byte(body)}
	}
	for relative, body := range head {
		if body == additiveDeleted {
			delete(files, relative)
			continue
		}
		files[relative] = body
	}
	writeFiles(t, root, files)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	records := golang.AdditiveRecords(baseFS, os.DirFS(root), graph, dirty)
	plan := affected.Select(graph.WithGoAdditive(records), dirty)
	return additiveRecordFor(t, records, "go:example.test/m/hub"), plan
}

func symbolEdit(t *testing.T, from, to string) map[string]string {
	t.Helper()
	if !strings.Contains(symbolHub, from) {
		t.Fatalf("mutation %q is not in the hub", from)
	}
	return map[string]string{"hub/hub.go": strings.Replace(symbolHub, from, to, 1)}
}

// TestADNV0660BodyEditSelectsOnlyReachingImporters: a body edit to one
// declaration of a hub selects the hub and the importers naming a
// declaration that reaches it, each with its witness, and leaves the others
// out as GoAdditiveExcluded (ADN-V0-660, ADN-V0-661; V1-3282 AC1 and AC2).
func TestADNV0660BodyEditSelectsOnlyReachingImporters(t *testing.T) {
	for _, mutation := range symbolMutations {
		t.Run(mutation.name, func(t *testing.T) {
			record, plan := symbolRun(t, symbolBase, symbolEdit(t, mutation.from, mutation.to), "hub/hub.go")
			if record.State != affected.GoAdditiveNarrowed || record.Mode != golang.GoSymbolMode {
				t.Fatalf("record %+v, want NARROWED %s", record, golang.GoSymbolMode)
			}
			if got := trimGo(selectedIDs(plan)); got != mutation.selected {
				t.Fatalf("selection %q, want %q (record %+v)", got, mutation.selected, record)
			}
			for _, selection := range plan.Selected {
				if selection.Witness.Kind == "" || selection.Witness.DirtyPath != "hub/hub.go" {
					t.Fatalf("selection %+v names no reason", selection)
				}
			}
			if len(plan.GoAdditiveExcluded) == 0 {
				t.Fatalf("no importer was left out: %+v", plan)
			}
		})
	}
}

// TestADNV0660ReceiverRule: an edited unexported method makes its receiver
// type reach the change, so the exported constructor that mentions the type
// is exposed and the others are not (ADN-V0-661).
func TestADNV0660ReceiverRule(t *testing.T) {
	record, _ := symbolRun(t, symbolBase, symbolEdit(t, "c.n++; return c.n", "c.n += 2; return c.n"), "hub/hub.go")
	if got := strings.Join(record.Declarations, " "); got != "counter.bump" {
		t.Fatalf("declarations %q, want counter.bump", got)
	}
	// Method names are not exposed: a value of the receiver type reaches an
	// importer only through a declaration that mentions the type.
	if got := strings.Join(record.Exposed, " "); got != "Counter" {
		t.Fatalf("exposed %q, want Counter", got)
	}
	if got := trimGo(record.Referencing); got != "count" {
		t.Fatalf("referencing %q, want count", got)
	}
}

// TestADNV0660DeletedDeclaration: removing an unexported declaration no one
// names reaches no importer; the hub's own tests still run.
func TestADNV0660DeletedDeclaration(t *testing.T) {
	base := maps.Clone(symbolBase)
	base["hub/hub.go"] = symbolHub + "\nfunc unused() {}\n"
	record, plan := symbolRun(t, base, map[string]string{"hub/hub.go": symbolHub}, "hub/hub.go")
	if record.State != affected.GoAdditiveNarrowed || strings.Join(record.Declarations, " ") != "unused" || len(record.Referencing) != 0 {
		t.Fatalf("record %+v, want NARROWED unused with no referencing importer", record)
	}
	if got := trimGo(selectedIDs(plan)); got != "hub" {
		t.Fatalf("selection %q, want hub", got)
	}
}

// TestADNV0662SymbolFallbacks: a change the symbol rules cannot bound is a
// FALLBACK record with its reason and the package-level selection.
func TestADNV0662SymbolFallbacks(t *testing.T) {
	helperEdit := func(hub string) string {
		return strings.Replace(hub, "strings.ToUpper(s)", "strings.ToUpper(s + \"!\")", 1)
	}
	for _, test := range []struct {
		name   string
		extra  map[string]string
		hubAdd string
		reason string
	}{
		{name: "init reaches", hubAdd: "\nfunc init() { _ = Shout(\"x\") }\n", reason: golang.SymbolAlwaysRun},
		{name: "var initializer reaches", hubAdd: "\nvar loud = helper(\"x\")\n", reason: golang.SymbolAlwaysRun},
		{name: "bodyless func", hubAdd: "\nfunc external() int\n", reason: golang.SymbolUnmodelled},
		{name: "assembly", extra: map[string]string{"hub/hub_amd64.s": "TEXT ·none(SB),0,$0\n\tRET\n"}, reason: golang.SymbolUnmodelled},
		{name: "linkname in hub", extra: map[string]string{"hub/link.go": "package hub\n\nimport _ \"unsafe\"\n\n//go:linkname now runtime.nanotime\nfunc now() int64 { return 0 }\n"}, reason: golang.SymbolUnmodelled},
		{name: "linkname in dependent", extra: map[string]string{"pull/pull.go": "package pull\n\nimport (\n\t_ \"unsafe\"\n\n\t_ \"example.test/m/quiet\"\n)\n\n//go:linkname helper example.test/m/hub.helper\nfunc helper(string) string\n"}, reason: golang.SymbolUnmodelledIn},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := maps.Clone(symbolBase)
			maps.Copy(base, test.extra)
			base["hub/hub.go"] = symbolHub + test.hubAdd
			record, plan := symbolRun(t, base, map[string]string{"hub/hub.go": helperEdit(base["hub/hub.go"])}, "hub/hub.go")
			if record.State != affected.GoAdditiveFallback || !strings.HasPrefix(record.Reason, test.reason) {
				t.Fatalf("record %+v, want FALLBACK %s", record, test.reason)
			}
			if len(plan.GoAdditiveExcluded) != 0 {
				t.Fatalf("fallback record excluded %+v", plan.GoAdditiveExcluded)
			}
		})
	}
	t.Run("reorder only", func(t *testing.T) {
		reordered := strings.Replace(symbolHub, "// Size is a constant.\nconst Size = 4\n", "", 1)
		reordered = strings.Replace(reordered, "import \"strings\"\n", "import \"strings\"\n\n// Size is a constant.\nconst Size = 4\n", 1)
		record, _ := symbolRun(t, symbolBase, map[string]string{"hub/hub.go": reordered}, "hub/hub.go")
		if record.State != affected.GoAdditiveFallback || record.Reason != golang.AdditiveDeclarationEdited {
			t.Fatalf("record %+v, want FALLBACK %s", record, golang.AdditiveDeclarationEdited)
		}
	})
}

// TestADNV0663SymbolSelectionCoversLinkedTestBinaries is the differential
// oracle (ADN-V0-663; V1-2744 AC1, AC3): every fixture package's test binary
// is linked by the Go toolchain with inlining off, and for each body edit
// every package whose binary links the edited symbol must be selected. The
// linker drops what no root reaches, so a linked symbol is the
// toolchain's own over-approximation of the binaries that can run the
// change; the oracle checks recall against it and that the selection still
// leaves at least one package out.
func TestADNV0663SymbolSelectionCoversLinkedTestBinaries(t *testing.T) {
	if testing.Short() {
		t.Skip("links test binaries")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain is unavailable")
	}
	root := t.TempDir()
	writeFiles(t, root, symbolBase)
	out := t.TempDir()
	linked := map[string]map[string]bool{}
	for relative := range symbolBase {
		if !strings.HasSuffix(relative, "_test.go") {
			continue
		}
		pkg := filepath.Dir(relative)
		binary := filepath.Join(out, pkg+".test")
		build := exec.Command("go", "test", "-c", "-gcflags=all=-l", "-o", binary, "./"+pkg)
		build.Dir = root
		build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=0")
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go test -c ./%s: %v: %s", pkg, err, output)
		}
		nm := exec.Command("go", "tool", "nm", binary)
		output, err := nm.Output()
		if err != nil {
			t.Fatalf("go tool nm %s: %v", binary, err)
		}
		symbols := map[string]bool{}
		scanner := bufio.NewScanner(bytes.NewReader(output))
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 3 {
				symbols[fields[len(fields)-1]] = true
			}
		}
		linked[pkg] = symbols
	}
	packages := slices.Sorted(maps.Keys(linked))
	recalled, observed, excluded := 0, 0, 0
	for _, mutation := range symbolMutations {
		if mutation.symbol == "" {
			continue
		}
		_, plan := symbolRun(t, symbolBase, symbolEdit(t, mutation.from, mutation.to), "hub/hub.go")
		selected := map[string]bool{}
		for _, id := range selectedIDs(plan) {
			selected[strings.TrimPrefix(id, "go:example.test/m/")] = true
		}
		for _, pkg := range packages {
			if !linked[pkg][mutation.symbol] {
				if !selected[pkg] {
					excluded++
				}
				continue
			}
			observed++
			if !selected[pkg] {
				t.Errorf("%s: the %s test binary links %s but the selection %v leaves it out", mutation.name, pkg, mutation.symbol, slices.Sorted(maps.Keys(selected)))
				continue
			}
			recalled++
		}
	}
	if observed == 0 || excluded == 0 {
		t.Fatalf("oracle observed %d linked binaries and %d exclusions; want both nonzero", observed, excluded)
	}
	t.Logf("recall %d/%d linked test binaries; %d unlinked binaries excluded", recalled, observed, excluded)
}

// TestADNV0662LinknameElsewhereKeepsNarrowing: a dependent's directive that
// pulls another package's symbol, and the directive's text in a string
// literal of the hub, leave the record NARROWED.
func TestADNV0662LinknameElsewhereKeepsNarrowing(t *testing.T) {
	base := maps.Clone(symbolBase)
	base["pull/pull.go"] = "package pull\n\nimport (\n\t_ \"unsafe\"\n\n\t_ \"example.test/m/quiet\"\n)\n\n//go:linkname nanotime runtime.nanotime\nfunc nanotime() int64\n"
	base["hub/text.go"] = "package hub\n\nconst directive = \"//go:linkname\"\n"
	record, _ := symbolRun(t, base, symbolEdit(t, "strings.ToUpper(s)", "strings.ToUpper(s + \"!\")"), "hub/hub.go")
	if record.State != affected.GoAdditiveNarrowed || record.Mode != golang.GoSymbolMode {
		t.Fatalf("record %+v, want NARROWED %s", record, golang.GoSymbolMode)
	}
}
