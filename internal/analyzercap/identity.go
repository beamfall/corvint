package analyzercap

import "sort"

func candidateSetDigest(snapshot RegistrySnapshot, capability Opaque) Identity {
	parts := []string{string(snapshot.Digest), string(snapshot.TrustEpoch), string(capability)}
	for _, release := range snapshot.Releases {
		if release.CapabilityID == capability {
			parts = append(parts, release.candidateIdentity())
		}
	}
	sort.Strings(parts[3:])
	return digest("candidate-set/v1", parts...)
}
func cacheIdentity(resolution Resolution, inputs CacheInputs) Identity {
	parts := []string{string(resolution.Identity), string(resolution.CapabilityID), string(resolution.RegistrySnapshotDigest), string(resolution.TrustEpoch), string(resolution.Release.PluginID), string(resolution.Release.ReleaseID), string(resolution.Release.BuildID), string(resolution.Release.ManifestDigest), string(resolution.Release.Protocol), string(resolution.Release.ArtifactDigest), string(resolution.Release.HostBinaryDigest), resolution.Release.Host.canonical(), string(resolution.Release.ProjectionDigest), string(resolution.Release.ProjectionScheme), string(resolution.Release.ResolverDigest), string(resolution.Profile.Identity()), resolution.Profile.Host.canonical(), resolution.Target.canonical(), string(resolution.Unit.ID), string(resolution.Clause.ID), string(resolution.CandidateSetDigest), string(resolution.DependencyDigest), string(inputs.FullSnapshotDigest), string(inputs.ClosedInputFamilyDigest), string(inputs.SelectedInputSetDigest), string(inputs.RequestBytesDigest), string(inputs.AdmissionVerifierDigest), string(inputs.ExactVerifierDigest)}
	for _, value := range inputs.CompilationInputDigests {
		parts = append(parts, string(value))
	}
	for _, input := range inputs.InputHandles {
		parts = append(parts, string(input.Handle), string(input.Digest))
	}
	return digest("cache/v1", parts...)
}
func resolutionIdentity(r Resolution) Identity {
	parts := []string{string(r.CapabilityID), string(r.Release.PluginID), string(r.Release.ReleaseID), string(r.Release.BuildID), string(r.Profile.Identity()), r.Release.Host.canonical(), r.Profile.Host.canonical(), r.Target.canonical(), string(r.Unit.ID), string(r.Clause.ID), string(r.CandidateSetDigest), string(r.DependencyDigest)}
	for _, target := range r.Unit.Targets {
		parts = append(parts, target.canonical())
	}
	return digest("resolution/v1", parts...)
}
func inputSetDigest(inputs []InputHandle) Identity {
	parts := make([]string, 0, len(inputs)*2)
	for _, input := range inputs {
		parts = append(parts, string(input.Handle), string(input.Digest))
	}
	return digest("selected-input-set/v2", parts...)
}
func containsOpaque(values []Opaque, needle Opaque) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
func containsPlatform(values []Platform, needle Platform) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
