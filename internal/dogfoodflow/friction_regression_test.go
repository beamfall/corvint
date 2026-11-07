// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// DWC-V0-044, DCW-V0-2143, DCW-V0-015: a standalone rebind whose only change
// is the replaced sidecar still runs the recorder, then ends PENDING with exit
// 3 and one commit-and-rerun fix; the coordination-time impact refusal the
// dirty sidecar causes is pending too, never a cascade of failures.
func TestDWCV0044RebindIsPendingAfterTheRecorder(t *testing.T) {
	root, base := preflightRepository(t, nil)
	// Sidecars are private derived state, as in the actual repository.
	writeTree(t, root, map[string]string{".git/info/exclude": ".corvint/\n"})
	var output bytes.Buffer
	if rc := cemcli.Run(context.Background(), root, []string{"prepare", "--base", base, "--target", "HEAD"}, &output, &output); rc != 0 {
		t.Fatal(output.String())
	}
	testGit(t, root, "add", "-f", ".corvint/change.cem.json")
	testGit(t, root, "commit", "-qm", "bind")
	writeTree(t, root, map[string]string{"extra.txt": "new change after committed bind\n"})
	testGit(t, root, "add", "extra.txt")
	testGit(t, root, "commit", "-qm", "fix")
	var draft, notes bytes.Buffer
	if rc, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &draft, &notes); rc != 0 || err != nil {
		t.Fatalf("draft %d %v %s", rc, err, &notes)
	}
	plan := writeInput(t, draft.String())
	var calls []string
	runner := Runner{Path: "corvint", Run: func(ctx context.Context, root string, args []string, out, stderr io.Writer) int {
		calls = append(calls, strings.Join(args, " "))
		if args[0] == "dogfood-observe" {
			return 0
		}
		if args[0] == "cem" {
			return cemcli.Run(ctx, root, args[1:], out, stderr)
		}
		if args[0] == "query" {
			io.WriteString(out, "{}\n")
			return 0
		}
		if args[0] == "impact" {
			io.WriteString(stderr, `{"code":"unsupported-impact-worktree","error":"range impact requires a clean worktree","ok":false}`+"\n")
			return 2
		}
		t.Logf("downstream step %v", args)
		return 1
	}}
	var stderr bytes.Buffer
	rc, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: runner, Citations: plan, IntentsFile: writeInput(t, "#no-intent-declared\n"), Verify: "go version", Outcome: "passed"}, &stderr)
	if err != nil || strings.Contains(stderr.String(), "commit-rebind-required") {
		t.Fatalf("change %d %v\n%s", rc, err, &stderr)
	}
	// DCW-V0-015: the recorder still runs after the rebound sidecar is cited.
	recorded := false
	for _, call := range calls {
		recorded = recorded || strings.HasPrefix(call, "dogfood-record ")
	}
	if !recorded {
		t.Fatalf("the recorder did not run after the rebind: %v", calls)
	}
	// DWC-V0-044 under DCW-V0-2143: the impact refusal the dirty sidecar causes
	// is cleared by the same commit, so beside the rows the bind commit clears
	// the pass is pending, not a failure.
	rows := []step{
		{"coordination-time-impact", "NOT_PRODUCED", "unsupported-impact-worktree"},
		{"ocm-prepare-1", "NOT_PRODUCED", "excluded-artifact-mismatch"},
		{"ocm-aggregate", "NOT_PRODUCED", "ocm-status-not-ready"},
		{"cem-status", "NOT_PRODUCED", "not-ready"},
		{"local-outcome", "NOT_PRODUCED", "record-index-failed"},
	}
	pending := func(rows []step) bool {
		c := &change{flow: flow{ctx: context.Background(), root: root, base: base, target: testGit(t, root, "rev-parse", "HEAD"), stderr: io.Discard}, rows: rows}
		return c.pendingSidecarCommit()
	}
	if !pending(rows) {
		t.Fatal("a rebind whose only extra row is the dirty-sidecar impact refusal is not pending")
	}
	// Controls: another impact refusal, or another dirty path, keeps FAIL.
	other := append([]step{{"coordination-time-impact", "NOT_PRODUCED", "impact-failed"}}, rows[1:]...)
	if pending(other) {
		t.Fatal("an unrelated impact failure was treated as pending")
	}
	writeTree(t, root, map[string]string{"extra.txt": "dirty beside the sidecar\n"})
	if pending(rows) {
		t.Fatal("a dirty path beside the sidecar was treated as pending")
	}
}

func TestDWCV0045RefreshDraftsNewHunkFromItsOwnContent(t *testing.T) {
	root, base := preflightRepository(t, nil)
	// Keep one row only: its usage-comment evidence must never spread to the
	// other, unmatched hunk of the same file.
	var initial, notes bytes.Buffer
	if rc, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &initial, &notes); rc != 0 || err != nil {
		t.Fatal(rc, err, notes.String())
	}
	rows := textLines(initial.Bytes())
	first, _, _ := strings.Cut(rows[0], "\t")
	plan := writeInput(t, first+"\tpre.go\t3:3\tdependency\n")
	var refreshed, stderr bytes.Buffer
	if rc, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, Refresh: plan}, &refreshed, &stderr); rc != 0 || err != nil {
		t.Fatal(rc, err, stderr.String())
	}
	now := textLines(refreshed.Bytes())
	if len(now) != 2 || now[0] != first+"\tpre.go\t3:3\tdependency" || now[1] != rows[1] || strings.Contains(stderr.String(), "carried forward:") {
		t.Fatalf("initial %s\nrefresh %s\n%s", &initial, &refreshed, &stderr)
	}
}

func TestDWCV0046ExternalReviewSeal(t *testing.T) {
	for _, kind := range []string{"pass", "legacy", "legacy-duplicate", "external-with-legacy", "external-prose", "legacy-prose", "wrong-head", "failed", "duplicate", "missing", "absent", "oversized", "malformed", "nonregular", "moved-during-check"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			testGit(t, root, "init", "-q")
			writeTree(t, root, map[string]string{"a.txt": "base\n"})
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "base")
			base := testGit(t, root, "rev-parse", "HEAD")
			writeTree(t, root, map[string]string{".corvint/change.cem.json": "bound evidence\n"})
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "bind")
			head := testGit(t, root, "rev-parse", "HEAD")
			raw := "dogfood-review: reviewed-head=" + head + " verdict=PASS\nIndependent external report.\n"
			switch kind {
			case "legacy", "legacy-duplicate":
				raw = "codex-review: reviewed-head=" + base + " tree=x dirty=no\nLegacy report.\n"
				if kind == "legacy-duplicate" {
					raw += raw
				}
			case "external-with-legacy":
				raw += "codex-review: reviewed-head=" + base + " tree=x dirty=no\n"
			case "external-prose":
				// A prose mention of the key is not a second header.
				raw += "The parser recognizes `dogfood-review:` headers.\n"
			case "legacy-prose":
				// Nor does it switch a legacy report to the external format.
				raw = "codex-review: reviewed-head=" + base + " tree=x dirty=no\nThe parser recognizes `dogfood-review:` headers.\n"
			case "wrong-head":
				raw = strings.Replace(raw, head, base, 1)
			case "failed":
				raw = strings.Replace(raw, "PASS", "FAIL", 1)
			case "duplicate":
				raw += raw
			case "oversized":
				raw += strings.Repeat("x", 1<<20)
			case "malformed":
				raw = "dogfood-review: reviewed-head=HEAD verdict=PASS\n"
			}
			path := writeInput(t, raw)
			if kind == "missing" {
				path += ".missing"
			}
			if kind == "absent" {
				path = ""
			}
			if kind == "nonregular" {
				path = t.TempDir()
			}
			t.Setenv("DOGFOOD_REVIEW", path)
			t.Setenv("GIT_AUTHOR_NAME", "t")
			t.Setenv("GIT_COMMITTER_NAME", "t")
			t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
			t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
			var out, stderr bytes.Buffer
			c := newCheck(context.Background(), "dogfood-seal", CheckOptions{Root: root}, &out, &stderr)
			c.base = base
			valid := kind == "pass" || kind == "legacy" || kind == "external-with-legacy" || kind == "external-prose" || kind == "legacy-prose" || kind == "absent"
			var rc int
			var err error
			if kind == "moved-during-check" {
				rc, err = boundary(c.ctx, func() int {
					c.sealReviewEvidence = c.sealReview()
					testGit(t, root, "commit", "--allow-empty", "-qm", "moved")
					return c.seal()
				}, func() {})
				if rc != 2 || err != nil || !strings.Contains(stderr.String(), "REFUSE review-not-current") {
					t.Fatalf("%d %v %s", rc, err, &stderr)
				}
				if got := testGit(t, root, "ls-files", ".corvint/change.cem.json"); got == "" {
					t.Fatal("moved-head refusal renamed the CEM")
				}
				return
			}
			if valid {
				rc, err = boundary(c.ctx, func() int { c.sealReviewEvidence = c.sealReview(); return c.seal() }, func() {})
			} else {
				// Public entry must reject supplied invalid reports before checking.
				rc, err = Seal(c.ctx, CheckOptions{Root: root, Base: base}, &out, &stderr)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !valid {
				if rc != 2 || testGit(t, root, "rev-parse", "HEAD") != head || !strings.Contains(stderr.String(), "REFUSE review-not-current") {
					t.Fatalf("%d %s%s", rc, &out, &stderr)
				}
				return
			}
			if rc != 0 {
				t.Fatalf("seal %d %s%s", rc, &out, &stderr)
			}
			if kind == "pass" || kind == "legacy" || kind == "external-with-legacy" || kind == "external-prose" || kind == "legacy-prose" {
				reviewed, verdict := head, "PASS"
				if kind == "legacy" || kind == "legacy-prose" {
					reviewed, verdict = base, "NOT_OBSERVED"
				}
				message := testGit(t, root, "log", "-1", "--format=%B")
				want := "review-before-seal OBSERVED path="
				if !strings.Contains(message, want) || !strings.Contains(message, "reviewed-head="+reviewed) || !strings.Contains(message, "verdict="+verdict) || !strings.Contains(message, "sha256=sha256:"+sha256Hex([]byte(raw))) {
					t.Fatal(message)
				}
				if !strings.Contains(stderr.String(), want) || !strings.Contains(stderr.String(), "verdict="+verdict) {
					t.Fatal(stderr.String())
				}
			} else if !strings.Contains(stderr.String(), "review-before-seal NOT_OBSERVED") {
				t.Fatal(stderr.String())
			}
			if got := testGit(t, root, "diff", "--name-status", "-M", "HEAD^"); !strings.HasPrefix(got, "R100\t.corvint/change.cem.json\t.corvint/changes/"+head) {
				t.Fatal(got)
			}
		})
	}
}

func TestDWCV0047HandbackCumulativeTickets(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	script, err := filepath.Abs("../../script/lane-handback.sh")
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		out, err := exec.Command("bash", append([]string{script}, args...)...).CombinedOutput()
		return string(out), err
	}
	template, err := run("template")
	if err != nil {
		t.Fatal(err, template)
	}
	head := strings.Repeat("a", 40)
	old := strings.ReplaceAll(template, "FULL_SHA", head)
	old = strings.Replace(old, "NOT_PRODUCED list all delivered ticket IDs", "V1-4545 V1-4546", 1)
	previous := writeInput(t, old)
	next, err := run("template", previous)
	if err != nil {
		t.Fatal(err, next)
	}
	next = strings.ReplaceAll(next, "FULL_SHA", head)
	if !strings.Contains(next, "Delivered tickets: V1-4545 V1-4546") {
		t.Fatal(next)
	}
	for _, test := range []struct {
		name, text string
		pass       bool
	}{
		{"valid", next, true},
		{"general-ticket-ids", strings.Replace(next, "V1-4545 V1-4546", "V1-4545 V1-4546 CORE-FOO-1 BUG-X-0930 OSC-Y-1", 1), true},
		{"lowercase-ticket-id", strings.Replace(next, "V1-4545 V1-4546", "V1-4545 V1-4546 bug-X-1", 1), false},
		{"no-sealed-head", strings.Replace(next, "Sealed head: NOT_PRODUCED seal pending\n", "", 1), false},
		{"empty-tickets", strings.Replace(next, "Delivered tickets: V1-4545 V1-4546", "Delivered tickets: ", 1), false},
		{"lost-cumulative-ticket", strings.Replace(next, "V1-4545 V1-4546", "V1-4546", 1), false},
		{"superseded", strings.Replace(strings.Replace(next, "Delivered tickets: V1-4545 V1-4546", "Delivered tickets: V1-4546", 1), "Superseded tickets: none", "Superseded tickets: V1-4545", 1), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, err := run("lint", writeInput(t, test.text), previous)
			if (err == nil) != test.pass {
				t.Fatalf("%v %s", err, out)
			}
		})
	}
}
