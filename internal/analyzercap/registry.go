package analyzercap

import "sort"

func providerSet(snapshot RegistrySnapshot, capability Opaque) ([]Opaque, error) {
	// Both sets are globally bounded at 64 entries. A provider slice plus a
	// bounded prior-candidate scan avoid per-resolution hash tables and keep
	// duplicate handling deterministic.
	providers := make([]Opaque, 0, min(len(snapshot.Releases), MaxProviderCandidates))
	count := 0
	for releaseIndex, release := range snapshot.Releases {
		if release.CapabilityID != capability {
			continue
		}
		count++
		if count > MaxCandidateReleases {
			return nil, fail(RegistrySnapshotUnavailable)
		}
		if err := release.validateIdentity(); err != nil {
			return nil, err
		}
		key := release.candidateIdentity()
		for priorIndex := 0; priorIndex < releaseIndex; priorIndex++ {
			prior := snapshot.Releases[priorIndex]
			if prior.CapabilityID == capability && prior.candidateIdentity() == key {
				return nil, fail(RegistrySnapshotUnavailable)
			}
		}
		if !containsOpaque(providers, release.PluginID) {
			providers = append(providers, release.PluginID)
		}
	}
	if len(providers) > MaxProviderCandidates {
		return nil, fail(RegistrySnapshotUnavailable)
	}
	if len(providers) > 1 {
		sort.Slice(providers, func(left, right int) bool { return providers[left] < providers[right] })
	}
	return providers, nil
}
func (r Release) validateIdentity() error {
	for _, value := range []Opaque{r.PluginID, r.CapabilityID, r.ReleaseID, r.BuildID, r.ManifestDigest, r.ArtifactDigest, r.HostBinaryDigest, r.Protocol, r.ProjectionDigest, r.ProjectionScheme, r.ResolverDigest, r.ClosedInputFamilyDigest} {
		if err := mustOpaque(value); err != nil {
			return fail(RegistrySnapshotUnavailable)
		}
	}
	if err := r.Host.validate(); err != nil {
		return fail(HostPlatformUnsupported)
	}
	return nil
}
func (r Release) validateManifest() error {
	if len(r.Clauses) == 0 || len(r.Clauses) > MaxClauses {
		return fail(CapabilityProjectionUnavailable)
	}
	keys := make([]string, len(r.Clauses))
	for index, clause := range r.Clauses {
		if err := clause.validate(); err != nil {
			return err
		}
		// Sort keys lead with the clause ID, so once they are strictly sorted a
		// repeated ID can only sit next to its twin.
		if index > 0 && r.Clauses[index-1].ID == clause.ID {
			return fail(CapabilityProjectionUnavailable)
		}
		keys[index] = clause.sortKey()
	}
	if !strictlySorted(keys) {
		return fail(CapabilityProjectionUnavailable)
	}
	return nil
}
func (r Release) candidateIdentity() string {
	return canonical("candidate/v1", string(r.CapabilityID), string(r.PluginID), string(r.ReleaseID), string(r.BuildID), string(r.ManifestDigest), string(r.ArtifactDigest), string(r.HostBinaryDigest), r.Host.canonical(), string(r.Protocol), string(r.ProjectionDigest), string(r.ProjectionScheme), string(r.ResolverDigest), string(r.ClosedInputFamilyDigest))
}
func validateLock(lock RepositoryLock, capability Opaque) error {
	for _, value := range []Opaque{lock.CapabilityID, lock.PluginID, lock.ReleaseID, lock.BuildID, lock.ManifestDigest, lock.Protocol, lock.ArtifactDigest, lock.HostBinaryDigest, lock.ProjectionDigest, lock.ProjectionScheme, lock.ResolverDigest} {
		if err := mustOpaque(value); err != nil {
			return fail(LockUnavailable)
		}
	}
	if lock.Profile == "" || lock.CapabilityID != capability {
		return fail(LockMismatch)
	}
	return nil
}
func exactRelease(releases []Release, lock RepositoryLock) (Release, bool) {
	var found *Release
	for index := range releases {
		release := &releases[index]
		if release.CapabilityID == lock.CapabilityID && release.PluginID == lock.PluginID && release.ReleaseID == lock.ReleaseID && release.BuildID == lock.BuildID && release.ManifestDigest == lock.ManifestDigest && release.Protocol == lock.Protocol && release.ArtifactDigest == lock.ArtifactDigest && release.HostBinaryDigest == lock.HostBinaryDigest && release.ProjectionDigest == lock.ProjectionDigest && release.ProjectionScheme == lock.ProjectionScheme && release.ResolverDigest == lock.ResolverDigest {
			if found != nil {
				return Release{}, false
			}
			found = release
		}
	}
	if found == nil {
		return Release{}, false
	}
	return *found, true
}
