package analyzercap

import "sort"

type Clause struct {
	ID           Opaque
	Host         Platform
	Target       Platform
	Profile      Identity
	ReleaseID    Opaque
	Protocol     Opaque
	Components   []ComponentRequirement
	Features     []Feature
	Dependencies []DependencyEdge
}
type ComponentRequirement struct {
	ScopeID             Opaque
	Role                Opaque
	EcosystemCoordinate Opaque
	StableInstanceID    Opaque
	Scheme              Opaque
	CanonicalValue      Opaque
}

func (r ComponentRequirement) validate() error {
	for _, value := range []Opaque{r.ScopeID, r.Role, r.EcosystemCoordinate, r.StableInstanceID} {
		if err := mustOpaque(value); err != nil {
			return fail(CapabilityProjectionUnavailable)
		}
	}
	return ValidateRequirementScheme(r.Scheme, r.CanonicalValue)
}
func (r ComponentRequirement) canonical() string {
	return canonical("component-requirement/v1", string(r.ScopeID), string(r.Role), string(r.EcosystemCoordinate), string(r.StableInstanceID), string(r.Scheme), string(r.CanonicalValue))
}
func (c Clause) validate() error {
	for _, value := range []Opaque{c.ID, c.ReleaseID, c.Protocol} {
		if err := mustOpaque(value); err != nil {
			return fail(CapabilityProjectionUnavailable)
		}
	}
	if c.Profile == "" {
		return fail(CapabilityProjectionUnavailable)
	}
	if err := c.Host.validate(); err != nil {
		return fail(HostPlatformUnsupported)
	}
	if err := c.Target.validate(); err != nil {
		return fail(TargetPlatformUnsupported)
	}
	if len(c.Components)+len(c.Features)+len(c.Dependencies) > MaxTermsPerClause {
		return fail(LimitExceeded)
	}
	componentKeys := make([]string, len(c.Components))
	for index, requirement := range c.Components {
		if err := requirement.validate(); err != nil {
			return err
		}
		componentKeys[index] = requirement.canonical()
	}
	if !strictlySorted(componentKeys) {
		return fail(CapabilityProjectionUnavailable)
	}
	featureKeys := make([]string, len(c.Features))
	featureNames := make(map[Opaque]struct{}, len(c.Features))
	for index, feature := range c.Features {
		if err := mustOpaque(feature.Name); err != nil || !feature.State.valid() || feature.State == FeatureUnknown {
			return fail(CapabilityProjectionUnavailable)
		}
		if _, duplicate := featureNames[feature.Name]; duplicate {
			return fail(CapabilityProjectionUnavailable)
		}
		featureNames[feature.Name] = struct{}{}
		featureKeys[index] = canonical("feature-requirement/v1", string(feature.Name), string(feature.State))
	}
	if !strictlySorted(featureKeys) {
		return fail(CapabilityProjectionUnavailable)
	}
	edgeKeys := make([]string, len(c.Dependencies))
	for index, edge := range c.Dependencies {
		if err := edge.validate(); err != nil {
			return err
		}
		edgeKeys[index] = edge.canonical()
	}
	if !strictlySorted(edgeKeys) {
		return fail(CapabilityProjectionUnavailable)
	}
	return nil
}
func (c Clause) canonical() string {
	parts := []string{string(c.ID), c.Host.canonical(), c.Target.canonical(), string(c.Profile), string(c.ReleaseID), string(c.Protocol)}
	for _, component := range c.Components {
		parts = append(parts, component.canonical())
	}
	for _, feature := range c.Features {
		parts = append(parts, canonical("feature-requirement/v1", string(feature.Name), string(feature.State)))
	}
	for _, dependency := range c.Dependencies {
		parts = append(parts, dependency.canonical())
	}
	return canonical("clause/v1", parts...)
}
func (c Clause) sortKey() string {
	return canonical("clause-sort/v1", string(c.ID), c.canonical())
}
func matchClause(release Release, profile Profile, target Platform, profiles []Profile) (Clause, Identity, error) {
	clause, err := selectClause(release, profile, target)
	if err != nil {
		return Clause{}, "", err
	}
	dependencyDigest, err := resolveDependencies(profile, clause.Dependencies, profiles)
	if err != nil {
		return Clause{}, "", err
	}
	return clause, dependencyDigest, nil
}

func selectClause(release Release, profile Profile, target Platform) (Clause, error) {
	matching := make([]Clause, 0, 2)
	unknown := false
	for _, clause := range release.Clauses {
		if clause.Host != profile.Host || clause.Target != target || clause.Profile != profile.Identity() || clause.ReleaseID != release.ReleaseID || clause.Protocol != release.Protocol {
			continue
		}
		features, hasUnknown := featuresMatch(profile.Features, clause.Features)
		if hasUnknown {
			unknown = true
			continue
		}
		if !features || !componentsMatch(profile.Components, clause.Components) {
			continue
		}
		matching = append(matching, clause)
	}
	if len(matching) == 0 && unknown {
		return Clause{}, fail(FailureFeatureUnknown)
	}
	if len(matching) == 0 {
		return Clause{}, fail(NoCompatibleClause)
	}
	if len(matching) > 1 {
		return Clause{}, fail(AmbiguousClause)
	}
	return matching[0], nil
}
func componentsMatch(actual []Component, requirements []ComponentRequirement) bool {
	for _, requirement := range requirements {
		matched := false
		for _, component := range actual {
			if component.ScopeID != requirement.ScopeID || component.Role != requirement.Role || component.EcosystemCoordinate != requirement.EcosystemCoordinate || component.StableInstanceID != requirement.StableInstanceID {
				continue
			}
			switch requirement.Scheme {
			case ExactScheme:
				matched = component.Scheme == ExactScheme && component.CanonicalExactValue == requirement.CanonicalValue
			case SemverRangeScheme:
				version, valid := parseSemver(string(component.CanonicalExactValue))
				rangeValue, err := ParseSemverRange(string(requirement.CanonicalValue))
				matched = component.Scheme == SemverExactScheme && valid && err == nil && rangeValue.Contains(version)
			case SemverExactScheme:
				matched = component.Scheme == SemverExactScheme && component.CanonicalExactValue == requirement.CanonicalValue
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
func featuresMatch(profile []Feature, requirements []Feature) (bool, bool) {
	values := make(map[Opaque]FeatureState, len(profile))
	for _, feature := range profile {
		values[feature.Name] = feature.State
	}
	for _, requirement := range requirements {
		actual, exists := values[requirement.Name]
		if !exists {
			return false, false
		}
		if actual == FeatureUnknown {
			return false, true
		}
		if actual != requirement.State {
			return false, false
		}
	}
	return true, false
}
func resolveDependencies(profile Profile, requirements []DependencyEdge, profiles []Profile) (Identity, error) {
	all := make(map[Identity]Profile, len(profiles)+1)
	all[profile.Identity()] = profile
	for _, candidate := range profiles {
		if candidate.Identity() == "" || candidate.validate() != nil {
			return "", fail(TransitiveProfileUnavailable)
		}
		if _, duplicate := all[candidate.Identity()]; duplicate {
			return "", fail(TransitiveProfileUnavailable)
		}
		all[candidate.Identity()] = candidate
	}
	if len(profile.Edges)+len(requirements) > MaxEdgesPerRequest {
		return "", fail(TransitiveProfileUnavailable)
	}
	rootEdges := map[string]DependencyEdge{}
	for _, edge := range profile.Edges {
		rootEdges[edge.canonical()] = edge
	}
	visiting := map[Identity]bool{}
	done := map[Identity]bool{}
	var visited int
	var visit func(DependencyEdge) error
	visit = func(edge DependencyEdge) error {
		candidate, ok := all[edge.DependentProfile]
		if !ok {
			return fail(TransitiveProfileUnavailable)
		}
		if done[candidate.Identity()] {
			return nil
		}
		if visiting[candidate.Identity()] {
			return fail(TransitiveProfileUnavailable)
		}
		if visited >= MaxEdgesPerRequest {
			return fail(TransitiveProfileUnavailable)
		}
		visited++
		visiting[candidate.Identity()] = true
		for _, next := range candidate.Edges {
			if err := visit(next); err != nil {
				return err
			}
		}
		delete(visiting, candidate.Identity())
		done[candidate.Identity()] = true
		return nil
	}
	keys := make([]string, len(requirements))
	for index, requirement := range requirements {
		if _, ok := rootEdges[requirement.canonical()]; !ok {
			return "", fail(TransitiveProfileUnavailable)
		}
		if err := visit(requirement); err != nil {
			return "", err
		}
		keys[index] = requirement.canonical()
	}
	sort.Strings(keys)
	return digest("dependency/v1", keys...), nil
}
