// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// liveEpoch is the forward-evidence epoch the CI workflows name today (AFP-V0-030), read from
// ci.yml, the single source (CIN-V0-960).
func liveEpoch(t *testing.T) string {
	t.Helper()
	return forwardEpoch(t)
}

// barJSON runs bar.jq for EPOCH over NAMES, newest first, with the reviewed AFP-V0-463 list.
func barJSON(t *testing.T, epoch string, names []string) doc {
	t.Helper()
	cmd := exec.Command("jq", "-R", "-s", "-r", "--arg", "epoch", epoch, "--arg", "format", "json",
		"--rawfile", "reclassified", script(t, "infra-reclassified.txt"), "-f", script(t, "bar.jq"))
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bar.jq: %v", err)
	}
	var v doc
	if err := jsonUnmarshal(string(out), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestCINV0001EpochTwoLeavesTheDeadPinRowsBehind pins CIN-V0-001 (V1-3314): the live epoch is
// at least 2, so the epoch-1 INCOMPLETE and MISS rows the dead b609ca4b pins produced neither
// count toward the AFP-V0-029 bar nor hold the kill switch, whose thresholds are unchanged.
func TestCINV0001EpochTwoLeavesTheDeadPinRowsBehind(t *testing.T) {
	epoch := liveEpoch(t)
	if n, err := strconv.Atoi(epoch); err != nil || n < 2 {
		t.Fatalf("live forward epoch %q, want 2 or later (V1-3314)", epoch)
	}
	// Newest first: two epoch rows, then five dead-pin INCOMPLETE rows and two MISS rows of epoch 1.
	names := []string{barNames("CLEAN", 401, epoch, "0"), barNames("CLEAN", 402, epoch, "1")}
	for pr := 300; pr < 305; pr++ {
		names = append(names, barNames("INCOMPLETE", pr, "1", "0"))
	}
	names = append(names, barNames("MISS", 305, "1", "1"), barNames("MISS", 306, "1", "0"))
	v := barJSON(t, epoch, names)
	rows, w := sub(v, "epochRows"), sub(v, "window")
	if v["killSwitch"] != "clear" || num(rows["rows"]) != 2 || num(rows["complete"]) != 2 || num(w["incomplete"]) != 0 ||
		num(w["miss"]) != 0 || v["covered"] != true || v["status"] != "NOT MET" {
		t.Fatalf("epoch %s verdict = %v", epoch, v)
	}
	// The same rows under epoch 1 still trip the switch: the advance clears the window only.
	if old := barJSON(t, "1", names); old["killSwitch"] != "TRIPPED" {
		t.Fatalf("epoch 1 verdict = %v", old)
	}
	// Each epoch-naming site moved together (AFP-V0-097 bindings check the rest).
	cp, err := os.ReadFile("../../.github/workflows/ci-control-plane.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cp), "FORWARD_EPOCH: \""+epoch+"\"") {
		t.Fatalf("ci-control-plane.yml names rows of another epoch than %s", epoch)
	}
}

// TestCINV0002UnreachablePinFailsTheShardLoudly pins CIN-V0-002 (V1-3309 AC3): the pin capture
// and build steps keep continue-on-error so the full suite still runs, and a last step of the
// shard names each failed pin in an error annotation and fails the job.
func TestCINV0002UnreachablePinFailsTheShardLoudly(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)
	shard := regexp.MustCompile(`(?ms)^  go-product-shard:\n(.*?)\n  ci-tested-tree-record:`).FindStringSubmatch(ci)
	if shard == nil {
		t.Fatal("no go-product-shard job")
	}
	for _, id := range []string{"pin-tool-capture", "pin-tool-build", "pin-shadow-capture", "pin-shadow-build"} {
		if !regexp.MustCompile(`(?m)^        id: ` + id + `\n        continue-on-error: true$`).MatchString(shard[1]) {
			t.Fatalf("pin step %s lacks its id or continue-on-error", id)
		}
	}
	m := regexp.MustCompile(`(?ms)^      - name: Fail loudly on an unreachable or unbuildable pin \(CIN-V0-002, V1-3309\)\n(.*?)\n        run: \|\n(.*)$`).FindStringSubmatch(shard[1])
	if m == nil {
		t.Fatal("go-product-shard lacks the CIN-V0-002 step as its last step")
	}
	if strings.Contains("\n"+m[1], "\n        continue-on-error:") || !strings.Contains(m[1], "if: always() && (steps.pin-tool-capture.outcome == 'failure' || steps.pin-tool-build.outcome == 'failure' || steps.pin-shadow-capture.outcome == 'failure' || steps.pin-shadow-build.outcome == 'failure')") {
		t.Fatalf("CIN-V0-002 step header:\n%s", m[1])
	}
	body := regexp.MustCompile(`(?m)^          `).ReplaceAllString(m[2], "")
	summary := t.TempDir() + "/summary"
	cmd := exec.Command("sh", "-c", body)
	cmd.Env = append(os.Environ(), "CORVINT_PR_TOOL_SOURCE="+strings.Repeat("b", 40), "CORVINT_TEST_SHADOW_SOURCE="+strings.Repeat("c", 40),
		"TOOL_CAPTURE=failure", "TOOL_BUILD=skipped", "SHADOW_CAPTURE=success", "SHADOW_BUILD=failure", "GITHUB_STEP_SUMMARY="+summary)
	out, err := cmd.Output()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Fatalf("exit %v, want 1:\n%s", err, out)
	}
	want := "::error title=CI pin unreachable (CIN-V0-002)::CORVINT_PR_TOOL_SOURCE " + strings.Repeat("b", 40) + " failed to fetch;" +
		" every forward row of this run is INCOMPLETE until it is repinned\n" +
		"::error title=CI pin unreachable (CIN-V0-002)::CORVINT_TEST_SHADOW_SOURCE " + strings.Repeat("c", 40) + " failed to build;" +
		" every forward row of this run is INCOMPLETE until it is repinned\n"
	if string(out) != want {
		t.Fatalf("annotations:\n%s\nwant:\n%s", out, want)
	}
}

// TestCINV0003VerdictListsBackToTheEpochSince pins CIN-V0-003 and CIN-V0-004 (V1-3091): with
// FORWARD_EPOCH_SINCE named, verdict.sh stops after the first page that reaches it, a listing
// that reached it counts as reaching the epoch's start, and the verdict records when its
// listing began and the names of its AFP-V0-030 window rows.
func TestCINV0003VerdictListsBackToTheEpochSince(t *testing.T) {
	since := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	// Page 1: 22 epoch rows, five failing, after the epoch began; page 2: artifacts of other
	// branches from before it; page 3, were it read, ends the listing.
	row := func(name string, created time.Time, branch string) doc {
		return doc{"name": name, "expired": false, "created_at": created.Format(time.RFC3339),
			"workflow_run": doc{"head_branch": branch, "repository_id": 1, "head_repository_id": 1}}
	}
	verdict := func(t *testing.T, sinceEnv string, pages string, missOld bool) (doc, string, int, error) {
		dir := t.TempDir()
		gh := fakeGH(t, dir)
		var p1, p2 []any
		for pr := 1; pr <= 22; pr++ {
			f := "0"
			if pr <= 5 {
				f = "1"
			}
			p1 = append(p1, row(barNames("CLEAN", pr, "1", f), since.Add(time.Duration(30-pr)*time.Hour), "main"))
		}
		for len(p1) < 100 {
			p1 = append(p1, row(fmt.Sprintf("other-%d", len(p1)), since.Add(time.Hour), "claude/x"))
		}
		for len(p2) < 100 {
			p2 = append(p2, row(fmt.Sprintf("old-%d", len(p2)), since.Add(-time.Hour), "claude/x"))
		}
		if missOld {
			// A MISS of the epoch older than FORWARD_EPOCH_SINCE cannot exist (CIN-V0-003);
			// here it shows the whole listing still reads past the page that reached it.
			p2[99] = row(barNames("MISS", 99, "1", "1"), since.Add(-2*time.Hour), "main")
		}
		writeDoc(t, filepath.Join(dir, "all-artifacts-1.json"), doc{"artifacts": p1})
		writeDoc(t, filepath.Join(dir, "all-artifacts-2.json"), doc{"artifacts": p2})
		writeDoc(t, filepath.Join(dir, "all-artifacts.json"), doc{"artifacts": p1})
		cmd := exec.Command("sh", script(t, "verdict.sh"))
		cmd.Env = []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
			"GH_TOKEN=x", "REPO=beamfall/corvint", "FORWARD_EPOCH=1", "RUNNER_TEMP=" + dir, "GITHUB_REF=refs/heads/main",
			"GITHUB_SHA=" + queueSHA, "GITHUB_RUN_ID=4242", "GITHUB_RUN_ATTEMPT=1", "VERDICT_PAGES=" + pages,
			"GITHUB_WORKFLOW_REF=beamfall/corvint/.github/workflows/ci-forward-bar.yml@refs/heads/main",
			"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary")}
		if sinceEnv != "" {
			cmd.Env = append(cmd.Env, "FORWARD_EPOCH_SINCE="+sinceEnv)
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, "", 0, fmt.Errorf("%v: %s", err, out)
		}
		r, _ := os.ReadFile(filepath.Join(dir, "requests"))
		summary, _ := os.ReadFile(filepath.Join(dir, "summary"))
		return readDoc(t, filepath.Join(dir, "afp-shadow-bar", "verdict.json")), string(summary), strings.Count(string(r), "\n"), nil
	}
	stamp := since.Format(time.RFC3339)
	v, summary, reads, err := verdict(t, stamp, "400", false)
	if err != nil {
		t.Fatal(err)
	}
	var window []string
	for pr := 1; pr <= 20; pr++ {
		f := "0"
		if pr <= 5 {
			f = "1"
		}
		window = append(window, barNames("CLEAN", pr, "1", f))
	}
	if v["status"] != "MET" || v["covered"] != true || v["exhausted"] != false || v["reachedEpochSince"] != true || v["epochSince"] != stamp ||
		num(v["pages"]) != 2 || reads != 2 || fmt.Sprint(v["windowNames"]) != fmt.Sprint(window) ||
		!regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`).MatchString(fmt.Sprint(v["listedAt"])) ||
		!strings.Contains(summary, "2 page(s), back to "+stamp) {
		t.Fatalf("verdict after %d reads = %v\n%s", reads, v, summary)
	}
	// Without it the listing reads on to its end: the same MET, one more page.
	if v, _, reads, err = verdict(t, "", "400", false); err != nil || v["status"] != "MET" || v["exhausted"] != true || reads != 3 {
		t.Fatalf("verdict without FORWARD_EPOCH_SINCE after %d reads = %v (%v)", reads, v, err)
	}
	if v, _, _, err = verdict(t, "", "400", true); err != nil || v["status"] != "NOT MET" {
		t.Fatalf("whole listing with an old MISS = %v (%v)", v, err)
	}
	// A budget that ends before FORWARD_EPOCH_SINCE does not reach the epoch's start.
	if v, _, _, err = verdict(t, stamp, "1", false); err != nil || v["status"] != "NOT ESTABLISHED" || v["reachedEpochSince"] != false {
		t.Fatalf("one-page verdict = %v (%v)", v, err)
	}
	if _, _, _, err = verdict(t, "2026-10-30", "400", false); err == nil || !strings.Contains(err.Error(), "is not a UTC time") {
		t.Fatalf("malformed FORWARD_EPOCH_SINCE: %v", err)
	}
	// The live value names a time no later than today's epoch advance (V1-3314).
	b, err := os.ReadFile("../../.github/workflows/ci-forward-bar.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "      FORWARD_EPOCH_SINCE: \"2026-10-30T00:00:00Z\"\n") || liveEpoch(t) != "2" {
		t.Fatal("ci-forward-bar.yml must name FORWARD_EPOCH_SINCE 2026-10-30T00:00:00Z for epoch 2; move both together")
	}
}
