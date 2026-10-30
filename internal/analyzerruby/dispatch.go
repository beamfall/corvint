package analyzerruby

import (
	"crypto/sha256"
	"encoding/hex"
)

// Input record families of the closed Ruby matrix. `ruby.gemspec`,
// `ruby.bundle-config`, and `ruby.rails-marker` are named by the spec but not
// implemented by this candidate, so they reject as a whole family rather than
// being accepted and ignored.
const (
	familyVersion = "ruby.version"
	familyGemfile = "ruby.gemfile"
	familyLock    = "ruby.gemfile-lock"
	familySource  = "ruby.source"
	familyGemspec = "ruby.gemspec"
	familyConfig  = "ruby.bundle-config"
	familyRails   = "ruby.rails-marker"
)

// routed groups one request's inputs by their record family.
type routed struct {
	version *decoded
	gemfile *decoded
	lock    *decoded
	sources []decoded
}

// analyze routes, parses, cross-checks, and emits. Every input is parsed before
// any fact is retained, so a request that rejects never emits a partial view.
func analyze(r Request, inputs []decoded, collector *factCollector) string {
	group, why := route(inputs)
	if why != "" {
		return why
	}
	declared, why := parseManifests(group)
	if why != "" {
		return why
	}
	if why := emitManifestFacts(r, group, declared, collector); why != "" {
		return why
	}
	return emitSourceFacts(r, group, collector)
}

// declaredState is the parsed manifest view shared by the cross-checks.
type declaredState struct {
	version string
	gemfile *gemfileDoc
	lock    *lockDoc
}

func route(inputs []decoded) (routed, string) {
	var group routed
	for i := range inputs {
		input := inputs[i]
		slot := (**decoded)(nil)
		switch input.in.Family {
		case familyVersion:
			slot = &group.version
		case familyGemfile:
			slot = &group.gemfile
		case familyLock:
			slot = &group.lock
		case familySource:
			group.sources = append(group.sources, input)
			continue
		case familyGemspec, familyConfig, familyRails:
			return routed{}, "UNSUPPORTED_SCHEMA"
		default:
			return routed{}, "UNSUPPORTED_SCHEMA"
		}
		if *slot != nil {
			return routed{}, "DUPLICATE_VALUE"
		}
		*slot = &inputs[i]
	}
	return group, ""
}

// parseManifests parses each manifest record and reconciles every Ruby version
// declaration across the records that carry one.
func parseManifests(group routed) (declaredState, string) {
	var state declaredState
	if group.version != nil {
		version, why := parseRubyVersionFile(group.version.body)
		if why != "" {
			return declaredState{}, why
		}
		state.version = version
	}
	if group.gemfile != nil {
		doc, why := parseGemfile(group.gemfile.body)
		if why != "" {
			return declaredState{}, why
		}
		state.gemfile = doc
	}
	if group.lock != nil {
		doc, why := parseGemfileLock(group.lock.body)
		if why != "" {
			return declaredState{}, why
		}
		state.lock = doc
	}
	if why := reconcileVersions(state); why != "" {
		return declaredState{}, why
	}
	if why := reconcileGems(group, state); why != "" {
		return declaredState{}, why
	}
	return state, ""
}

// reconcileVersions requires every present Ruby version declaration to agree.
// The lock's RUBY VERSION row contributes its CORE_VERSION prefix only.
func reconcileVersions(state declaredState) string {
	agreed := ""
	for _, declared := range versionClaims(state) {
		if declared == "" {
			continue
		}
		if agreed != "" && agreed != declared {
			return "CONFLICTING_VALUE"
		}
		agreed = declared
	}
	return ""
}

func versionClaims(state declaredState) [3]string {
	claims := [3]string{state.version, "", ""}
	if state.gemfile != nil {
		claims[1] = state.gemfile.rubyVersion
	}
	if state.lock != nil {
		claims[2] = state.lock.rubyRuntime
	}
	return claims
}

// reconcileGems binds the lock to its Gemfile exactly: the lock's DEPENDENCIES
// set must equal the Gemfile's declared gem set, and every locked dependency
// edge must resolve to a spec row at that exact version.
func reconcileGems(group routed, state declaredState) string {
	if state.lock == nil {
		return ""
	}
	if group.gemfile == nil {
		return "EXACT_BINDING_UNAVAILABLE"
	}
	resolved := make(map[string]string, len(state.lock.specs))
	for _, spec := range state.lock.specs {
		resolved[spec.name] = spec.version
	}
	declared := make(map[string]string, len(state.gemfile.gems))
	for _, gem := range state.gemfile.gems {
		declared[gem.name] = gem.version
	}
	if len(declared) != len(state.lock.dependencies) {
		return "CONFLICTING_VALUE"
	}
	for _, dependency := range state.lock.dependencies {
		if declared[dependency.name] != dependency.version {
			return "CONFLICTING_VALUE"
		}
		if resolved[dependency.name] != dependency.version {
			return "CONFLICTING_VALUE"
		}
	}
	for _, spec := range state.lock.specs {
		for _, edge := range spec.deps {
			if resolved[edge.name] != edge.version {
				return "EXACT_BINDING_UNAVAILABLE"
			}
		}
	}
	return ""
}

// sourceDigest is `sha256:` plus the digest of the literal rubygems remote. The
// URL itself is validated but never emitted as a fact value.
func sourceDigest() string {
	sum := sha256.Sum256([]byte(lockRemote))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func instanceOf(gem gemDecl) string { return gem.name + "@" + gem.version }

func emitManifestFacts(r Request, group routed, state declaredState, collector *factCollector) string {
	if group.version != nil {
		if why := collector.add(group.version.in, nil, "ruby.version.declaration", "ruby", "declares-version", state.version, r.ScopeID); why != "" {
			return why
		}
	}
	if group.gemfile != nil {
		if why := emitGemfileFacts(r, group, state, collector); why != "" {
			return why
		}
	}
	if group.lock != nil {
		if why := emitLockFacts(r, group, state, collector); why != "" {
			return why
		}
	}
	return ""
}

func emitGemfileFacts(r Request, group routed, state declaredState, collector *factCollector) string {
	if state.gemfile.rubyVersion != "" {
		if why := collector.add(group.gemfile.in, nil, "ruby.version.declaration", "ruby", "declares-version", state.gemfile.rubyVersion, r.ScopeID); why != "" {
			return why
		}
	}
	return collector.add(group.gemfile.in, nil, "ruby.source.identity", "rubygems", "source-digest", collector.digest, r.ScopeID)
}

func emitLockFacts(r Request, group routed, state declaredState, collector *factCollector) string {
	lock, in, related := state.lock, group.lock.in, &group.gemfile.in
	if why := collector.add(in, nil, "ruby.source.identity", "rubygems", "source-digest", collector.digest, r.ScopeID); why != "" {
		return why
	}
	if why := collector.add(in, nil, "ruby.bundler.version.locked", "bundler", "locked-at", lock.bundler, r.ScopeID); why != "" {
		return why
	}
	for _, platform := range lock.platforms {
		if why := collector.add(in, nil, "ruby.platform.locked", "ruby", "locks-platform", platform, r.ScopeID); why != "" {
			return why
		}
	}
	for _, spec := range lock.specs {
		parent := instanceOf(gemDecl{spec.name, spec.version})
		if why := collector.add(in, related, "ruby.gem.locked", spec.name, "locked-at", spec.version, parent); why != "" {
			return why
		}
		for _, edge := range spec.deps {
			if why := collector.add(in, related, "ruby.gem.dependency", parent, "depends-on", instanceOf(edge), parent); why != "" {
				return why
			}
		}
	}
	return ""
}

func emitSourceFacts(r Request, group routed, collector *factCollector) string {
	for _, source := range group.sources {
		observed, why := scanSource(source.in.Path, source.body)
		if why != "" {
			return why
		}
		path := source.in.Path
		if why := collector.add(source.in, nil, "ruby.source", path, "classifies", observed.classification(path), path); why != "" {
			return why
		}
		for _, token := range observedTokens(observed) {
			if why := collector.add(source.in, nil, "ruby.test.static", path, "observes-test-token", token, path); why != "" {
				return why
			}
		}
	}
	return ""
}

// observedTokens returns the closed test-token values in typed order.
func observedTokens(observed sourceObservation) []string {
	tokens := make([]string, 0, 2)
	if observed.cucumber {
		tokens = append(tokens, "cucumber")
	}
	if observed.rspec {
		tokens = append(tokens, "rspec")
	}
	return tokens
}
