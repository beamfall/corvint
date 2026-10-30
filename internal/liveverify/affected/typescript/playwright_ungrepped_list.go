package typescript

import (
	"slices"
	"strings"
)

// playwrightCaseKey identifies one listed execution across two listings of the same sources: its
// project, repository-relative file, location and title path.
type playwrightCaseKey struct {
	project, path, titles string
	line, column          int
}

// playwrightCaseListing is the per-project case multiset of one validated listing.
type playwrightCaseListing struct {
	counts map[playwrightCaseKey]int
	cases  map[playwrightCaseKey]playwrightListCase
	pairs  map[PlaywrightDiscoveryUnit]bool
}

// newPlaywrightCaseListing returns an empty case multiset that a walker visitor fills with
// add.
func newPlaywrightCaseListing() *playwrightCaseListing {
	return &playwrightCaseListing{counts: map[playwrightCaseKey]int{}, cases: map[playwrightCaseKey]playwrightListCase{}, pairs: map[PlaywrightDiscoveryUnit]bool{}}
}

// add records each execution of a listed spec, one per project.
func (listing *playwrightCaseListing) add(listed playwrightListedSpec) error {
	projects, err := playwrightSpecProjects(listed)
	if err != nil {
		return err
	}
	for _, project := range projects {
		key := playwrightCaseKey{
			project: project, path: listed.path, titles: strings.Join(listed.listCase.titles, "\x00"),
			line: listed.spec.Line, column: listed.spec.Column,
		}
		listing.counts[key]++
		listing.cases[key] = listed.listCase
		listing.pairs[PlaywrightDiscoveryUnit{Project: project, Test: listed.path}] = true
	}
	return nil
}

// playwrightUngreppedCheck holds a discovery listing and the ungrepped listing that accompanies it
// (TJAA-V0-118): the same `--list --reporter=json` report taken with every project grep and
// grepInvert removed from the config.
type playwrightUngreppedCheck struct {
	grepped, ungrepped *playwrightCaseListing
}

// verifiedProjects returns the projects whose grep omissions the ungrepped listing verifies
// (TJAA-V0-119). It verifies none unless the two listings agree outside grep: every execution of
// the discovery listing is in the ungrepped listing, every project without grep or grepInvert
// lists exactly the same executions in both, and no execution key repeats. A project with grep or
// grepInvert is verified only when both patterns are statically evaluable part by part and, for
// every execution the ungrepped listing places in it, the case is inside the matcher model and the
// configured patterns, applied to the project name, the title path and the reported explicit tags,
// keep it exactly when the discovery listing lists it. Any disagreement leaves that project's
// omissions to the existing rules (TJAA-V0-021), so they stay unverified or are refused.
func (check playwrightUngreppedCheck) verifiedProjects(byName map[string]PlaywrightProject) map[string]bool {
	verified := map[string]bool{}
	for key, count := range check.grepped.counts {
		if count != 1 || check.ungrepped.counts[key] != 1 {
			return verified
		}
	}
	for key, count := range check.ungrepped.counts {
		if count != 1 {
			return verified
		}
		if project := byName[key.project]; !project.hasGrep && !project.hasGrepInvert && check.grepped.counts[key] != 1 {
			return verified
		}
	}
	for name, project := range byName {
		if project.hasGrep || project.hasGrepInvert {
			verified[name] = true
		}
	}
	for key, listCase := range check.ungrepped.cases {
		if verified[key.project] && !playwrightUngreppedCaseAgrees(byName[key.project], listCase, check.grepped.counts[key] == 1) {
			delete(verified, key.project)
		}
	}
	return verified
}

// verifies reports whether the ungrepped listing verifies that the project's grep or grepInvert
// is why the discovery listing omits unit: the ungrepped listing places at least one case of the
// file in the project and the project is verified.
func (check playwrightUngreppedCheck) verifies(verified map[string]bool, unit PlaywrightDiscoveryUnit) bool {
	return verified[unit.Project] && check.ungrepped.pairs[unit] && !check.grepped.pairs[unit]
}

// playwrightUngreppedCaseAgrees reports whether a case of the ungrepped listing is inside the
// matcher model and the project's static grep and grepInvert keep it exactly when kept is true.
func playwrightUngreppedCaseAgrees(project PlaywrightProject, listCase playwrightListCase, kept bool) bool {
	grep, grepOK := playwrightTitleMatchers(project.hasGrep, project.Grep)
	invert, invertOK := playwrightTitleMatchers(project.hasGrepInvert, project.GrepInvert)
	if !grepOK || !invertOK || len(listCase.titles) == 0 {
		return false
	}
	fold := playwrightTitleMatchersFold(grep, invert)
	parts := append([]string{project.Name}, listCase.titles...)
	if slices.ContainsFunc(append(slices.Clone(parts), listCase.tags...), func(part string) bool {
		return playwrightOutsideBMP(part) || fold && strings.ContainsAny(part, "ſK")
	}) {
		return false
	}
	explicit, ok := playwrightExplicitTags(listCase.tags, playwrightTitleTags(strings.Join(listCase.titles[1:], " ")))
	if !ok {
		return false
	}
	tokens := append(parts, explicit...)
	predicted := !(project.hasGrepInvert && playwrightTitleMatch(invert, tokens)) && (!project.hasGrep || playwrightTitleMatch(grep, tokens))
	return predicted == kept
}
