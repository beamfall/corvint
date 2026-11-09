// SPDX-License-Identifier: Apache-2.0

package workflow

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEM-V1-8600: an ambiguous unedited run next to an insertion must not
// hide the unique stable run on the insertion's other side.
func TestCEMV18600RecoveryPastPureInsertion(t *testing.T) {
	for _, tc := range []struct{ name, base, head, want string }{
		{"after", "same\nsame\nedit\nsame\nunique\n", "same\nsame\nchanged\nsame\ninserted\nunique\n", "5:5"},
		{"before", "unique\nsame\nedit\nsame\nsame\n", "unique\ninserted\nsame\nchanged\nsame\nsame\n", "1:1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, base, target := makeCiteSpanRepoFrom(t, tc.base, tc.head)
			session := openSession(t, root)
			if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
				t.Fatal(err)
			}
			_, err := session.Cite(ctx(), CiteOptions{MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt", Lines: "3:3", Relation: "specification"})
			if cemcode.CodeOf(err) != cemcode.CiteSpanNotStable {
				t.Fatalf("got %v", err)
			}
			if !strings.Contains(cemcode.MessageOf(err), "--lines "+tc.want) {
				t.Fatalf("missing recovery %s: %v", tc.want, err)
			}
			if _, err := session.Cite(ctx(), CiteOptions{MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "src/app.txt", Lines: tc.want, Relation: "specification"}); err != nil {
				t.Fatalf("suggestion refused: %v", err)
			}
		})
	}
}
