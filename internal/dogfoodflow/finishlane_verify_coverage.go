package dogfoodflow

import (
	"maps"
	"path"
	"slices"
	"strings"
)

// noteRunSubsetPackages names, on a passing inputs-check, each Go package the
// change edits outside its tests while every verify line that runs it narrows
// it with a -run subset (DWC-V0-3727, V1-4108). corvint affected selects such a
// package whole (a changed declaration is not additive), so a hand-picked
// -run list can omit the package's cross-cutting tests. The packages come from
// BASE..HEAD's changed non-test .go files, not from a second affected run, so
// the NOTE is a cheap prompt to compare with the lane's affected plan; it
// changes no exit status.
func (l *finishLane) noteRunSubsetPackages() {
	verify, reason := verifyInput(l.options.Change)
	if reason != "" || len(verify) == 0 {
		return
	}
	changed, status, bounded := l.boundedGit("diff", "--name-only", "--no-renames", "--diff-filter=AM", l.base, l.target, "--", "*.go")
	if status != 0 || !bounded {
		return
	}
	edited := map[string]bool{}
	for _, file := range nonEmpty(strings.Split(changed, "\n")) {
		if !strings.HasSuffix(file, "_test.go") {
			edited[goPackageArgument(path.Dir(file))] = true
		}
	}
	whole, subset := verifyPackageRuns(verifyLines(l.options.Change))
	for _, pkg := range slices.Sorted(maps.Keys(subset)) {
		if edited[pkg] && !whole[pkg] {
			l.line("inputs-check NOTE verify-run-subset package=%s: the change edits non-test source there, which corvint affected selects whole, but every verify line that runs it narrows it with -run; compare with the affected plan and add a whole-package line if it selects the package whole (DWC-V0-3727)", pkg)
		}
	}
}

// verifyLines is the nonblank verify input lines.
func verifyLines(options ChangeOptions) []string {
	data := []byte(options.Verify + "\n")
	if options.VerifyFile != "" {
		read, err := readPrefix(options.VerifyFile, maxVerifyFileBytes)
		if err != nil {
			return nil
		}
		data = read
	}
	lines := []string{}
	for _, line := range textLines(data) {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// verifyPackageRuns sorts each ./ package a go test line names into whole,
// when the line has no -run or runs every test (-run '.' or '.*'), or subset.
func verifyPackageRuns(lines []string) (whole, subset map[string]bool) {
	whole, subset = map[string]bool{}, map[string]bool{}
	for _, line := range lines {
		words := strings.Fields(line)
		at := slices.Index(words, "test")
		if at < 1 || words[at-1] != "go" {
			continue
		}
		run, packages := "", []string{}
		for index := at + 1; index < len(words); index++ {
			word := words[index]
			switch {
			case word == "-run" && index+1 < len(words):
				index++
				run = strings.Trim(words[index], `'"`)
			case strings.HasPrefix(word, "-run="):
				run = strings.Trim(strings.TrimPrefix(word, "-run="), `'"`)
			case word == "." || strings.HasPrefix(word, "./") && !strings.Contains(word, "..."):
				packages = append(packages, path.Clean(word))
			}
		}
		for _, pkg := range packages {
			pkg = goPackageArgument(pkg)
			if run == "" || run == "." || run == ".*" {
				whole[pkg] = true
			} else {
				subset[pkg] = true
			}
		}
	}
	return whole, subset
}

// goPackageArgument spells a clean repository directory as go test names it.
func goPackageArgument(dir string) string {
	if dir == "." {
		return "."
	}
	return "./" + dir
}
