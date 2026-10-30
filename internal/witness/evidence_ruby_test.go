package witness

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
)

// stockRSpecReport is the shape `rspec --format json` (RSpec 3.13) writes:
// file_path and id are relative to the directory RSpec ran in, with a leading
// "./", and the run is in the "web" project below the repository root.
const stockRSpecReport = `{"version":"3.13.2","seed":4242,"examples":[
{"id":"./spec/models/user_spec.rb[1:1]","description":"saves","full_description":"User saves","status":"passed","file_path":"./spec/models/user_spec.rb","line_number":2,"run_time":0.000512,"pending_message":null},
{"id":"./spec/models/user_spec.rb[1:2]","description":"rejects a blank name","full_description":"User rejects a blank name","status":"failed","file_path":"./spec/models/user_spec.rb","line_number":4,"run_time":0.000731,"pending_message":null,"exception":{"class":"RSpec::Expectations::ExpectationNotMetError","message":"expected true","backtrace":["./spec/models/user_spec.rb:5"]}},
{"id":"./spec/models/user_spec.rb[1:3]","description":"later","full_description":"User later","status":"pending","file_path":"./spec/models/user_spec.rb","line_number":6,"run_time":0.00001,"pending_message":"Not yet implemented"}],
"summary":{"duration":0.0042,"example_count":3,"failure_count":1,"pending_count":1,"errors_outside_of_examples_count":0},
"summary_line":"3 examples, 1 failure, 1 pending"}`

// rubyEvidenceFixture commits a repository whose Rails project lives under
// web/, changes its model and its spec in the reported range, and runs the
// real corvint-ruby-test-provider runner against the head with an rspec
// stand-in that writes stockRSpecReport. It returns the head index, the base
// and the provider document the runner produced.
func rubyEvidenceFixture(t *testing.T) (*contextindex.Index, string, rubytestprovider.Document) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the rspec stand-in needs a POSIX shell")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := initTestRepo(t, git)
	fixtureGit := func(args ...string) string {
		t.Helper()
		command := exec.Command(git, append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test"}, args...)...)
		command.Dir, command.Env = root, gitEnvironment()
		raw, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %v: %s", args, err, raw)
		}
		return strings.TrimSpace(string(raw))
	}
	write := func(name, body string, mode os.FileMode) {
		t.Helper()
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("web/app/models/user.rb", "class User\nend\n", 0o644)
	write("web/spec/spec_helper.rb", "RSpec.configure { |config| config.order = :defined }\n", 0o644)
	write("web/spec/models/user_spec.rb", "RSpec.describe User do\n  it('saves') { expect(true).to be(true) }\nend\n", 0o644)
	fixtureGit("add", ".")
	fixtureGit("commit", "--quiet", "-m", "base")
	base := fixtureGit("rev-parse", "HEAD")
	write("web/app/models/user.rb", "class User\n  def valid? = !name.to_s.empty?\nend\n", 0o644)
	write("web/spec/models/user_spec.rb", "RSpec.describe User do\n  it('saves') { expect(true).to be(true) }\n\n  it('rejects a blank name') { expect(User.new).not_to be_valid }\n\n  it('later')\nend\n", 0o644)
	fixtureGit("commit", "--quiet", "-am", "change")

	bin := t.TempDir()
	canned := filepath.Join(bin, "report.json")
	if err := os.WriteFile(canned, []byte(stockRSpecReport), 0o644); err != nil {
		t.Fatal(err)
	}
	rspec := filepath.Join(bin, "rspec")
	script := "#!/bin/sh\nprevious=\nfor argument in \"$@\"; do\n  if [ \"$previous\" = --out ]; then cat '" + canned + "' > \"$argument\"; fi\n  previous=\"$argument\"\ndone\nexit 1\n"
	if err := os.WriteFile(rspec, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	document, err := rubytestprovider.Run(ctx, rubytestprovider.Config{Dir: filepath.Join(resolved, "web"), RSpec: []string{rspec}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index, base, document
}

func evidenceWitnesses(report *Report, path string) []Witness {
	var found []Witness
	for _, item := range report.Obligations {
		if item.Path != path {
			continue
		}
		for _, candidate := range item.Witnesses {
			if candidate.Source == evidenceSourceName {
				found = append(found, candidate)
			}
		}
	}
	return found
}

// TestRubyEvidenceBindsToTheHeadTree pins AGW-V0-010: a corvint-ruby-test-provider
// document from a clean run at the head binds, each example attaches a
// non-closing CALLER_REPORTED witness to its spec file's obligation (the
// project directory joined, RSpec's "./" dropped), and nothing attaches to a
// file the document does not name.
func TestRubyEvidenceBindsToTheHeadTree(t *testing.T) {
	index, base, document := rubyEvidenceFixture(t)
	if document.ProjectDirectory != "web" || len(document.Examples) != 3 || document.Examples[0].File != "web/spec/models/user_spec.rb" || document.Infrastructure != nil {
		t.Fatalf("the runner produced an unexpected document: %+v", document)
	}
	report, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{writeEvidence(t, document)}})
	if err != nil {
		t.Fatal(err)
	}
	source := report.Sources[1]
	if source.Status != "BOUND" || source.Basis != 3 || source.Unattached != 0 || source.Hunks != 3 {
		t.Fatalf("source = %+v, want BOUND with three attached examples", source)
	}
	witnesses := evidenceWitnesses(report, "web/spec/models/user_spec.rb")
	relations := map[string]string{}
	for _, candidate := range witnesses {
		if candidate.Closing || candidate.Authority != "CALLER_REPORTED" || candidate.EvidencePath != "web/spec/models/user_spec.rb" {
			t.Fatalf("witness = %+v, want a non-closing CALLER_REPORTED witness on the spec file", candidate)
		}
		relations[candidate.Test] = candidate.Relation
	}
	if relations["User saves"] != "test-run:passed" || relations["User rejects a blank name"] != "test-run:failed" || relations["User later"] != "test-run:pending" {
		t.Fatalf("relations = %v", relations)
	}
	if found := evidenceWitnesses(report, "web/app/models/user.rb"); len(found) != 0 {
		t.Fatalf("the model the document does not name has evidence %+v", found)
	}
}

// TestRubyEvidenceNotBoundToTheHeadTreeAttachesNothing pins the AGW-V0-010
// failure modes: each named mismatch reads UNBOUND and attaches nothing.
func TestRubyEvidenceNotBoundToTheHeadTreeAttachesNothing(t *testing.T) {
	index, base, real := rubyEvidenceFixture(t)
	other := strings.Repeat("ab", 20)
	mutate := func(change func(*rubytestprovider.Document)) rubytestprovider.Document {
		var document rubytestprovider.Document
		data, err := json.Marshal(real)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		change(&document)
		return document
	}
	cases := []struct {
		name     string
		document rubytestprovider.Document
		reason   string
	}{
		{"spec blob", mutate(func(d *rubytestprovider.Document) {
			for position := range d.Examples {
				d.Examples[position].SpecBlob = other
			}
		}), "blob ID for web/spec/models/user_spec.rb differs from the head blob"},
		{"helper blob", mutate(func(d *rubytestprovider.Document) {
			d.Helpers[0].Blob = other
			d.HelperSet = rubytestprovider.HelperSetDigest(d.Helpers)
			for position := range d.Examples {
				d.Examples[position].Helpers = d.HelperSet
			}
		}), "differs from the head blob"},
		{"absent file", mutate(func(d *rubytestprovider.Document) {
			d.Examples[0].File, d.Examples[0].ID = "web/spec/missing_spec.rb", "web/spec/missing_spec.rb:2"
		}), "web/spec/missing_spec.rb, which is not a blob in the head tree"},
		{"tree", mutate(func(d *rubytestprovider.Document) {
			d.RepositoryBefore.Tree, d.RepositoryAfter.Tree = other, other
		}), "not the report head tree"},
		{"revision", mutate(func(d *rubytestprovider.Document) {
			d.RepositoryBefore.Revision, d.RepositoryAfter.Revision = other, other
		}), "not the report head " + index.CommitRevision},
		{"dirty", mutate(func(d *rubytestprovider.Document) {
			d.RepositoryBefore.DirtyState, d.RepositoryBefore.DirtyDigest = "dirty", "sha256:"+strings.Repeat("0", 64)
			*d.RepositoryAfter = *d.RepositoryBefore
		}), "not clean during the RSpec run"},
		{"moved", mutate(func(d *rubytestprovider.Document) { d.RepositoryAfter.Revision = other }), "changed between the start and the end"},
		{"no after", mutate(func(d *rubytestprovider.Document) { d.RepositoryAfter = nil }), "both before and after"},
		{"infrastructure", mutate(func(d *rubytestprovider.Document) {
			d.Infrastructure = &rubytestprovider.Infrastructure{Reason: "runner-timeout", Detail: "rspec did not finish"}
		}), "infrastructure failure (runner-timeout)"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			report, err := Compile(context.Background(), index, Options{Base: base, Evidence: []string{writeEvidence(t, testCase.document)}})
			if err != nil {
				t.Fatal(err)
			}
			source := report.Sources[1]
			if source.Status != "UNBOUND" || !strings.Contains(source.Detail, testCase.reason) || source.Basis != 0 {
				t.Fatalf("source = %+v, want UNBOUND naming %q", source, testCase.reason)
			}
			if found := evidenceWitnesses(report, "web/spec/models/user_spec.rb"); len(found) != 0 {
				t.Fatalf("an unbound document attached %+v", found)
			}
		})
	}
}

// TestRubyEvidenceUnboundSpecAttachesNothing pins that an example whose spec
// file the provider could not bind to a blob attaches nothing and is counted.
func TestRubyEvidenceUnboundSpecAttachesNothing(t *testing.T) {
	document := &rubytestprovider.Document{Examples: []rubytestprovider.Example{
		{ID: "spec/a_spec.rb:1", File: "spec/a_spec.rb", Line: 1, Status: "passed", SpecBlobUnbound: "spec-file-dirty"},
		{ID: "spec/a_spec.rb:2", File: "spec/a_spec.rb", Line: 2, Status: "passed", SpecBlob: strings.Repeat("ab", 20)},
	}}
	obligations := []Obligation{{ID: "change:spec/a_spec.rb", Kind: KindChange, Path: "spec/a_spec.rb", Witnesses: []Witness{}}}
	attached, unattached := attachEvidence(obligations, rubyTests(document))
	if attached != 1 || unattached != 1 || obligations[0].Witnesses[0].Test != "spec/a_spec.rb:2" {
		t.Fatalf("attached=%d unattached=%d witnesses=%+v", attached, unattached, obligations[0].Witnesses)
	}
}
