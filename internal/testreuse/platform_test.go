// SPDX-License-Identifier: Apache-2.0

package testreuse

import (
	"os"
	"strings"
	"testing"
)

// TestMain pins the platform DigestPackage checks to linux, so the other
// tests exercise the reuse rules on every host; TestUnprovenPlatformRuns
// covers the refusal itself.
func TestMain(m *testing.M) {
	reusePlatform = "linux"
	os.Exit(m.Run())
}

// TestUnprovenPlatformRuns covers AFP-V0-2320: the reuse rules are proven by
// tests only on linux, so on any other platform no package is eligible and
// each runs (DIGEST_ERROR unproven-platform). The same record decided on
// linux is the negative control.
func TestUnprovenPlatformRuns(t *testing.T) {
	root := fixtureRepo(t)
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	wantUnits(t, "linux", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonRecordDigestEqual})
	for _, platform := range []string{"darwin", "windows", "freebsd"} {
		reusePlatform = platform
		units := decideAt(t, root, localPolicy, record)
		reusePlatform = "linux"
		wantUnits(t, platform, units, map[string]string{"a": ReasonDigestError, "b": ReasonDigestError})
		if !strings.HasPrefix(units["a"].Detail, ErrorUnprovenPlatform+": ") {
			t.Errorf("%s: missing unproven-platform detail: %+v", platform, units["a"])
		}
	}
}
