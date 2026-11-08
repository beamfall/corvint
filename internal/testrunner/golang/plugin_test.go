// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func source(raw string) Source {
	b := []byte(raw)
	sum := sha1.Sum(append([]byte("blob "+itoa(len(b))+"\x00"), b...))
	return Source{Mode: "100644", OID: hex.EncodeToString(sum[:]), Bytes: b}
}
func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
func events(rows ...goEvent) string {
	var b strings.Builder
	for _, r := range rows {
		raw, _ := json.Marshal(r)
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String()
}
func fixture(t *testing.T) *Plugin {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	c := Configuration{Checkout: strings.Repeat("a", 40), Runner: "go-test", OS: "linux", Arch: "amd64", Executable: filepath.Join(root, "go"), ExecutableDigest: strings.Repeat("b", 64), RunnerVersion: "go1.27.3", Package: "p", ImportPath: "example.test/m/p", Module: "example.test/m", Repository: "repo", Root: root, Environment: map[string]string{"GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0", "GOFLAGS": ""}}
	p, err := New(c, Capture{})
	if err != nil {
		t.Fatal(err)
	}
	deps, _ := json.Marshal(goPackage{Dir: filepath.Join(root, "p"), ImportPath: c.ImportPath, Name: "p", GoFiles: []string{"p.go"}, TestGoFiles: []string{"p_test.go"}})
	capture := Capture{Key: p.Key(), Complete: true, Dependencies: string(deps), List: events(goEvent{Action: "start", Package: c.ImportPath}, goEvent{Action: "output", Package: c.ImportPath, Output: "TestA\nTestB\n"}, goEvent{Action: "output", Package: c.ImportPath, Output: "ok  \t" + c.ImportPath + "\t0.001s\n"}, goEvent{Action: "pass", Package: c.ImportPath}), Sources: map[string]Source{"p/p_test.go": source("package p\nimport \"testing\"\nfunc TestA(t *testing.T) {}\nfunc TestB(t *testing.T) {}\n"), "p/p.go": source("package p\nfunc F() int {\n return 1\n}\n")}, Coverage: []linecov.Record{{Package: "p", Name: "TestA", Profile: []byte("mode: count\nexample.test/m/p/p.go:3.2,3.10 1 1\n")}}}
	p, err = p.BindCapture(capture)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func inventory(t *testing.T, p *Plugin) testrunner.RunnerInventory {
	t.Helper()
	inv, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: p.key, Output: p.capture.List})
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

func TestRSPV0002GoDeterministicInventoryAndConstraintCommands(t *testing.T) {
	p := fixture(t)
	a := inventory(t, p)
	b := inventory(t, p)
	one, err := testrunner.InventoryBytes(a, p.key)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := testrunner.InventoryBytes(b, p.key)
	if string(one) != string(two) || len(a.Tests) != 2 || a.Tests[0].ID != "p/TestA" || a.Tests[0].Path != "p/p_test.go" {
		t.Fatalf("%+v", a)
	}
	for _, purpose := range []string{"inventory", "loaded", "run"} {
		cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: p.key, Purpose: purpose, RunDir: p.config.Root})
		if err != nil || cmd.Env["GOOS"] != "linux" || cmd.Argv[len(cmd.Argv)-1] != "./p" {
			t.Fatalf("%s: %+v %v", purpose, cmd, err)
		}
	}
	other := p.config
	other.Tags = []string{"integration"}
	other.Race = true
	other.OS = "windows"
	other.Environment = map[string]string{"GOOS": "windows", "GOARCH": "amd64", "CGO_ENABLED": "1"}
	q, err := New(other, Capture{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := q.Command(context.Background(), registry.CommandRequest{Key: q.key, Purpose: "inventory", RunDir: other.Root})
	if err != nil {
		t.Fatal(err)
	}
	if q.key == p.key || !strings.Contains(strings.Join(cmd.Argv, " "), "-tags=integration -race") {
		t.Fatalf("%+v", cmd)
	}
	if _, err := q.BindCapture(p.capture); err == nil {
		t.Fatal("host capture reused across constraints")
	}
}

func TestRSPV0002GoIncompleteDiscoveryCannotNarrow(t *testing.T) {
	for _, mode := range []string{"truncated", "failed", "duplicate-json", "duplicate-test", "missing-test", "unbound", "cap", "missing-dependency", "wrong-target", "ginkgo", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			p := fixture(t)
			ctx := context.Background()
			switch mode {
			case "truncated":
				p.capture.List = strings.TrimSuffix(p.capture.List, events(goEvent{Action: "pass", Package: p.config.ImportPath}))
			case "failed":
				p.capture.Complete = false
			case "duplicate-json":
				p.capture.List = `{"Action":"pass","Action":"pass","Package":"example.test/m/p"}`
			case "duplicate-test":
				p.capture.List = strings.Replace(p.capture.List, "TestA\\nTestB", "TestA\\nTestA\\nTestB", 1)
			case "missing-test":
				p.capture.List = strings.Replace(p.capture.List, "TestB\\n", "", 1)
			case "unbound":
				s := p.capture.Sources["p/p_test.go"]
				s.OID = strings.Repeat("0", 40)
				p.capture.Sources["p/p_test.go"] = s
			case "cap":
				p.capture.List = strings.Repeat("x", MaxArtifactBytes+1)
			case "missing-dependency":
				p.capture.Dependencies = strings.Replace(p.capture.Dependencies, `"Imports":null`, `"Imports":["missing"]`, 1)
			case "wrong-target":
				p.capture.Key.OS = "darwin"
			case "ginkgo":
				p.config.Runner = "ginkgo-v2"
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			inv, err := p.Inventory(ctx, registry.InventoryRequest{Key: p.key, Output: p.capture.List})
			if err == nil || inv.Complete {
				t.Fatalf("admitted %+v", inv)
			}
			if plan := registry.NewSelectionSession(p, p.key, true).Plan(inv, nil, nil); !plan.Full || plan.Reason == "" {
				t.Fatalf("%+v", plan)
			}
		})
	}
}

func TestRSPV0005GoNativeEvidenceAndCoreTrace(t *testing.T) {
	p := fixture(t)
	native, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: "go-test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(native) != 2 {
		t.Fatal(len(native))
	}
	var executed testevidence.Component[testevidence.ExecutedFile]
	if err := json.Unmarshal(native[0].Executed, &executed); err != nil {
		t.Fatal(err)
	}
	if executed.State != testevidence.Present || len(executed.Items) != 1 || len(executed.Items[0].Executed) != 1 {
		t.Fatalf("%+v", executed)
	}
	var loaded testevidence.Component[testevidence.Blob]
	json.Unmarshal(native[0].Loaded, &loaded)
	if loaded.State != testevidence.Present || len(loaded.Items) != 2 {
		t.Fatalf("%+v", loaded)
	}
	header := testevidence.New("repo", "p/TestA", p.key.Checkout)
	header.Runner = testevidence.RunnerIdentity{Plugin: "go-test", PluginVersion: "1", ExecutableDigest: p.config.ExecutableDigest, RunnerVersion: p.config.RunnerVersion, ConfigurationDigest: p.key.ConfigDigest}
	header.Execution = testevidence.ExecutionIdentity{OS: p.key.OS, Architecture: p.key.Arch, TracingBackend: "fixture", TracingVersion: "1", EnvironmentDigest: strings.Repeat("c", 64)}
	header.CollectionCount = 1
	header.Sources = []testevidence.Producer{{CoreDigest: strings.Repeat("d", 64), VerifierRevision: p.key.Checkout, ConfigurationDigest: p.key.ConfigDigest, ContentDigest: strings.Repeat("e", 64), Ref: "refs/heads/main", Run: "fixture"}}
	missing := testevidence.New("repo", "p/TestA", p.key.Checkout)
	trace := testevidence.Trace{Reads: testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{p.blob("p/p.go", p.capture.Sources["p/p.go"])}}, Listings: missing.Listings, Lookups: missing.Lookups, Writes: missing.Writes, Execs: missing.Execs, External: missing.External}
	record, err := p.Assemble(context.Background(), header, trace)
	if err != nil {
		t.Fatal(err)
	}
	if record.Reads.State != testevidence.Present || len(record.Reads.Items) != 1 || record.External.State != testevidence.Absent || record.Regions.State != testevidence.Present {
		t.Fatalf("%+v", record)
	}
	if err := json.Unmarshal(native[1].Executed, &executed); err != nil {
		t.Fatal(err)
	}
	if executed.State != testevidence.Absent {
		t.Fatal("missing coverage certified")
	}
}

func TestRSPV0005GoMissingMalformedAndCappedEvidence(t *testing.T) {
	for _, mode := range []string{"mode", "unknown-test", "unbound", "malformed", "partial-graph", "unknown-artifact", "incomplete-profile"} {
		t.Run(mode, func(t *testing.T) {
			p := fixture(t)
			request := registry.EvidenceRequest{Runner: "go-test"}
			switch mode {
			case "mode":
				p.capture.Coverage[0].Profile = []byte("mode: future\n")
			case "unknown-test":
				p.capture.Coverage[0].Name = "TestUnknown"
			case "unbound":
				delete(p.capture.Sources, "p/p.go")
			case "malformed":
				p.capture.Coverage[0].Profile = []byte("mode: count\nmalformed\n")
			case "partial-graph":
				p.capture.Dependencies = strings.Replace(p.capture.Dependencies, `"GoFiles":["p.go"]`, `"GoFiles":["missing.go"]`, 1)
			case "unknown-artifact":
				request.Artifacts = []string{"untrusted.json"}
			case "incomplete-profile":
				p.capture.Coverage[0].Incomplete = true
			}
			got, err := p.Evidence(context.Background(), request)
			if mode == "partial-graph" {
				if err != nil {
					t.Fatal(err)
				}
				var c testevidence.Component[testevidence.Blob]
				json.Unmarshal(got[0].Loaded, &c)
				if c.State != testevidence.Incomplete || c.Reason == "" {
					t.Fatal(c)
				}
				return
			}
			if mode == "incomplete-profile" {
				if err != nil {
					t.Fatal(err)
				}
				var c testevidence.Component[testevidence.ExecutedFile]
				json.Unmarshal(got[0].Executed, &c)
				if c.State != testevidence.Incomplete {
					t.Fatal(c)
				}
				return
			}
			if err == nil {
				t.Fatal("malformed evidence admitted")
			}
		})
	}
}

func TestRSPV0033GoQualificationDeclaration(t *testing.T) {
	p := fixture(t)
	q, err := p.Qualification([]Corpus{{Repository: "corvint", Commit: p.key.Checkout}}, "traced")
	if err != nil {
		t.Fatal(err)
	}
	if q.Key != p.key || len(q.Classes) != 7 || len(q.MutationOperators) != 5 || q.Corpora[0].Repository != "corvint" {
		t.Fatalf("%+v", q)
	}
	q.Corpora[0].Repository = "changed"
	again, _ := p.Qualification([]Corpus{{Repository: "corvint", Commit: p.key.Checkout}}, "traced")
	if again.Corpora[0].Repository != "corvint" {
		t.Fatal("aliased")
	}
}
func TestRSPV0033GoQualificationRejectsUnknownInputs(t *testing.T) {
	p := fixture(t)
	for _, corpora := range [][]Corpus{nil, {{Repository: "repo", Commit: "main"}}, {{Repository: "repo", Commit: p.key.Checkout}, {Repository: "repo", Commit: p.key.Checkout}}} {
		if _, err := p.Qualification(corpora, "traced"); err == nil {
			t.Fatal("unpinned corpus accepted")
		}
	}
	if _, err := p.Qualification([]Corpus{{Repository: "repo", Commit: p.key.Checkout}}, "unknown"); err == nil {
		t.Fatal("unknown model accepted")
	}
}
func TestRSPV0034GoEveryDeclaredClassUnqualified(t *testing.T) {
	p := fixture(t)
	q, _ := p.Qualification([]Corpus{{Repository: "repo", Commit: p.key.Checkout}}, "traced")
	for _, c := range q.Classes {
		if c.Mode != "FULL" || c.Reason != "unqualified" {
			t.Fatal(c)
		}
	}
}
func TestRSPV0034GoCallerFlagCannotEnableClass(t *testing.T) {
	p := fixture(t)
	inv := inventory(t, p)
	plan := registry.NewSelectionSession(p, p.key, true).Plan(inv, []testrunner.Exclusion{{ID: "p/TestA", Fresh: true}}, nil)
	if !plan.Full || len(plan.Tests) != 2 || !strings.Contains(plan.Reason, "unqualified") {
		t.Fatal(plan)
	}
}
func TestRSPV0035GoDeclarationDoesNotInventQualificationStatistics(t *testing.T) {
	p := fixture(t)
	q, _ := p.Qualification([]Corpus{{Repository: "repo", Commit: p.key.Checkout}}, "traced")
	raw, _ := json.Marshal(q)
	for _, word := range []string{"Trials", "Misses", "Bound", "Seed"} {
		if strings.Contains(string(raw), word) {
			t.Fatalf("invented %s", word)
		}
	}
}
func TestRSPV0035GoEmptyCorpusCannotSupplyPopulation(t *testing.T) {
	p := fixture(t)
	if _, err := p.Qualification(nil, "traced"); err == nil {
		t.Fatal("empty population admitted")
	}
}
func TestRSPV0036GoIdentityChangesInvalidateCapturedInventory(t *testing.T) {
	p := fixture(t)
	for _, change := range []func(*Configuration){func(c *Configuration) { c.RunnerVersion = "new" }, func(c *Configuration) { c.ExecutableDigest = strings.Repeat("f", 64) }, func(c *Configuration) { c.Tags = []string{"integration"} }, func(c *Configuration) { c.Race = true }} {
		c := p.config
		change(&c)
		q, err := New(c, Capture{})
		if err != nil {
			t.Fatal(err)
		}
		if q.key == p.key {
			t.Fatal("identity not bound")
		}
		if _, err := q.BindCapture(p.capture); err == nil {
			t.Fatal("stale capture reused")
		}
	}
}
func TestRSPV0036GoUnchangedIdentityStillRequiresQualification(t *testing.T) {
	p := fixture(t)
	q, err := p.BindCapture(p.capture)
	if err != nil {
		t.Fatal(err)
	}
	if q.key != p.key || q.Pieces().FullReason(true) == "" {
		t.Fatal("unqualified instance enabled")
	}
}
func TestRSPV0040GoPiecesRemainShadow(t *testing.T) {
	p := fixture(t)
	if !p.Pieces().Inventory.Present || !p.Pieces().TestEvidence.Present {
		t.Fatal("pieces absent")
	}
	legacy, ok := registry.SelectionPlugin("go-test")
	if !ok || legacy.Pieces().FullReason(true) == "" {
		t.Fatal("legacy selection enabled")
	}
}
func TestRSPV0040GoMissingQualificationPreventsExclusion(t *testing.T) {
	p := fixture(t)
	plan := registry.NewSelectionSession(p, p.key, false).Plan(inventory(t, p), []testrunner.Exclusion{{ID: "p/TestA", Fresh: true}, {ID: "p/TestB", Fresh: true}}, nil)
	if !plan.Full || !reflect.DeepEqual(plan.Tests, []string{"p/TestA", "p/TestB"}) {
		t.Fatal(plan)
	}
}

func TestRSPV0002GoRunParsingPreservesFailuresAndSkipReasons(t *testing.T) {
	p := fixture(t)
	report := events(goEvent{Action: "fail", Package: p.config.ImportPath, Test: "TestA"}, goEvent{Action: "fail", Package: p.config.ImportPath})
	results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: "go-test", Report: report})
	if err == nil || len(results) != 1 || results[0].Outcome != "failed" {
		t.Fatalf("%+v %v", results, err)
	}
	report = events(goEvent{Action: "output", Package: p.config.ImportPath, Test: "TestA", Output: "requires service\n"}, goEvent{Action: "skip", Package: p.config.ImportPath, Test: "TestA"}, goEvent{Action: "pass", Package: p.config.ImportPath})
	results, err = p.Parse(context.Background(), registry.ParseRequest{Runner: "go-test", Report: report})
	if err != nil || len(results) != 1 || results[0].Reason != "requires service" {
		t.Fatalf("%+v %v", results, err)
	}
}

func TestRSPV0002GoInventoryRequiresEveryCIConstraint(t *testing.T) {
	p := fixture(t)
	c := p.config
	c.Tags = []string{"integration"}
	q, err := New(c, Capture{})
	if err != nil {
		t.Fatal(err)
	}
	capture := p.capture
	capture.Key = q.key
	q, err = q.BindCapture(capture)
	if err != nil {
		t.Fatal(err)
	}
	keys := []testrunner.InventoryKey{p.key, q.key}
	a, err := InventoryMatrix(context.Background(), keys, []*Plugin{q, p})
	if err != nil {
		t.Fatal(err)
	}
	b, err := InventoryMatrix(context.Background(), []testrunner.InventoryKey{q.key, p.key}, []*Plugin{p, q})
	if err != nil || len(a) != 2 || !reflect.DeepEqual(a, b) {
		t.Fatalf("%+v %+v %v", a, b, err)
	}
	for _, plugins := range [][]*Plugin{{p}, {p, p}, {p, nil}} {
		if inv, err := InventoryMatrix(context.Background(), keys, plugins); err == nil || inv != nil {
			t.Fatal("partial matrix admitted")
		}
	}
	q.capture.Complete = false
	if inv, err := InventoryMatrix(context.Background(), keys, []*Plugin{p, q}); err == nil || inv != nil {
		t.Fatal("incomplete target admitted")
	}
}

// RSP-V0-003: only decoded, known actions produce results; every error retains
// earlier terminal results, so a passing FULL retry cannot erase a failure.
func TestRSPV0003GoDamagedReportRetainsFailureAcrossRetry(t *testing.T) {
	for _, suffix := range []string{"", `{"Action":`, events(goEvent{Action: "future-action", Package: "example.test/m/p"}), events(goEvent{Action: "skip", Package: "example.test/m/p", Test: "TestB"}, goEvent{Action: "pass", Package: "example.test/m/p"}), strings.Repeat(" ", MaxArtifactBytes)} {
		t.Run(itoa(len(suffix)), func(t *testing.T) {
			p := fixture(t)
			broken := events(goEvent{Action: "fail", Package: p.config.ImportPath, Test: "TestA"}) + suffix
			results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: "go-test", Report: broken})
			if err == nil || len(results) != 1 || results[0].ID != "p/TestA" || results[0].Outcome != "failed" {
				t.Fatalf("lost failure or certified unknown suffix: %+v %v", results, err)
			}
			calls := 0
			out := registry.NewSelectionSession(p, p.key, false).Run(context.Background(), inventory(t, p), nil, nil, p.config.Root, true, func(_ context.Context, command registry.CommandResponse) (string, error) {
				calls++
				for _, arg := range command.Argv {
					if strings.HasPrefix(arg, "-run=") {
						t.Fatal("retry must run FULL")
					}
				}
				if calls == 1 {
					return broken, nil
				}
				return events(goEvent{Action: "pass", Package: p.config.ImportPath, Test: "TestA"}, goEvent{Action: "pass", Package: p.config.ImportPath, Test: "TestB"}, goEvent{Action: "pass", Package: p.config.ImportPath}), nil
			})
			if calls != 2 || out.Succeeded || !out.NeedsRequalification || !out.Attempts[1].Plan.Full || !out.Attempts[1].Reconciliation.Succeeded {
				t.Fatalf("passing retry erased prior failure: %+v", out)
			}
		})
	}
}

// RSP-V0-002/006/036: revisions bind captures independently of execution identity.
func TestRSPV0036GoCheckoutDoesNotChangeConfigurationDigest(t *testing.T) {
	p := fixture(t)
	config := p.config
	config.Checkout = strings.Repeat("c", 40)
	q, err := New(config, Capture{})
	if err != nil {
		t.Fatal(err)
	}
	if q.key.ConfigDigest != p.key.ConfigDigest || q.key.Checkout != config.Checkout || q.key == p.key {
		t.Fatalf("checkout/configuration bindings: base=%+v head=%+v", p.key, q.key)
	}
	if _, err := q.BindCapture(p.capture); err == nil {
		t.Fatal("base discovery reused as head inventory")
	}
}

func TestRSPV0007GoNestedModuleRepositoryPaths(t *testing.T) {
	for _, marker := range []string{"directory", "worktree-file"} {
		t.Run(marker, func(t *testing.T) {
			p := fixture(t)
			root := p.config.Root
			if marker == "worktree-file" {
				if err := os.Remove(filepath.Join(root, ".git")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /core-owned/git/worktrees/fixture\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			const prefix = "interop/cem01-go"
			config := p.config
			config.Root = filepath.Join(root, filepath.FromSlash(prefix))
			if err := os.MkdirAll(filepath.Join(config.Root, "p"), 0700); err != nil {
				t.Fatal(err)
			}
			q, err := New(config, Capture{})
			if err != nil {
				t.Fatal(err)
			}
			capture := p.capture
			capture.Key = q.key
			deps, err := decodeDependencies(capture.Dependencies)
			if err != nil {
				t.Fatal(err)
			}
			deps[0].Dir = filepath.Join(config.Root, "p")
			raw, err := json.Marshal(deps[0])
			if err != nil {
				t.Fatal(err)
			}
			capture.Dependencies = string(raw)
			capture.Sources = map[string]Source{}
			for name, s := range p.capture.Sources {
				capture.Sources[prefix+"/"+name] = s
			}
			q, err = q.BindCapture(capture)
			if err != nil {
				t.Fatal(err)
			}
			inv := inventory(t, q)
			for _, test := range inv.Tests {
				if test.Path != prefix+"/p/p_test.go" {
					t.Fatalf("module-relative inventory: %+v", test)
				}
			}
			records, err := q.native(context.Background())
			if err != nil || len(records) != 2 {
				t.Fatalf("records=%+v err=%v", records, err)
			}
			r := records[0]
			if r.Executed.State != testevidence.Present || len(r.Executed.Items) != 1 || r.Regions.State != testevidence.Present || len(r.Regions.Items) != 1 || r.Loaded.State != testevidence.Present || len(r.Loaded.Items) != 2 {
				t.Fatalf("incomplete nested evidence: %+v", r)
			}
			blobs := append([]testevidence.Blob{r.Executed.Items[0].Blob, r.Regions.Items[0].Blob}, r.Loaded.Items...)
			for _, blob := range blobs {
				if blob.Path.Repository != "repo" || !strings.HasPrefix(blob.Path.Lexical, prefix+"/p/") || blob.Path.Resolved != blob.Path.Lexical || blob.OID != capture.Sources[blob.Path.Lexical].OID {
					t.Fatalf("unbound repository path: %+v", blob)
				}
			}
			if r.Executed.Items[0].Blob.Path.Lexical != "interop/cem01-go/p/p.go" {
				t.Fatal("repository change would not match executed evidence")
			}
			// Module-relative captures must not certify repository-relative paths.
			capture.Sources = p.capture.Sources
			q, err = q.BindCapture(capture)
			if err != nil {
				t.Fatal(err)
			}
			if inv, err := q.Inventory(context.Background(), registry.InventoryRequest{Key: q.key, Output: capture.List}); err == nil || inv.Complete {
				t.Fatal("module-relative sources admitted as repository paths")
			}
		})
	}
}

func TestRSPV0007GoUnknownRepositoryBoundaryRefusesEvidence(t *testing.T) {
	p := fixture(t)
	if err := os.Remove(filepath.Join(p.config.Root, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(p.config, Capture{}); err == nil || err.Error() != "go-repository-root-unresolved" {
		t.Fatalf("unknown boundary admitted: %v", err)
	}
}

func TestRSPV0002GoGinkgoInventoryUnsupported(t *testing.T) {
	p := fixture(t)
	config := p.config
	config.Runner = "ginkgo-v2"
	q, err := New(config, Capture{})
	if err != nil {
		t.Fatal(err)
	}
	capture := p.capture
	capture.Key = q.key
	q, err = q.BindCapture(capture)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := q.Inventory(context.Background(), registry.InventoryRequest{Key: q.key, Output: capture.List})
	if err == nil || err.Error() != "go-ginkgo-inventory-unsupported" || inv.Complete || inv.Reason != "go-ginkgo-inventory-unsupported" {
		t.Fatalf("inventory=%+v err=%v", inv, err)
	}
	plan := registry.NewSelectionSession(q, q.key, true).Plan(inv, nil, nil)
	if !plan.Full || !strings.Contains(plan.Reason, "go-ginkgo-inventory-unsupported") {
		t.Fatalf("Ginkgo did not remain explicitly FULL: %+v", plan)
	}
}

func TestRSPV0003GoSkipRequiresNonLifecycleReason(t *testing.T) {
	for _, message := range []string{"", "    p_test.go:4: requires service\n"} {
		t.Run(message, func(t *testing.T) {
			p := fixture(t)
			report := events(
				goEvent{Action: "run", Package: p.config.ImportPath, Test: "TestA"},
				goEvent{Action: "output", Package: p.config.ImportPath, Test: "TestA", Output: "=== RUN   TestA\n=== PAUSE TestA\n=== CONT  TestA\n" + message + "--- SKIP: TestA (0.00s)\n"},
				goEvent{Action: "skip", Package: p.config.ImportPath, Test: "TestA"},
				goEvent{Action: "pass", Package: p.config.ImportPath})
			results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: "go-test", Report: report})
			if message == "" {
				if err == nil || err.Error() != "go-skip-reason-missing" || len(results) != 0 {
					t.Fatalf("harness output invented a reason: %+v %v", results, err)
				}
			} else if err != nil || len(results) != 1 || results[0].Outcome != "skipped" || results[0].Reason != strings.TrimSpace(message) {
				t.Fatalf("skip reason lost: %+v %v", results, err)
			}
		})
	}
}

// RSP-V0-003: only parent terminals certify completion; any descendant failure
// is retained against its owner, including unknown forms and missing terminals.
func TestRSPV0003GoChildFailureSurvivesFullRetry(t *testing.T) {
	for _, suffix := range []string{"", `{"Action":`, events(goEvent{Action: "future-action", Package: "example.test/m/p"}), events(goEvent{Action: "pass", Package: "example.test/m/p"}), events(goEvent{Action: "pass", Package: "example.test/m/p", Test: "TestA"}, goEvent{Action: "pass", Package: "example.test/m/p"})} {
		t.Run(itoa(len(suffix)), func(t *testing.T) {
			p := fixture(t)
			broken := events(goEvent{Action: "fail", Package: p.config.ImportPath, Test: "TestA/child"}, goEvent{Action: "fail", Package: p.config.ImportPath, Test: "TestA/other/nested"}) + suffix
			results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: "go-test", Report: broken})
			if err == nil || len(results) != 1 || results[0].ID != "p/TestA" || results[0].Outcome != "failed" {
				t.Fatalf("lost child failure or certified incomplete report: %+v %v", results, err)
			}
			if strings.Contains(suffix, "future-action") && err.Error() != "go-output-action-unknown" {
				t.Fatalf("unknown action did not fail closed: %v", err)
			}
			calls := 0
			out := registry.NewSelectionSession(p, p.key, false).Run(context.Background(), inventory(t, p), nil, nil, p.config.Root, true, func(_ context.Context, command registry.CommandResponse) (string, error) {
				calls++
				for _, arg := range command.Argv {
					if strings.HasPrefix(arg, "-run=") {
						t.Fatal("retry must run FULL")
					}
				}
				if calls == 1 {
					return broken, nil
				}
				return events(goEvent{Action: "pass", Package: p.config.ImportPath, Test: "TestA"}, goEvent{Action: "pass", Package: p.config.ImportPath, Test: "TestB"}, goEvent{Action: "pass", Package: p.config.ImportPath}), nil
			})
			if calls != 2 || len(out.Attempts) != 2 || out.Succeeded || !out.NeedsRequalification || !out.Attempts[1].Plan.Full || !out.Attempts[1].Reconciliation.Succeeded {
				t.Fatalf("passing retry erased child failure: %+v", out)
			}
		})
	}
	p := fixture(t)
	complete := events(goEvent{Action: "fail", Package: p.config.ImportPath, Test: "TestA/child"}, goEvent{Action: "fail", Package: p.config.ImportPath, Test: "TestA"}, goEvent{Action: "fail", Package: p.config.ImportPath})
	results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: "go-test", Report: complete})
	if err == nil || err.Error() != "go-package-failed" || len(results) != 1 || results[0].ID != "p/TestA" || results[0].Outcome != "failed" {
		t.Fatalf("child duplicated parent failure: %+v %v", results, err)
	}
}
