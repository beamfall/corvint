package analyzercap

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

type acceptingVerifier struct{}

func (acceptingVerifier) VerifySnapshot(snapshot RegistrySnapshot) error {
	snapshot.Releases[0].ReleaseID = "forged"
	return nil
}
func (acceptingVerifier) CloneSnapshotVerifier() SnapshotVerifier { return acceptingVerifier{} }
func (acceptingVerifier) coreBoundedSnapshotVerifier()            {}

type rejectingVerifier struct{}

func (rejectingVerifier) VerifySnapshot(RegistrySnapshot) error {
	return errors.New("bad fixture signature")
}

var testHost = actualHostPlatform()
var testTarget = Platform{OS: "linux", Architecture: "amd64", ABI: "gnu"}

func testProfile(t *testing.T, scope Opaque, features []Feature, edges []DependencyEdge) Profile {
	t.Helper()
	profile, err := NewProfile([]Component{{
		ScopeID: scope, Role: "compiler", EcosystemCoordinate: "example.module",
		StableInstanceID: "install1", Scheme: ExactScheme, CanonicalExactValue: "go1.27.0", SourceEvidenceDigest: "source1",
	}}, testHost, features, edges)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}
func testClause(profile Profile, id Opaque) Clause {
	return Clause{ID: id, Host: testHost, Target: testTarget, Profile: profile.Identity(), ReleaseID: "release1", Protocol: "protocol1"}
}
func testRelease(t *testing.T, profile Profile, plugin Opaque, clauses ...Clause) Release {
	t.Helper()
	release := Release{PluginID: plugin, CapabilityID: "capability1", ReleaseID: "release1", BuildID: "build1", ManifestDigest: "manifest1", ArtifactDigest: "artifact1", HostBinaryDigest: "hostbin1", Host: testHost, Protocol: "protocol1", ProjectionDigest: "projection1", ProjectionScheme: "projection-scheme1", ResolverDigest: "resolver1", ClosedInputFamilyDigest: "families1", Clauses: clauses}
	return release
}
func testLock(profile Profile, release Release) *RepositoryLock {
	return &RepositoryLock{CapabilityID: release.CapabilityID, PluginID: release.PluginID, ReleaseID: release.ReleaseID, BuildID: release.BuildID, ManifestDigest: release.ManifestDigest, Protocol: release.Protocol, ArtifactDigest: release.ArtifactDigest, HostBinaryDigest: release.HostBinaryDigest, ProjectionDigest: release.ProjectionDigest, ProjectionScheme: release.ProjectionScheme, ResolverDigest: release.ResolverDigest, Profile: profile.Identity()}
}
func testRequest(t *testing.T, profile Profile, releases ...Release) Request {
	t.Helper()
	inputs := []Opaque{"input1"}
	handles := []InputHandle{{Handle: "input1", Digest: inputs[0]}}
	return Request{Capability: "capability1", Lock: testLock(profile, releases[0]), Snapshot: RegistrySnapshot{Digest: "snapshot1", TrustEpoch: "epoch1", Releases: releases}, Verifier: acceptingVerifier{}, Profile: profile, Unit: CompilationUnit{ID: "unit1", Targets: []Platform{testTarget}}, Target: testTarget, Cache: CacheInputs{FullSnapshotDigest: "fullsnapshot1", ClosedInputFamilyDigest: releases[0].ClosedInputFamilyDigest, SelectedInputSetDigest: inputSetDigest(handles), CompilationInputDigests: inputs, InputHandles: handles, AdmissionVerifierDigest: "admission1", ExactVerifierDigest: "exact1"}}
}
func reason(t *testing.T, err error, want Reason) {
	t.Helper()
	var failure *Failure
	if !errors.As(err, &failure) || failure.Reason != want {
		t.Fatal(want)
	}
}
func TestProviders(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	one := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	t.Run("zero", func(t *testing.T) {
		request := testRequest(t, profile, one)
		request.Capability = "other"
		_, err := Resolve(request)
		reason(t, err, NoCapabilityProvider)
	})
	t.Run("one", func(t *testing.T) {
		resolution, err := Resolve(testRequest(t, profile, one))
		if err != nil {
			t.Fatal(err)
		}
		if resolution.PluginID != "pluginA" || resolution.Clause.ID != "clauseA" {
			t.Fatal("r")
		}
	})
	t.Run("multiple without lock", func(t *testing.T) {
		two := testRelease(t, profile, "pluginB", testClause(profile, "clauseA"))
		request := testRequest(t, profile, one, two)
		request.Lock = nil
		_, err := Resolve(request)
		reason(t, err, AmbiguousProvider)
	})
	t.Run("multiple locked exact provider", func(t *testing.T) {
		two := testRelease(t, profile, "pluginB", testClause(profile, "clauseA"))
		request := testRequest(t, profile, one, two)
		if _, err := Resolve(request); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("lock mismatch has no fallback", func(t *testing.T) {
		request := testRequest(t, profile, one)
		request.Lock.ReleaseID = "nearby-release"
		_, err := Resolve(request)
		reason(t, err, LockMismatch)
	})
	t.Run("side-by-side release lock selects only exact release", func(t *testing.T) {
		two := one
		two.ReleaseID, two.BuildID, two.ManifestDigest, two.ArtifactDigest, two.HostBinaryDigest = "release2", "build2", "manifest2", "artifact2", "hostbin2"
		two.Clauses = []Clause{testClause(profile, "clauseA")}
		two.Clauses[0].ReleaseID = two.ReleaseID
		request := testRequest(t, profile, one, two)
		request.Lock = testLock(profile, two)
		resolution, err := Resolve(request)
		if err != nil {
			t.Fatal(err)
		}
		if resolution.Release.ReleaseID != "release2" {
			t.Fatal("release")
		}
		request.Lock.ReleaseID = "release3"
		_, err = Resolve(request)
		reason(t, err, LockMismatch)
	})
}
func TestManifest(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	selected := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	oversized := selected
	oversized.ReleaseID = "release2"
	oversized.Clauses = make([]Clause, MaxClauses+1)
	request := testRequest(t, profile, selected, oversized)
	if _, err := Resolve(request); err != nil {
		t.Fatal("release")
	}
	unrelated := oversized
	unrelated.CapabilityID = "othercap"
	request = testRequest(t, profile, unrelated)
	request.Capability = "capability1"
	_, err := Resolve(request)
	reason(t, err, NoCapabilityProvider)
	request = testRequest(t, profile, oversized)
	request.Lock = testLock(profile, oversized)
	_, err = Resolve(request)
	reason(t, err, CapabilityProjectionUnavailable)
}
func TestMalformedCandidate(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	selected := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	unselected := selected
	unselected.ReleaseID = "release2"
	unselected.BuildID = ""
	request := testRequest(t, profile, selected, unselected)
	_, err := Resolve(request)
	reason(t, err, RegistrySnapshotUnavailable)
}
func TestDuplicateCandidate(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	release := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	_, err := Resolve(testRequest(t, profile, release, release))
	reason(t, err, RegistrySnapshotUnavailable)
}
func TestCandidateBound(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	release := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	releases := make([]Release, MaxCandidateReleases+1)
	for index := range releases {
		suffix := string(rune('!' + index))
		releases[index] = release
		releases[index].ReleaseID = Opaque("release" + suffix)
		releases[index].BuildID = Opaque("build" + suffix)
		releases[index].ManifestDigest = Opaque("manifest" + suffix)
		releases[index].ArtifactDigest = Opaque("artifact" + suffix)
		releases[index].HostBinaryDigest = Opaque("hostbin" + suffix)
		releases[index].Clauses[0].ReleaseID = releases[index].ReleaseID
	}
	_, err := Resolve(testRequest(t, profile, releases...))
	reason(t, err, RegistrySnapshotUnavailable)
}
func TestProfileBound(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	release := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	request := testRequest(t, profile, release)
	request.Profiles = make([]Profile, MaxProfiles+1)
	request.Profile.Edges = make([]DependencyEdge, MaxEdgesPerRequest+1)
	_, err := Resolve(request)
	reason(t, err, LimitExceeded)
}
func TestTrust(t *testing.T) {
	profileA := testProfile(t, "scopeA", nil, nil)
	profileB := testProfile(t, "scopeB", nil, nil)
	release := testRelease(t, profileA, "pluginA", testClause(profileA, "clauseA"))
	request := testRequest(t, profileA, release)
	request.Profile = profileB
	_, err := Resolve(request)
	reason(t, err, LockMismatch)
	request = testRequest(t, profileA, release)
	request.Verifier = rejectingVerifier{}
	_, err = Resolve(request)
	reason(t, err, RegistryUntrusted)
}

func TestHostABIAndResolutionSlicesAreClosed(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	release := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	otherHost := testHost
	otherHost.ABI = "other-abi"
	otherProfile, err := NewProfile(profile.Components, otherHost, profile.Features, profile.Edges)
	if err != nil {
		t.Fatal(err)
	}
	otherClause := testClause(otherProfile, "clauseA")
	otherClause.Host = otherHost
	otherRelease := testRelease(t, otherProfile, "pluginA", otherClause)
	request := testRequest(t, otherProfile, otherRelease)
	_, err = Resolve(request)
	reason(t, err, HostPlatformUnsupported)

	request = testRequest(t, profile, release)
	resolution, err := Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	request.Snapshot.Releases[0].Clauses[0].Components = append(request.Snapshot.Releases[0].Clauses[0].Components, ComponentRequirement{ScopeID: "forged"})
	request.Profile.Components[0].Role = "forged"
	request.Unit.Targets[0].ABI = "forged"
	request.Snapshot.Releases[0].Clauses[0].Dependencies = append(request.Snapshot.Releases[0].Clauses[0].Dependencies, DependencyEdge{DependentProfile: "forged"})
	if len(resolution.Release.Clauses[0].Components) != 0 || resolution.Profile.Components[0].Role == "forged" || resolution.Unit.Targets[0].ABI == "forged" || len(resolution.Clause.Dependencies) != 0 {
		t.Fatalf("resolution aliases request slices: %#v", resolution)
	}
}

func TestMissingProfileAndDependencyFieldsUseScopedReasons(t *testing.T) {
	component := Component{Role: "compiler", EcosystemCoordinate: "example.module", StableInstanceID: "install1", Scheme: ExactScheme, CanonicalExactValue: "go1.27.0", SourceEvidenceDigest: "source1"}
	_, err := NewProfile([]Component{component}, testHost, nil, nil)
	reason(t, err, ProfileUnavailable)
	component.ScopeID = "scopeA"
	_, err = NewProfile([]Component{component}, testHost, nil, []DependencyEdge{{DependentProfile: "profile1", RelationKind: "requires", RequiredState: "ready"}})
	reason(t, err, TransitiveProfileUnavailable)
}
func TestComponentAndRequirementSchemesUseClosedSchemeReasons(t *testing.T) {
	component := Component{ScopeID: "scopeA", Role: "compiler", EcosystemCoordinate: "example.module", StableInstanceID: "install1", CanonicalExactValue: "go1.27.0", SourceEvidenceDigest: "source1"}
	for _, test := range []struct {
		scheme Opaque
		want   Reason
	}{{"", SchemeMissing}, {"bad scheme", SchemeInvalid}} {
		component.Scheme = test.scheme
		_, err := NewProfile([]Component{component}, testHost, nil, nil)
		reason(t, err, test.want)
		profile := testProfile(t, "scopeA", nil, nil)
		clause := testClause(profile, "clauseA")
		clause.Components = []ComponentRequirement{{ScopeID: "scopeA", Role: "compiler", EcosystemCoordinate: "example.module", StableInstanceID: "install1", Scheme: test.scheme, CanonicalValue: "go1.27.0"}}
		_, err = Resolve(testRequest(t, profile, testRelease(t, profile, "pluginA", clause)))
		reason(t, err, test.want)
	}
}
func TestProfileRefusesRepeatedComponentIdentity(t *testing.T) {
	first := Component{ScopeID: "scopeA", Role: "compiler", EcosystemCoordinate: "example.module", StableInstanceID: "install1", Scheme: ExactScheme, CanonicalExactValue: "go1.26.0", SourceEvidenceDigest: "source1"}
	second := first
	second.CanonicalExactValue = "go1.27.0"
	_, err := NewProfile([]Component{first, second}, testHost, nil, nil)
	reason(t, err, ProfileUnavailable)
	second.StableInstanceID = "install2"
	if _, err := NewProfile([]Component{first, second}, testHost, nil, nil); err != nil {
		t.Fatal(err)
	}
}
func TestManifestRefusesRepeatedClauseID(t *testing.T) {
	profile := testProfile(t, "scopeA", []Feature{{Name: "featureA", State: FeatureEnabled}}, nil)
	clauseA := testClause(profile, "clauseA")
	repeated := testClause(profile, "clauseA")
	repeated.Features = []Feature{{Name: "featureA", State: FeatureEnabled}}
	_, err := Resolve(testRequest(t, profile, testRelease(t, profile, "pluginA", clauseA, repeated)))
	reason(t, err, CapabilityProjectionUnavailable)
}
func TestSchemes(t *testing.T) {
	valid, err := ParseSemverRange(">=1.2.3 <=2.0.0")
	if err != nil || !valid.Contains(Semver{Major: 1, Minor: 2, Patch: 3}) || !valid.Contains(Semver{Major: 2}) || valid.Contains(Semver{Major: 2, Minor: 0, Patch: 1}) {
		t.Fatal("range")
	}
	for _, raw := range []string{">=1.2.3 <2.0.0", ">=01.2.3 <=2.0.0", ">=1.2 <=2.0.0", ">=1.2.3-alpha <=2.0.0", ">=1.2.3+build <=2.0.0", ">=1.2.3  <=2.0.0", ">=2.0.0 <=1.2.3", ">=1.2.3 <=2.0.0 "} {
		if _, err := ParseSemverRange(raw); err == nil {
			t.Fatal("range")
		}
	}
	cases := []struct {
		scheme Opaque
		want   Reason
	}{
		{"", SchemeMissing}, {"unknown/1", SchemeUnknown}, {"semver-2.0.0-range/2", SchemeFutureVersion}, {"semver-2.0.0-range/0", SchemeInvalid}, {"legacy-equality/1", SchemeUnsupported},
	}
	for _, test := range cases {
		t.Run(string(test.scheme), func(t *testing.T) { reason(t, ValidateActualScheme(test.scheme, "value"), test.want) })
	}
	if err := ValidateActualScheme(ExactScheme, "exact-byte-value"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateActualScheme(SemverExactScheme, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	reason(t, ValidateActualScheme(SemverRangeScheme, ">=1.0.0 <=1.0.0"), SchemeUnsupported)
	if err := ValidateRequirementScheme(SemverRangeScheme, ">=1.0.0 <=1.0.0"); err != nil {
		t.Fatal(err)
	}
}
func TestRange(t *testing.T) {
	actual := Component{ScopeID: "scopeA", Role: "compiler", EcosystemCoordinate: "example.module", StableInstanceID: "install1", Scheme: SemverExactScheme, CanonicalExactValue: "1.2.3", SourceEvidenceDigest: "source1"}
	profile, err := NewProfile([]Component{actual}, testHost, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	clause := testClause(profile, "clauseA")
	clause.Components = []ComponentRequirement{{ScopeID: "scopeA", Role: "compiler", EcosystemCoordinate: "example.module", StableInstanceID: "install1", Scheme: SemverRangeScheme, CanonicalValue: ">=1.2.3 <=2.0.0"}}
	release := testRelease(t, profile, "pluginA", clause)
	if _, err := Resolve(testRequest(t, profile, release)); err != nil {
		t.Fatal(err)
	}
	actual.Scheme, actual.CanonicalExactValue = SemverRangeScheme, ">=1.2.3 <=2.0.0"
	_, err = NewProfile([]Component{actual}, testHost, nil, nil)
	reason(t, err, SchemeUnsupported)
	clause.Components[0].ScopeID = "scopeB"
	release = testRelease(t, profile, "pluginA", clause)
	_, err = Resolve(testRequest(t, profile, release))
	reason(t, err, NoCompatibleClause)
}
func TestDNF(t *testing.T) {
	profile := testProfile(t, "scopeA", []Feature{{Name: "featureA", State: FeatureEnabled}}, nil)
	clauseA := testClause(profile, "clauseA")
	clauseB := testClause(profile, "clauseB")
	release := testRelease(t, profile, "pluginA", clauseA, clauseB)
	request := testRequest(t, profile, release)
	_, err := Resolve(request)
	reason(t, err, AmbiguousClause)
	release = testRelease(t, profile, "pluginA", clauseA)
	request = testRequest(t, profile, release)
	request.Target = Platform{OS: "linux", Architecture: "arm64", ABI: "gnu"}
	request.Unit.Targets = []Platform{request.Target}
	_, err = Resolve(request)
	reason(t, err, NoCompatibleClause)
	unknown := testProfile(t, "scopeA", []Feature{{Name: "featureA", State: FeatureUnknown}}, nil)
	clause := testClause(unknown, "clauseA")
	clause.Features = []Feature{{Name: "featureA", State: FeatureEnabled}}
	release = testRelease(t, unknown, "pluginA", clause)
	_, err = Resolve(testRequest(t, unknown, release))
	reason(t, err, FailureFeatureUnknown)
	unknownClause := testClause(unknown, "a")
	unknownClause.Features = []Feature{{Name: "featureA", State: FeatureEnabled}}
	matchingClause := testClause(unknown, "z")
	release = testRelease(t, unknown, "pluginA", unknownClause, matchingClause)
	resolution, err := Resolve(testRequest(t, unknown, release))
	if err != nil || resolution.Clause.ID != "z" {
		t.Fatal("sibling")
	}
	missing := testProfile(t, "scopeA", nil, nil)
	missingClause := testClause(missing, "clauseA")
	missingClause.Features = []Feature{{Name: "featureA", State: FeatureEnabled}}
	release = testRelease(t, missing, "pluginA", missingClause)
	_, err = Resolve(testRequest(t, missing, release))
	reason(t, err, NoCompatibleClause)
}
func TestMalformed(t *testing.T) {
	base := testProfile(t, "scopeA", nil, nil)
	_, err := NewProfile(base.Components, base.Host, []Feature{{Name: "featureA", State: FeatureDisabled}, {Name: "featureA", State: FeatureEnabled}}, nil)
	reason(t, err, ProfileUnavailable)
	clause := testClause(base, "clauseA")
	clause.Features = []Feature{{Name: "featureA", State: FeatureDisabled}, {Name: "featureA", State: FeatureEnabled}}
	release := Release{PluginID: "pluginA", CapabilityID: "capability1", ReleaseID: "release1", BuildID: "build1", ManifestDigest: "manifest1", ArtifactDigest: "artifact1", HostBinaryDigest: "hostbin1", Host: testHost, Protocol: "protocol1", ProjectionDigest: "projection1", ProjectionScheme: "projection-scheme1", ResolverDigest: "resolver1", ClosedInputFamilyDigest: "families1", Clauses: []Clause{clause}}
	_, err = Resolve(testRequest(t, base, release))
	reason(t, err, CapabilityProjectionUnavailable)
	release = testRelease(t, base, "pluginA", testClause(base, "clauseA"))
	request := testRequest(t, base, release)
	request.Unit.Targets = []Platform{testTarget, {OS: "linux\n", Architecture: "arm64", ABI: "gnu"}}
	_, err = Resolve(request)
	reason(t, err, TargetPlatformUnsupported)
}
func TestCache(t *testing.T) {
	leaf := testProfile(t, "leaf", nil, nil)
	edge := DependencyEdge{DependentProfile: leaf.Identity(), RelationKind: "requires", Optional: false, RequiredState: "ready", EvidenceDigest: "edge1"}
	root := testProfile(t, "root", nil, []DependencyEdge{edge})
	clause := testClause(root, "clauseA")
	clause.Dependencies = []DependencyEdge{edge}
	release := testRelease(t, root, "pluginA", clause)
	request := testRequest(t, root, release)
	_, err := Resolve(request)
	reason(t, err, TransitiveProfileUnavailable)
	request.Profiles = []Profile{leaf}
	first, err := Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	request.Cache.FullSnapshotDigest = "fullsnapshot2"
	second, err := Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	if first.CacheIdentity != "" || second.CacheIdentity != "" {
		t.Fatal("ordinary resolution manufactured cache identity")
	}
	request.Snapshot.TrustEpoch = "epoch2"
	third, err := Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	if second.CandidateSetDigest == third.CandidateSetDigest {
		t.Fatal("epoch")
	}
	request.Cache.ClosedInputFamilyDigest = "open-inputs"
	_, err = Resolve(request)
	reason(t, err, CapabilityProjectionUnavailable)
	request.Cache.ClosedInputFamilyDigest = release.ClosedInputFamilyDigest
	request.Cache.SelectedInputSetDigest = "forged"
	_, err = Resolve(request)
	reason(t, err, InputEvidenceMismatch)
}
func TestOpaque(t *testing.T) {
	if canonical("a", "bc") == canonical("ab", "c") || canonical("same", "a", "bc") == canonical("same", "ab", "c") || canonical("one", "value") == canonical("two", "value") {
		t.Fatal("framing")
	}
	for _, raw := range []string{"unicode-\u00e9", "nul\x00value", "line\nbreak", string(make([]byte, MaxOpaqueBytes+1))} {
		if _, err := ParseOpaque(raw); err == nil {
			t.Fatal("opaque")
		}
	}
	max := strings.Repeat("a", MaxOpaqueBytes)
	if _, err := ParseOpaque(max); err != nil {
		t.Fatal("opaque")
	}
}
func TestTargetCache(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	targetB := Platform{OS: "linux", Architecture: "arm64", ABI: "gnu"}
	clauseA := testClause(profile, "clauseA")
	clauseB := testClause(profile, "clauseB")
	clauseB.Target = targetB
	release := testRelease(t, profile, "pluginA", clauseA, clauseB)
	request := testRequest(t, profile, release)
	request.Unit.Targets = []Platform{testTarget, targetB}
	first, err := Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	request.Target = targetB
	second, err := Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity == second.Identity {
		t.Fatal("target")
	}
	request.Unit.Targets = append(request.Unit.Targets, Platform{"darwin", "arm64", "native"})
	third, err := Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	if second.Identity == third.Identity {
		t.Fatal("set")
	}
}
func TestCandidateSet(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	one := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	first, err := Resolve(testRequest(t, profile, one))
	if err != nil {
		t.Fatal(err)
	}
	two := one
	two.ReleaseID, two.BuildID, two.ManifestDigest, two.ArtifactDigest, two.HostBinaryDigest = "release2", "build2", "manifest2", "artifact2", "hostbin2"
	two.Clauses = []Clause{testClause(profile, "clauseA")}
	two.Clauses[0].ReleaseID = two.ReleaseID
	request := testRequest(t, profile, one, two)
	second, err := Resolve(request)
	if err != nil {
		t.Fatal(err)
	}
	if first.CandidateSetDigest == second.CandidateSetDigest {
		t.Fatal("c")
	}
}
func TestCapability(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	one := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	two := one
	two.CapabilityID = "capability2"
	firstRequest := testRequest(t, profile, one, two)
	first, err := Resolve(firstRequest)
	if err != nil {
		t.Fatal(err)
	}
	secondRequest := testRequest(t, profile, one, two)
	secondRequest.Capability = two.CapabilityID
	secondRequest.Lock = testLock(profile, two)
	second, err := Resolve(secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	if first.CandidateSetDigest == second.CandidateSetDigest || first.Identity == second.Identity {
		t.Fatal("c")
	}
}
func TestEdgeBound(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	release := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	request := testRequest(t, profile, release)
	request.Target = Platform{OS: "linux\n", Architecture: "amd64", ABI: "gnu"}
	_, err := Resolve(request)
	reason(t, err, TargetPlatformUnsupported)
	edges := make([]DependencyEdge, MaxEdgesPerProfile)
	for index := range edges {
		suffix := string(rune('!' + index))
		edges[index] = DependencyEdge{DependentProfile: Identity("dependent" + suffix), RelationKind: "requires", RequiredState: "ready", EvidenceDigest: Opaque("evidence" + suffix)}
	}
	request = testRequest(t, profile, release)
	request.Profiles = make([]Profile, MaxProfiles)
	for index := range request.Profiles {
		request.Profiles[index] = testProfile(t, Opaque("scope"+string(rune('!'+index))), nil, edges)
	}
	_, err = Resolve(request)
	reason(t, err, TransitiveProfileUnavailable)
}

func TestDependencyResolutionMemoizesSharedTransitiveWork(t *testing.T) {
	leaf := testProfile(t, "leaf", nil, nil)
	middle := testProfile(t, "middle", nil, []DependencyEdge{{DependentProfile: leaf.Identity(), RelationKind: "edge-middle", RequiredState: "ready", EvidenceDigest: "e-middle"}})
	shared := testProfile(t, "shared", nil, []DependencyEdge{{DependentProfile: middle.Identity(), RelationKind: "edge-shared", RequiredState: "ready", EvidenceDigest: "e-shared"}})
	edges := make([]DependencyEdge, MaxEdgesPerProfile)
	for index := range edges {
		suffix := fmt.Sprintf("%02d", index)
		edges[index] = DependencyEdge{DependentProfile: shared.Identity(), RelationKind: Opaque("edge" + suffix), RequiredState: "ready", EvidenceDigest: Opaque("e" + suffix)}
	}
	root := testProfile(t, "root", nil, edges)
	if _, err := resolveDependencies(root, edges, []Profile{shared, middle, leaf}); err != nil {
		t.Fatalf("shared dependency DAG exceeded linear work bound: %v", err)
	}
}
func BenchmarkResolve(b *testing.B) {
	profile := testProfile(&testing.T{}, "scopeA", nil, nil)
	release := testRelease(&testing.T{}, profile, "pluginA", testClause(profile, "clauseA"))
	request := testRequest(&testing.T{}, profile, release)
	benchmarkResolve(b, request)
}

func benchmarkResolve(b *testing.B, request Request) {
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		resolution, err := Resolve(request)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(resolution)
	}
}

func TestResolveAllocationRatchet(t *testing.T) {
	profile := testProfile(t, "scopeA", nil, nil)
	release := testRelease(t, profile, "pluginA", testClause(profile, "clauseA"))
	request := testRequest(t, profile, release)
	allocations := testing.AllocsPerRun(100, func() {
		resolution, err := Resolve(request)
		if err != nil {
			t.Fatal(err)
		}
		runtime.KeepAlive(resolution)
	})
	if allocations > 66 {
		t.Fatalf("Resolve allocations = %.0f, want <= 66", allocations)
	}
	result := testing.Benchmark(func(b *testing.B) { benchmarkResolve(b, request) })
	if bytes := result.AllocedBytesPerOp(); bytes > 8700 {
		t.Fatalf("Resolve allocation bytes = %d, want <= 8700", bytes)
	}
	if allocations := result.AllocsPerOp(); allocations > 66 {
		t.Fatalf("Resolve allocations/op = %d, want <= 66", allocations)
	}
}

func TestDigestStreamsCanonicalPreimageWithoutChangingIdentity(t *testing.T) {
	parts := []string{"", "alpha", string(make([]byte, 1024)), "omega"}
	want := sha256.Sum256([]byte(canonical("identity-regression/v1", parts...)))
	if got := digest("identity-regression/v1", parts...); got != Identity(hex.EncodeToString(want[:])) {
		t.Fatalf("digest=%s want=%x", got, want)
	}
}
