package contextindex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// coreSource line 3 carries trailing spaces so a passing anchor also proves the
// right-trim rule the citation gate applies before hashing.
const coreSource = "package core\n\nfunc Core() int { return 1 }   \n\nfunc Other() {}\n"

func anchorPin(t *testing.T, cited ...string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(strings.Join(cited, "\n")))
	return hex.EncodeToString(digest[:])[:8]
}

// authorityFixture is one repository whose three authority kinds all cite
// pkg/core.go: binding instructions, an accepted decision, and an accepted spec.
func authorityFixture(t *testing.T) *Index {
	t.Helper()
	line3 := anchorPin(t, "func Core() int { return 1 }")
	span := anchorPin(t, "package core", "", "func Core() int { return 1 }")
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                      "module example.test/fixture\n\ngo 1.27.0\n",
		"pkg/core.go":                 coreSource,
		"pkg/quiet.go":                "package core\n\nfunc Quiet() {}\n",
		"AGENTS.md":                   "# Instructions\n\nEvery change to `pkg/core.go:3@" + line3 + "` keeps its contract.\n",
		"docs/decisions/0001-core.md": "# Decision 0001\n\nStatus: accepted\n\nThe span `pkg/core.go:1-3@" + span + "` is the ratified shape.\n",
		"docs/specs/core-v0.md": "# Core V0\n\nStatus: accepted\n\nThe entry point is `pkg/core.go:5` and its old form was `pkg/core.go:3@0000dead`.\n\n" +
			"```\nA fenced `pkg/quiet.go:1` citation is not a trigger.\n```\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func triggerRows(t *testing.T, result map[string]any) []map[string]any {
	t.Helper()
	rows := make([]map[string]any, 0)
	for _, row := range result["results"].([]any) {
		rows = append(rows, row.(map[string]any))
	}
	return rows
}

// ATI-V0-001..004, ATI-V0-007: a citation fires only for a requested path, is
// reported as `cites` rather than a verdict, carries an evidence handle pinned to
// the citing document's blob, and never comes from inside a fenced block.
func TestAuthorityTriggersFireOnlyForCitedRequestedPaths(t *testing.T) {
	index := authorityFixture(t)
	result, err := LookupAuthorityTriggers(index, []string{"pkg/core.go"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	rows := triggerRows(t, result)
	if len(rows) != 4 || result["state"] != "READY" || result["mode"] != "authority" {
		t.Fatalf("core triggers = %v", result)
	}
	first := rows[0]
	handle := first["evidence"].([]any)[0].(map[string]any)
	if first["relation"] != "cites" || first["id"] != "AGENTS.md" || first["path"] != "pkg/core.go" {
		t.Fatalf("first trigger = %v", first)
	}
	if handle["path"] != "AGENTS.md" || handle["line"] != 3 || handle["blob_hash"] != index.Documents["AGENTS.md"].BlobHash {
		t.Fatalf("evidence handle = %v", handle)
	}
	if !strings.Contains(handle["reason"].(string), "cites pkg/core.go:3@") {
		t.Fatalf("reason does not explain inclusion: %v", handle)
	}
	quiet, err := LookupAuthorityTriggers(index, []string{"pkg/quiet.go"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(triggerRows(t, quiet)) != 0 || quiet["state"] != "NO_CANDIDATES" {
		t.Fatalf("fenced citation fired a trigger: %v", quiet)
	}
}

// ATI-V0-006: the three anchor states a cited path can be in, computed the way
// script/check-line-citations.sh computes them, including over a written range.
func TestAuthorityTriggerAnchorStatesMatchTheCitationGate(t *testing.T) {
	index := authorityFixture(t)
	result, err := LookupAuthorityTriggers(index, []string{"pkg/core.go"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	states := make(map[string]string)
	for _, row := range triggerRows(t, result) {
		states[row["citation"].(string)] = row["anchor"].(string)
	}
	if got := states["pkg/core.go:3@"+anchorPin(t, "func Core() int { return 1 }")]; got != anchorPinned {
		t.Fatalf("trailing whitespace broke a matching anchor: %v", states)
	}
	if got := states["pkg/core.go:1-3@"+anchorPin(t, "package core", "", "func Core() int { return 1 }")]; got != anchorPinned {
		t.Fatalf("range anchor = %q, want pinned: %v", got, states)
	}
	if states["pkg/core.go:3@0000dead"] != anchorDrifted || states["pkg/core.go:5"] != anchorUnpinned {
		t.Fatalf("anchor states = %v", states)
	}
}

// ATI-V0-005, ATI-V0-008: invariant-3 authority orders the table, anchor state
// breaks its ties, and both are deterministic.
func TestAuthorityTriggersOrderByAuthorityThenAnchor(t *testing.T) {
	index := authorityFixture(t)
	result, err := LookupAuthorityTriggers(index, []string{"pkg/core.go"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	labels := make([]string, 0)
	anchors := make([]string, 0)
	for _, row := range triggerRows(t, result) {
		labels = append(labels, row["authority"].(string))
		anchors = append(anchors, row["anchor"].(string))
	}
	wantLabels := "project-instructions,accepted-decision,accepted-spec,accepted-spec"
	if strings.Join(labels, ",") != wantLabels {
		t.Fatalf("authority order = %v, want %s", labels, wantLabels)
	}
	if strings.Join(anchors[2:], ",") != "unpinned,drifted" {
		t.Fatalf("anchor tie-break = %v", anchors)
	}
}

// ATI-V0-009, ATI-V0-010: the table is bounded and its two blind spots are named
// rather than absorbed into the silence.
func TestAuthorityTriggersBoundResultsAndReportBlindSpots(t *testing.T) {
	index := authorityFixture(t)
	limited, err := LookupAuthorityTriggers(index, []string{"pkg/core.go"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	coverage := limited["coverage"].(map[string]any)
	if len(triggerRows(t, limited)) != 1 || coverage["candidates"] != 4 || coverage["omitted_results"] != 3 {
		t.Fatalf("bounded table = %v", limited)
	}
	absent, err := LookupAuthorityTriggers(index, []string{"pkg/new.go"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	untracked := absent["untracked_paths"].([]any)
	if absent["state"] != "NO_CANDIDATES" || len(untracked) != 1 || untracked[0] != "pkg/new.go" {
		t.Fatalf("untracked path silently reported as uncited: %v", absent)
	}
	if len(absent["unread_documents"].([]any)) != 0 {
		t.Fatalf("fixture documents did not all load: %v", absent["unread_documents"])
	}
}

// ATI-V0-011: a path the index could never resolve is a typed refusal, not an
// empty answer that would read as "no authority cites this".
func TestAuthorityTriggersRefuseUnusablePaths(t *testing.T) {
	index := authorityFixture(t)
	if _, err := LookupAuthorityTriggers(index, nil, 20); err == nil {
		t.Fatal("no paths was accepted")
	}
	if _, err := LookupAuthorityTriggers(index, []string{"../escape.go"}, 20); err == nil {
		t.Fatal("path outside the repository was accepted")
	}
	if _, err := LookupAuthorityTriggers(index, []string{"pkg/core.go"}, 0); err == nil {
		t.Fatal("limit 0 was accepted")
	}
}

// V1-0049: a written `last` of 0 collides with the "no range" sentinel, and an
// inverted range never satisfies `first <= last`; citedNumbers accepts both
// rather than refusing them, degrading to a smaller number set instead.
func TestAuthorityCiteCitedNumbersDegradesMalformedRanges(t *testing.T) {
	cases := []struct {
		name  string
		token string
		want  []int
	}{
		{name: "trailing zero last falls back to first only", token: "`docs/x/y.md:5-0`", want: []int{5}},
		{name: "inverted range cites no lines", token: "`docs/x/y.md:9-3`", want: []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cites := documentCitations(tc.token)
			if len(cites) != 1 {
				t.Fatalf("documentCitations(%q) = %v, want exactly one citation", tc.token, cites)
			}
			numbers, inRange := cites[0].citedNumbers(20)
			if !inRange {
				t.Fatalf("citedNumbers(%q) reported false, want the malformed range accepted", tc.token)
			}
			if len(numbers) != len(tc.want) {
				t.Fatalf("citedNumbers(%q) = %v, want %v", tc.token, numbers, tc.want)
			}
			for i, number := range numbers {
				if number != tc.want[i] {
					t.Fatalf("citedNumbers(%q) = %v, want %v", tc.token, numbers, tc.want)
				}
			}
		})
	}
}

// A citation may name any digit run, so an absurd range must be bounded against
// the cited target before it is expanded: the lookup returns cleanly and reports
// the anchor as unreadable rather than allocating over the written range.
func TestAuthorityTriggersBoundAbsurdCitationRanges(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":      "module example.test/fixture\n\ngo 1.27.0\n",
		"pkg/core.go": coreSource,
		"AGENTS.md": "# Instructions\n\nSee `pkg/core.go:1-9223372036854775807@deadbeef`, " +
			"`pkg/core.go:0-9223372036854775807@deadbeef` and `pkg/core.go:1-9223372036854775807`.\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := LookupAuthorityTriggers(index, []string{"pkg/core.go"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	rows := triggerRows(t, result)
	if len(rows) != 3 {
		t.Fatalf("absurd citations = %v", result)
	}
	for _, row := range rows {
		want := anchorUnreadable
		if !strings.Contains(row["citation"].(string), "@") {
			want = anchorUnpinned
		}
		if row["anchor"] != want {
			t.Fatalf("anchor for %v = %v, want %s", row["citation"], row["anchor"], want)
		}
	}
}
