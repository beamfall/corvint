package contextindex

import "fmt"

// The fail-closed bound contract of the contextindex analyzers (TSA-V0-460 to TSA-V0-463,
// V1-1720). An analyzer that may call an answer complete -- setup-select's "selected", or an
// impact receipt without a matching unknown -- bounds its work so a crafted repository stays
// linear. Hitting a bound is never a negative answer: the analyzer stops and reports the
// bound's typed outcome, a reason code or unknown code its caller already turns into ALL or a
// counted uncertainty. Every such bound is declared below, so one table-driven test drives each
// one past its limit and asserts that no complete answer comes back.
//
// A result that an exhausted bound or an in-progress recursion marker shaped depends on the path
// that reached it. It is never stored as a permanent answer: a cache keeps only results whose
// computation met neither (TSA-V0-462).
//
// Caps that only shorten a ranked or displayed list (graph hops and pushes, evidence rows, the
// dependency-link walk, the reason list of an ALL result) claim no completeness and are not
// analyzer bounds in this sense. Neither are the symbol-extraction caps, which record an
// extraction note when reached, nor input caps that refuse the whole request with a typed error
// (impact paths, range hunks and target lines).

// analyzerBound is one declared bound. limit is the largest admitted count; it is a field so a
// test can lower a bound whose default is too large to reach cheaply.
type analyzerBound struct {
	name    string
	limit   int
	unit    string
	outcome string
}

// admits reports whether count stays within the bound.
func (bound *analyzerBound) admits(count int) bool { return count <= bound.limit }

// exhausted is the detail of the unresolved outcome for subject, which the bound stopped.
func (bound *analyzerBound) exhausted(subject string) string {
	return fmt.Sprintf("%s past the %s bound of %d %s", subject, bound.name, bound.limit, bound.unit)
}

var (
	// setupClosureBound bounds the files one setup-select closure visits.
	setupClosureBound = &analyzerBound{"setup-closure", 10000, "files", "bound-exceeded"}
	// sharedReexportBound bounds the `export ... from` chain followed to a shared constant.
	sharedReexportBound = &analyzerBound{"shared-reexport", 8, "hops", "shared-constant-key-unresolved"}
	// sharedLookupBound bounds the export-table lookups of one shared-constant resolution, so an
	// `export *` graph with cycles, whose partial results are not cached, stays linear.
	sharedLookupBound = &analyzerBound{"shared-lookup", 4096, "lookups", "shared-constant-key-unresolved"}
	// sharedTemplateBound bounds template substitutions nested inside substitutions.
	sharedTemplateBound = &analyzerBound{"shared-template", 8, "nested substitutions", "shared-constant-key-unresolved"}
	// webConfigBytesBound bounds one tsconfig/jsconfig/package.json read.
	webConfigBytesBound = &analyzerBound{"web-config-bytes", 1 << 20, "bytes", "import-unresolved"}
	// webConfigDepthBound bounds one tsconfig extends chain.
	webConfigDepthBound = &analyzerBound{"web-config-depth", 16, "links", "import-unresolved"}
	// webConfigVisitBound bounds the configs one leaf's extends graph reads, so array extends
	// sharing ancestors cannot grow the walk exponentially.
	webConfigVisitBound = &analyzerBound{"web-config-visits", 64, "configs", "import-unresolved"}
	// registryManifestBound bounds one application-map manifest read for registries.
	registryManifestBound = &analyzerBound{"registry-manifest", 256 << 10, "bytes", "registry-unresolved"}
	// registryDeclarationBound bounds the registries one manifest declares.
	registryDeclarationBound = &analyzerBound{"registry-declarations", 64, "entries", "registry-unresolved"}
	// registryExcludedJSONBound bounds the Git read of one `.json` source the index excluded for
	// size, which is checked for a registries-declaring manifest it could not otherwise see.
	registryExcludedJSONBound = &analyzerBound{"registry-excluded-json", 4 << 20, "bytes", "registry-unresolved"}
	// registryAccessorBound bounds the accessors one registry declares.
	registryAccessorBound = &analyzerBound{"registry-accessors", 16, "names", "registry-unresolved"}
	// registryKeyBound bounds one literal registry key. A registry entry past it is unreadable
	// (registry-unresolved); a call-site key past it is read as a computed lookup, also ALL.
	registryKeyBound = &analyzerBound{"registry-key", 256, "bytes", "registry-unresolved"}
	// rubyNestingBound bounds the blocks a Ruby source may nest.
	rubyNestingBound = &analyzerBound{"ruby-nesting", 256, "blocks", "ruby-reference-index-bounded"}
	// rubyResolutionBound bounds the namespaces an unbalanced Ruby file resolves a reference against.
	rubyResolutionBound = &analyzerBound{"ruby-resolutions", 64, "namespaces", "ruby-reference-index-bounded"}
	// rubySiteBound bounds the reference sites the Ruby autoload index retains.
	rubySiteBound = &analyzerBound{"ruby-reference-sites", 1 << 20, "sites", "ruby-reference-index-bounded"}
	// rubyManifestBound bounds one Gemfile, Gemfile.lock or Rakefile read for the Ruby
	// verification command; past it the command is withheld as unknown.
	rubyManifestBound = &analyzerBound{"ruby-manifest", 256 << 10, "bytes", "unknown-language-verification"}
	// sourceBytesBound records the index's per-source size admission (maxSourceBytes): a larger
	// source is excluded and every analyzer reads it as unreadable. It is not lowerable.
	sourceBytesBound = &analyzerBound{"source-bytes", maxSourceBytes, "bytes", "source-unreadable"}
)

// sourceSizeExclusion is the Exclusion reason admittedEntries (index.go) gives a source past
// maxSourceBytes; TestOversizedRegistryManifestFailsClosed fails if the two drift.
const sourceSizeExclusion = "source exceeds size bound"

// analyzerBounds is every declared bound, in a fixed order.
var analyzerBounds = []*analyzerBound{
	setupClosureBound, sharedReexportBound, sharedLookupBound, sharedTemplateBound,
	webConfigBytesBound, webConfigDepthBound, webConfigVisitBound,
	registryManifestBound, registryExcludedJSONBound, registryDeclarationBound, registryAccessorBound,
	registryKeyBound,
	rubyNestingBound, rubyResolutionBound, rubySiteBound, rubyManifestBound, sourceBytesBound,
}
