package analyzercap

import (
	"context"
	"github.com/Beamfall/corvint/internal/analyzerexec"
	"time"
)

type SnapshotVerifier interface{ VerifySnapshot(RegistrySnapshot) error }
type frozen interface {
	SnapshotVerifier
	CloneSnapshotVerifier() SnapshotVerifier
	coreBoundedSnapshotVerifier()
}

// PinnedSnapshotVerifier is the production snapshot authority for the
// experimental static runtime. It is deliberately data-only: arbitrary
// callbacks cannot enter Core's hard-wall interval. A persisted signed
// registry replaces this pin in a promoted profile.
type PinnedSnapshotVerifier struct {
	Digest     Opaque
	TrustEpoch Opaque
}

func (verify PinnedSnapshotVerifier) VerifySnapshot(snapshot RegistrySnapshot) error {
	if mustOpaque(verify.Digest) != nil || mustOpaque(verify.TrustEpoch) != nil ||
		snapshot.Digest != verify.Digest || snapshot.TrustEpoch != verify.TrustEpoch || snapshot.Revoked {
		return fail(RegistryUntrusted)
	}
	return nil
}

func (verify PinnedSnapshotVerifier) CloneSnapshotVerifier() SnapshotVerifier { return verify }
func (PinnedSnapshotVerifier) coreBoundedSnapshotVerifier()                   {}

type RegistrySnapshot struct {
	Digest     Opaque
	TrustEpoch Opaque
	Revoked    bool
	Releases   []Release
}
type Release struct {
	PluginID                Opaque
	CapabilityID            Opaque
	ReleaseID               Opaque
	BuildID                 Opaque
	ManifestDigest          Opaque
	ArtifactDigest          Opaque
	HostBinaryDigest        Opaque
	Host                    Platform
	Protocol                Opaque
	ProjectionDigest        Opaque
	ProjectionScheme        Opaque
	ResolverDigest          Opaque
	ClosedInputFamilyDigest Opaque
	Clauses                 []Clause
}
type RepositoryLock struct {
	CapabilityID     Opaque
	PluginID         Opaque
	ReleaseID        Opaque
	BuildID          Opaque
	ManifestDigest   Opaque
	Protocol         Opaque
	ArtifactDigest   Opaque
	HostBinaryDigest Opaque
	ProjectionDigest Opaque
	ProjectionScheme Opaque
	ResolverDigest   Opaque
	Profile          Identity
}
type CacheInputs struct {
	FullSnapshotDigest      Opaque
	ClosedInputFamilyDigest Opaque
	SelectedInputSetDigest  Identity
	CompilationInputDigests []Opaque
	InputHandles            []InputHandle
	RequestBytesDigest      Opaque
	AdmissionVerifierDigest Opaque
	ExactVerifierDigest     Opaque
}
type Request struct {
	Capability Opaque
	Lock       *RepositoryLock
	Snapshot   RegistrySnapshot
	Verifier   SnapshotVerifier
	Profile    Profile
	Profiles   []Profile
	Unit       CompilationUnit
	Target     Platform
	Cache      CacheInputs
}
type Resolution struct {
	CapabilityID           Opaque
	PluginID               Opaque
	Release                Release
	Profile                Profile
	Unit                   CompilationUnit
	Target                 Platform
	Clause                 Clause
	CandidateSetDigest     Identity
	DependencyDigest       Identity
	Identity               Identity
	CacheIdentity          Identity
	RegistrySnapshotDigest Opaque
	TrustEpoch             Opaque
}

func Resolve(request Request) (Resolution, error) {
	resolution, _, err := resolveWithStages(request)
	return resolution, err
}

// resolveWithStages is the only resolver path used by Core and ordinary
// resolution. Each Core stage owns one ordered, monotonic interval. A failed
// stage returns immediately, leaving every later stage NOT_RUN.
func resolveWithStages(request Request) (Resolution, []StageDiagnostic, error) {
	return resolveWithStagesAt(request, time.Now())
}

func resolveWithStagesAt(request Request, origin time.Time) (Resolution, []StageDiagnostic, error) {
	return resolveWithReceiptStagesContext(context.TODO(), request, origin, initialAuthorityStages(), time.Since(origin), true, false)
}

// resolveWithReceiptStages extends an already-started Registry interval. Core
// starts that interval before it reads the live state, so all authority work
// shares one measured monotonic timeline.
func resolveWithReceiptStages(request Request, origin time.Time, stages []StageDiagnostic, registryStarted time.Duration, validateInputHandles, coreOwnsRequestEvidence bool) (Resolution, []StageDiagnostic, error) {
	return resolveWithReceiptStagesContext(context.TODO(), request, origin, stages, registryStarted, validateInputHandles, coreOwnsRequestEvidence)
}

func resolveWithReceiptStagesContext(ctx context.Context, request Request, origin time.Time, stages []StageDiagnostic, registryStarted time.Duration, validateInputHandles, coreOwnsRequestEvidence bool) (Resolution, []StageDiagnostic, error) {
	var profileEdges int
	var providers []Opaque
	var release Release
	var clause Clause
	var dependencyDigest Identity

	if err := runCoreStageContext(ctx, &stages, origin, StageRegistry, registryStarted, func() error {
		if len(request.Profiles) > MaxProfiles {
			return fail(LimitExceeded)
		}
		var bounded bool
		profileEdges, bounded = profileInputEdges(request)
		if !bounded {
			return fail(TransitiveProfileUnavailable)
		}
		if err := validateLookupRequest(request); err != nil {
			return err
		}
		verifier, bounded := request.Verifier.(frozen)
		if !bounded || verifier.VerifySnapshot(cloneReg(request.Snapshot)) != nil || request.Snapshot.Revoked {
			return fail(RegistryUntrusted)
		}
		var err error
		providers, err = providerSet(request.Snapshot, request.Capability)
		if err != nil {
			return err
		}
		if len(providers) == 0 {
			return fail(NoCapabilityProvider)
		}
		return nil
	}); err != nil {
		return Resolution{}, stages, err
	}
	if err := runCoreStageContext(ctx, &stages, origin, StageLock, time.Since(origin), func() error {
		if request.Lock == nil {
			if len(providers) > 1 {
				return fail(AmbiguousProvider)
			}
			return fail(LockUnavailable)
		}
		if err := validateLock(*request.Lock, request.Capability); err != nil {
			return err
		}
		if !containsOpaque(providers, request.Lock.PluginID) {
			return fail(LockMismatch)
		}
		var ok bool
		release, ok = exactRelease(request.Snapshot.Releases, *request.Lock)
		if !ok {
			return fail(LockMismatch)
		}
		return release.validateManifest()
	}); err != nil {
		return Resolution{}, stages, err
	}
	if err := runCoreStageContext(ctx, &stages, origin, StageResolve, time.Since(origin), func() error {
		return validateSelectedRequest(request, release, profileEdges, validateInputHandles, coreOwnsRequestEvidence)
	}); err != nil {
		return Resolution{}, stages, err
	}
	if err := runCoreStageContext(ctx, &stages, origin, StageCompatibility, time.Since(origin), func() error {
		if release.Host != request.Profile.Host || release.Host != actualHostPlatform() {
			return fail(HostPlatformUnsupported)
		}
		if request.Cache.ClosedInputFamilyDigest != release.ClosedInputFamilyDigest {
			return fail(CapabilityProjectionUnavailable)
		}
		if request.Profile.Identity() == "" || request.Lock.Profile != request.Profile.Identity() {
			return fail(LockMismatch)
		}
		var err error
		clause, err = selectClause(release, request.Profile, request.Target)
		return err
	}); err != nil {
		return Resolution{}, stages, err
	}
	if err := runCoreStageContext(ctx, &stages, origin, StageDependency, time.Since(origin), func() error {
		var err error
		dependencyDigest, err = resolveDependencies(request.Profile, clause.Dependencies, request.Profiles)
		return err
	}); err != nil {
		return Resolution{}, stages, err
	}
	resolution := Resolution{CapabilityID: request.Capability, PluginID: request.Lock.PluginID, Release: cloneRel(release), Profile: cloneProf(request.Profile), Unit: request.Unit, Target: request.Target, Clause: cloneC(clause), CandidateSetDigest: candidateSetDigest(request.Snapshot, request.Capability), DependencyDigest: dependencyDigest, RegistrySnapshotDigest: request.Snapshot.Digest, TrustEpoch: request.Snapshot.TrustEpoch}
	resolution.Unit.Targets = clone(request.Unit.Targets)
	resolution.Identity = resolutionIdentity(resolution)
	// A final cache identity requires Core's SHA-256 of the exact canonical
	// invocation bytes. Ordinary resolution has no such trusted byte sequence.
	resolution.CacheIdentity = ""
	return resolution, stages, nil
}

// actualHostPlatform is the executing Core's native-process ABI tuple. ABI
// names the bounded V0 process-launch contract, not a target tuple.
var nativeHost = loadNativeHostPlatform()

func actualHostPlatform() Platform { return nativeHost }

func loadNativeHostPlatform() Platform {
	host, err := analyzerexec.ActualHostPlatform()
	if err != nil {
		return Platform{}
	}
	return Platform{OS: Opaque(host.OS), Architecture: Opaque(host.Architecture), ABI: Opaque(host.ABI)}
}

func runCoreStage(stages *[]StageDiagnostic, origin time.Time, stage Stage, run func() error) error {
	return runCoreStageAt(stages, origin, stage, time.Since(origin), run)
}

func runCoreStageAt(stages *[]StageDiagnostic, origin time.Time, stage Stage, started time.Duration, run func() error) error {
	err := run()
	*stages = recordStageInterval(*stages, stage, resultFor(err), started, time.Since(origin))
	return err
}

func runCoreStageContext(ctx context.Context, stages *[]StageDiagnostic, origin time.Time, stage Stage, started time.Duration, run func() error) error {
	err := coreContextFailure(ctx)
	if err == nil {
		err = run()
	}
	if err == nil {
		err = coreContextFailure(ctx)
	}
	*stages = recordStageInterval(*stages, stage, resultFor(err), started, time.Since(origin))
	return err
}
func validateLookupRequest(request Request) error {
	if err := mustOpaque(request.Capability); err != nil {
		return fail(ProfileUnavailable)
	}
	if err := mustOpaque(request.Snapshot.Digest); err != nil {
		return fail(RegistrySnapshotUnavailable)
	}
	if err := mustOpaque(request.Snapshot.TrustEpoch); err != nil {
		return fail(RegistrySnapshotUnavailable)
	}
	return nil
}
func validateSelectedRequest(request Request, release Release, profileEdges int, validateInputHandles, coreOwnsRequestEvidence bool) error {
	if request.Profile.Identity() == "" || request.Profile.validate() != nil {
		return fail(ProfileUnavailable)
	}
	if err := request.Unit.validate(); err != nil {
		return err
	}
	if err := request.Target.validate(); err != nil {
		return fail(TargetPlatformUnsupported)
	}
	if !containsPlatform(request.Unit.Targets, request.Target) {
		return fail(TargetPlatformUnsupported)
	}
	if len(request.Cache.CompilationInputDigests) > MaxInputDigests {
		return fail(LimitExceeded)
	}
	// RequestBytesDigest is Core-produced protocol evidence. A public resolver
	// must reject it rather than treating a printable caller value as evidence.
	inputs := append([]Opaque{request.Cache.FullSnapshotDigest, request.Cache.ClosedInputFamilyDigest, request.Cache.AdmissionVerifierDigest, request.Cache.ExactVerifierDigest}, request.Cache.CompilationInputDigests...)
	for _, value := range inputs {
		if err := mustOpaque(value); err != nil {
			return fail(InputEvidenceMismatch)
		}
	}
	if !coreOwnsRequestEvidence && request.Cache.RequestBytesDigest != "" {
		return fail(InputEvidenceMismatch)
	}
	cacheKeys := make([]string, len(request.Cache.CompilationInputDigests))
	for index, value := range request.Cache.CompilationInputDigests {
		cacheKeys[index] = string(value)
	}
	if !strictlySorted(cacheKeys) {
		return fail(InputEvidenceMismatch)
	}
	if validateInputHandles && (len(request.Cache.InputHandles) != len(request.Cache.CompilationInputDigests) || validateInputs(request.Cache.InputHandles) != nil) {
		return fail(InputEvidenceMismatch)
	}
	if validateInputHandles {
		for index, input := range request.Cache.InputHandles {
			if input.Digest != request.Cache.CompilationInputDigests[index] {
				return fail(InputEvidenceMismatch)
			}
		}
		if request.Cache.SelectedInputSetDigest == "" || request.Cache.SelectedInputSetDigest != inputSetDigest(request.Cache.InputHandles) {
			return fail(InputEvidenceMismatch)
		}
	}
	if !selectedManifestEdgesWithinLimit(profileEdges, release) {
		return fail(TransitiveProfileUnavailable)
	}
	return nil
}
func profileInputEdges(request Request) (int, bool) {
	total := len(request.Profile.Edges)
	if total > MaxEdgesPerRequest {
		return 0, false
	}
	for _, profile := range request.Profiles {
		if len(profile.Edges) > MaxEdgesPerRequest-total {
			return 0, false
		}
		total += len(profile.Edges)
	}
	return total, true
}
func selectedManifestEdgesWithinLimit(total int, release Release) bool {
	for _, clause := range release.Clauses {
		if len(clause.Dependencies) > MaxEdgesPerRequest-total {
			return false
		}
		total += len(clause.Dependencies)
	}
	return true
}
