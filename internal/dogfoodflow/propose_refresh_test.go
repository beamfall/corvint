package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0441V0442RefreshRekeysReHunkedNewFile pins V1-3240:
// a plan drafted before a new file was edited again (by a later commit or an
// amend) names a hunk ID the prepared map no longer has. The pre-flight
// refuses it naming the row, the stale ID and the current ID, with a
// `--refresh` fix; `propose-citations --refresh` carries the unchanged cite
// rows verbatim, re-keys the re-hunked row by path and BASE range, and the
// refreshed plan passes. Fails on BASE, which refused without naming the
// current ID and had no --refresh. The re-keyed unknown row's NOTE is carried
// marked stale (DCW-V0-884, V1-3289).
func TestProposeCitationsDCWV0441V0442RefreshRekeysReHunkedNewFile(t *testing.T) {
	const logPath = "docs/build-log/2026-10-30-x.md"
	for _, mode := range []string{"commit", "amend"} {
		t.Run(mode, func(t *testing.T) {
			root, base := preflightRepository(t, map[string]string{logPath: "# Entry\n\nFirst draft.\n"})
			var stdout, stderr bytes.Buffer
			if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
				t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
			}
			// The author replaces the pre.go unknown rows with cites.
			var plan []string
			oldLogID := ""
			cites := []string{"pre.go\t3:3\timplementation", "pre.go\t20:20\timplementation"}
			for _, row := range textLines(stdout.Bytes()) {
				id, _, _ := strings.Cut(row, "\t")
				if strings.Contains(stderr.String(), "hunk "+id+" edits "+logPath+" at BASE ") {
					oldLogID = id
					plan = append(plan, row)
					continue
				}
				plan = append(plan, id+"\t"+cites[0])
				cites = cites[1:]
			}
			if oldLogID == "" || len(plan) != 3 || len(cites) != 0 {
				t.Fatalf("draft rows=%q log=%q\n%s", plan, oldLogID, stderr.String())
			}
			planPath := writeInput(t, strings.Join(plan, "\n")+"\n")
			// The new file is edited after the plan was drafted.
			writeTree(t, root, map[string]string{logPath: "# Entry\n\nFirst draft.\n\nSecond thought.\n"})
			testGit(t, root, "add", ".")
			if mode == "amend" {
				testGit(t, root, "commit", "-q", "--amend", "--no-edit")
			} else {
				testGit(t, root, "commit", "-q", "-m", "edit the entry")
			}
			code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: planPath})
			if code == 0 || !strings.Contains(out+errText, "citation-plan-map-mismatch") {
				t.Fatalf("stale plan was not refused: code=%d\nstdout=%s\nstderr=%s", code, out, errText)
			}
			row := 0
			for index, line := range plan {
				if strings.HasPrefix(line, oldLogID+"\t") {
					row = index + 1
				}
			}
			detail := "row " + string(rune('0'+row)) + " hunk ID " + oldLogID + " is not in the prepared map: a later edit or amend re-hunked it (DCW-V0-441)"
			for _, want := range []string{detail, "; current hunk ID: hunk ", `path="` + logPath + `"`, "propose-citations " + base + " --refresh " + planPath} {
				if !strings.Contains(out+errText, want) {
					t.Errorf("refusal lacks %q:\nstdout=%s\nstderr=%s", want, out, errText)
				}
			}

			stdout.Reset()
			stderr.Reset()
			if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, Refresh: planPath}, &stdout, &stderr); code != 0 || err != nil {
				t.Fatalf("ProposeCitations --refresh = %d, %v\n%s", code, err, stderr.String())
			}
			refreshed := textLines(stdout.Bytes())
			for _, line := range plan {
				if strings.Contains(line, "\tpre.go\t") && !containsLine(refreshed, line) {
					t.Errorf("unchanged row %q was not carried verbatim:\n%s", line, stdout.String())
				}
			}
			if containsPrefix(refreshed, oldLogID+"\t") || !strings.Contains(stderr.String(), "re-keyed "+oldLogID+" -> hunk:sha256:") ||
				!strings.Contains(stderr.String(), "content changed, so review the row") || !strings.Contains(stderr.String(), `refreshed="`+planPath+`" carried=3 rekeyed=1 dropped=0`) {
				t.Errorf("refresh did not re-key the stale row:\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
			}
			// DCW-V0-884 (V1-3289): the re-keyed unknown row's NOTE was
			// written for the old hunk, so it is carried marked stale.
			stale := "\tunknown\t"
			for _, line := range refreshed {
				if strings.Contains(line, "\tunknown\t") {
					stale = line
				}
			}
			if fields := strings.Split(stale, "\t"); len(fields) != 4 || !strings.HasPrefix(fields[3], "[stale: re-keyed from "+oldLogID+", review] ") ||
				!strings.Contains(stderr.String(), "note marked stale: it was written for "+oldLogID) {
				t.Errorf("re-keyed unknown row note not marked stale: %q\n%s", stale, stderr.String())
			}
			next := filepath.Join(filepath.Dir(planPath), "plan.new")
			if err := os.WriteFile(next, stdout.Bytes(), 0o666); err != nil {
				t.Fatal(err)
			}
			if code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: next}); code != 0 {
				t.Fatalf("refreshed plan refused: code=%d\nstdout=%s\nstderr=%s", code, out, errText)
			}
		})
	}
}

func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

func containsPrefix(lines []string, prefix string) bool {
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// TestDCWV0884StaleNoteMarkIsReplaced: a second re-key replaces the earlier
// stale mark rather than stacking marks on the NOTE.
func TestDCWV0884StaleNoteMarkIsReplaced(t *testing.T) {
	note := staleNotePrefix + "hunk:a, review] why"
	if got := staleNotePrefix + "hunk:b, review] " + staleNote.ReplaceAllString(note, ""); got != staleNotePrefix+"hunk:b, review] why" {
		t.Errorf("re-marked note = %q", got)
	}
}
