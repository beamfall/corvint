package frontierrepo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/tcq"
)

// stableFixture is the frontierrepo fixture converted to cem/1.0: the same
// change, its exact stable sidecar committed at a new target, six artifacts
// under an explicit root, and the OCM rebound to the stable map and target.
type stableFixture struct {
	fixture
	stable stablefixture.Stable
	ocm    []byte
}

func newStableFixture(t *testing.T, linked bool) stableFixture {
	t.Helper()
	stablefixture.RequireStableHost(t)
	f := newFixture(t, linked)
	stable := stablefixture.Stabilize(t, f.root, f.cemRaw, f.hunkID, func(args ...string) string {
		return gitCmd(t, f.root, args...)
	})
	return stableFixture{fixture: f, stable: stable, ocm: stablefixture.RebindOCM(t, f.ocmRaw, stable.Target, digestHex(stable.Map))}
}

func (f stableFixture) request() StableRequest {
	return StableRequest{
		CEMBytes: f.stable.Map, OCMBytes: f.ocm, ExpectedBase: f.base, Target: f.stable.Target, ArtifactRoot: f.stable.ArtifactRoot,
	}
}

// TestCEMV1125StableUpstreamAccepts (CEM-V1-122, CEM-V1-125, CEM-V1-126): a
// verified cem/1.0 change yields a tcq-stable/0 result bound to the exact map
// and patch, keeps the caller-reported claim unmatched and produces no frontier.
func TestCEMV1125StableUpstreamAccepts(t *testing.T) {
	f := newStableFixture(t, true)
	upstream, err := RecomputeStable(context.Background(), f.root, f.request())
	if err != nil {
		t.Fatal(err)
	}
	if upstream.FrontierState != StableFrontierState || upstream.Universe.Spec != wire.StableSpec ||
		upstream.Universe.MapSHA256 != digestHex(f.stable.Map) || upstream.Universe.TargetRevision != f.stable.Target {
		t.Fatalf("upstream binding: %+v", upstream)
	}
	if !strings.HasPrefix(upstream.TCQ.ID(), "tcq-stable:sha256:") {
		t.Fatalf("stable TCQ identity %q", upstream.TCQ.ID())
	}
	document, err := wire.Parse(upstream.TCQ.Raw())
	if err != nil {
		t.Fatal(err)
	}
	if profile, _ := document.Obj.Get("profile"); profile.Str != tcq.StableProfile {
		t.Fatalf("profile = %q, want %s", profile.Str, tcq.StableProfile)
	}
	cem, _ := document.Obj.Get("cem")
	for key, want := range map[string]string{
		"spec": wire.StableSpec, "mapSha256": digestHex(f.stable.Map), "patchSha256": f.patchSHA256, "verification": "ACCEPT",
	} {
		if got, _ := cem.Obj.Get(key); got.Str != want {
			t.Fatalf("cem.%s = %q, want %q", key, got.Str, want)
		}
	}
	if bytes.Contains(upstream.TCQ.Raw(), []byte(`"tcq/0"`)) {
		t.Fatal("stable result relabels the frozen tcq/0 profile")
	}
	// A criterion link and its runner receipt never grant TCQ adequacy: with
	// no dynamic tuple the linked claim stays unmatched and carries no relation.
	if len(upstream.TCQClaims) != 1 || upstream.TCQClaims[0].Relation != "" || len(upstream.TCQClaims[0].Reasons) == 0 {
		t.Fatalf("stable TCQ claims = %+v", upstream.TCQClaims)
	}
	if len(upstream.Obligations) != 1 || upstream.Obligations[0].Disposition != "linked" {
		t.Fatalf("obligations = %+v", upstream.Obligations)
	}
}

// TestCEMV1126StableUpstreamKeepsUnknownObligation (CEM-V1-126): an unknown OCM
// obligation crosses the stable route with its reason; nothing closes it.
func TestCEMV1126StableUpstreamKeepsUnknownObligation(t *testing.T) {
	f := newStableFixture(t, false)
	upstream, err := RecomputeStable(context.Background(), f.root, f.request())
	if err != nil {
		t.Fatal(err)
	}
	if len(upstream.Obligations) != 1 || upstream.Obligations[0].Disposition != "unknown" || upstream.Obligations[0].Reason != "unassessed" {
		t.Fatalf("obligations = %+v", upstream.Obligations)
	}
	if len(upstream.TCQClaims) != 0 || upstream.FrontierState != StableFrontierState {
		t.Fatalf("claims=%+v state=%s", upstream.TCQClaims, upstream.FrontierState)
	}
}

// TestCEMV1127StableUpstreamRefusals (CEM-V1-127): a wrong base, edited map
// bytes, a tampered artifact and a missing artifact root each refuse with the
// stable verifier's retained result and yield no upstream.
func TestCEMV1127StableUpstreamRefusals(t *testing.T) {
	f := newStableFixture(t, true)
	edited := append(bytes.TrimSuffix(append([]byte(nil), f.stable.Map...), []byte("\n")), ' ', '\n')
	cases := map[string]func(*StableRequest){
		"wrong base":            func(r *StableRequest) { r.ExpectedBase = f.target },
		"edited map bytes":      func(r *StableRequest) { r.CEMBytes = edited },
		"missing artifact root": func(r *StableRequest) { r.ArtifactRoot = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			request := f.request()
			mutate(&request)
			upstream, err := RecomputeStable(context.Background(), f.root, request)
			requireStableUpstreamRefusal(t, upstream, err)
		})
	}
	t.Run("tampered artifact", func(t *testing.T) {
		path := filepath.Join(f.stable.ArtifactRoot, filepath.FromSlash(f.stable.ArtifactPaths[0]))
		if err := os.WriteFile(path, []byte(`{"fixture":"tampered"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		upstream, err := RecomputeStable(context.Background(), f.root, f.request())
		requireStableUpstreamRefusal(t, upstream, err)
	})
}

func requireStableUpstreamRefusal(t *testing.T, upstream *StableUpstream, err error) {
	t.Helper()
	var refusal *lrfrepo.StableRefusal
	if upstream != nil || !errors.As(err, &refusal) || refusal.Result.Outcome == "ACCEPT" {
		t.Fatalf("upstream=%v err=%v, want a retained stable refusal", upstream != nil, err)
	}
}

// TestCEMV1123StableTCQMemoNeverBypasses (CEM-V1-123): the stable upstream seam
// returns the accepted binding only for the exact verified bytes; other OCM
// bytes run a real stable verification, which refuses.
func TestCEMV1123StableTCQMemoNeverBypasses(t *testing.T) {
	f := newStableFixture(t, true)
	upstream, err := RecomputeStable(context.Background(), f.root, f.request())
	if err != nil {
		t.Fatal(err)
	}
	request := f.request()
	verifier := &stableVerifier{ctx: context.Background(), accepted: upstream.Universe,
		options:   lrfrepo.StableUniverseOptions{Root: f.root, ExpectedBase: f.base, Target: f.stable.Target, ArtifactRoot: f.stable.ArtifactRoot},
		cemSHA256: digestHex(request.CEMBytes), ocmSHA256: digestHex(request.OCMBytes)}
	binding, err := verifier.VerifyStableOCM(request.CEMBytes, request.OCMBytes, f.base, f.stable.Target)
	if err != nil || binding.MapSha256 != digestHex(f.stable.Map) || binding.PatchSha256 != f.patchSHA256 {
		t.Fatalf("memo hit: %+v %v", binding, err)
	}
	stale := stablefixture.RebindOCM(t, f.ocmRaw, f.stable.Target, digestHex(f.cemRaw))
	if _, err := verifier.VerifyStableOCM(request.CEMBytes, stale, f.base, f.stable.Target); err == nil {
		t.Fatal("an OCM bound to another map passed the memo")
	}
}

// TestCEMV1121LegacyRoutesRefuseStable (CEM-V1-121, CEM-V1-124, CEM-V1-217):
// tcq/0 and frontier/0 are deleted with the cem/0.x wire, so the stable
// routes are the only routes, and each refuses a retired cem/0.2 map before
// reading it: tcq-stable/0 with unsupported-cem-profile and the frontier
// stable view with unsupported-frontier-context.
func TestCEMV1121LegacyRoutesRefuseStable(t *testing.T) {
	f := newStableFixture(t, true)
	ctx := context.Background()
	retired := []byte(`{"spec":"cem/0.2"}` + "\n")
	repository, err := gitauth.Open(f.root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	reader := treeReader{adapter: New(ctx, f.root), repository: repository}
	verifier := &stableVerifier{ctx: ctx, options: lrfrepo.StableUniverseOptions{Root: f.root, ArtifactRoot: f.stable.ArtifactRoot}}
	_, err = tcq.EvaluateStable(reader, verifier, tcq.Request{CEM: retired, OCM: f.ocm, ExpectedBase: f.base, Target: f.stable.Target})
	if tcqCode(err) != tcq.CodeUnsupportedCEMProfile {
		t.Fatalf("tcq-stable/0 over cem/0.2: %v", err)
	}
	request := f.viewRequest()
	request.CEMBytes = retired
	if _, encoded, err := frontier.ComputeStable(ctx, New(ctx, f.root), request); frontier.CodeOf(err) != frontier.CodeUnsupportedContext || encoded != nil {
		t.Fatalf("frontier stable view over cem/0.2: %v", err)
	}
}

func tcqCode(err error) string {
	var typed *tcq.Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

func (f stableFixture) viewRequest() frontier.StableViewRequest {
	return frontier.StableViewRequest{
		CEMBytes: f.stable.Map, OCMBytes: f.ocm, ExpectedBase: f.base, Target: f.stable.Target, ArtifactRoot: f.stable.ArtifactRoot,
	}
}

// TestV1304StableViewAccepts: frontier.ComputeStable over a verified cem/1.0
// map yields the frontier-stable-view/0 bytes bound to the exact map, OCM and
// stable TCQ identity, with the unknown obligation kept and state NOT_PRODUCED.
func TestV1304StableViewAccepts(t *testing.T) {
	f := newStableFixture(t, false)
	ctx := context.Background()
	view, encoded, err := frontier.ComputeStable(ctx, New(ctx, f.root), f.viewRequest())
	if err != nil {
		t.Fatal(err)
	}
	if view.FrontierState != "NOT_PRODUCED" || view.ExitCode() != 1 || view.MapSHA256 != digestHex(f.stable.Map) ||
		view.OCMSHA256 != digestHex(f.ocm) || view.TargetRevision != f.stable.Target || view.PatchSHA256 != f.patchSHA256 {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Obligations) != 1 || view.Obligations[0].Disposition != "unknown" || view.Obligations[0].Reason != "unassessed" {
		t.Fatalf("obligations = %+v", view.Obligations)
	}
	document, err := wire.Parse(encoded)
	if err != nil || !bytes.HasSuffix(encoded, []byte("}\n")) {
		t.Fatalf("encoded view: %v %q", err, encoded)
	}
	for key, want := range map[string]string{"profile": "frontier-stable-view/0", "frontierState": "NOT_PRODUCED"} {
		if got, _ := document.Obj.Get(key); got.Str != want {
			t.Fatalf("%s = %q, want %q", key, got.Str, want)
		}
	}
	if bytes.Contains(encoded, []byte(`"frontier/0"`)) || bytes.Contains(encoded, []byte(`"EMPTY"`)) {
		t.Fatal("stable view relabels a frontier/0 document or claims EMPTY")
	}
}

// TestV1304StableViewRefusals: a tampered map, a wrong base, a tampered
// artifact and a missing artifact root each refuse with a closed code and no
// view; a cem/0.2 map refuses the stable route.
func TestV1304StableViewRefusals(t *testing.T) {
	f := newStableFixture(t, true)
	edited := append(bytes.TrimSuffix(append([]byte(nil), f.stable.Map...), []byte("\n")), ' ', '\n')
	cases := map[string]struct {
		mutate func(*frontier.StableViewRequest)
		code   string
	}{
		"tampered map":          {func(r *frontier.StableViewRequest) { r.CEMBytes = edited }, "excluded-artifact-mismatch"},
		"wrong base":            {func(r *frontier.StableViewRequest) { r.ExpectedBase = f.target }, "base-revision-mismatch"},
		"missing artifact root": {func(r *frontier.StableViewRequest) { r.ArtifactRoot = "" }, frontier.CodeInvalidInput},
		"cem/0.2 map":           {func(r *frontier.StableViewRequest) { r.CEMBytes = []byte(`{"spec":"cem/0.2"}` + "\n") }, frontier.CodeUnsupportedContext},
	}
	ctx := context.Background()
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			request := f.viewRequest()
			tc.mutate(&request)
			view, encoded, err := frontier.ComputeStable(ctx, New(ctx, f.root), request)
			if encoded != nil || view.FrontierState != "" || frontier.CodeOf(err) != tc.code {
				t.Fatalf("code = %q, want %q (view=%+v)", frontier.CodeOf(err), tc.code, view)
			}
		})
	}
	t.Run("tampered artifact", func(t *testing.T) {
		path := filepath.Join(f.stable.ArtifactRoot, filepath.FromSlash(f.stable.ArtifactPaths[0]))
		if err := os.WriteFile(path, []byte(`{"fixture":"tampered"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		view, encoded, err := frontier.ComputeStable(ctx, New(ctx, f.root), f.viewRequest())
		if encoded != nil || view.FrontierState != "" || frontier.CodeOf(err) != "artifact-digest-mismatch" {
			t.Fatalf("code = %q", frontier.CodeOf(err))
		}
	})
}
