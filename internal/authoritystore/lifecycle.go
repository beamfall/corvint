package authoritystore

import (
	"context"
	"os"
	"time"

	"github.com/Beamfall/corvint/internal/authorityevent"
	"github.com/Beamfall/corvint/internal/localauthority"
)

// LifecycleScope reports an independently admitted capability and the actual
// held repository. It does not authenticate occurrence of a caller's event.
// No process IDs, raw session identity or caller-selected paths are emitted.
// Candidate ExpectedTarget is an internal guard: the trusted observer must compare
// both common repository snapshots before context and before returning.
type LifecycleScope struct {
	Direct                         bool
	HostSHA256                     string
	RuntimeAdmissionEvidenceSHA256 string
	RepositoryRoot                 string
	ExpectedTarget                 string
	QualifiedHostSHA256            string
	EvidenceSHA256                 string
	AppSHA256                      string
	EngineSHA256                   string
	AdapterSHA256                  string
	OSBuild                        string
	Architecture                   string
	SupportScope                   string
	QualifiedSurfaces              []SurfaceQualification
}

type lifecycleProfile string

type LifecycleObserver func(context.Context, LifecycleScope) error

type LifecycleResolution struct {
	Scope LifecycleScope
	Stop  authorityevent.Resolution
}

// ResolveLifecycle uses only the fixed protected store. Non-Stop context never
// opens an enrollment, publication, target capsule or private state. Stop obtains
// the active handle inside the protected scope and computes exactly once.
func ResolveLifecycle(ctx context.Context, stop bool, observe LifecycleObserver) (LifecycleResolution, error) {
	files, err := openProtectedFiles()
	if err != nil || observe == nil {
		return LifecycleResolution{}, errUnavailable
	}
	return resolveLifecycle(ctx, stop, files, time.Now(), observe)
}

// ResolveDirectLifecycle selects only the closed QLF/1 root/2 pairing. It never
// accepts a root path, host identity or alternate computation from the caller.
func ResolveDirectLifecycle(ctx context.Context, stop bool, observe LifecycleObserver) (LifecycleResolution, error) {
	files, err := openProtectedFiles()
	if err != nil || observe == nil {
		return LifecycleResolution{}, errUnavailable
	}
	return resolveLifecycleProfile(ctx, stop, files, time.Now(), observe, DirectRootProfile)
}
func resolveLifecycle(ctx context.Context, stop bool, files protectedFiles, now time.Time, observe LifecycleObserver) (LifecycleResolution, error) {
	return resolveLifecycleProfile(ctx, stop, files, now, observe, RootProfile)
}
func resolveLifecycleProfile(ctx context.Context, stop bool, files protectedFiles, now time.Time, observe LifecycleObserver, profile lifecycleProfile) (LifecycleResolution, error) {
	var result LifecycleResolution
	capture := func(ctx context.Context, scope LifecycleScope) error {
		result.Scope = scope
		return observe(ctx, scope)
	}
	if stop {
		resolved, err := resolveObservedProfile(ctx, "", files, now, capture, profile)
		if err != nil {
			return LifecycleResolution{}, err
		}
		result.Stop = resolved
		return result, nil
	}
	return resolveLifecycleContextProfile(ctx, files, observe, verifyLifecycleRuntime, profile)
}

// runtimeCheck is private and production always supplies verifyLifecycleRuntime. Tests can
// exercise the held file/cwd bracketing without claiming a native qualification.
func resolveLifecycleContext(ctx context.Context, files protectedFiles, observe LifecycleObserver, runtimeCheck func(context.Context, RootDocument, *qualificationCampaign) error) (LifecycleResolution, error) {
	return resolveLifecycleContextProfile(ctx, files, observe, runtimeCheck, RootProfile)
}
func resolveLifecycleContextProfile(ctx context.Context, files protectedFiles, observe LifecycleObserver, runtimeCheck func(context.Context, RootDocument, *qualificationCampaign) error, profile lifecycleProfile) (LifecycleResolution, error) {
	var result LifecycleResolution
	if observe == nil || runtimeCheck == nil || ctx.Err() != nil {
		return result, errUnavailable
	}
	rootBytes, err := files.read("accepted-root.json", 0, localauthority.MaxReceiptBytes)
	if err != nil {
		return LifecycleResolution{}, errUnavailable
	}
	floorBytes, err := files.read("minimum-generation.json", 0, 4096)
	if err != nil {
		return LifecycleResolution{}, errUnavailable
	}
	root, _, owner, err := decodeRoot(rootBytes, floorBytes)
	if err != nil || root.Profile != string(profile) || owner == uint32(os.Getuid()) || os.Getuid() == 0 {
		return LifecycleResolution{}, errUnavailable
	}
	policyBytes, err := files.read("execution-policy.json", 0, localauthority.MaxReceiptBytes)
	if err != nil || localauthority.BytesDigest(policyBytes) != root.PolicySHA256 {
		return LifecycleResolution{}, errUnavailable
	}
	admission, err := readLifecycleContextAdmission(files, root, floorBytes, time.Now())
	if err != nil || runtimeCheck(ctx, root, admission.campaign) != nil {
		return result, errUnavailable
	}
	binding, err := bindCurrentRepository(ctx, root)
	if err != nil {
		return LifecycleResolution{}, errUnavailable
	}
	defer binding.close()
	scope, err := admission.lifecycleScope(root)
	if err != nil || observe(ctx, scope) != nil {
		return LifecycleResolution{}, errUnavailable
	}
	if unchanged(files, "accepted-root.json", 0, rootBytes) != nil ||
		unchanged(files, "minimum-generation.json", 0, floorBytes) != nil ||
		unchanged(files, "execution-policy.json", 0, policyBytes) != nil ||
		runtimeCheck(ctx, root, admission.campaign) != nil || binding.unchanged(ctx, root) != nil || ctx.Err() != nil {
		return LifecycleResolution{}, errUnavailable
	}
	if admission.campaign != nil && !campaignStillCurrentScope(files, admission.raw, root, floorBytes, admission.campaign.EnrollmentHandle, admission.campaign.Target, time.Now(), lifecycleCampaignScope) {
		return result, errUnavailable
	}
	result.Scope = scope
	return result, nil
}

// A present completed qualification cannot borrow a campaign if it is invalid.
// Candidate context reads only the fixed root-owned campaign, never enrollment.
func readLifecycleContextAdmission(files protectedFiles, root RootDocument, floor []byte, now time.Time) (runtimeAdmission, error) {
	if root.hasQualification() {
		return runtimeAdmission{}, nil
	}
	raw, err := files.read(campaignPath, 0, maxCampaign)
	var candidate qualificationCampaign
	if err != nil || localauthority.Decode(raw, &candidate) != nil || !candidate.validScope(root, floor, candidate.EnrollmentHandle, now, lifecycleCampaignScope) {
		return runtimeAdmission{}, errUnavailable
	}
	return runtimeAdmission{campaign: &candidate, raw: raw, scope: lifecycleCampaignScope}, nil
}
func verifyLifecycleRuntime(ctx context.Context, root RootDocument, candidate *qualificationCampaign) error {
	if candidate == nil {
		return verifyRuntime(ctx, root)
	}
	if root.hasQualification() {
		return errUnavailable
	}
	return verifyCampaignRuntime(ctx, root, *candidate)
}
func (admission runtimeAdmission) lifecycleScope(root RootDocument) (LifecycleScope, error) {
	if admission.campaign == nil {
		return lifecycleScope(root)
	}
	if root.hasQualification() || admission.scope != lifecycleCampaignScope {
		return LifecycleScope{}, errUnavailable
	}
	if root.Profile == DirectRootProfile {
		if admission.campaign.Profile != directCampaignProfile || admission.campaign.DirectRuntime == nil || !admission.campaign.DirectRuntime.valid() {
			return LifecycleScope{}, errUnavailable
		}
		scope := directLifecycleScope(root, *admission.campaign.DirectRuntime)
		scope.ExpectedTarget = admission.campaign.Target
		scope.SupportScope = "candidate-direct-native-runtime"
		return scope, nil
	}
	candidate := admission.campaign.Runtime
	scope := LifecycleScope{RepositoryRoot: root.RepositoryRoot, ExpectedTarget: admission.campaign.Target, AppSHA256: candidate.App.SHA256, EngineSHA256: candidate.Engine.SHA256, AdapterSHA256: root.Adapter.SHA256,
		OSBuild: candidate.OSBuild, Architecture: candidate.Architecture, SupportScope: "candidate-native-runtime", QualifiedSurfaces: []SurfaceQualification{}}
	if candidate.Topology == "shared-daemon" {
		scope.SupportScope = "candidate-shared-runtime"
	}
	return scope, nil
}

func readActiveEnrollment(files protectedFiles, owner uint32) ([]byte, string, error) {
	raw, err := files.read("public/active-enrollment.json", owner, 1024)
	var active ActiveEnrollment
	if err != nil || localauthority.Decode(raw, &active) != nil || active.Profile != "corvint-protected-active-enrollment/0" || !hexDigest(active.EnrollmentHandle) {
		return nil, "", errUnavailable
	}
	return raw, active.EnrollmentHandle, nil
}

func lifecycleScope(root RootDocument) (LifecycleScope, error) {
	scope := LifecycleScope{RepositoryRoot: root.RepositoryRoot}
	if root.Profile == DirectRootProfile {
		if !root.DirectQualification.valid() || root.HostQualification != nil {
			return LifecycleScope{}, errUnavailable
		}
		q := root.DirectQualification
		scope = directLifecycleScope(root, q.Runtime)
		digest, err := localauthority.Digest(q)
		if err != nil {
			return LifecycleScope{}, errUnavailable
		}
		scope.QualifiedHostSHA256 = digest
		scope.EvidenceSHA256 = q.EvidenceSHA256
		scope.SupportScope = "qualified-direct-native-runtime"
		scope.QualifiedSurfaces = []SurfaceQualification{{Surface: "codex-cli", EvidenceSHA256: q.EvidenceSHA256}}
		return scope, nil
	}
	q := root.HostQualification
	if q == nil {
		// The existing bounded Stop campaign is explicitly unqualified.
		return scope, nil
	}
	digest, err := localauthority.Digest(q)
	if err != nil {
		return LifecycleScope{}, errUnavailable
	}
	scope.QualifiedHostSHA256, scope.EvidenceSHA256 = digest, q.EvidenceSHA256
	scope.AppSHA256, scope.EngineSHA256, scope.AdapterSHA256 = q.App.SHA256, q.Engine.SHA256, root.Adapter.SHA256
	scope.OSBuild, scope.Architecture = q.OSBuild, q.Architecture
	scope.SupportScope = "qualified-native-runtime"
	scope.QualifiedSurfaces = []SurfaceQualification{{Surface: q.Surface, EvidenceSHA256: q.EvidenceSHA256}}
	if q.Topology == "shared-daemon" {
		scope.SupportScope = "qualified-shared-runtime"
		scope.QualifiedSurfaces = append([]SurfaceQualification(nil), q.QualifiedSurfaces...)
	}
	return scope, nil
}

func directLifecycleScope(root RootDocument, r DirectRuntime) LifecycleScope {
	return LifecycleScope{Direct: true, RepositoryRoot: root.RepositoryRoot, HostSHA256: r.HostImage.SHA256, RuntimeAdmissionEvidenceSHA256: r.RuntimeAdmissionEvidenceSHA256, AdapterSHA256: root.Adapter.SHA256, OSBuild: r.OSBuild, Architecture: r.Architecture, QualifiedSurfaces: []SurfaceQualification{}}
}
