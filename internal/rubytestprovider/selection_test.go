package rubytestprovider

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const userSpec = "spec/models/user_spec.rb"

// LPCV-V0-131: a --spec value is a project-relative file or directory,
// path:line[:line...] or path[scoped-id,...]; it is validated, passed to RSpec
// verbatim and recorded as the selection, or withheld when secret-shaped.
func TestLPCVV0131SelectorsValidatedAndRecorded(t *testing.T) {
	for value, want := range map[string]selector{
		userSpec:                   {kind: SelectorPath, path: userSpec},
		"spec":                     {kind: SelectorPath, path: "spec"},
		"./" + userSpec + ":4":     {kind: SelectorLine, path: userSpec, lines: []int{4}},
		userSpec + ":2:6":          {kind: SelectorLine, path: userSpec, lines: []int{2, 6}},
		userSpec + "[1:2]":         {kind: SelectorID, path: userSpec, ids: []string{"1:2"}},
		userSpec + "[1:2,1:3]":     {kind: SelectorID, path: userSpec, ids: []string{"1:2", "1:3"}},
		"spec/odd[name]_spec.rb":   {kind: SelectorPath, path: "spec/odd[name]_spec.rb"},
		"spec/odd[x]":              {kind: SelectorPath, path: "spec/odd[x]"},
		userSpec + ":0":            {kind: SelectorPath, path: userSpec + ":0"},
		userSpec + ":100000000":    {kind: SelectorPath, path: userSpec + ":100000000"},
		"spec/models/../user.rb:3": {kind: SelectorLine, path: "spec/user.rb", lines: []int{3}},
	} {
		got, err := parseSelector(value)
		if err != nil || got.kind != want.kind || got.path != want.path || !slices.Equal(got.lines, want.lines) || !slices.Equal(got.ids, want.ids) {
			t.Fatalf("parseSelector(%q) = %+v, %v; want %+v", value, got, err, want)
		}
	}
	for _, value := range []string{"", "../" + userSpec + ":2", "/abs/" + userSpec, ":4", ".:4", ".[1]", "spec/../../x_spec.rb[1]", strings.Repeat("a", maxSelectorBytes+1), "spec/a\x00b"} {
		if got, err := parseSelector(value); err == nil {
			t.Fatalf("parseSelector(%q) accepted as %+v", value, got)
		}
	}

	requireHost(t)
	root := repository(t)
	rspec := []string{fakeRSpec(t, cannedReport, 1)}
	specs := []string{userSpec + ":4", userSpec + "[1:3]", "spec/models"}
	document := run(t, Config{Dir: root, RSpec: rspec, Specs: specs})
	if !slices.Equal(document.Command[len(document.Command)-len(specs):], specs) {
		t.Fatalf("selectors not passed verbatim: %v", document.Command)
	}
	if len(document.Selection) != len(specs) || document.SelectionWithheld != "" {
		t.Fatalf("selection=%+v withheld=%q", document.Selection, document.SelectionWithheld)
	}
	for index, outcome := range document.Selection {
		if outcome.Selector != specs[index] || outcome.Kind != []string{SelectorLine, SelectorID, SelectorPath}[index] {
			t.Fatalf("selection[%d]=%+v", index, outcome)
		}
	}
	if err := Validate(document); err != nil {
		t.Fatal(err)
	}
	if plain := run(t, Config{Dir: root, RSpec: rspec}); plain.Selection != nil || plain.SelectionWithheld != "" {
		t.Fatalf("a run without --spec records a selection: %+v", plain)
	}

	secret := "spec/ghp_" + strings.Repeat("A", 36) + "/user_spec.rb:2"
	withheld := run(t, Config{Dir: root, RSpec: rspec, Specs: []string{secret}})
	if withheld.Selection != nil || withheld.SelectionWithheld != "secret-shaped" || withheld.Command != nil {
		t.Fatalf("secret-shaped selector recorded: %+v", withheld)
	}
	if err := Validate(withheld); err != nil {
		t.Fatal(err)
	}

	for _, specs := range [][]string{{"../outside_spec.rb:1"}, {"/etc/x_spec.rb[1]"}, make([]string, MaxSelectors+1)} {
		for index := range specs {
			if specs[index] == "" {
				specs[index] = userSpec
			}
		}
		if _, err := Run(t.Context(), Config{Dir: root, RSpec: rspec, Specs: specs}); err == nil {
			t.Fatalf("Run accepted %d selectors starting %q", len(specs), specs[0])
		}
	}

	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Document){
		"unknown kind":      func(d *Document) { d.Selection[0].Kind = "glob" },
		"kind mismatch":     func(d *Document) { d.Selection[0].Kind = SelectorPath },
		"traversal":         func(d *Document) { d.Selection[0].Selector = "../x_spec.rb:4" },
		"secret selector":   func(d *Document) { d.Selection[0].Selector = secret },
		"open outcome":      func(d *Document) { d.Selection[0].Outcome = "PARTIAL" },
		"matched without n": func(d *Document) { d.Selection[0].Matched = 0 },
		"unmatched with n":  func(d *Document) { d.Selection[0].Outcome, d.Selection[0].Matched = SelectorNoExample, 1 },
		"withheld and kept": func(d *Document) { d.SelectionWithheld = "secret-shaped" },
		"open withheld":     func(d *Document) { d.Selection, d.SelectionWithheld = nil, "too-long" },
		"selection over bound": func(d *Document) {
			d.Selection = slices.Repeat(d.Selection[:1], MaxSelectors+1)
		},
	} {
		var mutated Document
		if err := json.Unmarshal(data, &mutated); err != nil {
			t.Fatal(err)
		}
		mutate(&mutated)
		if Validate(mutated) == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// LPCV-V0-132: each selector's outcome comes from the report's example id and
// declared file:line; a run without a readable report leaves it NOT_EVALUATED.
func TestLPCVV0132SelectorOutcomes(t *testing.T) {
	requireHost(t)
	root := repository(t)
	want := []SelectorOutcome{
		{Selector: userSpec + ":4", Kind: SelectorLine, Outcome: SelectorMatched, Matched: 1},
		{Selector: userSpec + ":2:6", Kind: SelectorLine, Outcome: SelectorMatched, Matched: 2},
		{Selector: userSpec + ":3", Kind: SelectorLine, Outcome: SelectorUnattributed},
		{Selector: userSpec + "[1:2]", Kind: SelectorID, Outcome: SelectorMatched, Matched: 1},
		{Selector: userSpec + "[1]", Kind: SelectorID, Outcome: SelectorMatched, Matched: 3},
		{Selector: userSpec + "[1:9]", Kind: SelectorID, Outcome: SelectorNoExample},
		{Selector: "spec/models/admin_spec.rb[1:1]", Kind: SelectorID, Outcome: SelectorNoExample},
		{Selector: "spec/other_spec.rb:1", Kind: SelectorLine, Outcome: SelectorNoExample},
		{Selector: "spec/models", Kind: SelectorPath, Outcome: SelectorMatched, Matched: 3},
		{Selector: "spec/mod", Kind: SelectorPath, Outcome: SelectorNoExample},
	}
	specs := make([]string, 0, len(want))
	for _, outcome := range want {
		specs = append(specs, outcome.Selector)
	}
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, cannedReport, 1)}, Specs: specs})
	if !slices.Equal(document.Selection, want) {
		t.Fatalf("selection=%+v\nwant=%+v", document.Selection, want)
	}
	if err := Validate(document); err != nil {
		t.Fatal(err)
	}
	// An empty report run: every selector matched no example.
	empty := `{"version":"3.13.2","examples":[],"summary":{"example_count":0,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":0}}`
	none := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, empty, 0)}, Specs: []string{userSpec + ":4"}})
	if len(none.Selection) != 1 || none.Selection[0].Outcome != SelectorNoExample {
		t.Fatalf("empty report selection=%+v", none.Selection)
	}
	missing := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, "", 1)}, Specs: []string{userSpec + ":4"}})
	if missing.Infrastructure == nil || len(missing.Selection) != 1 || missing.Selection[0].Outcome != SelectorNotEvaluated {
		t.Fatalf("missing report selection=%+v infrastructure=%+v", missing.Selection, missing.Infrastructure)
	}
	if err := Validate(missing); err != nil {
		t.Fatal(err)
	}
}

// LPCV-V0-133: under --exec-prefix the runtime identity is what a separate
// probe invocation of the prefix reports; anything it does not report is an
// explicit unknown, and the image digest is never claimed.
func TestLPCVV0133RuntimeObservedInsidePrefix(t *testing.T) {
	requireHost(t)
	root := repository(t)
	rspec := []string{fakeRSpec(t, cannedReport, 1)}
	container := strings.Repeat("0123456789abcdef", 4)
	prefix := func(probe string) []string {
		path := filepath.Join(t.TempDir(), "container")
		write(t, path, "#!/bin/sh\nif [ \"$1\" = ruby ]; then\n:\n"+probe+"\nfi\nexec \"$@\"\n", 0o755)
		return []string{path}
	}
	notInside := RuntimeFact{Unknown: runtimeNotInsidePfx}
	for name, test := range map[string]struct {
		probe string
		want  Runtime
	}{
		"reported": {"echo noise; echo 'corvint-runtime-identity ruby=3.3.5 bundler=2.5.16 container=" + container + "'; exit 0",
			Runtime{RuntimeBasis, RuntimeFact{Value: "3.3.5"}, RuntimeFact{Value: "2.5.16"}, RuntimeFact{Value: container}, notInside, nil}},
		"partial": {"echo 'corvint-runtime-identity ruby=3.3.5 bundler=2/5 container=ABC ruby=3.3.5'; exit 0",
			Runtime{RuntimeBasis, RuntimeFact{Unknown: runtimeNotReported}, RuntimeFact{Unknown: runtimeNotReported}, RuntimeFact{Unknown: runtimeNotReported}, notInside, nil}},
		"failed": {"echo 'corvint-runtime-identity ruby=3.3.5'; exit 1",
			Runtime{RuntimeBasis, RuntimeFact{Unknown: runtimeProbeFailed}, RuntimeFact{Unknown: runtimeProbeFailed}, RuntimeFact{Unknown: runtimeProbeFailed}, notInside, nil}},
		"two lines": {"echo 'corvint-runtime-identity ruby=3.3.5'; echo 'corvint-runtime-identity ruby=3.2.0'; exit 0",
			Runtime{RuntimeBasis, RuntimeFact{Unknown: runtimeProbeFailed}, RuntimeFact{Unknown: runtimeProbeFailed}, RuntimeFact{Unknown: runtimeProbeFailed}, notInside, nil}},
		"stderr overflow": {"head -c 70000 /dev/zero | tr '\\0' x >&2; echo 'corvint-runtime-identity ruby=3.3.5 bundler=2.5.16'; exit 0",
			Runtime{RuntimeBasis, RuntimeFact{Unknown: runtimeProbeFailed}, RuntimeFact{Unknown: runtimeProbeFailed}, RuntimeFact{Unknown: runtimeProbeFailed}, notInside, nil}},
	} {
		document := run(t, Config{Dir: root, ExecPrefix: prefix(test.probe), RSpec: rspec})
		if document.Runtime == nil || *document.Runtime != test.want {
			t.Fatalf("%s: runtime=%+v", name, document.Runtime)
		}
		if document.Infrastructure != nil || len(document.Examples) != 3 {
			t.Fatalf("%s: the probe changed the run: %+v", name, document)
		}
		if err := Validate(document); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if document := run(t, Config{Dir: root, RSpec: rspec}); document.Runtime != nil {
		t.Fatalf("a host run records a runtime identity: %+v", document.Runtime)
	}

	reported := Runtime{RuntimeBasis, RuntimeFact{Value: "3.3.5"}, RuntimeFact{Value: "2.5.16"}, RuntimeFact{Value: container}, notInside, nil}
	if err := validateRuntime(&reported); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Runtime){
		"basis":         func(r *Runtime) { r.Basis = "host" },
		"both":          func(r *Runtime) { r.Ruby.Unknown = runtimeNotReported },
		"neither":       func(r *Runtime) { r.Bundler = RuntimeFact{} },
		"open reason":   func(r *Runtime) { r.ProbeContainer = RuntimeFact{Unknown: "host"} },
		"bad version":   func(r *Runtime) { r.Ruby.Value = "3.3 5" },
		"bad container": func(r *Runtime) { r.ProbeContainer.Value = strings.ToUpper(container) },
		"bad image":     func(r *Runtime) { r.Image = RuntimeFact{Value: container} },
		"image claimed": func(r *Runtime) { r.Image = RuntimeFact{Value: "sha256:" + container} },
	} {
		mutated := reported
		mutate(&mutated)
		if validateRuntime(&mutated) == nil {
			t.Fatalf("%s accepted", name)
		}
	}

	// The real probe script, run by the host Ruby through a pass-through
	// prefix, reports that Ruby's version (skipped without Ruby).
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Log("ruby is unavailable; the real probe script is NOT_RUN")
		return
	}
	version, err := exec.Command(ruby, "-e", "print RUBY_VERSION").Output()
	if err != nil {
		t.Fatal(err)
	}
	document := run(t, Config{Dir: root, ExecPrefix: prefix(""), RSpec: rspec})
	if document.Runtime == nil || document.Runtime.Ruby.Value != string(version) || document.Runtime.Image != notInside {
		t.Fatalf("real probe runtime=%+v want ruby %s", document.Runtime, version)
	}
	if err := Validate(document); err != nil {
		t.Fatal(err)
	}
}

// LPCV-V0-134: the shared-example origin comes from the report's
// shared_group_inclusion_backtrace; without it the example records why.
func TestLPCVV0134SharedExampleOrigin(t *testing.T) {
	requireHost(t)
	root := repository(t)
	report := strings.Replace(cannedReport, `"line_number":2}`, `"line_number":2,"shared_group_inclusion_backtrace":[{"shared_group_name":"a record","inclusion_location":"./spec/models/user_spec.rb:9"},{"shared_group_name":"a model","inclusion_location":"./spec/support/models.rb:3"}]}`, 1)
	report = strings.Replace(report, `"line_number":4}`, `"line_number":4,"shared_group_inclusion_backtrace":[]}`, 1)
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, report, 1)}})
	shared := document.Examples[0].SharedExample
	if shared == nil || shared.Origin != userSpec+":2" || !slices.Equal(shared.IncludedAt, []string{userSpec + ":9", "spec/support/models.rb:3"}) || document.Examples[0].SharedExampleAbsent != "" {
		t.Fatalf("shared example=%+v", document.Examples[0])
	}
	if document.Examples[1].SharedExample != nil || document.Examples[1].SharedExampleAbsent != sharedNotShared ||
		document.Examples[2].SharedExample != nil || document.Examples[2].SharedExampleAbsent != sharedNotReported {
		t.Fatalf("absences=%+v %+v", document.Examples[1], document.Examples[2])
	}
	if err := Validate(document); err != nil {
		t.Fatal(err)
	}

	frame := func(location string) string { return `{"inclusion_location":` + location + `}` }
	for name, backtrace := range map[string]string{
		"not an array":   `"spec/x_spec.rb:1"`,
		"no location":    `[{"shared_group_name":"x"}]`,
		"null location":  `[` + frame(`null`) + `]`,
		"no line":        `[` + frame(`"./spec/x_spec.rb"`) + `]`,
		"zero line":      `[` + frame(`"./spec/x_spec.rb:0"`) + `]`,
		"traversal":      `[` + frame(`"../x_spec.rb:1"`) + `]`,
		"absolute":       `[` + frame(`"/gems/x.rb:1"`) + `]`,
		"secret-shaped":  `[` + frame(`"./spec/ghp_`+strings.Repeat("A", 36)+`.rb:1"`) + `]`,
		"over bound":     `[` + frame(`"./spec/`+strings.Repeat("a", maxInclusionLocation)+`.rb:1"`) + `]`,
		"too many":       `[` + strings.Repeat(frame(`"./spec/x_spec.rb:1"`)+",", maxInclusions) + frame(`"./spec/x_spec.rb:1"`) + `]`,
		"bare dot frame": `[` + frame(`"./:1"`) + `]`,
		"NUL in path":    `[` + frame(`"./spec/a\u0000.rb:7"`) + `]`,
	} {
		if shared, absent := observeShared(json.RawMessage(backtrace), ".", userSpec+":2"); shared != nil || absent != sharedInclusionUnbnd {
			t.Fatalf("%s: %+v %q", name, shared, absent)
		}
	}
	if shared, _ := observeShared(json.RawMessage(`[`+frame(`"./spec/x_spec.rb:7"`)+`]`), "app", "app/spec/y_spec.rb:2"); shared == nil || shared.IncludedAt[0] != "app/spec/x_spec.rb:7" {
		t.Fatalf("project-relative inclusion not made worktree-relative: %+v", shared)
	}

	legacy := document.Examples[2]
	legacy.SharedExampleAbsent = ""
	if validateShared(legacy) != nil {
		t.Fatal("an example without either member is refused")
	}
	for name, mutate := range map[string]func(*Example){
		"both":         func(e *Example) { e.SharedExampleAbsent = sharedNotReported },
		"other origin": func(e *Example) { e.SharedExample.Origin = userSpec + ":3" },
		"no inclusion": func(e *Example) { e.SharedExample.IncludedAt = []string{} },
		"too many": func(e *Example) {
			e.SharedExample.IncludedAt = slices.Repeat([]string{userSpec + ":9"}, maxInclusions+1)
		},
		"bad inclusion": func(e *Example) { e.SharedExample.IncludedAt = []string{"../x.rb:1"} },
		"NUL inclusion": func(e *Example) { e.SharedExample.IncludedAt = []string{"spec/a\x00.rb:7"} },
		"open absence":  func(e *Example) { e.SharedExample, e.SharedExampleAbsent = nil, "unknown" },
	} {
		mutated := document.Examples[0]
		copied := *mutated.SharedExample
		mutated.SharedExample = &copied
		mutate(&mutated)
		if validateShared(mutated) == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
