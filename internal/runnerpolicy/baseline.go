// SPDX-License-Identifier: Apache-2.0
package runnerpolicy

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testrunner"
)

// BaselineUpdate accompanies a soundness-driven widening. This internal check
// accepts the updated baseline only when it includes the fixed selection and
// the build-log entry names the fix and every test newly selected by it.
type BaselineUpdate struct {
	Fix, BuildLog string
	Reselected    []string
}

func CheckBaselineUpdate(before, fixed, baseline testrunner.RunSet, entry BaselineUpdate) error {
	for _, run := range []testrunner.RunSet{before, fixed, baseline} {
		if len(run.Tests) > testrunner.MaxTests {
			return fmt.Errorf("baseline-input-capped")
		}
		seen := map[string]bool{}
		for _, test := range run.Tests {
			if test == "" || len(test) > 4096 || !utf8.ValidString(test) || strings.TrimSpace(test) != test || strings.IndexFunc(test, unicode.IsControl) >= 0 || seen[test] {
				return fmt.Errorf("baseline-test-invalid")
			}
			seen[test] = true
		}
	}
	if len(entry.Reselected) > testrunner.MaxTests {
		return fmt.Errorf("baseline-entry-capped")
	}
	// FULL is a valid conservative baseline even with unknown inventory.
	if !baseline.Full && (fixed.Full || !containsAll(baseline.Tests, fixed.Tests)) {
		return fmt.Errorf("baseline-conflicts-with-soundness")
	}
	if strings.TrimSpace(entry.Fix) == "" || !strings.HasPrefix(entry.BuildLog, "docs/build-log/") || !strings.HasSuffix(entry.BuildLog, ".md") || strings.Contains(entry.BuildLog, "..") {
		return fmt.Errorf("baseline-build-log-missing")
	}
	if !before.Full {
		for _, test := range fixed.Tests {
			if !containsAll(before.Tests, []string{test}) && !containsAll(entry.Reselected, []string{test}) {
				return fmt.Errorf("baseline-reselected-test-missing:%s", test)
			}
		}
	}
	return nil
}
