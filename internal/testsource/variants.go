package testsource

import (
	"path"
	"sort"
	"strings"
)

// RunEvidence is one Playwright report reduced to what the cross-variant rule joins (TSA-V0-031):
// the test revision and repository-relative test root the run declares ("" when undeclared) and
// each test occurrence's project, rootDir-relative file, title path and aggregate status.
type RunEvidence struct {
	Revision string
	Root     string
	Tests    []RunTest
}

// RunTest is one test occurrence of one project. Status is Playwright's aggregate (expected,
// unexpected, flaky or skipped); Executed is false when no result ran.
type RunTest struct {
	Project   string
	File      string
	TitlePath []string
	Status    string
	Executed  bool
}

// CrossVariant states whether pass/fail evidence split by project at the analysed revision was
// available (TSA-V0-031, TSA-V0-032). Status is PRODUCED or NOT_PRODUCED with a typed reason.
type CrossVariant struct {
	Status   string         `json:"status"`
	Reason   string         `json:"reason,omitempty"`
	Runs     int            `json:"runs"`
	Projects []string       `json:"projects"`
	Splits   []VariantSplit `json:"splits"`
}

// VariantSplit is one linted test that passes in some projects and fails in others at the
// revision; ExactText records that an exact-text locator was named as a likely cause.
type VariantSplit struct {
	Path      string   `json:"path"`
	TitlePath []string `json:"titlePath"`
	Passed    []string `json:"passed"`
	Failed    []string `json:"failed"`
	ExactText bool     `json:"exactText"`
}

// Cross-variant NOT_PRODUCED reasons (TSA-V0-032).
const (
	ReasonNoRunEvidence       = "no-run-evidence"
	ReasonRevisionNotDeclared = "revision-not-declared"
	ReasonNoRunAtRevision     = "no-run-at-revision"
	ReasonSingleProject       = "single-project"
)

// sameRevision reports a declared run revision naming the analysed full revision.
func sameRevision(declared, revision string) bool {
	return declared != "" && revision != "" && (declared == revision || len(declared) >= 7 && strings.HasPrefix(revision, declared))
}

// joinRuns keeps the runs at the revision, states whether they can show a split, and records the
// split of every linted test they name (TSA-V0-031, TSA-V0-032).
func (c *lintContext) joinRuns(runs []RunEvidence, revision string, linted []string) CrossVariant {
	out := CrossVariant{Status: "NOT_PRODUCED", Projects: []string{}, Splits: []VariantSplit{}}
	if len(runs) == 0 {
		out.Reason = ReasonNoRunEvidence
		return out
	}
	kept, declared := []RunEvidence{}, false
	for _, run := range runs {
		declared = declared || run.Revision != ""
		if sameRevision(run.Revision, revision) {
			kept = append(kept, run)
		}
	}
	out.Runs = len(kept)
	if len(kept) == 0 {
		out.Reason = ReasonNoRunAtRevision
		if !declared {
			out.Reason = ReasonRevisionNotDeclared
		}
		return out
	}
	projects := map[string]bool{}
	type outcome struct{ passed, failed bool }
	byTest := map[string]map[string]*outcome{}
	keys := map[string]VariantSplit{}
	for _, run := range kept {
		for _, t := range run.Tests {
			projects[t.Project] = true
			file, ok := repoPath(run.Root, t.File, linted)
			if !ok {
				continue
			}
			key := variantKey(file, t.TitlePath)
			if byTest[key] == nil {
				byTest[key] = map[string]*outcome{}
				keys[key] = VariantSplit{Path: file, TitlePath: append([]string{}, t.TitlePath...)}
			}
			o := byTest[key][t.Project]
			if o == nil {
				o = &outcome{}
				byTest[key][t.Project] = o
			}
			switch {
			case t.Status == "unexpected":
				o.failed = true
			case t.Status == "expected" && t.Executed:
				o.passed = true
			}
		}
	}
	for name := range projects {
		out.Projects = append(out.Projects, name)
	}
	sort.Strings(out.Projects)
	if len(out.Projects) < 2 {
		out.Reason = ReasonSingleProject
		return out
	}
	out.Status = "PRODUCED"
	for key, perProject := range byTest {
		split := keys[key]
		split.Passed, split.Failed = []string{}, []string{}
		for project, o := range perProject {
			switch {
			case o.failed:
				split.Failed = append(split.Failed, project)
			case o.passed:
				split.Passed = append(split.Passed, project)
			}
		}
		if len(split.Passed) > 0 && len(split.Failed) > 0 {
			sort.Strings(split.Passed)
			sort.Strings(split.Failed)
			c.splits[key] = split
		}
	}
	return out
}

// repoPath maps a report's rootDir-relative file to a linted repository path: joined with the
// declared test root, or, when the root is undeclared, the one linted path it is a suffix of.
func repoPath(root, file string, linted []string) (string, bool) {
	if root != "" {
		joined := path.Join(root, file)
		for _, name := range linted {
			if name == joined {
				return name, true
			}
		}
		return "", false
	}
	found := ""
	for _, name := range linted {
		if name == file || strings.HasSuffix(name, "/"+file) {
			if found != "" {
				return "", false
			}
			found = name
		}
	}
	return found, found != ""
}

// collectSplits lists the recorded splits in path and title order once findings are applied.
func (c *lintContext) collectSplits(cv *CrossVariant) {
	for _, split := range c.splits {
		cv.Splits = append(cv.Splits, split)
	}
	sort.Slice(cv.Splits, func(a, b int) bool {
		return variantKey(cv.Splits[a].Path, cv.Splits[a].TitlePath) < variantKey(cv.Splits[b].Path, cv.Splits[b].TitlePath)
	})
}
