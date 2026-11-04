// SPDX-License-Identifier: Apache-2.0
package main

// AFP-V0-105 (V1-4234): `ci-reuse-plan routes` reports whether each CI fast route still fires. It
// reads retained workflow-runs listings and each run's latest jobs listing (with steps), never a
// log or an artifact, and decides eligibility and hits from job and step conclusions alone.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const routesProfile = "corvint-ci-route-health/0"

// step is one entry of a job's steps in a jobs listing.
type step struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// stepJob is a jobs-listing entry with its steps.
type stepJob struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Steps      []step `json:"steps"`
}

// signal names a job (a trailing * matches every job with that prefix, as a matrix job's
// entries) and optionally one of its steps, and the conclusion that signals. With All, every
// matching job must signal; otherwise one is enough.
type signal struct {
	Job, Step, Conclusion string
	All                   bool
}

// fastRoute is one CI fast route: the events it serves, the signal that the deciding step ran
// (the run is eligible), and the signal that the route fired (a hit).
type fastRoute struct {
	Name     string
	Events   []string
	Eligible signal
	Hit      signal
}

// fastRoutes are ci.yml's fast routes. A renamed job or step makes its route SIGNAL_MISSING.
var fastRoutes = []fastRoute{
	{Name: "DOCS", Events: []string{"pull_request"},
		Eligible: signal{Job: "docs-plan", Step: "Classify and verify immutable documentation inputs", Conclusion: "success"},
		Hit:      signal{Job: "docs-plan", Step: "Plan affected-first package order from trusted base code (advisory)", Conclusion: "skipped"}},
	{Name: "REUSE", Events: []string{"push", "merge_group"},
		Eligible: signal{Job: "docs-plan", Step: "Find a passed result for this exact tree", Conclusion: "success"},
		Hit:      signal{Job: "go-product-shard*", Conclusion: "skipped", All: true}},
	{Name: "NARROWED", Events: []string{"pull_request"},
		Eligible: signal{Job: "narrowing-verdict", Conclusion: "success"},
		Hit:      signal{Job: "ci-narrowed-tree-record", Conclusion: "success"}},
	{Name: "INTEGRITY_REUSE", Events: []string{"merge_group"},
		Eligible: signal{Job: "artifact-integrity", Step: "Find a passed pull-request verdict for this tree", Conclusion: "success"},
		Hit:      signal{Job: "artifact-integrity", Step: "Verify native Go archives", Conclusion: "skipped"}},
	{Name: "DOC_GATES_REUSE", Events: []string{"push", "merge_group"},
		Eligible: signal{Job: "doc-gates-reuse", Step: "Find a passed doc-gates verdict for this tree", Conclusion: "success"},
		Hit:      signal{Job: "doc-gates", Step: "Shared documentation gates (including host versions and use-case pins)", Conclusion: "skipped"}},
}

// observe returns whether s's job and step appear in jobs (present) and whether s signals.
func (s signal) observe(jobs []stepJob) (present, signals bool) {
	matched := 0
	for _, j := range jobs {
		if prefix, ok := strings.CutSuffix(s.Job, "*"); !(ok && strings.HasPrefix(j.Name, prefix)) && j.Name != s.Job {
			continue
		}
		got := ""
		if s.Step == "" {
			present, got = true, j.Conclusion
		} else {
			for _, st := range j.Steps {
				if st.Name == s.Step {
					present, got = true, st.Conclusion
				}
			}
			// A step also skips when an earlier step of its job failed; only a passed job's
			// step conclusion signals.
			if j.Conclusion != "success" {
				got = ""
			}
		}
		if got == s.Conclusion {
			matched++
			if !s.All {
				return present, true
			}
		} else if s.All {
			return present, false
		}
	}
	return present, s.All && matched > 0
}

type routeReport struct {
	Name     string `json:"name"`
	Eligible int    `json:"eligible"`
	Hits     int    `json:"hits"`
	Status   string `json:"status"`
	// LastHitRunID is the newest hit in the window, or null.
	LastHitRunID *int64 `json:"lastHitRunId"`
	// Unread counts runs of the route's events whose jobs listing was unreadable.
	Unread int `json:"unread"`
}

type routesReport struct {
	Profile     string        `json:"profile"`
	Window      int           `json:"window"`
	MinEligible int           `json:"minEligible"`
	Routes      []routeReport `json:"routes"`
	OK          bool          `json:"ok"`
}

// RouteHealth counts, for each route, its eligible runs among the newest completed runs of
// this repository's CI workflow (newest id first) up to window, and its hits among them.
// Eligible runs below minEligible are NOT_OBSERVED; no hit among at least minEligible is DEAD;
// a hit signal absent from every eligible run, or an eligible signal absent from every readable
// run of the route's events, is SIGNAL_MISSING. Only HEALTHY and NOT_OBSERVED leave OK true.
// A run id listed more than once (overlapping --runs listings) counts once, as its first listing.
func RouteHealth(repository string, runs []run, jobs func(int64) ([]stepJob, bool), window, minEligible int) routesReport {
	sorted := make([]run, 0, len(runs))
	seen := make(map[int64]bool, len(runs))
	for _, r := range runs {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		sorted = append(sorted, r)
	}
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID > sorted[j].ID })
	rep := routesReport{Profile: routesProfile, Window: window, MinEligible: minEligible, Routes: []routeReport{}, OK: true}
	for _, rt := range fastRoutes {
		rr := routeReport{Name: rt.Name}
		eligibleSeen, hitSeen := false, false
		for _, r := range sorted {
			if rr.Eligible >= window {
				break
			}
			if r.ID <= 0 || r.Status != "completed" || r.Path != workflowPath || r.Repository.FullName != repository || !contains(rt.Events, r.Event) {
				continue
			}
			listed, ok := jobs(r.ID)
			if !ok {
				rr.Unread++
				continue
			}
			present, eligible := rt.Eligible.observe(listed)
			eligibleSeen = eligibleSeen || present
			if !eligible {
				continue
			}
			rr.Eligible++
			hp, hit := rt.Hit.observe(listed)
			hitSeen = hitSeen || hp
			if hit {
				if rr.Hits == 0 {
					id := r.ID
					rr.LastHitRunID = &id
				}
				rr.Hits++
			}
		}
		switch {
		case rr.Eligible > 0 && !hitSeen, rr.Eligible == 0 && !eligibleSeen && rr.Unread == 0 && anyRunOf(sorted, repository, rt.Events):
			rr.Status = "SIGNAL_MISSING"
		case rr.Eligible < minEligible:
			rr.Status = "NOT_OBSERVED"
		case rr.Hits == 0:
			rr.Status = "DEAD"
		default:
			rr.Status = "HEALTHY"
		}
		if rr.Status == "DEAD" || rr.Status == "SIGNAL_MISSING" {
			rep.OK = false
		}
		rep.Routes = append(rep.Routes, rr)
	}
	return rep
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// anyRunOf reports whether runs hold a completed run of this workflow for one of events.
func anyRunOf(runs []run, repository string, events []string) bool {
	for _, r := range runs {
		if r.ID > 0 && r.Status == "completed" && r.Path == workflowPath && r.Repository.FullName == repository && contains(events, r.Event) {
			return true
		}
	}
	return false
}

// routesMain is `ci-reuse-plan routes`: it prints the AFP-V0-105 report and exits 1 when a
// route is DEAD or SIGNAL_MISSING, 2 on a usage error.
func routesMain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("routes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repository := fs.String("repository", "", "OWNER/NAME of this repository")
	var runsPaths pathList
	fs.Var(&runsPaths, "runs", "retained workflow-runs API response of .github/workflows/ci.yml (repeatable)")
	jobsDir := fs.String("jobs", "", "directory of retained RUN_ID.json latest run-jobs API responses, with steps")
	window := fs.Int("window", 30, "newest eligible runs counted per route")
	minEligible := fs.Int("min-eligible", 10, "eligible runs below which a route is NOT_OBSERVED rather than DEAD")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: ci-reuse-plan routes --repository OWNER/NAME --runs FILE [--runs FILE] --jobs DIR [--window N] [--min-eligible N]")
		fmt.Fprintln(stderr, "Reports each CI fast route's eligible runs and hits (AFP-V0-105); exits 1 on a DEAD or SIGNAL_MISSING route.")
		fmt.Fprintln(stderr, "example: ci-reuse-plan routes --repository beamfall/corvint --runs runs.json --jobs jobs --window 30 --min-eligible 10")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || *repository == "" || len(runsPaths) == 0 || *jobsDir == "" || *window < 1 || *minEligible < 1 || *minEligible > *window {
		fs.Usage()
		return 2
	}
	runs, ok := readRuns(runsPaths)
	if !ok {
		fmt.Fprintln(stderr, "ci-reuse-plan routes: no workflow-runs listing was readable")
		return 2
	}
	rep := RouteHealth(*repository, runs, func(id int64) ([]stepJob, bool) {
		var page struct {
			Jobs []stepJob `json:"jobs"`
		}
		ok := readBounded(filepath.Join(*jobsDir, strconv.FormatInt(id, 10)+".json"), &page)
		return page.Jobs, ok
	}, *window, *minEligible)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		return 1
	}
	if !rep.OK {
		return 1
	}
	return 0
}
