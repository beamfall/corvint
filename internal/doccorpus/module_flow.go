package doccorpus

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ProductionScope is a manifest's optional declared production-source scope (DCP-V1-720): the
// module paths whose ownership by a flow is assessed. Each pattern is "[REPOSITORY:]GLOB", the
// repository a declared alias or root-commit identity and, absent, the primary one; a glob's
// segments are path.Match patterns or a whole `**`. A module is in scope when an include pattern
// matches it and no exclude pattern does.
type ProductionScope struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude,omitempty"`
}

// OwnershipGap is a module-without-flow or module-claims-without-flow gap (DCP-V1-721,
// DCP-V1-722). Claims lists the citing claim IDs, at most maxOwnershipClaims of them, and
// ClaimsOmitted counts the rest.
type OwnershipGap struct {
	Subject       string   `json:"subject"`
	Kind          string   `json:"kind"`
	Reason        string   `json:"reason"`
	Claims        []string `json:"claims,omitempty"`
	ClaimsOmitted int      `json:"claims_omitted,omitempty"`
}

const (
	maxScopePatterns   = 256
	maxOwnershipClaims = 32
	// ModuleWithoutFlow and ModuleClaimsWithoutFlow are the ownership gap kinds.
	ModuleWithoutFlow       = "module-without-flow"
	ModuleClaimsWithoutFlow = "module-claims-without-flow"
)

const productionScopeUndeclared = "module-to-flow ownership NOT_ASSESSED: the manifest declares no production_scope, so no module is counted"

// validateProductionScope admits 1 to 256 sorted, distinct include patterns and at most 256 sorted,
// distinct exclude patterns, each naming a declared repository, if any, and a valid glob.
func validateProductionScope(m Manifest) error {
	scope := m.ProductionScope
	if scope == nil {
		return nil
	}
	for _, list := range [][]string{scope.Include, scope.Exclude} {
		if len(list) > maxScopePatterns || !sort.StringsAreSorted(list) {
			return fail("invalid production scope: more than 256 or unsorted patterns")
		}
		for i, pattern := range list {
			_, glob, ok := m.scopePattern(pattern)
			if !ok || i > 0 && list[i-1] == pattern || !extractorFilePattern(glob) {
				return fail("invalid production scope pattern " + pattern)
			}
		}
	}
	if len(scope.Include) == 0 || scope.Exclude != nil && len(scope.Exclude) == 0 {
		return fail("invalid production scope: include is required and exclude, when present, is not empty")
	}
	return nil
}

// scopePattern splits a pattern into the root-commit identity of the repository it names and its
// glob. A pattern without ':' names the primary repository; one whose prefix before the first ':'
// names no declared repository is not admitted, so a mistyped qualifier never matches nothing
// silently.
func (m Manifest) scopePattern(pattern string) (string, string, bool) {
	prefix, glob, qualified := strings.Cut(pattern, ":")
	if !qualified {
		return m.Repository.ID, pattern, true
	}
	id := m.repositoryOfAlias(prefix)
	return id, glob, m.declaresRepository(id)
}

func (m Manifest) scopeMatches(patterns []string, repository, p string) bool {
	for _, pattern := range patterns {
		if id, glob, ok := m.scopePattern(pattern); ok && id == repository && extractorMatch(glob, p) {
			return true
		}
	}
	return false
}

// inProductionScope reports whether the module at repository and path is in the declared scope.
func (m Manifest) inProductionScope(repository, p string) bool {
	return m.scopeMatches(m.ProductionScope.Include, repository, p) && !m.scopeMatches(m.ProductionScope.Exclude, repository, p)
}

// productionScopeDigest identifies the declared scope a count was made under.
func productionScopeDigest(scope *ProductionScope) string {
	digest, err := hashValue(scope)
	if err != nil {
		return ""
	}
	return "sha256:" + digest
}

func repositoryPathKey(repository, p string) string { return repository + "\x00" + p }

// moduleOwnership assesses every module subject against the declared production scope. A module
// is owned when a flow or variation subject anchors its path, a joined flow-relation record's
// source anchors it, or a declared relation joins a flow or variation subject to the module or to
// a subject anchored at its path (DCP-V1-721). It returns the ownership gaps and the counts.
func moduleOwnership(ctx context.Context, a *Artifact) (gaps []OwnershipGap, owned, unowned, outOfScope int, err error) {
	m := a.Manifest
	flows, ownedPaths, ownedIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	subjects := map[string]Subject{}
	for _, s := range a.Subjects {
		subjects[s.ID] = s
		if s.Kind == "flow" || s.Kind == "variation" {
			flows[s.ID] = true
			for _, anchor := range s.Evidence.Anchors {
				ownedPaths[repositoryPathKey(anchor.Repository, anchor.Path)] = true
			}
		}
	}
	for _, j := range a.FlowRelations {
		if j.Status == "joined" && j.Record.Source != nil {
			ownedPaths[repositoryPathKey(j.Record.Source.Repository, j.Record.Source.Path)] = true
		}
	}
	for _, r := range a.Relations {
		for _, end := range [][2]string{{r.From, r.To}, {r.To, r.From}} {
			if !flows[end[0]] || flows[end[1]] {
				continue
			}
			ownedIDs[end[1]] = true
			for _, anchor := range subjects[end[1]].Evidence.Anchors {
				ownedPaths[repositoryPathKey(anchor.Repository, anchor.Path)] = true
			}
		}
	}
	// A claim cites a module when a non-native provider supplied it about the module or anchored
	// it at the module's path.
	native := nativeProviders(a)
	citing := map[string]map[string]bool{}
	cite := func(key, claim string) {
		if citing[key] == nil {
			citing[key] = map[string]bool{}
		}
		citing[key][claim] = true
	}
	for _, c := range a.Claims {
		if native[c.Provider] {
			continue
		}
		cite("id\x00"+c.Subject, c.ID)
		for _, anchor := range c.Evidence.Anchors {
			cite(repositoryPathKey(anchor.Repository, anchor.Path), c.ID)
		}
	}
	digest := productionScopeDigest(m.ProductionScope)
	for _, s := range a.Subjects {
		if err := ctx.Err(); err != nil {
			return nil, 0, 0, 0, err
		}
		if s.Kind != "module" {
			continue
		}
		if len(s.Evidence.Anchors) == 0 {
			outOfScope++
			continue
		}
		anchor := s.Evidence.Anchors[0]
		if !m.inProductionScope(anchor.Repository, anchor.Path) {
			outOfScope++
			continue
		}
		key := repositoryPathKey(anchor.Repository, anchor.Path)
		if ownedIDs[s.ID] || ownedPaths[key] {
			owned++
			continue
		}
		unowned++
		where := m.repositoryLabel(anchor.Revision) + ":" + anchor.Path
		gaps = append(gaps, OwnershipGap{Subject: s.ID, Kind: ModuleWithoutFlow, Reason: fmt.Sprintf("no flow or variation reaches production module %s through a declared relation or anchor (production scope %s)", where, digest)})
		claims := map[string]bool{}
		for id := range citing["id\x00"+s.ID] {
			claims[id] = true
		}
		for id := range citing[key] {
			claims[id] = true
		}
		if len(claims) == 0 {
			continue
		}
		ids := make([]string, 0, len(claims))
		for id := range claims {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		g := OwnershipGap{Subject: s.ID, Kind: ModuleClaimsWithoutFlow, Reason: fmt.Sprintf("%d claims cite production module %s, which no flow or variation reaches (production scope %s)", len(ids), where, digest), Claims: ids}
		if len(ids) > maxOwnershipClaims {
			g.Claims, g.ClaimsOmitted = ids[:maxOwnershipClaims], len(ids)-maxOwnershipClaims
		}
		gaps = append(gaps, g)
	}
	return gaps, owned, unowned, outOfScope, nil
}

// hasFlowSubjects reports whether the corpus declares any flow or variation subject.
func hasFlowSubjects(a *Artifact) bool {
	for _, s := range a.Subjects {
		if s.Kind == "flow" || s.Kind == "variation" {
			return true
		}
	}
	return false
}

// moduleOwnershipMetric is the coverage row for module-to-flow ownership (DCP-V1-723): present
// when the manifest declares a production scope, or, as undeclared, when the corpus has flows.
func moduleOwnershipMetric(ctx context.Context, a *Artifact) (map[string]any, error) {
	row := map[string]any{"metric": "module_flow_ownership", "definition": "in-scope module subjects some flow or variation reaches through a declared relation or anchor / all in-scope module subjects", "revision": a.Manifest.Repository.Revision}
	if a.Manifest.ProductionScope == nil {
		if !hasFlowSubjects(a) {
			return nil, nil
		}
		row["scope"], row["defined"], row["limitations"] = "undeclared", false, []string{productionScopeUndeclared}
		return row, nil
	}
	_, owned, unowned, outOfScope, err := moduleOwnership(ctx, a)
	if err != nil {
		return nil, err
	}
	row["scope"], row["scope_digest"] = "declared", productionScopeDigest(a.Manifest.ProductionScope)
	row["value"], row["denominator"], row["defined"] = owned, owned+unowned, owned+unowned > 0
	row["owned"], row["unowned"], row["out_of_scope"] = owned, unowned, outOfScope
	row["limitations"] = []string{"ownership is declared reachability only; a reached module's behaviour and test adequacy are unknown"}
	return row, nil
}
