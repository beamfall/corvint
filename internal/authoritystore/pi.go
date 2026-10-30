package authoritystore

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/localauthority"
)

const PiRootProfile = "corvint-protected-root/3"
const piCampaignProfile = "corvint-native-qualification-campaign/2"
const piQualificationProfile = "corvint-native-qualified-pi-host/0"

// PiRuntime pins the closed SDK image, which contains both native surfaces and
// every executable asset. It is a process lifetime, never an exec epoch.
type PiRuntime DirectRuntime

type PiQualification struct {
	Profile           string                 `json:"profile"`
	EvidenceSHA256    string                 `json:"evidenceSHA256"`
	Runtime           PiRuntime              `json:"runtime"`
	QualifiedSurfaces []SurfaceQualification `json:"qualifiedSurfaces"`
}

func (r PiRuntime) valid() bool {
	return r.Topology == "protected-pi-sdk" && r.Host == "pi" && r.Surface == "pi-native" && validNativePin(DirectRuntime(r))
}
func (q *PiQualification) valid() bool {
	return q != nil && q.Profile == piQualificationProfile && hexDigest(q.EvidenceSHA256) && q.Runtime.valid() && len(q.QualifiedSurfaces) == 2 && q.QualifiedSurfaces[0].Surface == "pi-tui" && q.QualifiedSurfaces[1].Surface == "pi-rpc" && hexDigest(q.QualifiedSurfaces[0].EvidenceSHA256) && hexDigest(q.QualifiedSurfaces[1].EvidenceSHA256)
}

// ResolvePiLifecycle selects only QLF/2 and the fixed root/3 store. A different
// installed host cannot be selected, substituted or downgraded by request input.
func ResolvePiLifecycle(ctx context.Context, stop bool, observe LifecycleObserver) (LifecycleResolution, error) {
	files, err := openProtectedFiles()
	if err != nil || observe == nil {
		return LifecycleResolution{}, errUnavailable
	}
	return resolveLifecycleProfile(ctx, stop, files, time.Now(), observe, PiRootProfile)
}
func piLifecycleScope(root RootDocument, r PiRuntime) LifecycleScope {
	return LifecycleScope{Pi: true, RepositoryRoot: root.RepositoryRoot, HostSHA256: r.HostImage.SHA256, RuntimeAdmissionEvidenceSHA256: r.RuntimeAdmissionEvidenceSHA256, AdapterSHA256: root.Adapter.SHA256, OSBuild: r.OSBuild, Architecture: r.Architecture, QualifiedSurfaces: []SurfaceQualification{}}
}
func qualifiedPiScope(root RootDocument) (LifecycleScope, error) {
	q := root.PiQualification
	if !q.valid() || root.DirectQualification != nil || root.HostQualification != nil {
		return LifecycleScope{}, errUnavailable
	}
	scope := piLifecycleScope(root, q.Runtime)
	digest, err := localauthority.Digest(q)
	if err != nil {
		return LifecycleScope{}, errUnavailable
	}
	scope.QualifiedHostSHA256 = digest
	scope.EvidenceSHA256 = q.EvidenceSHA256
	scope.SupportScope = "qualified-protected-pi-runtime"
	scope.QualifiedSurfaces = append([]SurfaceQualification(nil), q.QualifiedSurfaces...)
	return scope, nil
}
