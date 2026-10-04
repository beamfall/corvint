package postmergeworkflow

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/intake"
	connector "github.com/Beamfall/corvint/internal/postmergeconnector"
)

type nativeFixture struct {
	root, fixture, policy string
	f                     Fixture
	p                     NativePolicy
	m                     NativeManifest
	candidate             []byte
}

func nativeSetup(t *testing.T) nativeFixture {
	t.Helper()
	root, ff, _, f, old := setup(t, "must-not-launch")
	root, e := filepath.EvalSymlinks(root)
	if e != nil {
		t.Fatal(e)
	}
	inputs, e := filepath.EvalSymlinks(filepath.Dir(ff))
	if e != nil {
		t.Fatal(e)
	}
	admitted, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	outputParent, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	f.Profile = NativeProfile
	candidate, _ := json.Marshal(intake.Record{Profile: intake.Profile, Base: old.Connector.Expected.Base, Head: old.Connector.Expected.Merge, Intent: "FEATURE", Behaviours: []intake.Behaviour{{Kind: "CHANGE", Path: "a.txt"}}, Flags: []string{}, Tests: []string{}, Comparison: "UNKNOWN", Concerns: []string{}, WorkItems: []intake.WorkItem{}})
	m := NativeManifest{Profile: NativeManifestProfile, Mode: "candidate-qualification", FixtureSHA256: valueHash(f)}
	m.Implementation = Implementation{SourceCommit: git(t, "../..", "rev-parse", "HEAD"), SourceTree: git(t, "../..", "rev-parse", "HEAD^{tree}"), ExecutableSHA256: old.ExecutableSHA256}
	m.Product.Base = old.Connector.Expected.Base
	m.Product.Merge = old.Connector.Expected.Merge
	m.Product.Tree = git(t, root, "rev-parse", m.Product.Merge+"^{tree}")
	m.ReaderCandidate.Path = filepath.Join(inputs, "candidate.json")
	m.ReaderCandidate.SHA256 = SHA256(candidate)
	m.RetainedOutputRoot = filepath.Join(outputParent, "retained")
	p := NativePolicy{Profile: NativeProfile, Connector: old.Connector, Manifest: filepath.Join(admitted, "manifest.json")}
	n := nativeFixture{root: root, fixture: filepath.Join(inputs, "native-fixture.json"), policy: filepath.Join(inputs, "native-policy.json"), f: f, p: p, m: m, candidate: candidate}
	n.write(t)
	return n
}
func (n *nativeFixture) write(t *testing.T) {
	t.Helper()
	writeJSON(t, n.fixture, n.f)
	b, e := os.ReadFile(n.fixture)
	if e != nil {
		t.Fatal(e)
	}
	n.m.FixtureSHA256 = SHA256(b)
	if e = os.WriteFile(n.m.ReaderCandidate.Path, n.candidate, 0600); e != nil {
		t.Fatal(e)
	}
	n.m.ReaderCandidate.SHA256 = SHA256(n.candidate)
	writeJSON(t, n.p.Manifest, n.m)
	b, e = os.ReadFile(n.p.Manifest)
	if e != nil {
		t.Fatal(e)
	}
	n.p.ManifestSHA256 = SHA256(b)
	writeJSON(t, n.policy, n.p)
}
func readNativeRef(t *testing.T, r ArtifactRef) []byte {
	t.Helper()
	b, e := os.ReadFile(r.Path)
	if e != nil {
		t.Fatal(e)
	}
	if int64(len(b)) != r.Bytes || SHA256(b) != r.SHA256 {
		t.Fatalf("unbound reference %+v", r)
	}
	i, e := os.Stat(r.Path)
	if e != nil || i.Mode().Perm() != 0600 {
		t.Fatalf("private artifact permissions %v %v", i, e)
	}
	return b
}

// PMR-V1-001/002/007/010: actual pinned native consumers, private exact bytes,
// no expectation comparison or downstream authority, and mandatory refusal.
func TestNativeActualIntakeThenDeltaRefusal(t *testing.T) {
	n := nativeSetup(t)
	before := git(t, n.root, "status", "--porcelain")
	r, e := ReplayNative(context.Background(), n.root, n.fixture, n.policy, n.f.Connector.Forge.Change.Binding.Change)
	if e == nil || r.Profile != NativeProfile || r.Status != "BLOCKED" || r.ComparisonStatus != "NOT_RUN" || r.WorkflowQualification != "NOT_OBSERVED" || len(r.Stages) != 16 {
		t.Fatalf("report %+v error %v", r, e)
	}
	if len(r.GeneratedMismatches)+len(r.HumanVerifiedMismatches) != 0 {
		t.Fatal("uncomputed labels treated as comparisons")
	}
	for i := 1; i <= 2; i++ {
		if r.Stages[i].Disposition != "observed" || r.Stages[i].Output == nil {
			t.Fatalf("native not observed %+v", r.Stages[i])
		}
	}
	actualConnector, e := connector.Read(context.Background(), n.root, n.f.Connector, n.p.Connector, n.f.SourceItem)
	if e != nil {
		t.Fatal(e)
	}
	cb, _ := json.Marshal(actualConnector)
	if string(readNativeRef(t, *r.Stages[1].Output)) != string(cb) {
		t.Fatal("connector output not its native bytes")
	}
	actualIntake, e := intake.BuildAuthorInput(context.Background(), n.root, n.m.Product.Base, n.m.Product.Merge, n.candidate)
	if e != nil {
		t.Fatal(e)
	}
	if string(readNativeRef(t, *r.Stages[2].Output)) != string(actualIntake) {
		t.Fatal("intake output not its native bytes")
	}
	if r.Stages[1].Output.Path == r.Stages[2].Output.Path {
		t.Fatal("output references aliased")
	}
	candidate := readNativeRef(t, r.Stages[2].Inputs[1])
	if string(candidate) != string(n.candidate) {
		t.Fatal("original candidate replaced")
	}
	if strings.Contains(string(actualIntake), "HOSTILE") || strings.Contains(string(actualIntake), "raw body") || strings.Contains(string(actualIntake), "affected_flows") {
		t.Fatal("fixture prose/expectations crossed author boundary")
	}
	if r.Stages[3].Disposition != "blocked" || r.Stages[3].Reasons[0] != "actual-delta-unavailable" {
		t.Fatalf("delta %+v", r.Stages[3])
	}
	for i, s := range r.Stages {
		if i == 1 || i == 2 {
			continue
		}
		if i == 0 && (s.Disposition != "not-run" || s.Reasons[0] != "host-trigger-not-executed") {
			t.Fatalf("trigger falsely observed %+v", s)
		}
		if i >= 4 && (s.Disposition != "not-run" || s.Reasons[0] != "dependency-delta-blocked") {
			t.Fatalf("dependent stage not refused %+v", s)
		}
		if s.NativeProfile != "" || s.Output != nil {
			t.Fatalf("unexecuted native output %+v", s)
		}
	}
	if before != git(t, n.root, "status", "--porcelain") {
		t.Fatal("product checkout mutated")
	}
	i, e := os.Stat(n.m.RetainedOutputRoot)
	if e != nil || i.Mode().Perm() != 0700 {
		t.Fatal("output not private")
	}
}

// PMR-V1-001/003/004/009: binding and closed admission cannot launch stages,
// authoring, validation, recording or operational reuse.
func TestNativeAdmissionFailures(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		change       func(*nativeFixture)
	}{
		{"fixture-profile", "fixture-policy-binding-invalid", func(n *nativeFixture) { n.f.Profile = Profile }},
		{"human-label", "label-qualification-unavailable", func(n *nativeFixture) {
			x := n.f.Expected.Labels["followup"]
			x.Basis = "human-verified"
			n.f.Expected.Labels["followup"] = x
		}},
		{"operational", "operational-qualification-unavailable", func(n *nativeFixture) { n.m.Mode = "operational" }},
		{"tree-pin", "runtime-product-pin-invalid", func(n *nativeFixture) { n.m.Product.Tree = strings.Repeat("a", 40) }},
		{"executable-pin", "executable-pin-mismatch", func(n *nativeFixture) { n.m.Implementation.ExecutableSHA256 = strings.Repeat("a", 64) }},
		{"output-in-product", "runtime-output-invalid", func(n *nativeFixture) { n.m.RetainedOutputRoot = filepath.Join(n.root, "retained") }},
		{"manifest-in-inputs", "runtime-manifest-invalid", func(n *nativeFixture) { n.p.Manifest = filepath.Join(filepath.Dir(n.fixture), "manifest.json") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := nativeSetup(t)
			tc.change(&n)
			n.write(t)
			r, e := ReplayNative(context.Background(), n.root, n.fixture, n.policy, n.f.Connector.Forge.Change.Binding.Change)
			if e == nil || len(r.Reasons) != 1 || r.Reasons[0] != tc.reason {
				t.Fatalf("%+v %v", r, e)
			}
			for _, s := range r.Stages {
				if s.Disposition == "observed" {
					t.Fatal("admission failure executed stage")
				}
			}
		})
	}
}

// PMR-V1-002/010: genuine native rejection retains error and prior observations.
func TestNativeIntakeRejectsInvalidCandidate(t *testing.T) {
	n := nativeSetup(t)
	var fields map[string]any
	if json.Unmarshal(n.candidate, &fields) != nil {
		t.Fatal("candidate")
	}
	fields["verified"] = true
	n.candidate, _ = json.Marshal(fields)
	n.write(t)
	r, e := ReplayNative(context.Background(), n.root, n.fixture, n.policy, n.f.Connector.Forge.Change.Binding.Change)
	if e == nil || r.Stages[1].Disposition != "observed" || r.Stages[2].Disposition != "blocked" || r.Stages[2].Output != nil || r.Stages[2].NativeProfile != "" {
		t.Fatalf("%+v %v", r, e)
	}
	b, e := os.ReadFile(filepath.Join(n.m.RetainedOutputRoot, "intake-error.txt"))
	if e != nil || string(b) != intake.ErrSchema.Error() {
		t.Fatalf("native original error %q %v", b, e)
	}
}

// PMR-V1-001/010: aliases, duplicates, missing/unknown fields, existing evidence
// and cancellation refuse rather than granting an observation from a digest.
func TestNativeStrictInputsAndRetention(t *testing.T) {
	for _, kind := range []string{"duplicate", "unknown", "missing", "case-alias", "symlink", "existing-output", "cancelled", "missing-expectation", "oversized", "trailing", "null-expectation"} {
		t.Run(kind, func(t *testing.T) {
			n := nativeSetup(t)
			ctx := context.Background()
			switch kind {
			case "duplicate":
				b, _ := os.ReadFile(n.policy)
				b = append([]byte(`{"profile":"postmerge-replay/1",`), b[1:]...)
				os.WriteFile(n.policy, b, 0600)
			case "unknown":
				b, _ := os.ReadFile(n.policy)
				b = append([]byte(`{"verified":true,`), b[1:]...)
				os.WriteFile(n.policy, b, 0600)
			case "missing":
				b, _ := os.ReadFile(n.policy)
				var x map[string]any
				json.Unmarshal(b, &x)
				delete(x, "manifest_sha256")
				writeJSON(t, n.policy, x)
			case "case-alias":
				b, _ := os.ReadFile(n.policy)
				b = []byte(strings.Replace(string(b), `"profile"`, `"Profile"`, 1))
				os.WriteFile(n.policy, b, 0600)
			case "symlink":
				alias := filepath.Join(filepath.Dir(n.policy), "alias.json")
				if e := os.Symlink(n.policy, alias); e != nil {
					t.Fatal(e)
				}
				n.policy = alias
			case "existing-output":
				if e := os.Mkdir(n.m.RetainedOutputRoot, 0700); e != nil {
					t.Fatal(e)
				}
			case "missing-expectation":
				b, _ := os.ReadFile(n.fixture)
				var x map[string]any
				json.Unmarshal(b, &x)
				delete(x["expected"].(map[string]any), "followup")
				writeJSON(t, n.fixture, x)
			case "null-expectation":
				b, _ := os.ReadFile(n.fixture)
				var x map[string]any
				json.Unmarshal(b, &x)
				x["expected"].(map[string]any)["followup"] = nil
				writeJSON(t, n.fixture, x)
			case "oversized":
				os.WriteFile(n.policy, []byte(strings.Repeat(" ", MaxBytes+1)), 0600)
			case "trailing":
				b, _ := os.ReadFile(n.policy)
				os.WriteFile(n.policy, append(b, []byte(" {}")...), 0600)
			case "cancelled":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			r, e := ReplayNative(ctx, n.root, n.fixture, n.policy, n.f.Connector.Forge.Change.Binding.Change)
			if e == nil || r.Status != "BLOCKED" {
				t.Fatalf("%+v %v", r, e)
			}
			for _, s := range r.Stages {
				if s.Disposition == "observed" {
					t.Fatal("unsafe input yielded native observation")
				}
			}
		})
	}
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	r, e := newNativeRetention(filepath.Join(dir, "bounded"), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer r.root.Close()
	r.bytes = 16 << 20
	if _, e = r.save("overflow", []byte("x")); e == nil {
		t.Fatal("retention overflow accepted")
	}
}

// PMR-V1-007/009: real companion CLI emits closed /1 BLOCKED and exits2;
// this synthetic native conformance fixture grants no historical/host qualification.
func TestNativeCLIExitTwo(t *testing.T) {
	n := nativeSetup(t)
	bin := filepath.Join(t.TempDir(), "workflow")
	cmd := exec.Command("go", "build", "-o", bin, "../../cmd/corvint-postmerge-workflow")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build %s %v", b, e)
	}
	b, e := os.ReadFile(bin)
	if e != nil {
		t.Fatal(e)
	}
	n.m.Implementation.ExecutableSHA256 = SHA256(b)
	n.write(t)
	cmd = exec.Command(bin, "replay", "--change", n.f.Connector.Forge.Change.Binding.Change, "--dry-run", "--fixture", n.fixture, "--policy", n.policy, "--root", n.root)
	b, e = cmd.Output()
	exit, ok := e.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("exit %v body %s", e, b)
	}
	var report NativeReport
	if Decode(b, &report) != nil || report.Profile != NativeProfile || report.Status != "BLOCKED" || report.Stages[2].Disposition != "observed" {
		t.Fatalf("closed CLI report %s", b)
	}
}

// PMR-V1-010: the descriptor anchor does not certify a replaced public path or
// altered earlier bytes, and failed exclusive saves cannot overwrite evidence.
func TestNativeRetentionRejectsChangedEvidence(t *testing.T) {
	for _, kind := range []string{"changed-file", "replaced-directory", "duplicate-name", "record-limit", "file-limit"} {
		t.Run(kind, func(t *testing.T) {
			parent, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			r, e := newNativeRetention(filepath.Join(parent, "retained"), nil)
			if e != nil {
				t.Fatal(e)
			}
			defer r.root.Close()
			ref, e := r.save("original", []byte("original"))
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "changed-file":
				if e = os.WriteFile(ref.Path, []byte("altered"), 0600); e != nil {
					t.Fatal(e)
				}
				if r.verify() == nil {
					t.Fatal("changed evidence accepted")
				}
			case "replaced-directory":
				if e = os.Rename(r.path, r.path+".old"); e != nil {
					t.Fatal(e)
				}
				if e = os.Mkdir(r.path, 0700); e != nil {
					t.Fatal(e)
				}
				if r.verify() == nil {
					t.Fatal("replacement accepted")
				}
				if _, e = r.save("later", []byte("x")); e == nil {
					t.Fatal("write after replacement accepted")
				}
			case "duplicate-name":
				if _, e = r.save("original", []byte("replacement")); e == nil {
					t.Fatal("overwritten")
				}
				if string(readNativeRef(t, ref)) != "original" {
					t.Fatal("original changed")
				}
			case "record-limit":
				r.records = 32
				if _, e = r.save("overflow", nil); e == nil {
					t.Fatal("record overflow accepted")
				}
			case "file-limit":
				if _, e = r.save("oversized", []byte(strings.Repeat("x", MaxBytes+1))); e == nil {
					t.Fatal("file overflow accepted")
				}
			}
		})
	}
}

// PMR-V1-001/010: rejected identity fields remain private input bytes; a fixed
// refusal report cannot echo arbitrary unadmitted provenance or exceed its bound.
func TestNativeAdmissionDoesNotEchoInvalidIdentity(t *testing.T) {
	marker := "PRIVATE_UNADMITTED_IDENTITY " + strings.Repeat("x", 256<<10)
	for _, field := range []string{"forge", "repository", "change", "base", "merge", "source-commit", "source-tree", "executable-hash"} {
		t.Run(field, func(t *testing.T) {
			n := nativeSetup(t)
			binding := &n.f.Connector.Forge.Change.Binding
			switch field {
			case "forge":
				binding.Forge = marker
			case "repository":
				binding.Repository = marker
			case "change":
				binding.Change = marker
			case "base":
				binding.Base = marker
			case "merge":
				binding.Merge = marker
			case "source-commit":
				n.m.Implementation.SourceCommit = marker
			case "source-tree":
				n.m.Implementation.SourceTree = marker
			case "executable-hash":
				n.m.Implementation.ExecutableSHA256 = marker
			}
			n.p.Connector.Expected = *binding
			n.write(t)
			r, e := ReplayNative(context.Background(), n.root, n.fixture, n.policy, binding.Change)
			if e == nil || r.Status != "BLOCKED" {
				t.Fatalf("invalid identity admitted %+v %v", r, e)
			}
			b, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			if strings.Contains(string(b), "PRIVATE_UNADMITTED_IDENTITY") || len(b) > 32<<10 {
				t.Fatalf("unadmitted identity echoed in %d-byte refusal report", len(b))
			}
			for _, s := range r.Stages {
				if s.Disposition == "observed" {
					t.Fatal("malformed identity yielded native observation")
				}
			}
		})
	}
}

// PMR-V1-001/010: actual directory identities, including alternate case on
// insensitive filesystems, exclude output and manifest before observations.
func TestNativeFilesystemAliasAdmission(t *testing.T) {
	for _, destination := range []string{"output", "manifest"} {
		t.Run(destination, func(t *testing.T) {
			n := nativeSetup(t)
			product := filepath.Join(filepath.Dir(n.root), "Product")
			if e := os.Rename(n.root, product); e != nil {
				t.Fatal(e)
			}
			n.root = product
			t.Cleanup(func() { os.RemoveAll(product) })
			alias := filepath.Join(filepath.Dir(product), "PRODUCT")
			a, e := os.Stat(alias)
			if os.IsNotExist(e) {
				t.Skip("case-sensitive filesystem; distinct-directory control covers this platform")
			}
			if e != nil {
				t.Fatal(e)
			}
			original, e := os.Stat(product)
			if e != nil || !os.SameFile(a, original) {
				t.Fatal("alias identity unavailable")
			}
			if destination == "output" {
				n.m.RetainedOutputRoot = filepath.Join(alias, "retained")
			} else {
				n.p.Manifest = filepath.Join(alias, "manifest.json")
			}
			n.write(t)
			r, e := ReplayNative(context.Background(), n.root, n.fixture, n.policy, n.f.Connector.Forge.Change.Binding.Change)
			want := "runtime-output-invalid"
			if destination == "manifest" {
				want = "runtime-manifest-invalid"
			}
			if e == nil || len(r.Reasons) != 1 || r.Reasons[0] != want {
				t.Fatalf("alias admitted: reasons=%v error=%v", r.Reasons, e)
			}
			for _, stage := range r.Stages {
				if stage.Disposition == "observed" {
					t.Fatal("protected alias yielded native observation")
				}
			}
			if _, e := os.Stat(n.m.RetainedOutputRoot); !os.IsNotExist(e) {
				t.Fatalf("refusal created output directory: %v", e)
			}
			if destination == "manifest" && r.ManifestSHA256 != "" {
				t.Fatal("protected manifest read before destination admission")
			}
		})
	}
	for _, spelling := range []string{"Protected", "PROTECTED"} {
		t.Run("directory-identity-"+spelling, func(t *testing.T) {
			parent, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			protected := filepath.Join(parent, "Protected")
			if e := os.Mkdir(protected, 0700); e != nil {
				t.Fatal(e)
			}
			destination := filepath.Join(parent, spelling)
			if _, e := os.Stat(destination); os.IsNotExist(e) {
				if e := os.Mkdir(destination, 0700); e != nil {
					t.Fatal(e)
				}
			} else if e != nil {
				t.Fatal(e)
			}
			p, e := os.Stat(protected)
			if e != nil {
				t.Fatal(e)
			}
			d, e := os.Stat(destination)
			if e != nil {
				t.Fatal(e)
			}
			output := filepath.Join(destination, "retained")
			r, e := newNativeRetention(output, []string{protected})
			if os.SameFile(p, d) {
				if e == nil {
					r.root.Close()
					t.Fatal("same protected identity admitted")
				}
				if _, e := os.Stat(output); !os.IsNotExist(e) {
					t.Fatalf("protected identity wrote before refusal: %v", e)
				}
			} else {
				if e != nil {
					t.Fatalf("distinct case-sensitive directory refused: %v", e)
				}
				defer r.root.Close()
				if _, e := r.save("control", []byte("distinct")); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}

// PMR-V1-001: every required nested policy/binding and generated label member
// must exist with a non-null value, before any native stage can be observed.
func TestNativeRequiredNestedAdmission(t *testing.T) {
	type missingField struct {
		input string
		keys  []string
	}
	var cases []missingField
	for _, key := range []string{"profile", "expected", "hierarchy_level", "allowed_classes", "max_findings", "url_origins"} {
		cases = append(cases, missingField{"policy", []string{"connector", key}})
	}
	for _, key := range []string{"forge", "repository", "change", "base", "merge"} {
		cases = append(cases, missingField{"policy", []string{"connector", "expected", key}})
	}
	for _, field := range []string{"affected_flows", "followup", "test_gaps", "defects"} {
		for _, key := range []string{"basis", "evidence_sha256", "human", "approval"} {
			cases = append(cases, missingField{"fixture", []string{"expected", "labels", field, key}})
		}
	}
	for _, tc := range cases {
		for _, mutation := range []string{"missing", "null"} {
			t.Run(tc.input+"/"+strings.Join(tc.keys, "/")+"/"+mutation, func(t *testing.T) {
				n := nativeSetup(t)
				name := n.fixture
				if tc.input == "policy" {
					name = n.policy
				}
				b, e := os.ReadFile(name)
				if e != nil {
					t.Fatal(e)
				}
				var document map[string]any
				if e := json.Unmarshal(b, &document); e != nil {
					t.Fatal(e)
				}
				object := document
				for _, key := range tc.keys[:len(tc.keys)-1] {
					object = object[key].(map[string]any)
				}
				key := tc.keys[len(tc.keys)-1]
				if mutation == "missing" {
					delete(object, key)
				} else {
					object[key] = nil
				}
				writeJSON(t, name, document)
				if tc.input == "fixture" {
					b, e := os.ReadFile(name)
					if e != nil {
						t.Fatal(e)
					}
					n.m.FixtureSHA256 = SHA256(b)
					writeJSON(t, n.p.Manifest, n.m)
					b, e = os.ReadFile(n.p.Manifest)
					if e != nil {
						t.Fatal(e)
					}
					n.p.ManifestSHA256 = SHA256(b)
					writeJSON(t, n.policy, n.p)
				}
				r, e := ReplayNative(context.Background(), n.root, n.fixture, n.policy, n.f.Connector.Forge.Change.Binding.Change)
				if e == nil || r.Status != "BLOCKED" {
					t.Fatal("missing/null required member admitted")
				}
				for _, stage := range r.Stages {
					if stage.Disposition == "observed" {
						t.Fatal("missing/null required member yielded native observation")
					}
				}
				if _, e := os.Stat(n.m.RetainedOutputRoot); !os.IsNotExist(e) {
					t.Fatalf("nested admission created output: %v", e)
				}
			})
		}
	}
}
