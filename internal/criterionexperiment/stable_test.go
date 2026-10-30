//go:build darwin || linux

package criterionexperiment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

type stableExperiment struct {
	dir, base, legacyTarget string
	legacy                  []byte
	stable                  stablefixture.Stable
	request                 Request
	repo                    *gitauth.Repository
}

// newStableExperiment builds the historical-verify repository, cites its one
// hunk, converts the change to a verified cem/1.0 map committed at a new
// target, and returns a stable-profile request over that target.
func newStableExperiment(t *testing.T) stableExperiment {
	t.Helper()
	stablefixture.RequireStableHost(t)
	ctx := context.Background()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "init", "-q")
	_ = os.Mkdir(filepath.Join(dir, "sample"), 0700)
	sources := map[string]string{"sample/go.mod": "module example.test/sample\ngo 1.27.1\n", "sample/source.go": "package sample\nconst Value=0\n", "sample/oracle_test.go": "package sample\nimport \"testing\"\nfunc TestRepair(t *testing.T){if Value!=1{t.Fatal(\"EXPECTED\")}}\n", "contract.txt": "accepted criterion\n"}
	for p, s := range sources {
		if e := os.WriteFile(filepath.Join(dir, p), []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "base")
	base := gitTest(t, dir, "rev-parse", "HEAD")
	_ = os.WriteFile(filepath.Join(dir, "sample/source.go"), []byte("package sample\nconst Value=1\n"), 0600)
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "candidate")
	legacyTarget := gitTest(t, dir, "rev-parse", "HEAD")
	s, e := workflow.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Prepare(ctx, workflow.PrepareOptions{Base: base, Target: legacyTarget}); e != nil {
		t.Fatal(e)
	}
	if s, e = workflow.Open(dir); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Cite(ctx, workflow.CiteOptions{MapPath: cw.ExcludedCEMPath, Hunk: "1", EvidencePath: "contract.txt", Lines: "1:1", Relation: "specification"}); e != nil {
		t.Fatal(e)
	}
	legacy, e := os.ReadFile(filepath.Join(dir, cw.ExcludedCEMPath))
	if e != nil {
		t.Fatal(e)
	}
	// The producer map is the stable fixture's input; its hunk ID is read as
	// plain JSON. The retired map the refusal cases need is the same bytes
	// labelled cem/0.2: no cem/0.x producer remains (CEM-V1-216), and the
	// admission refuses on the spec alone (CEM-V1-210).
	var m struct {
		Hunks []struct {
			ID string `json:"id"`
		} `json:"hunks"`
	}
	if e = json.Unmarshal(legacy, &m); e != nil || len(m.Hunks) == 0 {
		t.Fatalf("legacy fixture hunks: %v", e)
	}
	stable := stablefixture.Stabilize(t, dir, legacy, m.Hunks[0].ID, func(args ...string) string { return gitTest(t, dir, args...) })
	retired := bytes.Replace(legacy, []byte(`"spec":"cem/1.0"`), []byte(`"spec":"cem/0.2"`), 1)
	if bytes.Equal(retired, legacy) {
		t.Fatal("producer map does not declare the compact cem/1.0 spec")
	}
	legacy = retired
	repo, e := gitauth.Open(dir, gitrun.NewDefaultBudget())
	if e != nil {
		t.Fatal(e)
	}
	r := goRequest(t)
	r.Schema = StableProfile
	r.Base, r.Target = base, stable.Target
	r.Criteria[0].Oracle.Commit = base
	r.Criteria[0].Authority.AnchorCommit = base
	r.Criteria[0].Authority.AnchorSha256 = Digest([]byte(sources["contract.txt"]))
	r.Criteria[0].Hunks = []string{m.Hunks[0].ID}
	r.Criteria[0].Controls = []string{base}
	return stableExperiment{dir: dir, base: base, legacyTarget: legacyTarget, legacy: legacy, stable: stable, request: r, repo: repo}
}

// TestCEMV1136StableExperimentRunsAndVerifies (CEM-V1-136, CEM-V1-138): a
// stable request admitted over a verified cem/1.0 map is planned by the
// planning core from verified native captures, then runs and verifies under
// the stable schema; the frozen phases refuse the same plan, and a missing
// artifact root or tampered artifact refuses verification.
func TestCEMV1136StableExperimentRunsAndVerifies(t *testing.T) {
	ctx := context.Background()
	f := newStableExperiment(t)
	tasks := testTasks(t)
	captures := fixtureCaptures(t, f.base, tasks)
	rec := captures.Verification.Binding
	r := f.request
	r.Ticket, r.Attempt = rec.TicketID.Raw, rec.AttemptID
	r.Criteria[0].AcceptanceSha256 = Digest([]byte(rec.AcceptanceCriteria[0]))
	binaryLink := filepath.Join(t.TempDir(), "go")
	if e := os.Symlink(r.GoBinary, binaryLink); e != nil {
		t.Fatal(e)
	}
	r.GoBinary = binaryLink
	cem := filepath.Join(t.TempDir(), "cem.json")
	if e := os.WriteFile(cem, f.stable.Map, 0o600); e != nil {
		t.Fatal(e)
	}
	r.CEM = cem
	if e := r.validate(); e != nil {
		t.Fatal(e)
	}
	if e := admitCEM(ctx, f.dir, r, f.stable.Map, f.stable.ArtifactRoot); e != nil {
		t.Fatal(e)
	}
	planDir := filepath.Join(t.TempDir(), "plan")
	p, e := planWithCaptures(ctx, f.repo, r, f.stable.Map, captures, planDir)
	if e != nil {
		t.Fatal(e)
	}
	marker, e := os.ReadFile(filepath.Join(planDir, "EXPERIMENT-DISPOSABLE"))
	if e != nil || p.Schema != StableProfile || p.CEMSha256 != stablefixture.Digest(f.stable.Map) || string(marker) != StableProfile+"\n" {
		t.Fatalf("stable plan %q %q %v", p.Schema, marker, e)
	}
	planPath := filepath.Join(planDir, "plan.json")
	if _, e = Run(ctx, f.dir, planPath, CanonicalDigest(p), filepath.Join(t.TempDir(), "frozen"), true, true, tasks); e == nil {
		t.Fatal("frozen run admitted a stable plan without an artifact root")
	}
	out := filepath.Join(t.TempDir(), "run")
	receipt, e := RunStable(ctx, f.dir, planPath, CanonicalDigest(p), out, f.stable.ArtifactRoot, true, true, tasks)
	if e != nil {
		t.Fatal(e)
	}
	if receipt.Schema != StableProfile {
		t.Fatalf("receipt schema %q", receipt.Schema)
	}
	receiptPath := filepath.Join(out, "receipt.json")
	summary, e := VerifyStable(ctx, f.dir, planPath, receiptPath, f.stable.ArtifactRoot, false, tasks)
	if e != nil || summary.Schema != StableProfile || summary.State != "VERIFIED_SATISFIED" || summary.CEMSha256 != stablefixture.Digest(f.stable.Map) {
		t.Fatalf("stable verify %+v %v", summary, e)
	}
	if _, e = Verify(ctx, f.dir, planPath, receiptPath, false, tasks); e == nil {
		t.Fatal("frozen verify admitted a stable plan")
	}
	var refusal *StableRefusal
	if _, e = VerifyStable(ctx, f.dir, planPath, receiptPath, "", false, tasks); !errors.As(e, &refusal) {
		t.Fatalf("missing artifact root: %v", e)
	}
	artifact := filepath.Join(f.stable.ArtifactRoot, filepath.FromSlash(f.stable.ArtifactPaths[0]))
	if e = os.WriteFile(artifact, []byte(`{"fixture":"tampered"}`+"\n"), 0o644); e != nil {
		t.Fatal(e)
	}
	if _, e = VerifyStable(ctx, f.dir, planPath, receiptPath, f.stable.ArtifactRoot, false, tasks); !errors.As(e, &refusal) {
		t.Fatalf("tampered artifact: %v", e)
	}
}

// TestCEMV1137StableExperimentAdmission (CEM-V1-137, CEM-V1-139): the stable
// admission accepts only the verified map at the request base and target and
// refuses a wrong base, edited bytes, a missing root, a tampered artifact and
// an unknown hunk; the two profiles never cross.
func TestCEMV1137StableExperimentAdmission(t *testing.T) {
	ctx := context.Background()
	f := newStableExperiment(t)
	if e := admitCEM(ctx, f.dir, f.request, f.stable.Map, f.stable.ArtifactRoot); e != nil {
		t.Fatalf("stable admission: %v", e)
	}
	edited := append(bytes.TrimSuffix(append([]byte(nil), f.stable.Map...), []byte("\n")), ' ', '\n')
	wrongBase := f.request
	wrongBase.Base = f.legacyTarget
	unknownHunk := f.request
	unknownHunk.Criteria = append([]Criterion(nil), f.request.Criteria...)
	unknownHunk.Criteria[0].Hunks = []string{"hunk:sha256:" + Digest([]byte("absent"))}
	frozen := f.request
	frozen.Schema = Profile
	legacyAtTarget := f.request
	legacyAtTarget.Target = f.legacyTarget
	cases := map[string]struct {
		request Request
		raw     []byte
		root    string
		refusal bool
	}{
		"wrong base":            {wrongBase, f.stable.Map, f.stable.ArtifactRoot, true},
		"edited map bytes":      {f.request, edited, f.stable.ArtifactRoot, true},
		"missing artifact root": {f.request, f.stable.Map, "", true},
		"unknown hunk":          {unknownHunk, f.stable.Map, f.stable.ArtifactRoot, false},
		"frozen over cem/1.0":   {frozen, f.stable.Map, "", false},
		"frozen with root":      {frozen, f.legacy, f.stable.ArtifactRoot, false},
		"stable over cem/0.2":   {legacyAtTarget, f.legacy, f.stable.ArtifactRoot, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := admitCEM(ctx, f.dir, tc.request, tc.raw, tc.root)
			var refusal *StableRefusal
			if e == nil || errors.As(e, &refusal) != tc.refusal {
				t.Fatalf("admission = %v, want refusal=%v", e, tc.refusal)
			}
			if tc.refusal && refusal.Result.Outcome == "ACCEPT" {
				t.Fatal("refusal retains an ACCEPT")
			}
		})
	}
	// CEM-V1-210: the stable profile refuses a retired map with the recovery
	// line, and the frozen profile admitted only cem/0.2, which is retired.
	if e := admitCEM(ctx, f.dir, legacyAtTarget, f.legacy, f.stable.ArtifactRoot); cemcode.CodeOf(e) != cemcode.UnsupportedSpec || !strings.Contains(e.Error(), cw.RetiredSpecFix) {
		t.Fatalf("stable over cem/0.2: %v", e)
	}
	if e := admitCEM(ctx, f.dir, frozen, f.legacy, ""); cemcode.CodeOf(e) != cemcode.UnsupportedSpec || !strings.Contains(e.Error(), cw.RetiredSpecFix) {
		t.Fatalf("frozen over cem/0.2: %v", e)
	}
	if e := admitCEM(ctx, f.dir, frozen, f.stable.Map, ""); cemcode.CodeOf(e) != cemcode.UnsupportedSpec || !strings.Contains(e.Error(), StableProfile) {
		t.Fatalf("frozen over cem/1.0: %v", e)
	}
	t.Run("plan entries", func(t *testing.T) {
		requestDir := t.TempDir()
		cemPath := filepath.Join(requestDir, "cem.json")
		if e := os.WriteFile(cemPath, f.stable.Map, 0o600); e != nil {
			t.Fatal(e)
		}
		stableRequest, frozenRequest := f.request, frozen
		stableRequest.CEM, frozenRequest.CEM = cemPath, cemPath
		var refusal *StableRefusal
		if _, e := PlanExperiment(ctx, f.dir, stableRequest, filepath.Join(t.TempDir(), "plan"), TasksVerifierConfig{}); !errors.As(e, &refusal) {
			t.Fatalf("stable request without an artifact root: %v", e)
		}
		if _, e := PlanStableExperiment(ctx, f.dir, frozenRequest, filepath.Join(t.TempDir(), "plan"), f.stable.ArtifactRoot, TasksVerifierConfig{}); e == nil || errors.As(e, &refusal) {
			t.Fatalf("frozen request with an artifact root: %v", e)
		}
	})
	t.Run("tampered artifact", func(t *testing.T) {
		artifact := filepath.Join(f.stable.ArtifactRoot, filepath.FromSlash(f.stable.ArtifactPaths[0]))
		if e := os.WriteFile(artifact, []byte(`{"fixture":"tampered"}`+"\n"), 0o644); e != nil {
			t.Fatal(e)
		}
		var refusal *StableRefusal
		if e := admitCEM(ctx, f.dir, f.request, f.stable.Map, f.stable.ArtifactRoot); !errors.As(e, &refusal) {
			t.Fatalf("tampered artifact: %v", e)
		}
	})
}
