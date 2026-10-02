package corpusrepublish

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/corpusindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/postmergeconnector"
)

// Build has no filesystem output or network effect. expectedPolicy is supplied
// by a trusted host invocation, independently of the author-authored request.
func Build(ctx context.Context, root string, requestRaw, policyRaw []byte, expectedPolicy string, previousResult, previousArtifact, previousSidecar []byte) (Output, error) {
	out := Output{}
	if !wire.IsSha256(expectedPolicy) || doccorpus.Digest(policyRaw) != expectedPolicy {
		return out, fmt.Errorf("republish trusted policy digest mismatch")
	}
	var req Request
	var policy Policy
	if err := Decode(requestRaw, &req); err != nil {
		return out, err
	}
	if err := Decode(policyRaw, &policy); err != nil {
		return out, err
	}
	if err := admit(ctx, root, req, policy); err != nil {
		return out, err
	}
	var previous *doccorpus.Artifact
	var prior Result
	if policy.PreviousResultSHA256 == "NONE" && policy.PreviousArtifactSHA256 == "NONE" {
		if len(previousResult) > 0 || len(previousArtifact) > 0 || len(previousSidecar) > 0 || policy.PreviousSidecarProfile != "NONE" || policy.PreviousSidecarSHA256 != "NONE" || policy.ExpectedPendingGeneration != 0 || policy.ExpectedPendingSHA256 != "NONE" {
			return out, fmt.Errorf("republish first publication baseline invalid")
		}
	} else {
		if !wire.IsSha256(policy.PreviousResultSHA256) || !wire.IsSha256(policy.PreviousArtifactSHA256) || doccorpus.Digest(previousResult) != policy.PreviousResultSHA256 || doccorpus.Digest(previousArtifact) != policy.PreviousArtifactSHA256 {
			return out, fmt.Errorf("republish previous bytes missing or mismatched")
		}
		if err := Decode(previousResult, &prior); err != nil {
			return out, err
		}
		canonical, e := Encode(prior)
		if e != nil || !bytes.Equal(canonical, previousResult) || prior.Profile != ResultProfile || prior.CorpusSHA256 != policy.PreviousArtifactSHA256 || prior.Provenance.Binding.Repository != req.Binding.Repository || prior.Provenance.Binding.Target != req.Binding.Target {
			return out, fmt.Errorf("republish previous result invalid")
		}
		previous, e = doccorpus.Open(ctx, root, previousArtifact)
		if e != nil {
			return out, e
		}
		pm, _ := doccorpus.Encode(previous.Manifest)
		if prior.Provenance.Binding.SourceRevision != previous.Manifest.Repository.Revision || prior.Provenance.Binding.ManifestSHA256 != doccorpus.Digest(pm) || !same(prior.Provenance.Inputs, sortedInputs(previous.Manifest.Inputs)) || !same(prior.Provenance.Providers, sortedProviders(previous.Manifest.Providers)) || prior.Provenance.Builder != previous.Builder {
			return out, fmt.Errorf("republish previous provenance mismatch")
		}
		index, e := corpusindex.Build(ctx, root, previousArtifact)
		if e != nil || doccorpus.Digest(index) != prior.IndexSHA256 {
			return out, fmt.Errorf("republish previous indexed provenance mismatch")
		}
	}
	eligibility, err := doccorpus.EligibilityDigest(req.Binding.ReuseEligibility)
	if err != nil {
		return out, err
	}
	expectedCache := ""
	if len(req.Binding.ReuseEligibility) > 0 && previous != nil {
		if policy.PreviousSidecarProfile != doccorpus.IncrementalSchema || prior.IncrementalSidecar.Profile != policy.PreviousSidecarProfile || prior.IncrementalSidecar.SHA256 != policy.PreviousSidecarSHA256 || !wire.IsSha256(policy.PreviousSidecarSHA256) || doccorpus.Digest(previousSidecar) != policy.PreviousSidecarSHA256 || prior.ReuseEligibilitySHA256 != eligibility {
			return out, fmt.Errorf("republish prior sidecar profile digest or eligibility mismatch")
		}
		expectedCache = policy.PreviousSidecarSHA256
	} else if len(previousSidecar) > 0 || policy.PreviousSidecarProfile != "NONE" || policy.PreviousSidecarSHA256 != "NONE" {
		return out, fmt.Errorf("republish cold build must not consume cache")
	}
	var a *doccorpus.Artifact
	if len(req.Binding.ReuseEligibility) > 0 {
		var cache *doccorpus.IncrementalCache
		a, cache, out.Stats, err = doccorpus.BuildIncremental(ctx, root, req.Manifest, req.Binding.ReuseEligibility, previousSidecar, expectedCache)
		if err == nil {
			out.Sidecar, err = doccorpus.EncodeIncrementalCache(*cache)
		}
	} else {
		a, err = doccorpus.Build(ctx, root, req.Manifest)
	}
	if err != nil {
		return out, err
	}
	out.Corpus, err = doccorpus.Encode(a)
	if err != nil {
		return out, err
	}
	parity, err := compare(previous, a, policy.AllowedShardRetirements)
	if err != nil {
		return out, err
	}
	// This existing producer independently rederives the full corpus. It remains
	// the correctness oracle, even when import contribution work was reused.
	out.Index, err = corpusindex.Build(ctx, root, out.Corpus)
	if err != nil {
		return out, err
	}
	reader, err := corpusindex.Open(ctx, out.Index, doccorpus.Digest(out.Index))
	if err != nil {
		return out, err
	}
	consumer, err := reader.Query(ctx, doccorpus.Request{Operation: "inventory", Limit: 1})
	if err != nil {
		return out, err
	}
	pin := SidecarPin{"NONE", "NONE"}
	if len(out.Sidecar) > 0 {
		pin = SidecarPin{doccorpus.IncrementalSchema, doccorpus.Digest(out.Sidecar)}
	}
	provenance := Provenance{Binding: req.Binding, PolicySHA256: expectedPolicy, DecisionRecord: policy.DecisionRecord, Builder: a.Builder, CompanionRevision: a.Builder.Revision, GoVersion: runtime.Version(), Schemas: []string{Profile, ResultProfile, a.Schema, req.Manifest.Schema, corpusindex.Schema}, Providers: sortedProviders(req.Manifest.Providers), Inputs: sortedInputs(req.Manifest.Inputs), Limits: []string{"trusted-local host-policy provenance; forge approval authenticity NOT_OBSERVED", "host environment and credential isolation NOT_OBSERVED", "indexed consumer retains historical source-validation and producer-authentication limits", "incremental counters measure shard import only; full producer validation still rederives corpus"}}
	if strings.Contains(a.Builder.Revision, "dirty") || a.Builder.Revision == "unrecorded" {
		provenance.Limits = append(provenance.Limits, "builder is dirty or unrecorded; immutable release provenance NOT_QUALIFIED")
	}
	out.ConsumerBytes, err = doccorpus.Encode(consumer)
	if err != nil {
		return out, err
	}
	out.Result = Result{ResultProfile, provenance, doccorpus.Digest(out.Corpus), doccorpus.Digest(out.Index), parity, eligibility, pin, ConsumerWitness{consumer.Operation, doccorpus.Digest(out.ConsumerBytes), consumer.Envelope}}
	out.ResultBytes, err = Encode(out.Result)
	if err != nil {
		return out, err
	}
	added, retired, changed := parityCounts(parity)
	out.Plan, err = postmergeconnector.BuildRepublishPlan(postmergeconnector.RepublishInput{Repository: req.Binding.Repository, Target: req.Binding.Target, DocumentationRevision: req.Binding.DocumentationRevision, SourceRevision: req.Binding.SourceRevision, PolicySHA256: expectedPolicy, ResultSHA256: doccorpus.Digest(out.ResultBytes), CorpusSHA256: out.Result.CorpusSHA256, IndexSHA256: out.Result.IndexSHA256, Added: added, Retired: retired, Changed: changed, EvidenceURL: req.EvidenceURL, URLOrigins: policy.URLOrigins})
	return out, err
}
func admit(ctx context.Context, root string, req Request, p Policy) error {
	if req.Profile != Profile || p.Profile != Profile || p.Decision != "APPROVED" || len(p.DecisionRecord) == 0 || len(p.DecisionRecord) > 128 || strings.ContainsAny(p.DecisionRecord, "\x00\r\n\t") || !same(req.Binding, p.Binding) {
		return fmt.Errorf("republish approval absent or binding mismatch")
	}
	b := req.Binding
	if !wire.IsGitOid(b.Repository) || !wire.IsGitOid(b.DocumentationRevision) || !wire.IsGitOid(b.SourceRevision) || !wire.IsSha256(b.ManifestSHA256) || req.Manifest.Schema != doccorpus.ManifestSchemaV2 || req.Manifest.Repository != (doccorpus.Repository{ID: b.Repository, Revision: b.SourceRevision}) || req.Manifest.BuiltAt != b.BuiltAt {
		return fmt.Errorf("republish immutable manifest binding invalid")
	}
	if _, err := time.Parse(time.RFC3339, b.BuiltAt); err != nil {
		return fmt.Errorf("republish explicit timestamp invalid")
	}
	if err := doccorpus.ValidateEligibility(b.ReuseEligibility); err != nil {
		return err
	}
	if err := doccorpus.ValidateEligibility(p.AllowedShardRetirements); err != nil {
		return err
	}
	raw, err := doccorpus.Encode(req.Manifest)
	if err != nil || doccorpus.Digest(raw) != b.ManifestSHA256 {
		return fmt.Errorf("republish manifest digest mismatch")
	}
	// Prove both events are actual commits, including an approval event with no
	// provider record. Full blob/content identity is then checked by doccorpus.
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return err
	}
	defer auth.BeginObjectSession()()
	ctx = gitstatus.WithIsolation(ctx)
	for _, rev := range []string{b.DocumentationRevision, b.SourceRevision} {
		actual, e := auth.Resolve(ctx, rev)
		if e != nil || actual != rev {
			return fmt.Errorf("republish approved commit unavailable")
		}
	}
	for _, in := range req.Manifest.Inputs {
		expected := b.SourceRevision
		if in.Purpose == "provider" {
			expected = b.DocumentationRevision
		}
		if in.Revision != expected {
			return fmt.Errorf("republish documentation or source input revision mismatch")
		}
		if in.Purpose == "source" && strings.HasSuffix(in.Path, ".md") {
			entry, ok, e := auth.LookupTreeEntry(ctx, b.DocumentationRevision, in.Path)
			if e != nil || !ok || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || entry.OID != in.Blob {
				return fmt.Errorf("republish document bytes not at approved revision")
			}
		}
	}
	for _, provider := range req.Manifest.Providers {
		expected := b.SourceRevision
		if provider.Kind == "records" {
			expected = b.DocumentationRevision
		}
		if provider.Revision != expected {
			return fmt.Errorf("republish provider revision mismatch")
		}
	}
	// Reject invalid renderer identities/URL policy before build or cache work.
	placeholder := strings.Repeat("0", 64)
	if _, err := postmergeconnector.BuildRepublishPlan(postmergeconnector.RepublishInput{Repository: b.Repository, Target: b.Target, DocumentationRevision: b.DocumentationRevision, SourceRevision: b.SourceRevision, PolicySHA256: placeholder, ResultSHA256: placeholder, CorpusSHA256: placeholder, IndexSHA256: placeholder, EvidenceURL: req.EvidenceURL, URLOrigins: p.URLOrigins}); err != nil {
		return err
	}
	if p.ExpectedPendingSHA256 != "NONE" && !wire.IsSha256(p.ExpectedPendingSHA256) {
		return fmt.Errorf("republish pending digest invalid")
	}
	if (p.ExpectedPendingGeneration == 0) != (p.ExpectedPendingSHA256 == "NONE") {
		return fmt.Errorf("republish pending generation digest mismatch")
	}
	return nil
}
func sortedInputs(in []doccorpus.Input) []doccorpus.Input {
	out := append([]doccorpus.Input{}, in...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Provider+"\x00"+out[i].Revision+"\x00"+out[i].Path+"\x00"+out[i].Purpose < out[j].Provider+"\x00"+out[j].Revision+"\x00"+out[j].Path+"\x00"+out[j].Purpose
	})
	return out
}
func sortedProviders(in []doccorpus.Provider) []doccorpus.Provider {
	out := append([]doccorpus.Provider{}, in...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
