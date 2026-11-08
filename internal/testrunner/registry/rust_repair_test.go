// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// RSP-V0-005, LPCV-V0-952: a zero LCOV count on an arm cannot certify
// the pattern's columns. Replacing 1 with 2 affects a test calling value(2).
func TestRSPV0005RustLCOVMatchPatternSelectsEnclosingBody(t *testing.T) {
	o := rustOptions("cargo-test")
	text := "pub fn value(x: u8) -> u8 {\n    match x {\n        1 => 10,\n        _ => 20,\n    }\n}\n"
	h := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(text), text)))
	src := o.Sources["src/lib.rs"]
	src.Data, src.Blob.OID = []byte(text), hex.EncodeToString(h[:])
	o.Sources["src/lib.rs"] = src
	// Adjacent zero-count lines must not union into a whole-line certificate.
	for _, counts := range []string{"DA:2,1\nDA:3,0\nDA:4,1\n", "DA:2,1\nDA:3,0\nDA:4,0\n"} {
		o.Artifacts["lines"] = []byte("SF:" + filepath.Join(o.Root, "src/lib.rs") + "\n" + counts + "end_of_record\n")
		native, err := rustNew(t, o).Evidence(context.Background(), registry.EvidenceRequest{Runner: o.Key.Runner, Artifacts: []string{"lines"}})
		if err != nil || len(native) == 0 {
			t.Fatalf("fixture evidence failed: %v", err)
		}
		r := rustNativeRecord(t, native[0])
		if r.Executed.State != testevidence.Present || r.Regions.State != testevidence.Present || len(r.Regions.Items[0].Bodies) != 1 {
			t.Fatalf("fixture did not reach line/body selection: %+v", r)
		}
		// Core admission is synthetic here to exercise the actual selector;
		// the Rust plugin itself still declares qualification absent.
		r.Runner = testevidence.RunnerIdentity{Plugin: "fixture", PluginVersion: "1", ExecutableDigest: strings.Repeat("b", 64), RunnerVersion: "1", ConfigurationDigest: o.Key.ConfigDigest}
		r.Execution = testevidence.ExecutionIdentity{OS: o.Key.OS, Architecture: o.Key.Arch, TracingBackend: "fixture", TracingVersion: "1", EnvironmentDigest: strings.Repeat("c", 64)}
		r.Sources = []testevidence.Producer{{CoreDigest: strings.Repeat("d", 64), VerifierRevision: o.Key.Checkout, ConfigurationDigest: o.Key.ConfigDigest, Ref: "refs/heads/main", Run: "fixture", ContentDigest: strings.Repeat("e", 64)}}
		r.CollectionCount = 1
		r.Reads = testevidence.Component[testevidence.Blob]{State: testevidence.Present}
		r.Listings = testevidence.Component[testevidence.Path]{State: testevidence.Present}
		r.Lookups = testevidence.Component[testevidence.Path]{State: testevidence.Present}
		r.Writes = testevidence.Component[testevidence.Write]{State: testevidence.Present}
		r.Execs = testevidence.Component[testevidence.Executable]{State: testevidence.Present}
		r.External = testevidence.Component[testevidence.Contact]{State: testevidence.Present}
		// Exercise the collection union before selection too.
		r, err = testevidence.Union(r, r)
		if err != nil {
			t.Fatal(err)
		}
		head := strings.Repeat("9", 40)
		key := o.Key
		key.Checkout = head
		present := testrunner.RunnerPiece{Present: true}
		runner := runnerselection.Runner{Name: key.Runner, Identity: r.Runner, Execution: r.Execution,
			Pieces:        testrunner.RunnerPieces{Inventory: present, TestEvidence: present, Qualification: present},
			Inventory:     testrunner.RunnerInventory{Key: key, Complete: true, Tests: []testrunner.InventoryTest{{ID: r.Test, Path: "src/lib.rs"}}},
			Qualification: runnerselection.Qualification{Admitted: true, Passed: true, SamplingEnabled: true, SingleCollectionQualified: true, Model: "traced", Classes: []runnerselection.Class{runnerselection.BodyEdit}, Runner: r.Runner, Execution: r.Execution},
			Records:       []runnerselection.Evidence{{Record: r, Admitted: true}}, IndexBase: r.Commit, IndexVerified: true}
		snapshot := testevidence.Snapshot{Blobs: map[testevidence.PathKey]string{}}
		for name, source := range o.Sources {
			snapshot.Blobs[testevidence.PathKey{Repository: "repo", Path: name}] = source.Blob.OID
		}
		plan := runnerselection.Select(runnerselection.Input{Base: r.Commit, Head: head, Repository: "repo", Repositories: map[string]bool{"repo": true}, DiffComplete: true,
			Changes:  []runnerselection.Change{{Repository: "repo", Path: "src/lib.rs", Kind: "modify", Decoded: true, Hunks: []runnerselection.Hunk{{Start: 3, End: 3}}}},
			Snapshot: snapshot, Runners: []runnerselection.Runner{runner}}).Runners[0]
		if plan.Mode != "selected" || len(plan.Excluded) != 0 || len(plan.Selected) != 1 || !slices.Contains(plan.Selected[0].Rules, "rule:b") {
			t.Fatalf("match pattern change escaped enclosing body: %+v", plan)
		}
	}
}

func TestRSPV0002RustNextestNativeCaseKind(t *testing.T) {
	raw, err := os.ReadFile("testdata/rust-nextest-list.json")
	if err != nil {
		t.Fatal(err)
	}
	o := rustOptions("nextest")
	p := rustNew(t, o)
	for _, kind := range []string{`"test"`, `null`} {
		input := strings.ReplaceAll(string(raw), `"kind": "test"`, `"kind": `+kind)
		in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: input})
		if err != nil || !in.Complete || len(in.Tests) != 3 {
			t.Fatalf("native inventory %s: %+v %v", kind, in, err)
		}
	}
	for _, kind := range []string{`"bench"`, `"future"`, `42`, `{}`} {
		input := strings.ReplaceAll(string(raw), `"kind": "test"`, `"kind": `+kind)
		in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: input})
		if err == nil || in.Complete {
			t.Fatalf("unsupported native kind %s: %+v %v", kind, in, err)
		}
	}
}

func TestRSPV0005RustCollectionCompilerFlagPrecedence(t *testing.T) {
	for _, runner := range []string{"cargo-test", "nextest"} {
		for _, tc := range []struct {
			name string
			env  map[string]string
			want string
		}{
			{"plain", map[string]string{"RUSTFLAGS": "--cfg custom"}, "--cfg\x1fcustom\x1f-C\x1finstrument-coverage"},
			{"encoded-wins", map[string]string{"RUSTFLAGS": "--cfg wrong", "CARGO_ENCODED_RUSTFLAGS": "--cfg\x1fcustom=\"two words\""}, "--cfg\x1fcustom=\"two words\"\x1f-C\x1finstrument-coverage"},
			{"empty-encoded-wins", map[string]string{"RUSTFLAGS": "--cfg wrong", "CARGO_ENCODED_RUSTFLAGS": ""}, "-C\x1finstrument-coverage"},
		} {
			t.Run(runner+"/"+tc.name, func(t *testing.T) {
				o := rustOptions(runner)
				o.CompilerEnvironment = tc.env
				p := rustNew(t, o)
				// Caller mutation cannot alter the captured effective flags.
				for key := range tc.env {
					tc.env[key] = "--cfg mutated"
				}
				cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: "collect", RunDir: o.Root})
				if err != nil || cmd.Env["CARGO_ENCODED_RUSTFLAGS"] != tc.want {
					t.Fatalf("effective flags lost or instrumentation overridden: %+v %v", cmd, err)
				}
			})
		}
		o := rustOptions(runner)
		o.CompilerEnvironment = nil
		p := rustNew(t, o)
		// Config-only flags (including --cfg) are unresolved by this artifact-
		// bound SPI. Refuse collection rather than replace Cargo's config.
		if _, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: "collect", RunDir: o.Root}); err == nil {
			t.Fatal("uncaptured/config-only flags silently replaced")
		}
		if _, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: "inventory"}); err != nil {
			t.Fatal("flag capture unexpectedly required for inventory", err)
		}
	}
}

func TestRSPV0005RustAncestorCargoConfigurationLoaded(t *testing.T) {
	for _, config := range []string{".cargo/config", ".cargo/config.toml"} {
		o := rustOptions("cargo-test")
		originalRoot := o.Root
		nestedRoot := filepath.Join(o.Root, "workspace")
		o.Manifest = "workspace/Cargo.toml"
		oldJSON, _ := json.Marshal(originalRoot)
		newJSON, _ := json.Marshal(nestedRoot)
		oldPrefix, newPrefix := string(oldJSON[1:len(oldJSON)-1]), string(newJSON[1:len(newJSON)-1])
		o.Metadata = []byte(strings.ReplaceAll(string(o.Metadata), oldPrefix, newPrefix))
		o.BuildMessages = []byte(strings.ReplaceAll(string(o.BuildMessages), oldPrefix, newPrefix))
		sources := map[string]registry.RustSource{}
		for name, src := range o.Sources {
			src.Blob.Path.Lexical, src.Blob.Path.Resolved = "workspace/"+name, "workspace/"+name
			sources["workspace/"+name] = src
		}
		for id, name := range o.TestSources {
			o.TestSources[id] = "workspace/" + name
		}
		o.NativeSources = nil
		o.Sources = sources
		var before string
		for _, data := range []string{"[build]\nrustflags=['--cfg=before']\n", "[build]\nrustflags=['--cfg=after']\n"} {
			h := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(data), data)))
			o.Sources[config] = registry.RustSource{Data: []byte(data), Blob: testevidence.Blob{Path: testevidence.Path{Repository: "repo", Lexical: config, Resolved: config}, OID: hex.EncodeToString(h[:])}}
			native, err := rustNew(t, o).Evidence(context.Background(), registry.EvidenceRequest{Runner: o.Key.Runner})
			if err != nil || len(native) == 0 {
				t.Fatal(native, err)
			}
			loaded := rustNativeRecord(t, native[0]).Loaded
			if loaded.State != testevidence.Present {
				t.Fatal(loaded)
			}
			found := ""
			for _, blob := range loaded.Items {
				if blob.Path.Lexical == config {
					found = blob.OID
				}
			}
			if found == "" || found == before {
				t.Fatalf("ancestor config omitted or stale: %+v", loaded)
			}
			before = found
		}
	}
}

func TestRSPV0002RustIntegrationTestsFailClosed(t *testing.T) {
	for _, mode := range []string{"auto-discovered", "explicit-target", "unknown-universe"} {
		o := rustOptions("cargo-test")
		want := "rust-integration-tests-unsupported"
		switch mode {
		case "auto-discovered":
			src := o.Sources["src/lib.rs"]
			src.Blob.Path.Lexical, src.Blob.Path.Resolved = "tests/integration.rs", "tests/integration.rs"
			o.Sources["tests/integration.rs"] = src
		case "explicit-target":
			o.Metadata = []byte(strings.ReplaceAll(string(o.Metadata), `"lib"`, `"test"`))
		case "unknown-universe":
			o.SourcesComplete = false
			want = "rust-library-target-universe-incomplete"
		}
		p := rustNew(t, o)
		in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: "a: test\nb: test\nc: test\n\n3 tests, 0 benchmarks\n"})
		if err == nil || err.Error() != want || in.Complete || in.Reason != want {
			t.Fatalf("%s: %+v %v", mode, in, err)
		}
		for _, purpose := range []string{"inventory", "run", "collect"} {
			cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: purpose, RunDir: t.TempDir()})
			if err == nil || err.Error() != want || len(cmd.Argv) != 0 {
				t.Fatalf("%s/%s: %+v %v", mode, purpose, cmd, err)
			}
		}
	}
}

// RSP-V0-002: --lib can certify only the single-library target universe.
func TestRSPV0002RustTargetAllowlist(t *testing.T) {
	for _, kinds := range [][]string{{"bin"}, {"future-target"}, {"example"}, {"bench"}, {}, {"lib", "future-target"}, {"lib"}} {
		t.Run(fmt.Sprint(kinds), func(t *testing.T) {
			o := rustOptions("cargo-test")
			text := "fn main() {}\n#[test] fn binary_test() {}\n"
			h := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(text), text)))
			o.Sources["src/main.rs"] = registry.RustSource{Data: []byte(text), Blob: testevidence.Blob{Path: testevidence.Path{Repository: "repo", Lexical: "src/main.rs", Resolved: "src/main.rs"}, OID: hex.EncodeToString(h[:])}}
			var metadata map[string]any
			if err := json.Unmarshal(o.Metadata, &metadata); err != nil {
				t.Fatal(err)
			}
			pkg := metadata["packages"].([]any)[0].(map[string]any)
			pkg["targets"] = append(pkg["targets"].([]any), map[string]any{"kind": kinds, "src_path": filepath.Join(o.Root, "src/main.rs")})
			o.Metadata, _ = json.Marshal(metadata)
			p := rustNew(t, o)
			const want = "rust-library-target-universe-incomplete"
			in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: rustListing(o.Key.Runner)})
			if err == nil || err.Error() != want || in.Complete || in.Reason != want {
				t.Fatalf("unsupported target admitted: %+v %v", in, err)
			}
			for _, purpose := range []string{"inventory", "run", "collect"} {
				cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: purpose, RunDir: t.TempDir()})
				if err == nil || err.Error() != want || len(cmd.Argv) != 0 {
					t.Fatalf("%s: %+v %v", purpose, cmd, err)
				}
			}
		})
	}
}
