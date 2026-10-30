package rubytestprovider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// sharedReport is a Corvint-formatter report: spec/models/user_spec.rb line 8
// includes a shared group defined in spec/support/shared.rb, and every other
// example carries an empty inclusion backtrace.
const sharedReport = `{"version":"3.13.6","examples":[
{"id":"./spec/models/user_spec.rb[1:1]","full_description":"User saves","status":"passed","file_path":"./spec/models/user_spec.rb","line_number":2,"shared_group_inclusion_backtrace":[]},
{"id":"./spec/models/user_spec.rb[1:2:1]","full_description":"User behaves like a record persists","status":"passed","file_path":"./spec/support/shared.rb","line_number":2,"shared_group_inclusion_backtrace":[{"inclusion_location":"./spec/models/user_spec.rb:8"}]}],
"summary":{"example_count":2,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":0}CORVINT_RUNTIME}`

func corvintReport(runtime string) string {
	return strings.Replace(sharedReport, "CORVINT_RUNTIME", runtime, 1)
}

// LPCV-V0-136, LPCV-V0-137: the Corvint formatter is written for the run,
// required by path, removed afterwards and recorded; its inclusion backtrace
// becomes the shared-example origin and attributes the inclusion line.
func TestLPCVV0136CorvintFormatterRecordsInclusion(t *testing.T) {
	requireHost(t)
	root := repository(t)
	specs := []string{userSpec + ":8", userSpec + ":2", userSpec + "[1:2]", userSpec + ":40"}
	document := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, corvintReport(""), 0)}, Formatter: FormatterCorvint, Specs: specs})
	if document.Infrastructure != nil || document.Formatter != FormatterCorvint {
		t.Fatalf("formatter run: %+v", document)
	}
	required := slices.Index(document.Command, "--require")
	if required < 0 || !strings.HasPrefix(document.Command[required+1], "./"+reportDirectory+"/") || !strings.HasSuffix(document.Command[required+1], "-formatter.rb") ||
		!slices.Contains(document.Command, formatterClass) || slices.Contains(document.Command, "json") {
		t.Fatalf("command=%q", document.Command)
	}
	entries, err := os.ReadDir(filepath.Join(root, reportDirectory))
	if err != nil || slices.ContainsFunc(entries, func(entry os.DirEntry) bool { return strings.HasSuffix(entry.Name(), ".rb") }) {
		t.Fatalf("formatter file left behind: %v %v", entries, err)
	}
	shared := document.Examples[1].SharedExample
	if shared == nil || shared.Origin != "spec/support/shared.rb:2" || !slices.Equal(shared.IncludedAt, []string{userSpec + ":8"}) {
		t.Fatalf("shared example=%+v", document.Examples[1])
	}
	if document.Examples[0].SharedExampleAbsent != sharedNotShared {
		t.Fatalf("plain example=%+v", document.Examples[0])
	}
	want := []SelectorOutcome{
		{Selector: userSpec + ":8", Kind: SelectorLine, Outcome: SelectorMatched, Matched: 1},
		{Selector: userSpec + ":2", Kind: SelectorLine, Outcome: SelectorMatched, Matched: 1},
		{Selector: userSpec + "[1:2]", Kind: SelectorID, Outcome: SelectorMatched, Matched: 1},
		{Selector: userSpec + ":40", Kind: SelectorLine, Outcome: SelectorUnattributed},
	}
	if !slices.Equal(document.Selection, want) {
		t.Fatalf("selection=%+v", document.Selection)
	}
	if err := Validate(document); err != nil {
		t.Fatal(err)
	}

	// The same run under the stock formatter states no inclusions: the
	// inclusion line stays unattributed and the origin not reported.
	stock := strings.ReplaceAll(corvintReport(""), `,"shared_group_inclusion_backtrace":[{"inclusion_location":"./spec/models/user_spec.rb:8"}]`, "")
	stock = strings.ReplaceAll(stock, `,"shared_group_inclusion_backtrace":[]`, "")
	legacy := run(t, Config{Dir: root, RSpec: []string{fakeRSpec(t, stock, 0)}, Specs: specs[:1]})
	if legacy.Formatter != "" || legacy.Examples[1].SharedExampleAbsent != sharedNotReported || legacy.Selection[0].Outcome != SelectorUnattributed || !slices.Contains(legacy.Command, "json") {
		t.Fatalf("stock run=%+v", legacy)
	}
	if err := Validate(legacy); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(t.Context(), Config{Dir: root, Formatter: "progress"}); err == nil {
		t.Fatal("an unknown formatter was admitted")
	}
	mutated := document
	mutated.Formatter = "corvint-rspec-json/1"
	if Validate(mutated) == nil {
		t.Fatal("an unknown formatter value validated")
	}
}

// LPCV-V0-138: under a docker prefix and the Corvint formatter, the run
// container is what the RSpec process reported, and the image is the host
// docker's image ID for it, inspected through the same daemon flags; each
// unobservable step is a closed unknown.
func TestLPCVV0138RunContainerAndHostImage(t *testing.T) {
	requireHost(t)
	root := repository(t)
	container := strings.Repeat("0123456789abcdef", 4)
	image := "sha256:" + strings.Repeat("fedcba9876543210", 4)
	withContainer := corvintReport(`,"corvint_runtime":{"container":"` + container + `"}`)
	prefix := func(name, inspect string) ([]string, string) {
		bin := t.TempDir()
		log := filepath.Join(bin, "inspect.log")
		script := "#!/bin/sh\nif [ \"$1\" = -H ]; then shift 2; fi\n" +
			"if [ \"$1\" = container ] && [ \"$2\" = inspect ]; then echo \"$*\" >> '" + log + "'; " + inspect + "; fi\n" +
			"shift\nif [ \"$1\" = ruby ]; then echo 'corvint-runtime-identity ruby=3.4.11 bundler=2.6.9 container=" + strings.Repeat("a", 64) + "'; exit 0; fi\n" +
			"exec \"$@\"\n"
		path := filepath.Join(bin, name)
		write(t, path, script, 0o755)
		return []string{path, "-H", "unix:///nonexistent.sock", "run"}, log
	}
	for name, test := range map[string]struct {
		binary, inspect, report, formatter string
		container                          *RuntimeFact
		image                              RuntimeFact
		inspected                          bool
	}{
		"observed":      {"docker", "echo " + image + "; exit 0", withContainer, FormatterCorvint, &RuntimeFact{Value: container}, RuntimeFact{Value: image}, true},
		"removed":       {"docker", "echo 'No such container' >&2; exit 1", withContainer, FormatterCorvint, &RuntimeFact{Value: container}, RuntimeFact{Unknown: runtimeNotInspectable}, true},
		"malformed":     {"docker", "echo " + container + "; exit 0", withContainer, FormatterCorvint, &RuntimeFact{Value: container}, RuntimeFact{Unknown: runtimeNotInspectable}, true},
		"not reported":  {"docker", "exit 0", corvintReport(`,"corvint_runtime":{}`), FormatterCorvint, &RuntimeFact{Unknown: runtimeNotReported}, RuntimeFact{Unknown: runtimeRunContainerUnknown}, false},
		"bad container": {"docker", "exit 0", corvintReport(`,"corvint_runtime":{"container":"ABC"}`), FormatterCorvint, &RuntimeFact{Unknown: runtimeNotReported}, RuntimeFact{Unknown: runtimeRunContainerUnknown}, false},
		"not docker":    {"podman", "exit 0", withContainer, FormatterCorvint, &RuntimeFact{Value: container}, RuntimeFact{Unknown: runtimeHostInspectUnavail}, false},
		"stock":         {"docker", "echo " + image + "; exit 0", withContainer, "", nil, RuntimeFact{Unknown: runtimeNotInsidePfx}, false},
	} {
		execPrefix, log := prefix(test.binary, test.inspect)
		document := run(t, Config{Dir: root, ExecPrefix: execPrefix, RSpec: []string{fakeRSpec(t, test.report, 0)}, Formatter: test.formatter})
		runtime := document.Runtime
		if document.Infrastructure != nil || runtime == nil || runtime.Image != test.image || (runtime.RunContainer == nil) != (test.container == nil) ||
			test.container != nil && *runtime.RunContainer != *test.container {
			t.Fatalf("%s: infrastructure=%+v runtime=%+v", name, document.Infrastructure, runtime)
		}
		calls, _ := os.ReadFile(log)
		if want := "container inspect --format {{.Image}} " + container + "\n"; test.inspected != (string(calls) == want) {
			t.Fatalf("%s: inspect calls=%q", name, calls)
		}
		if err := Validate(document); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	execPrefix, _ := prefix("docker", "echo "+image+"; exit 0")
	observed := run(t, Config{Dir: root, ExecPrefix: execPrefix, RSpec: []string{fakeRSpec(t, withContainer, 0)}, Formatter: FormatterCorvint})
	for name, mutate := range map[string]func(*Document){
		"stock run container":   func(d *Document) { d.Formatter = "" },
		"no run container":      func(d *Document) { d.Runtime.RunContainer = nil },
		"image without run":     func(d *Document) { d.Runtime.RunContainer = &RuntimeFact{Unknown: runtimeNotReported} },
		"image not a digest":    func(d *Document) { d.Runtime.Image = RuntimeFact{Value: container} },
		"probe reason on image": func(d *Document) { d.Runtime.Image = RuntimeFact{Unknown: runtimeNotInsidePfx} },
		"run reason open":       func(d *Document) { d.Runtime.RunContainer = &RuntimeFact{Unknown: runtimeProbeFailed} },
		"run container both": func(d *Document) {
			d.Runtime.RunContainer = &RuntimeFact{Value: container, Unknown: runtimeNotReported}
		},
	} {
		mutated := observed
		copied := *observed.Runtime
		mutated.Runtime = &copied
		mutate(&mutated)
		if Validate(mutated) == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
