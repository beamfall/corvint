// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"fmt"
	"path"
	"strings"
)

// Levels a record may carry (MCJ-V0-007).
const (
	LevelUnit        = "unit"
	LevelIntegration = "integration"
	LevelE2E         = "e2e"
)

// LevelSource names which rule assigned a record's level (MCJ-V0-007,
// MCJ-V0-040).
type LevelSource string

const (
	// LevelDeclared: the project declared the level, in its runner config
	// (for example RSpec metadata or a Playwright project) or a build tag.
	LevelDeclared LevelSource = "declared"
	// LevelPathRule: a path rule the repository's Corvint configuration
	// declares; a project declaration too, below a runner declaration.
	LevelPathRule LevelSource = "path-rule"
	// LevelRunner: the runner family's own convention: a browser E2E family,
	// or a runner type the family assigns (RSpec `type: :feature`).
	LevelRunner LevelSource = "runner"
	// LevelPath: the family's conventional directory (RSpec `spec/system`).
	LevelPath LevelSource = "path"
	// LevelDefault: nothing applied, so the level is unknown.
	LevelDefault LevelSource = "default"
)

// LevelRule is one declared path rule: a test file at or below Prefix (a
// slash-separated repository-relative directory or file) has Level.
type LevelRule struct {
	Prefix string
	Level  string
}

// LevelInput is what MCJ-V0-007 reads for one record.
type LevelInput struct {
	// Declared is the level the runner config or a build tag declares.
	Declared string
	// Rules are the repository's declared path rules; the longest matching
	// prefix wins.
	Rules []LevelRule
	// Family is the runner family (LPCV-V0-955), for example "playwright",
	// "rspec" or "go".
	Family string
	// RunnerType is a type the runner itself assigns the test, for example
	// the RSpec `type` metadata ("feature", "system", "request").
	RunnerType string
	// Path is the repository-relative test file.
	Path string
}

// browserE2E lists the browser E2E families of LPCV-V0-955: their records
// are e2e unless the project declares otherwise.
var browserE2E = map[string]bool{
	"playwright": true, "cypress": true, "webdriverio": true, "testcafe": true,
	"nightwatch": true, "storybook": true, "detox": true,
}

// rspecTypes maps the RSpec and rspec-rails example types whose meaning is
// fixed by the runner to their level (MCJ-V0-040). Other types (model,
// helper, ...) say nothing about level: unknown.
var rspecTypes = map[string]string{
	"feature": LevelE2E, "system": LevelE2E,
	"request": LevelIntegration, "integration": LevelIntegration,
}

// rspecDirs are the directories rspec-rails infers those same types from,
// at any depth so a project in a repository subdirectory matches too.
var rspecDirs = []struct{ dir, level string }{
	{"spec/features/", LevelE2E}, {"spec/system/", LevelE2E},
	{"spec/requests/", LevelIntegration}, {"spec/integration/", LevelIntegration},
}

// ValidLevel reports whether l is one MCJ-V0-007 level.
func ValidLevel(l string) bool {
	switch l {
	case LevelUnit, LevelIntegration, LevelE2E, LevelUnknown:
		return true
	}
	return false
}

// ResolveLevel assigns a record's level: a runner or build-tag declaration
// wins, then a declared path rule, then the family's runner convention, then
// its path convention, else unknown (MCJ-V0-007, MCJ-V0-040). A declared
// level that is not one of the four refuses, so a typo never reads as a
// level. Level groups reports only; it never changes selection.
func ResolveLevel(in LevelInput) (string, LevelSource, error) {
	if in.Declared != "" {
		if !ValidLevel(in.Declared) {
			return "", "", fmt.Errorf("linecov: declared level %q is not unit, integration, e2e or unknown", in.Declared)
		}
		return in.Declared, LevelDeclared, nil
	}
	file := path.Clean(strings.TrimPrefix(in.Path, "./"))
	best, bestLen := "", -1
	for _, r := range in.Rules {
		if !ValidLevel(r.Level) {
			return "", "", fmt.Errorf("linecov: path rule %q declares level %q", r.Prefix, r.Level)
		}
		p := strings.TrimSuffix(path.Clean(r.Prefix), "/")
		if (p == "." || file == p || strings.HasPrefix(file, p+"/")) && len(p) > bestLen {
			best, bestLen = r.Level, len(p)
		}
	}
	if bestLen >= 0 {
		return best, LevelPathRule, nil
	}
	if browserE2E[in.Family] {
		return LevelE2E, LevelRunner, nil
	}
	if in.Family == "rspec" {
		if l, ok := rspecTypes[in.RunnerType]; ok {
			return l, LevelRunner, nil
		}
		if in.RunnerType == "" {
			for _, d := range rspecDirs {
				if strings.Contains("/"+file, "/"+d.dir) {
					return d.level, LevelPath, nil
				}
			}
		}
	}
	return LevelUnknown, LevelDefault, nil
}
