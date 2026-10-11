package main

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedMaxAppmapImpacts bounds repeatable --appmap-impact (ANL-V0-031).
const affectedMaxAppmapImpacts = 16

// affectedAppNavigation is one app's navigation verdict routed from its application-map-impact/0
// reports into the affected receipt (ANL-V0-031). An app may be mapped by several maps (one per
// router or root); their reports aggregate into one row (ANL-V0-034). It is learned and advisory:
// it never adds to or removes from advice.checks, the plan or the provider projection. Verdict is
// the aggregated reported verdict unless the receipt can see that the reports do not cover the
// change, in which case it is full-suite-required and Reasons say why; full-suite-required is also
// an advice.unknown line.
type affectedAppNavigation struct {
	App             string   `json:"app"`
	Authority       string   `json:"authority"`
	ImpactDigests   []string `json:"impact_digests"`
	MapDigests      []string `json:"map_digests"`
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
	type appReports struct {
		nav     affectedAppNavigation
		screens map[string]bool
		specs   map[string]bool
		unknown bool
	}
	apps, maps := map[string]*appReports{}, map[string]bool{}
	for _, name := range invocation.AppmapImpacts {
		r, err := appmap.LoadImpactReport(name)
		if err != nil {
			return err
		}
		if r.Base != invocation.Base || r.Head != revision {
			return &gokernel.Error{Code: "appmap-invalid-query", Message: "the impact report for app " + r.App + " was not built for --base " + invocation.Base + " at HEAD " + revision}
		}
		if maps[r.App+"\x00"+r.MapDigest] {
			return &gokernel.Error{Code: "appmap-invalid-query", Message: "two impact reports name app " + r.App + " and map " + r.MapDigest}
		}
		maps[r.App+"\x00"+r.MapDigest] = true
		a := apps[r.App]
		if a == nil {
			a = &appReports{nav: affectedAppNavigation{App: r.App, Authority: appmap.AuthorityLearned, ReportedVerdict: appmap.VerdictNoNavigation}, screens: map[string]bool{}, specs: map[string]bool{}}
			apps[r.App] = a
		}
		a.nav.ImpactDigests = append(a.nav.ImpactDigests, r.Digest)
		a.nav.MapDigests = append(a.nav.MapDigests, r.MapDigest)
		a.unknown = a.unknown || len(r.Unknown) > 0
		switch {
		case r.Verdict == appmap.VerdictFullSuite:
			a.nav.ReportedVerdict = appmap.VerdictFullSuite
		case r.Verdict == appmap.VerdictTargeted && a.nav.ReportedVerdict == appmap.VerdictNoNavigation:
			a.nav.ReportedVerdict = appmap.VerdictTargeted
		}
		for _, sc := range r.Screens {
			a.screens[sc.ID] = true
		}
		for _, f := range r.SpecFiles {
			a.specs[f] = true
		}
	}
	out := []affectedAppNavigation{}
	for _, a := range apps {
		nav := a.nav
		sort.Strings(nav.ImpactDigests)
		sort.Strings(nav.MapDigests)
		nav.Screens, nav.Reasons, nav.Verdict = len(a.screens), []string{}, nav.ReportedVerdict
		nav.SpecFiles = []string{}
		for f := range a.specs {
			nav.SpecFiles = append(nav.SpecFiles, f)
		}
		sort.Strings(nav.SpecFiles)
		if a.unknown {
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
			advice.Unknown = append(advice.Unknown, affectedAppNavigationUnknown+": "+nav.App+": "+strings.Join(nav.Reasons, ","))
		}
		out = append(out, nav)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].App < out[j].App })
	advice.AppNavigation = out
	advice.Unknown = sortedUniqueStrings(advice.Unknown)
	return nil
}
