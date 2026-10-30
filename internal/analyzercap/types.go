package analyzercap

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
)

const (
	MaxOpaqueBytes        = 128
	MaxComponents         = 64
	MaxFeatures           = 64
	MaxEdgesPerProfile    = 64
	MaxEdgesPerRequest    = 128
	MaxProfiles           = 64
	MaxUnits              = 64
	MaxTargetsPerUnit     = 16
	MaxClauses            = 32
	MaxTermsPerClause     = 64
	MaxInputDigests       = 64
	MaxCandidateReleases  = 64
	MaxProviderCandidates = MaxCandidateReleases
)

type Opaque string

func ParseOpaque(raw string) (Opaque, error) {
	if len(raw) == 0 || len(raw) > MaxOpaqueBytes {
		return "", fail(LimitExceeded)
	}
	for index := 0; index < len(raw); index++ {
		c := raw[index]
		if c < 0x21 || c > 0x7e {
			return "", fail(ProfileUnavailable)
		}
	}
	return Opaque(raw), nil
}
func mustOpaque(value Opaque) error {
	_, err := ParseOpaque(string(value))
	return err
}

type Identity string

func digest(domain string, parts ...string) Identity {
	hasher := sha256.New()
	var length [4]byte
	write := func(value string) {
		_, _ = hasher.Write([]byte(value))
	}
	write("analyzercap/canonical/v1")
	binary.BigEndian.PutUint32(length[:], uint32(len(domain)))
	_, _ = hasher.Write(length[:])
	write(domain)
	binary.BigEndian.PutUint32(length[:], uint32(len(parts)))
	_, _ = hasher.Write(length[:])
	for _, part := range parts {
		binary.BigEndian.PutUint32(length[:], uint32(len(part)))
		_, _ = hasher.Write(length[:])
		write(part)
	}
	var sum [sha256.Size]byte
	digestBytes := hasher.Sum(sum[:0])
	var encoded [sha256.Size * 2]byte
	hex.Encode(encoded[:], digestBytes)
	return Identity(string(encoded[:]))
}
func canonical(domain string, parts ...string) string {
	size := len("analyzercap/canonical/v1") + 4 + len(domain) + 4
	for _, part := range parts {
		size += 4 + len(part)
	}
	encoded := make([]byte, size)
	position := copy(encoded, "analyzercap/canonical/v1")
	binary.BigEndian.PutUint32(encoded[position:], uint32(len(domain)))
	position += 4 + copy(encoded[position+4:], domain)
	binary.BigEndian.PutUint32(encoded[position:], uint32(len(parts)))
	position += 4
	for _, part := range parts {
		binary.BigEndian.PutUint32(encoded[position:], uint32(len(part)))
		position += 4 + copy(encoded[position+4:], part)
	}
	return string(encoded)
}

type Platform struct {
	OS           Opaque
	Architecture Opaque
	ABI          Opaque
}

func (p Platform) validate() error {
	if err := mustOpaque(p.OS); err != nil {
		return err
	}
	if err := mustOpaque(p.Architecture); err != nil {
		return err
	}
	return mustOpaque(p.ABI)
}
func (p Platform) canonical() string {
	return canonical("platform/v1", string(p.OS), string(p.Architecture), string(p.ABI))
}

type Component struct {
	ScopeID              Opaque
	Role                 Opaque
	EcosystemCoordinate  Opaque
	StableInstanceID     Opaque
	Scheme               Opaque
	CanonicalExactValue  Opaque
	SourceEvidenceDigest Opaque
}

func (c Component) validate() error {
	for _, value := range []Opaque{c.ScopeID, c.Role, c.EcosystemCoordinate, c.StableInstanceID, c.SourceEvidenceDigest} {
		if err := mustOpaque(value); err != nil {
			return fail(ProfileUnavailable)
		}
	}
	return ValidateActualScheme(c.Scheme, c.CanonicalExactValue)
}
func (c Component) sameIdentity(other Component) bool {
	return c.ScopeID == other.ScopeID && c.Role == other.Role && c.EcosystemCoordinate == other.EcosystemCoordinate && c.StableInstanceID == other.StableInstanceID
}
func (c Component) canonical() string {
	return canonical("component/v1", string(c.ScopeID), string(c.Role), string(c.EcosystemCoordinate), string(c.StableInstanceID), string(c.Scheme), string(c.CanonicalExactValue), string(c.SourceEvidenceDigest))
}

type FeatureState string

const (
	FeatureEnabled  FeatureState = "enabled"
	FeatureDisabled FeatureState = "disabled"
	FeatureUnknown  FeatureState = "unknown"
)

func (s FeatureState) valid() bool {
	return s == FeatureEnabled || s == FeatureDisabled || s == FeatureUnknown
}

type Feature struct {
	Name  Opaque
	State FeatureState
}
type DependencyEdge struct {
	DependentProfile Identity
	RelationKind     Opaque
	Optional         bool
	RequiredState    Opaque
	EvidenceDigest   Opaque
}

func (e DependencyEdge) validate() error {
	if e.DependentProfile == "" {
		return fail(TransitiveProfileUnavailable)
	}
	for _, value := range []Opaque{e.RelationKind, e.RequiredState, e.EvidenceDigest} {
		if err := mustOpaque(value); err != nil {
			return fail(TransitiveProfileUnavailable)
		}
	}
	return nil
}
func (e DependencyEdge) canonical() string {
	return canonical("dependency-edge/v1", string(e.DependentProfile), string(e.RelationKind), boolString(e.Optional), string(e.RequiredState), string(e.EvidenceDigest))
}

type Profile struct {
	Components []Component
	Host       Platform
	Features   []Feature
	Edges      []DependencyEdge
	identity   Identity
}

func NewProfile(components []Component, host Platform, features []Feature, edges []DependencyEdge) (Profile, error) {
	if len(components) == 0 || len(components) > MaxComponents || len(features) > MaxFeatures || len(edges) > MaxEdgesPerProfile {
		return Profile{}, fail(LimitExceeded)
	}
	if err := host.validate(); err != nil {
		return Profile{}, fail(ProfileUnavailable)
	}
	componentKeys := make([]string, len(components))
	for index, component := range components {
		if err := component.validate(); err != nil {
			return Profile{}, err
		}
		// Canonical keys lead with the identity fields, so once the keys are
		// strictly sorted a repeated identity can only sit next to its twin.
		if index > 0 && components[index-1].sameIdentity(component) {
			return Profile{}, fail(ProfileUnavailable)
		}
		componentKeys[index] = component.canonical()
	}
	if !strictlySorted(componentKeys) {
		return Profile{}, fail(ProfileUnavailable)
	}
	featureKeys := make([]string, len(features))
	featureNames := make(map[Opaque]struct{}, len(features))
	for index, feature := range features {
		if err := mustOpaque(feature.Name); err != nil || !feature.State.valid() {
			return Profile{}, fail(ProfileUnavailable)
		}
		if _, duplicate := featureNames[feature.Name]; duplicate {
			return Profile{}, fail(ProfileUnavailable)
		}
		featureNames[feature.Name] = struct{}{}
		featureKeys[index] = canonical("feature/v1", string(feature.Name), string(feature.State))
	}
	if !strictlySorted(featureKeys) {
		return Profile{}, fail(ProfileUnavailable)
	}
	edgeKeys := make([]string, len(edges))
	for index, edge := range edges {
		if err := edge.validate(); err != nil {
			return Profile{}, err
		}
		edgeKeys[index] = edge.canonical()
	}
	if !strictlySorted(edgeKeys) {
		return Profile{}, fail(TransitiveProfileUnavailable)
	}
	parts := []string{host.canonical()}
	parts = append(parts, componentKeys...)
	parts = append(parts, featureKeys...)
	parts = append(parts, edgeKeys...)
	return Profile{Components: clone(components), Host: host, Features: clone(features), Edges: clone(edges), identity: digest("profile/v1", parts...)}, nil
}
func (p Profile) Identity() Identity { return p.identity }
func (p Profile) validate() error {
	rebuilt, err := NewProfile(p.Components, p.Host, p.Features, p.Edges)
	if err != nil {
		return err
	}
	if rebuilt.identity != p.identity {
		return fail(ProfileUnavailable)
	}
	return nil
}

type CompilationUnit struct {
	ID      Opaque
	Targets []Platform
}

func (u CompilationUnit) validate() error {
	if err := mustOpaque(u.ID); err != nil {
		return err
	}
	if len(u.Targets) == 0 || len(u.Targets) > MaxTargetsPerUnit {
		return fail(LimitExceeded)
	}
	keys := make([]string, len(u.Targets))
	for index, target := range u.Targets {
		if err := target.validate(); err != nil {
			return fail(TargetPlatformUnsupported)
		}
		keys[index] = target.canonical()
	}
	if !strictlySorted(keys) {
		return fail(TargetPlatformUnsupported)
	}
	return nil
}
func strictlySorted(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index-1] >= values[index] {
			return false
		}
	}
	return true
}
func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func clone[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}
func sortedOpaque(values []Opaque) []Opaque {
	result := clone(values)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
