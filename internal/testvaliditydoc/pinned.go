package testvaliditydoc

import (
	"sort"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// ProjectPinned recomputes native observations and binds their original source
// digest keys through the caller's explicit immutable source lookup. It grants
// no adequacy or run-authenticity claim, and an opaque Go session identity stays
// unverifiable, matching retained-evidence discovery.
func ProjectPinned(input Input, lookup func(string) ([]byte, bool)) Document {
	document := Project(input)
	freshness := pinnedFreshness(input, lookup)
	document.Run.Freshness = freshness
	for i := range document.Tests {
		document.Tests[i].Projection.Freshness = freshness
	}
	return document
}
func pinnedFreshness(input Input, lookup func(string) ([]byte, bool)) testvalidity.Axis {
	unknown := func(reason string) testvalidity.Axis {
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: reason}
	}
	if input.goSession != nil {
		if input.goSession.State == "stale" {
			return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: "workspace-execution-identity-mismatch"}
		}
		return unknown("retained-session-identity-unverifiable")
	}
	if input.js == nil {
		return unknown("retained-identity-unbound")
	}
	if input.js.StaleAppBuild {
		return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: "retained-digest-mismatch"}
	}
	identity := input.js.Identity
	bound := map[string]string{}
	for p, d := range identity.TestFileDigests {
		bound[p] = d
	}
	if identity.ConfigFile != "" {
		bound[identity.ConfigFile] = identity.ConfigDigest
	}
	if len(bound) == 0 {
		return unknown("retained-identity-unbound")
	}
	paths := []string{}
	for p := range bound {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		data, ok := lookup(p)
		if !ok {
			return unknown("retained-bound-source-unreadable")
		}
		if digestHex(data) != bound[p] {
			return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: "retained-digest-mismatch"}
		}
	}
	if identity.PackageDigest != "" && identity.PackageDigest != emptyPackageDigest {
		return unknown("retained-package-identity-unverifiable")
	}
	if input.js.Kind == "e2e" || input.js.AppBuildAtPublish.Digest != "" {
		return unknown("retained-app-build-identity-unverifiable")
	}
	return testvalidity.Axis{State: testvalidity.FreshnessCurrent}
}
