package main

import (
	"context"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
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
	App             string                         `json:"app"`
	Authority       string                         `json:"authority"`
	Bindings        []affectedAppNavigationBinding `json:"bindings"`
	ImpactDigests   []string                       `json:"impact_digests"`
	MapDigests      []string                       `json:"map_digests"`
	Reasons         []string                       `json:"reasons"`
	ReportedVerdict string                         `json:"reported_verdict"`
	Screens         int                            `json:"screens"`
	SpecFiles       []string                       `json:"spec_files"`
	Verdict         string                         `json:"verdict"`
}

// affectedAppNavigationBinding records which checkout one report was checked against: the
// receipt's own --root range when Repository is empty, else the declared --repository ID whose
// --repository-base is Base and whose checkout HEAD is Head (ANL-V0-060).
type affectedAppNavigationBinding struct {
	Base         string `json:"base"`
	Head         string `json:"head"`
	ImpactDigest string `json:"impact_digest"`
	Repository   string `json:"repository,omitempty"`
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
// (the report's base is --base and its head is the revision) or to a declared --repository
// checkout (ANL-V0-060), and derives the effective verdict: uncommitted paths the report cannot
// have seen, paths it ignored, or a spec list over the bound, fail closed to full-suite-required;
// nothing here narrows a selection (ANL-V0-031, ANL-V0-061). A report whose tests came from a
// pinned test root other than this receipt's HEAD fails closed too (ANL-V0-067). The returned
// recheck reads every bound checkout again and refuses if one moved (ANL-V0-066).
func affectedAppNavigations(ctx context.Context, gitExecutable string, invocation affectedInvocation, revision string, worktree []string, advice *affectedAdvice) (func() error, error) {
	recheck := func() error { return nil }
	if len(invocation.AppmapImpacts) == 0 {
		return recheck, nil
	}
	type appReports struct {
		nav     affectedAppNavigation
		screens map[string]bool
		specs   map[string]bool
		unknown bool
		ignored bool
		dirty   bool
		testRev bool
	}
	apps, maps := map[string]*appReports{}, map[string]bool{}
	checkouts := map[string]affectedBoundCheckout{}
	for _, name := range invocation.AppmapImpacts {
		r, err := appmap.LoadImpactReport(name)
		if err != nil {
			return nil, err
		}
		binding, dirty, err := affectedBindImpactReport(ctx, gitExecutable, invocation, revision, r, checkouts)
		if err != nil {
			return nil, err
		}
		if maps[r.App+"\x00"+r.MapDigest] {
			return nil, &gokernel.Error{Code: "appmap-invalid-query", Message: "two impact reports name app " + r.App + " and map " + r.MapDigest}
		}
		maps[r.App+"\x00"+r.MapDigest] = true
		a := apps[r.App]
		if a == nil {
			a = &appReports{nav: affectedAppNavigation{App: r.App, Authority: appmap.AuthorityLearned, ReportedVerdict: appmap.VerdictNoNavigation}, screens: map[string]bool{}, specs: map[string]bool{}}
			apps[r.App] = a
		}
		a.nav.Bindings = append(a.nav.Bindings, binding)
		a.nav.ImpactDigests = append(a.nav.ImpactDigests, r.Digest)
		a.ignored = a.ignored || r.Ignored > 0
		a.dirty = a.dirty || dirty
		a.testRev = a.testRev || (r.TestRoot != nil && r.TestRoot.Revision != revision)
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
		sort.Slice(nav.Bindings, func(i, j int) bool { return nav.Bindings[i].ImpactDigest < nav.Bindings[j].ImpactDigest })
		nav.Screens, nav.Reasons, nav.Verdict = len(a.screens), []string{}, nav.ReportedVerdict
		nav.SpecFiles = []string{}
		for f := range a.specs {
			nav.SpecFiles = append(nav.SpecFiles, f)
		}
		sort.Strings(nav.SpecFiles)
		if a.unknown {
			nav.Reasons = append(nav.Reasons, "impact-unknown-paths")
		}
		if a.ignored {
			// A path the report counted but did not place is never "no tests affected" (ANL-V0-061).
			nav.Reasons = append(nav.Reasons, "impact-ignored-paths")
		}
		if a.dirty {
			nav.Reasons = append(nav.Reasons, "repository-worktree-not-covered")
		}
		if len(worktree) > 0 {
			nav.Reasons = append(nav.Reasons, "worktree-not-covered")
		}
		if a.testRev {
			nav.Reasons = append(nav.Reasons, "test-root-not-at-head")
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
	recheck = func() error {
		for id, c := range checkouts {
			if c.reason != "" {
				continue
			}
			head, err := appflows.ResolveRevision(ctx, c.dir, "HEAD")
			dirty, statusErr := affected.DirtyPaths(ctx, gitExecutable, c.dir)
			if err != nil || statusErr != nil || head != c.head || !equalStringSlices(dirty, c.paths) {
				return &gokernel.Error{Code: "unsupported-affected-drift", Message: "repository " + id + " changed while the plan was compiled"}
			}
		}
		return nil
	}
	return recheck, nil
}

// affectedBoundCheckout is one declared --repository checkout opened for report binding.
type affectedBoundCheckout struct {
	dir    string
	head   string
	dirty  bool
	paths  []string // the dirty paths observed at binding; the recheck refuses any change
	reason string
}

// affectedBindImpactReport checks one report against each declared --repository whose
// --repository-base is the report's base and whose checkout HEAD is its head, else against the
// receipt's --base and HEAD, and reports whether that checkout has uncommitted paths the report never saw. A
// report bound to neither refuses and names the supported two-repository layout (ANL-V0-060).
func affectedBindImpactReport(ctx context.Context, gitExecutable string, invocation affectedInvocation, revision string, r *appmap.ImpactReport, checkouts map[string]affectedBoundCheckout) (affectedAppNavigationBinding, bool, error) {
	binding := affectedAppNavigationBinding{Base: r.Base, Head: r.Head, ImpactDigest: r.Digest}
	// A declared checkout carrying the report's base is checked first, so its uncommitted paths
	// are never hidden by --root happening to share both commits.
	reasons := []string{}
	for _, checkout := range invocation.Checkouts {
		if invocation.RepositoryBases[checkout.ID] != r.Base {
			continue
		}
		c, seen := checkouts[checkout.ID]
		if !seen {
			var dir string
			dir, c.head, c.reason = appflows.OpenCheckout(ctx, invocation.Root, checkout.Source)
			c.dir = dir
			if c.reason == "" {
				if base, err := appflows.ResolveRevision(ctx, dir, r.Base); err != nil || base != r.Base {
					c.reason = "repository base is not a commit of the checkout"
				} else if dirty, err := affected.DirtyPaths(ctx, gitExecutable, dir); err != nil {
					c.reason = "checkout status unreadable"
				} else if head, err := appflows.ResolveRevision(ctx, dir, "HEAD"); err != nil || head != c.head {
					c.reason = "checkout moved while being read"
				} else {
					c.dirty, c.paths = len(dirty) > 0, dirty
				}
			}
			checkouts[checkout.ID] = c
		}
		switch {
		case c.reason != "":
			reasons = append(reasons, "repository "+checkout.ID+": "+c.reason)
		case c.head != r.Head:
			reasons = append(reasons, "repository "+checkout.ID+" is at HEAD "+c.head)
		default:
			binding.Repository = checkout.ID
			return binding, c.dirty, nil
		}
	}
	if r.Base == invocation.Base && r.Head == revision {
		return binding, false, nil
	}
	message := "the impact report for app " + r.App + " (base " + r.Base + ", head " + r.Head + ") was not built for --base " + invocation.Base + " at HEAD " + revision +
		" nor for a declared checkout; for an application in another repository pass --repository ID=DIR --repository-base ID=" + r.Base + " for its checkout at " + r.Head
	if len(reasons) > 0 {
		message += " (" + strings.Join(reasons, "; ") + ")"
	}
	return affectedAppNavigationBinding{}, false, &gokernel.Error{Code: "appmap-invalid-query", Message: message}
}

// impactUnsupportedLayoutOptions are the two-root and explicit-revision options single-root
// `impact` does not take: --app-root and --test-root select the two-root impact before parsing
// (ANL-V0-064), so on this path each refuses with a typed code naming it (ANL-V0-063).
var impactUnsupportedLayoutOptions = map[string]bool{"--app-root": true, "--test-root": true, "--at": true, "--revision": true}

func impactUnsupportedLayout(name string) string {
	return "argument " + name + ": single-root impact reads one --root at its HEAD; for an application repository and a separate test repository, " +
		"run `corvint impact --app-root DIR --test-root ALIAS=ABSOLUTE_ROOT --manifest FILE --base COMMIT [--at REV] [--test-at REV]`, " +
		"or join a `corvint flows appmap impact` report in the test repository with " +
		"`corvint affected --base FULL_COMMIT --appmap-impact REPORT --repository ID=DIR --repository-base ID=FULL_COMMIT`"
}

// parseImpactArgumentsWithLayout refuses an unsupported layout option before any other impact
// argument is read, then parses the arguments as before (ANL-V0-063).
func parseImpactArgumentsWithLayout(result options, arguments []string, platform string) (options, error) {
	if nativePlatformQualified(platform) {
		for _, argument := range arguments {
			if argument == "--" {
				break
			}
			if name, _, _ := strings.Cut(argument, "="); impactUnsupportedLayoutOptions[name] {
				return result, &gokernel.Error{Code: "unsupported-impact-layout", Message: impactUnsupportedLayout(name)}
			}
		}
	}
	return parseImpactArgumentsForPlatform(result, arguments, platform)
}
