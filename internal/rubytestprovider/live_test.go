package rubytestprovider

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// liveImage is the owner-approved disposable qualification image (RSpec gems
// installed from checksum-verified archives on ruby:3.4-bookworm).
const liveImage = "corvint-rspec-qual:2026-10-20"

// liveImageID is the image ID the retained live documents observed.
const liveImageID = "sha256:b4202eef8ad08df2c18d7d38008a02910377d5f37943c54b13a9827a83b532db"

const liveShared = "RSpec.shared_examples 'a priced item' do\n  it 'has a positive price' do\n    expect(subject.price).to be > 0\n  end\nend\n"

const liveWidget = "require_relative 'support/shared'\n\nWidget = Struct.new(:price)\n\nRSpec.describe Widget do\n  subject { Widget.new(3) }\n  it 'keeps its price' do\n    expect(subject.price).to eq(3)\n  end\n  it_behaves_like 'a priced item'\nend\n"

// The selectors each live run records: the example line (7), the shared-group
// inclusion line (10), a scoped id, and a spec file with no example.
var liveRuns = []struct {
	name, formatter string
	keep            bool
	specs           []string
}{
	{"corvint-lines", FormatterCorvint, false, []string{"spec/widget_spec.rb:7", "spec/widget_spec.rb:10", "spec/empty_spec.rb"}},
	{"corvint-ids-kept", FormatterCorvint, true, []string{"spec/widget_spec.rb[1:2]"}},
	{"stock-lines", "", false, []string{"spec/widget_spec.rb:10"}},
}

// LPCV-V0-139: real RSpec runs through a docker exec prefix. Run by hand with
// CORVINT_RECORD_LIVE_RSPEC=1 to rewrite testdata/live; skipped without docker
// or the qualification image.
func TestLPCVV0139LiveRSpecThroughDocker(t *testing.T) {
	requireHost(t)
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("LPCV-V0-139 live runs need docker on PATH")
	}
	imageID, err := exec.Command(docker, "image", "inspect", "--format", "{{.Id}}", liveImage).Output()
	if err != nil {
		t.Skipf("LPCV-V0-139 live runs need the %s image", liveImage)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("LPCV-V0-139 home directory unavailable")
	}
	// Desktop docker VMs share the home directory, not the system temporary
	// directory, so the project lives under a disposable home cache directory.
	if err := os.MkdirAll(filepath.Join(home, ".cache"), 0o700); err != nil {
		t.Skipf("LPCV-V0-139 home cache unavailable: %v", err)
	}
	project, err := os.MkdirTemp(filepath.Join(home, ".cache"), "corvint-live-rspec-")
	if err != nil {
		t.Skipf("LPCV-V0-139 home cache unavailable: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(project) })
	if project, err = filepath.EvalSymlinks(project); err != nil {
		t.Fatal(err)
	}
	git(t, project, "init", "-q", "-b", "main")
	git(t, project, "config", "user.email", "test@example.invalid")
	git(t, project, "config", "user.name", "test")
	write(t, filepath.Join(project, "spec", "support", "shared.rb"), liveShared, 0o644)
	write(t, filepath.Join(project, "spec", "widget_spec.rb"), liveWidget, 0o644)
	write(t, filepath.Join(project, "spec", "empty_spec.rb"), "# no examples\n", 0o644)
	git(t, project, "add", ".")
	git(t, project, "commit", "-q", "-m", "fixture")

	documents := map[string]Document{}
	for _, live := range liveRuns {
		prefix := []string{"docker", "run", "--network", "none", "-v", ".:/work", "-w", "/work", liveImage}
		if !live.keep {
			prefix = slices.Insert(prefix, 2, "--rm")
		}
		document := run(t, Config{Dir: project, ExecPrefix: prefix, RSpec: []string{"rspec"}, Formatter: live.formatter, Specs: live.specs})
		if live.keep && document.Runtime != nil {
			// Only containers this run reported are removed, by ID.
			for _, fact := range []*RuntimeFact{document.Runtime.RunContainer, &document.Runtime.ProbeContainer} {
				if fact != nil && fact.Value != "" {
					_ = exec.Command(docker, "rm", fact.Value).Run()
				}
			}
		}
		documents[live.name] = document
	}
	checkLiveDocuments(t, documents, strings.TrimSpace(string(imageID)))
	if os.Getenv("CORVINT_RECORD_LIVE_RSPEC") != "1" {
		return
	}
	for name, document := range documents {
		data, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join("testdata", "live", name+".json"), string(data)+"\n", 0o644)
	}
}

// LPCV-V0-139: the retained live documents still validate and still carry
// the observed outcomes.
func TestLPCVV0139RetainedLiveDocuments(t *testing.T) {
	documents := map[string]Document{}
	for _, live := range liveRuns {
		data, err := os.ReadFile(filepath.Join("testdata", "live", live.name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var document Document
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&document); err != nil {
			t.Fatalf("%s: %v", live.name, err)
		}
		documents[live.name] = document
	}
	checkLiveDocuments(t, documents, liveImageID)
}

func checkLiveDocuments(t *testing.T, documents map[string]Document, imageID string) {
	t.Helper()
	const widget, shared = "spec/widget_spec.rb", "spec/support/shared.rb"
	for name, document := range documents {
		if err := Validate(document); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		runtime := document.Runtime
		if document.Infrastructure != nil || document.Summary == nil || document.Summary.Failures != 0 || document.RunnerVersion == "" ||
			runtime == nil || runtime.Ruby.Value == "" || runtime.Bundler.Value == "" || runtime.ProbeContainer.Value == "" {
			t.Fatalf("%s: document=%+v runtime=%+v", name, document, runtime)
		}
	}

	lines := documents["corvint-lines"]
	if !slices.Equal(lines.Selection, []SelectorOutcome{
		{Selector: widget + ":7", Kind: SelectorLine, Outcome: SelectorMatched, Matched: 1},
		{Selector: widget + ":10", Kind: SelectorLine, Outcome: SelectorMatched, Matched: 1},
		{Selector: "spec/empty_spec.rb", Kind: SelectorPath, Outcome: SelectorNoExample},
	}) {
		t.Fatalf("corvint line selection=%+v", lines.Selection)
	}
	if len(lines.Examples) != 2 || lines.Examples[0].SharedExampleAbsent != sharedNotShared {
		t.Fatalf("corvint line examples=%+v", lines.Examples)
	}
	if origin := lines.Examples[1].SharedExample; origin == nil || origin.Origin != shared+":2" || !slices.Equal(origin.IncludedAt, []string{widget + ":10"}) {
		t.Fatalf("shared origin=%+v", lines.Examples[1])
	}
	// --rm removed the run container before the host could inspect it.
	if run := lines.Runtime.RunContainer; run == nil || run.Value == "" || run.Value == lines.Runtime.ProbeContainer.Value ||
		lines.Runtime.Image != (RuntimeFact{Unknown: runtimeNotInspectable}) {
		t.Fatalf("corvint --rm runtime=%+v", lines.Runtime)
	}

	ids := documents["corvint-ids-kept"]
	if !slices.Equal(ids.Selection, []SelectorOutcome{{Selector: widget + "[1:2]", Kind: SelectorID, Outcome: SelectorMatched, Matched: 1}}) ||
		len(ids.Examples) != 1 || ids.Examples[0].SharedExample == nil {
		t.Fatalf("corvint id run=%+v", ids)
	}
	if run := ids.Runtime.RunContainer; run == nil || run.Value == "" || ids.Runtime.Image != (RuntimeFact{Value: imageID}) {
		t.Fatalf("corvint kept-container runtime=%+v want image %s", ids.Runtime, imageID)
	}

	// The stock formatter states no inclusion: the same inclusion-line
	// selector stays unattributed and the origin not reported.
	stock := documents["stock-lines"]
	if stock.Formatter != "" || stock.Runtime.RunContainer != nil || stock.Runtime.Image != (RuntimeFact{Unknown: runtimeNotInsidePfx}) ||
		!slices.Equal(stock.Selection, []SelectorOutcome{{Selector: widget + ":10", Kind: SelectorLine, Outcome: SelectorUnattributed}}) ||
		len(stock.Examples) != 1 || stock.Examples[0].SharedExampleAbsent != sharedNotReported {
		t.Fatalf("stock run=%+v runtime=%+v", stock, stock.Runtime)
	}
}
