package testvaliditymodel

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
	if input.GoSession != nil {
		if input.GoSession.State == "stale" {
			return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: "workspace-execution-identity-mismatch"}
		}
		return unknown("retained-session-identity-unverifiable")
	}
	if input.JS == nil {
		return unknown("retained-identity-unbound")
	}
	if input.JS.StaleAppBuild {
		return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: "retained-digest-mismatch"}
	}
	identity := input.JS.Identity
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
		if DigestHex(data) != bound[p] {
			return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: "retained-digest-mismatch"}
		}
	}
	if identity.PackageDigest != "" && identity.PackageDigest != EmptyPackageDigest {
		return unknown("retained-package-identity-unverifiable")
	}
	if input.JS.Kind == "e2e" || input.JS.AppBuildAtPublish.Digest != "" {
		return unknown("retained-app-build-identity-unverifiable")
	}
	return testvalidity.Axis{State: testvalidity.FreshnessCurrent}
}
