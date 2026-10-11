package main

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedMaxAppmapImpacts bounds repeatable --appmap-impact (ANL-V0-031).
const affectedMaxAppmapImpacts = 16

// affectedAppNavigation is one app's navigation verdict routed from an application-map-impact/0
// report into the affected receipt (ANL-V0-031). It is learned and advisory: it never adds to or
// removes from advice.checks, the plan or the provider projection. Verdict is the reported
// verdict unless the receipt can see that the report does not cover the change, in which case it
// is full-suite-required and Reasons say why; full-suite-required is also an advice.unknown line.
type affectedAppNavigation struct {
	App             string   `json:"app"`
	Authority       string   `json:"authority"`
	ImpactDigest    string   `json:"impact_digest"`
	MapDigest       string   `json:"map_digest"`
	Reasons         []string `json:"reasons"`
	ReportedVerdict string   `json:"reported_verdict"`
	Screens         int      `json:"screens"`
	SpecFiles       []string `json:"spec_files"`
	Verdict         string   `json:"verdict"`
}

// affectedAppNavigationUnknown prefixes the advice.unknown line of a full-suite-required app.
const affectedAppNavigationUnknown = "APP_NAVIGATION_FULL_SUITE_REQUIRED"

func addAffectedAppmapImpact(invocation *affectedInvocation, value string) error {
	if value == "" {
		return argumentError("--appmap-impact requires exactly one nonempty value")
	}
	if len(invocation.AppmapImpacts) == affectedMaxAppmapImpacts {
		return argumentError("--appmap-impact may be given at most 16 times")
	}
	invocation.AppmapImpacts = append(invocation.AppmapImpacts, value)
	return nil
}

// affectedAppNavigations loads every --appmap-impact report, binds it to this receipt's range
// (the report's base is --base and its head is the revision) and derives the effective verdict:
// uncommitted paths the report cannot have seen, or a spec list over the bound, fail closed to
// full-suite-required; nothing here narrows a selection (ANL-V0-031).
func affectedAppNavigations(invocation affectedInvocation, revision string, worktree []string, advice *affectedAdvice) error {
	if len(invocation.AppmapImpacts) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := []affectedAppNavigation{}
	for _, name := range invocation.AppmapImpacts {
		r, err := appmap.LoadImpactReport(name)
		if err != nil {
			return err
		}
		if r.Base != invocation.Base || r.Head != revision {
			return &gokernel.Error{Code: "appmap-invalid-query", Message: "the impact report for app " + r.App + " was not built for --base " + invocation.Base + " at HEAD " + revision}
		}
		if seen[r.App] {
			return &gokernel.Error{Code: "appmap-invalid-query", Message: "two impact reports name app " + r.App}
		}
		seen[r.App] = true
		nav := affectedAppNavigation{App: r.App, Authority: r.Authority, ImpactDigest: r.Digest, MapDigest: r.MapDigest,
			Reasons: []string{}, ReportedVerdict: r.Verdict, Screens: len(r.Screens), SpecFiles: append([]string{}, r.SpecFiles...), Verdict: r.Verdict}
		if len(r.Unknown) > 0 {
			nav.Reasons = append(nav.Reasons, "impact-unknown-paths")
		}
		if len(worktree) > 0 {
			nav.Reasons = append(nav.Reasons, "worktree-not-covered")
		}
		if len(nav.SpecFiles) > affectedSelectionMaxItems {
			nav.SpecFiles = nav.SpecFiles[:affectedSelectionMaxItems]
			nav.Reasons = append(nav.Reasons, "spec-files-over-bound")
		}
		if len(nav.Reasons) > 0 {
			nav.Verdict = appmap.VerdictFullSuite
			advice.Unknown = append(advice.Unknown, affectedAppNavigationUnknown+": "+r.App+": "+strings.Join(nav.Reasons, ","))
		}
		out = append(out, nav)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].App < out[j].App })
	advice.AppNavigation = out
	advice.Unknown = sortedUniqueStrings(advice.Unknown)
	return nil
}
