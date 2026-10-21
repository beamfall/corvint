package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

const dashboardOCMPath = ".corvint/change.ocm.json"

// dashboardStableFixture is one real cem/1.0 change: prepared and cited by
// the producer workflow, paired with an OCM from PrepareOCM, stabilized with
// six artifacts and committed as the exact sidecar at its target.
type dashboardStableFixture struct {
	root, base, producerTarget string
	producer, producerOCM      []byte
	stable                     stablefixture.Stable
	ocm                        []byte
}

func newDashboardStableFixture(t *testing.T, witness bool) dashboardStableFixture {
	t.Helper()
	stablefixture.RequireStableHost(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		command.Dir = root
		command.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1", "HOME="+home, "XDG_CONFIG_HOME="+home,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, data string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	write("docs/specs/intent.md", "# Intent\n\n## Requirements\n\n- `DSH-TEST-001`: render the widget.\n- `DSH-TEST-002`: keep the gap visible.\n\n## Non-goals\n")
	write("src/app.txt", "alpha\n")
	git("add", ".")
	git("commit", "-qm", "base")
	f := dashboardStableFixture{root: root, base: git("rev-parse", "HEAD")}
	write("src/app.txt", "alpha\nwidget renderer\n")
	git("add", ".")
	git("commit", "-qm", "target")
	f.producerTarget = git("rev-parse", "HEAD")
	ctx := context.Background()
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(ctx, workflow.PrepareOptions{Base: f.base, Target: f.producerTarget}); err != nil {
		t.Fatal(err)
	}
	if session, err = workflow.Open(root); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Cite(ctx, workflow.CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/specs/intent.md", Lines: "5:5", Relation: "specification",
	}); err != nil {
		t.Fatal(err)
	}
	if f.producer, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))); err != nil {
		t.Fatal(err)
	}
	if _, err := lrfrepo.PrepareOCM(ctx, root, lrfrepo.PrepareOptions{
		MapPath: dashboardOCMPath, CEMPath: wire.ExcludedCEMPath, IntentPath: "docs/specs/intent.md",
		ExpectedBase: f.base, Target: f.producerTarget,
	}); err != nil {
		t.Fatal(err)
	}
	if f.producerOCM, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(dashboardOCMPath))); err != nil {
		t.Fatal(err)
	}
	document, err := wire.ParseStable(f.producer)
	if err != nil || len(document.Change.Hunks) != 1 {
		t.Fatalf("producer map: %v", err)
	}
	producer := f.producer
	if witness {
		// An uncovered coverage witness is a complete optional witness the
		// projection must carry byte for byte.
		document.Change.Hunks[0].Coverage = &wire.CoverageWitness{
			ProfileSha256: stablefixture.Digest([]byte("coverage-profile")), TestRun: "go test ./src/...",
			Mode: "set", State: wire.CoverageUncovered, Covered: []wire.Range{},
		}
		if producer, err = wire.EncodeStable(document); err != nil {
			t.Fatal(err)
		}
	}
	f.stable = stablefixture.Stabilize(t, root, producer, document.Change.Hunks[0].ID, git)
	f.ocm = stablefixture.RebindOCM(t, f.producerOCM, f.stable.Target, stablefixture.Digest(f.stable.Map))
	write(dashboardOCMPath, string(f.ocm))
	return f
}

func (f dashboardStableFixture) arguments() []string {
	return []string{"cem-stable", "--root", f.root, "--cem", wire.ExcludedCEMPath, "--ocm", dashboardOCMPath,
		"--expected-base", f.base, "--target", f.stable.Target, "--artifact-root", f.stable.ArtifactRoot}
}

func runCEMStable(t *testing.T, arguments []string) (int, []byte, []byte) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exit := run(arguments, &stdout, &stderr)
	return exit, stdout.Bytes(), stderr.Bytes()
}

func objectKeys(t *testing.T, raw []byte) []string {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compactJSON(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, raw); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// TestCEMV1290DashboardCEMStableReadsRealUniverse (CEM-V1-290..292): a real
// cem/1.0 map, with and without an optional witness, is read, verified with
// its OCM over the C05 universe and projected with the exact raw stable
// graph, the whole verifier result, every artifact check, explicit unknown
// authority axes, every obligation and NOT_OBSERVED completion.
func TestCEMV1290DashboardCEMStableReadsRealUniverse(t *testing.T) {
	for _, witness := range []bool{false, true} {
		f := newDashboardStableFixture(t, witness)
		exit, stdout, stderr := runCEMStable(t, f.arguments())
		if exit != 0 || len(stderr) != 0 {
			t.Fatalf("witness=%v exit=%d stderr=%s", witness, exit, stderr)
		}
		if !bytes.HasSuffix(stdout, []byte("}\n")) || bytes.Count(stdout, []byte("\n")) != 1 {
			t.Fatalf("stdout is not one LF-terminated document: %q", stdout)
		}
		var view struct {
			Profile               string                `json:"profile"`
			Input                 cemStableInput        `json:"input"`
			Spec                  string                `json:"spec"`
			MapSha256             string                `json:"mapSha256"`
			BaseRevision          string                `json:"baseRevision"`
			TargetRevision        string                `json:"targetRevision"`
			ObjectFormat          string                `json:"objectFormat"`
			PatchSha256           string                `json:"patchSha256"`
			OCMSha256             string                `json:"ocmSha256"`
			Intent                cemStableIntent       `json:"intent"`
			Obligations           []cemStableObligation `json:"obligations"`
			Verification          json.RawMessage       `json:"verification"`
			MapBase64             []byte                `json:"mapBase64"`
			TaskCompletion        string                `json:"taskCompletion"`
			CriterionSatisfaction string                `json:"criterionSatisfaction"`
		}
		decoder := json.NewDecoder(bytes.NewReader(stdout))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&view); err != nil {
			t.Fatal(err)
		}
		want := []string{"baseRevision", "criterionSatisfaction", "input", "intent", "mapBase64", "mapSha256", "objectFormat",
			"obligations", "ocmSha256", "patchSha256", "profile", "spec", "targetRevision", "taskCompletion", "verification"}
		if got := objectKeys(t, stdout); !reflect.DeepEqual(got, want) {
			t.Fatalf("view keys = %v", got)
		}
		mapDigest := stablefixture.Digest(f.stable.Map)
		if view.Profile != cemStableProfile || view.Spec != wire.StableSpec || view.MapSha256 != mapDigest ||
			view.BaseRevision != f.base || view.TargetRevision != f.stable.Target || view.ObjectFormat != "sha1" ||
			view.Input.CEMSha256 != mapDigest || view.Input.OCMSha256 != stablefixture.Digest(f.ocm) ||
			view.Input.CEMPath != wire.ExcludedCEMPath || view.Input.OCMPath != dashboardOCMPath || view.OCMSha256 == "" ||
			view.TaskCompletion != "NOT_OBSERVED" || view.CriterionSatisfaction != "NOT_OBSERVED" {
			t.Fatalf("binding = %+v", view)
		}
		// The raw stable graph is the exact authenticated map bytes, final
		// LF and witness presence or absence included, and their digest is
		// the emitted map digest.
		onDisk, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(wire.ExcludedCEMPath)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(view.MapBase64, f.stable.Map) || !bytes.Equal(view.MapBase64, onDisk) ||
			!bytes.HasSuffix(view.MapBase64, []byte("}\n")) || stablefixture.Digest(view.MapBase64) != view.MapSha256 {
			t.Fatal("projected map differs from the authenticated stable bytes")
		}
		if got := bytes.Contains(view.MapBase64, []byte(`"coverage":`)); got != witness {
			t.Fatalf("witness presence = %v, want %v", got, witness)
		}
		parsed, err := wire.ParseStable(view.MapBase64)
		if err != nil || len(parsed.Criteria) != 1 || len(parsed.Artifacts) != 6 || len(parsed.Receipts) != 1 || len(parsed.Links) != 1 ||
			(parsed.Change.Hunks[0].Coverage != nil) != witness {
			t.Fatalf("projected graph: err=%v", err)
		}
		// The verifier result is whole: every key, byte for byte equal to an
		// independent verifier run over the same inputs.
		direct, exitCode := verify.Stable(context.Background(), f.stable.Map, verify.StableOptions{
			Repository: f.root, ExpectedBase: f.base, Target: f.stable.Target, ArtifactRoot: f.stable.ArtifactRoot,
		})
		directRaw, err := json.Marshal(direct)
		if err != nil || exitCode != 0 {
			t.Fatalf("direct verifier: exit=%d err=%v", exitCode, err)
		}
		if !bytes.Equal(compactJSON(t, view.Verification), directRaw) {
			t.Fatalf("verification differs:\n%s\n%s", view.Verification, directRaw)
		}
		if got, wantKeys := objectKeys(t, view.Verification), objectKeys(t, directRaw); !reflect.DeepEqual(got, wantKeys) || len(got) != 23 {
			t.Fatalf("verification keys = %v", got)
		}
		if direct.Outcome != "ACCEPT" || len(direct.ArtifactChecks) != 6 || direct.Sidecar != "EXACT" {
			t.Fatalf("direct result = %+v", direct)
		}
		for _, axis := range []string{"nativeAuthority", "authentication", "runnerExecution", "executionAtCommit", "criterionAdequacy"} {
			if direct.Axes[axis] != "NOT_OBSERVED" {
				t.Fatalf("axis %s = %q, want explicit NOT_OBSERVED", axis, direct.Axes[axis])
			}
		}
		// Every intent obligation is kept, unknowns with their reason.
		if len(view.Obligations) != 2 || view.Intent.Path != "docs/specs/intent.md" {
			t.Fatalf("obligations=%+v intent=%+v", view.Obligations, view.Intent)
		}
		for _, row := range view.Obligations {
			if !strings.HasPrefix(row.ID, "DSH-TEST-00") || row.Disposition == "" || row.HunkIDs == nil || row.ClaimIDs == nil ||
				row.Disposition != "supported" && row.Reason == "" {
				t.Fatalf("obligation = %+v", row)
			}
		}
	}
}

type cemStableRefusalView struct {
	Profile            string          `json:"profile"`
	Code               string          `json:"code"`
	Input              cemStableInput  `json:"input"`
	VerifierStage      string          `json:"verifierStage"`
	VerificationReason string          `json:"verificationReason"`
	Verification       json.RawMessage `json:"verification"`
}

// TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings
// (CEM-V1-293): a stale base or target, an edited or 0.x map, a tampered or
// missing artifact and an OCM bound to another map all refuse with exit 2,
// no stdout, the input digests and, when the verifier ran, its whole result;
// the inputs stay byte-identical.
func TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings(t *testing.T) {
	f := newDashboardStableFixture(t, false)
	artifact := filepath.Join(f.stable.ArtifactRoot, filepath.FromSlash(f.stable.ArtifactPaths[0]))
	original, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	restore := func() {
		if err := os.WriteFile(artifact, original, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeInput := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(f.root, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeInput("producer.cem.json", f.producer)
	writeInput("legacy.cem.json", []byte(`{"spec":"cem/0.2","baseRevision":"`+f.base+`"}`+"\n"))
	writeInput("unbound.ocm.json", f.producerOCM)
	with := func(flag, value string) []string {
		arguments := f.arguments()
		for index := range arguments {
			if arguments[index] == flag {
				arguments[index+1] = value
			}
		}
		return arguments
	}
	cases := []struct {
		name      string
		arguments []string
		mutate    func()
		stage     string
	}{
		{"stale-base", with("--expected-base", f.producerTarget), nil, verifierStageRefused},
		// The verifier accepts at the other target; its sidecar is inexact.
		{"stale-target", with("--target", f.producerTarget), nil, verifierStageAccepted},
		{"edited-map", with("--cem", "producer.cem.json"), nil, verifierStageRefused},
		{"tampered-artifact", f.arguments(), func() { _ = os.WriteFile(artifact, []byte("{}\n"), 0o644) }, verifierStageRefused},
		{"missing-artifact", f.arguments(), func() { _ = os.Remove(artifact) }, verifierStageRefused},
		// The verifier accepts; the OCM/LRF admission refuses.
		{"unbound-ocm", with("--ocm", "unbound.ocm.json"), nil, verifierStageAccepted},
		{"legacy-map", with("--cem", "legacy.cem.json"), nil, verifierStageNotRun},
	}
	for _, test := range cases {
		inputs := map[string][]byte{}
		for _, name := range []string{wire.ExcludedCEMPath, dashboardOCMPath, "producer.cem.json", "unbound.ocm.json", "legacy.cem.json"} {
			data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			inputs[name] = data
		}
		if test.mutate != nil {
			test.mutate()
		}
		exit, stdout, stderr := runCEMStable(t, test.arguments)
		restore()
		if exit != 2 || len(stdout) != 0 {
			t.Fatalf("%s: exit=%d stdout=%s stderr=%s", test.name, exit, stdout, stderr)
		}
		var refusal cemStableRefusalView
		decoder := json.NewDecoder(bytes.NewReader(stderr))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&refusal); err != nil {
			t.Fatalf("%s: stderr=%s err=%v", test.name, stderr, err)
		}
		if refusal.Profile != cemStableRefusalProfile || refusal.Code == "" || refusal.Code == errorInternal ||
			refusal.Input.CEMSha256 == "" || refusal.Input.OCMSha256 == "" {
			t.Fatalf("%s: refusal = %+v", test.name, refusal)
		}
		// CEM-V1-349: each refusal names the verifier step reached and keeps
		// its result, or says why the verifier did not run.
		verified := len(refusal.Verification) > 0 && string(refusal.Verification) != "null"
		if refusal.VerifierStage != test.stage || verified != (test.stage != verifierStageNotRun) ||
			(refusal.VerificationReason != "") != (test.stage == verifierStageNotRun) {
			t.Fatalf("%s: stage %q reason %q verification retained = %v, want stage %q (%s)", test.name, refusal.VerifierStage, refusal.VerificationReason, verified, test.stage, stderr)
		}
		if verified {
			var result verify.StableResult
			if err := json.Unmarshal(refusal.Verification, &result); err != nil || (result.Outcome == "ACCEPT") != (test.stage == verifierStageAccepted) ||
				len(objectKeys(t, refusal.Verification)) != 23 {
				t.Fatalf("%s: retained result = %s", test.name, refusal.Verification)
			}
		}
		for name, data := range inputs {
			after, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(name)))
			if err != nil || !bytes.Equal(after, data) {
				t.Fatalf("%s: input %s changed", test.name, name)
			}
		}
		t.Logf("%s: %s stage=%s reason=%s", test.name, refusal.Code, refusal.VerifierStage, refusal.VerificationReason)
	}
	// Argument shape is checked before any path opens.
	for _, arguments := range [][]string{
		{"cem-stable"},
		with("--artifact-root", "relative"),
		with("--cem", "../outside.json"),
		append(f.arguments(), "--root", f.root),
	} {
		exit, stdout, stderr := runCEMStable(t, arguments)
		if exit != 2 || len(stdout) != 0 || string(stderr) != `{"code":"DASHBOARD_INVALID_ARGUMENT","profile":"corvint-dashboard-error/0"}`+"\n" {
			t.Fatalf("%v: exit=%d stdout=%s stderr=%s", arguments, exit, stdout, stderr)
		}
	}
}
