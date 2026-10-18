package sharedpath

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// commitWithCommitterDate commits files with an explicit committer time.
func (f *fixture) commitWithCommitterDate(msg, committed string, files map[string]string) string {
	f.t.Helper()
	for name, content := range files {
		f.write(name, content)
	}
	f.git("add", "-A")
	cmd := exec.Command("git", "-C", f.root, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit", "-q", "-m", msg)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Ada Author", "GIT_AUTHOR_EMAIL=ada@example.invalid", "GIT_AUTHOR_DATE="+committed,
		"GIT_COMMITTER_NAME=Cy Committer", "GIT_COMMITTER_EMAIL=cy@example.invalid", "GIT_COMMITTER_DATE="+committed)
	if out, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("commit: %v %s", err, out)
	}
	return f.git("rev-parse", "HEAD")
}

// repositoryState digests every file under .git by path, mode and content.
func (f *fixture) repositoryState() map[string]string {
	f.t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(filepath.Join(f.root, ".git"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		digest := info.Mode().String()
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(raw)
			digest += " " + hex.EncodeToString(sum[:]) + " " + info.ModTime().String()
		}
		state[p] = digest
		return nil
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return state
}

// behindFixture: feature branches from base and edits a shared helper; main
// then gains a feature commit, a mixed shared commit on the same helper, a
// generated-only commit, a shared script commit and a merge of a side branch.
func behindFixture(t *testing.T) (*fixture, map[string]string) {
	f := newFixture(t)
	f.commit("base", map[string]string{"internal/testutil/a.go": "a\n", "cmd/x.go": "x\n", "docs/specs/REQUIREMENTS.tsv": "r\n"})
	f.git("checkout", "-q", "-b", "feature")
	f.commit("feat: use helper", map[string]string{"internal/testutil/a.go": "a\nfeature\n", "cmd/f.go": "f\n"})
	f.git("checkout", "-q", "main")
	ids := map[string]string{}
	ids["feature-only"] = f.commit("main: feature", map[string]string{"cmd/x.go": "main x\n"})
	ids["mixed"] = f.commit("shared-path: helper", map[string]string{"internal/testutil/a.go": "main a\n", "cmd/y.go": "y\n"})
	ids["generated"] = f.commit("regenerate", map[string]string{"docs/specs/REQUIREMENTS.tsv": "main r\n"})
	ids["script"] = f.commit("shared-path: script", map[string]string{"script/run.sh": "run\n"})
	f.git("checkout", "-q", "-b", "side")
	ids["side"] = f.commit("side: z", map[string]string{"cmd/z.go": "z\n"})
	f.git("checkout", "-q", "main")
	f.git("-c", "user.useConfigOnly=false", "merge", "-q", "--no-ff", "-m", "merge side", "side")
	ids["merge"] = f.git("rev-parse", "HEAD")
	return f, ids
}

func TestSPCV0009BehindCountsOnlySharedCommits(t *testing.T) {
	f, ids := behindFixture(t)
	r, err := Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: "feature", Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if r.Counted != 6 || !reflect.DeepEqual(r.SkippedMerges, []string{ids["merge"]}) || len(r.Behind) != 2 {
		t.Fatalf("counted %d, skipped %v, behind %+v", r.Counted, r.SkippedMerges, r.Behind)
	}
	mixed, script := r.Behind[0], r.Behind[1]
	if mixed.Commit != ids["mixed"] || mixed.Subject != "shared-path: helper" || !reflect.DeepEqual(mixed.SharedPaths, []string{"internal/testutil/a.go"}) ||
		!reflect.DeepEqual(mixed.HeadTouches, []string{"internal/testutil/a.go"}) || !mixed.ConflictRisk || mixed.CommittedAt != "2026-02-03T11:05:06Z" {
		t.Fatalf("mixed: %+v", mixed)
	}
	if script.Commit != ids["script"] || !reflect.DeepEqual(script.SharedPaths, []string{"script/run.sh"}) || len(script.HeadTouches) != 0 || script.ConflictRisk {
		t.Fatalf("script: %+v", script)
	}
	if _, err := Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: "no-such-branch", Config: testConfig()}); err == nil || err.Error() != CodeRangeUnavailable {
		t.Fatalf("unknown head: %v", err)
	}
	orphan := f.git("commit-tree", "-m", "orphan", f.git("rev-parse", "HEAD^{tree}"))
	if _, err := Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: orphan, Config: testConfig()}); err == nil || err.Error() != CodeRangeUnavailable {
		t.Fatalf("no merge base: %v", err)
	}
}

func TestSPCV0009BehindRefusesTooLargeRangeBeforeListing(t *testing.T) {
	f := newFixture(t)
	var stream strings.Builder
	for i := 0; i <= maxCommits+1; i++ {
		fmt.Fprintf(&stream, "commit refs/heads/main\ncommitter C <c@example.invalid> %d +0000\ndata 3\nc%02d\nM 644 inline internal/testutil/a.go\ndata %d\n%d\n\n", 1700000000+i, i%100, len(fmt.Sprint(i))+1, i)
	}
	cmd := exec.Command("git", "-C", f.root, "fast-import", "--quiet")
	cmd.Stdin = strings.NewReader(stream.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fast-import: %v %s", err, out)
	}
	_, err := Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: fmt.Sprintf("main~%d", maxCommits+1), Config: testConfig()})
	var tooLarge *RangeTooLargeError
	if !errors.As(err, &tooLarge) || err.Error() != CodeRangeTooLarge || tooLarge.Limit != maxCommits || tooLarge.Commits != maxCommits+1 {
		t.Fatalf("too large: %v %+v", err, tooLarge)
	}
	r, err := Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: fmt.Sprintf("main~%d", maxCommits), Config: testConfig()})
	if err != nil || r.Counted != maxCommits || len(r.Behind) != maxCommits {
		t.Fatalf("at the bound: %v counted %d behind %d", err, r.Counted, len(r.Behind))
	}
}

func TestSPCV0009BehindIsReadOnly(t *testing.T) {
	f, _ := behindFixture(t)
	f.git("remote", "add", "origin", filepath.Join(t.TempDir(), "absent.git"))
	before := f.repositoryState()
	r, err := Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: "feature", Ticket: "V1-1710", StaleHours: 1, Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if r.Mutates || r.DryMerge.Result != "conflict" {
		t.Fatalf("report: %+v", r)
	}
	if after := f.repositoryState(); !reflect.DeepEqual(before, after) {
		for p, d := range after {
			if before[p] != d {
				t.Errorf("changed or added: %s", p)
			}
		}
		t.Fatalf("behind wrote repository state: %d -> %d entries", len(before), len(after))
	}
}

func TestSPCV0010DryMergeClassifiesCleanGeneratedOnlyAndConflict(t *testing.T) {
	f := newFixture(t)
	f.commit("base", map[string]string{"docs/specs/REQUIREMENTS.tsv": "a\n", "cmd/x.go": "x\n", "internal/testutil/a.go": "a\n"})
	f.git("checkout", "-q", "-b", "feature")
	f.commit("feat: f", map[string]string{"cmd/f.go": "f\n"})
	f.git("checkout", "-q", "main")
	f.commit("shared-path: helper", map[string]string{"internal/testutil/a.go": "main a\n"})
	dry := func(want string, conflicts, generated []string) {
		t.Helper()
		r, err := Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: "feature", Config: testConfig()})
		if err != nil {
			t.Fatal(err)
		}
		if r.DryMerge.Result != want || !reflect.DeepEqual(r.DryMerge.Conflicts, conflicts) || !reflect.DeepEqual(r.DryMerge.GeneratedConflicts, generated) {
			t.Fatalf("want %s %v %v, got %+v", want, conflicts, generated, r.DryMerge)
		}
	}
	dry("clean", []string{}, []string{})
	f.commit("main: regenerate", map[string]string{"docs/specs/REQUIREMENTS.tsv": "main\n"})
	f.git("checkout", "-q", "feature")
	f.commit("feat: regenerate", map[string]string{"docs/specs/REQUIREMENTS.tsv": "feature\n"})
	dry("generated-only", []string{}, []string{"docs/specs/REQUIREMENTS.tsv"})
	f.commit("feat: edit x and helper", map[string]string{"cmd/x.go": "feature x\n", "internal/testutil/a.go": "feature a\n"})
	f.git("checkout", "-q", "main")
	f.commit("main: edit x", map[string]string{"cmd/x.go": "main x\n"})
	dry("conflict", []string{"cmd/x.go", "internal/testutil/a.go"}, []string{"docs/specs/REQUIREMENTS.tsv"})
}

func TestSPCV0011BehindReportRecordsBaseHeadCommitsAndDigest(t *testing.T) {
	f, ids := behindFixture(t)
	raw := []byte(`{"profile":"corvint-shared-paths/0","shared":["internal/testutil/**","script/*.sh"],"generated":["docs/specs/REQUIREMENTS.tsv"],"marker":"shared-path:"}`)
	file := filepath.Join(t.TempDir(), "shared-paths.json")
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	r, err := Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: "feature", Now: now, Config: c})
	if err != nil {
		t.Fatal(err)
	}
	if r.Profile != BehindProfile || r.Mutates || r.Base != ids["merge"] || r.Head != f.git("rev-parse", "feature") || r.MergeBase != f.git("merge-base", "main", "feature") ||
		r.ConfigDigest != "sha256:"+hex.EncodeToString(sum[:]) || r.ObservedAt != "2026-10-18T12:00:00Z" {
		t.Fatalf("report header: %+v", r)
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(encoded, &v); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"profile", "mutates", "base", "head", "mergeBase", "configDigest", "observedAt", "counted", "behind", "skippedMerges", "dryMerge", "findings"} {
		if _, ok := v[key]; !ok {
			t.Fatalf("report lacks %s: %s", key, encoded)
		}
	}
	commits := v["behind"].([]any)
	if len(commits) != 2 || commits[0].(map[string]any)["commit"] != ids["mixed"] {
		t.Fatalf("commits: %v", commits)
	}
}

func TestSPCV0012StaleFindingNamesTicketOldestCommitAndAge(t *testing.T) {
	f := newFixture(t)
	f.commitWithCommitterDate("base", "2026-02-27T00:00:00Z", map[string]string{"internal/testutil/a.go": "a\n"})
	f.git("branch", "feature")
	f.commitWithCommitterDate("shared-path: newer first", "2026-03-02T00:00:00Z", map[string]string{"internal/testutil/a.go": "b\n"})
	older := f.commitWithCommitterDate("shared-path: older second", "2026-03-01T00:00:00Z", map[string]string{"script/run.sh": "run\n"})
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	behind := func(ticket string, hours int) (BehindReport, error) {
		return Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: "feature", Ticket: ticket, StaleHours: hours, Now: now, Config: testConfig()})
	}
	r, err := behind("V1-1710", 48)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Findings) != 1 {
		t.Fatalf("findings: %+v", r.Findings)
	}
	got := r.Findings[0]
	if got.Kind != StaleKind || got.Who != "V1-1710" || got.Commit != older || got.AgeSeconds != 48*3600 ||
		!strings.Contains(got.Detail, "V1-1710") || !strings.Contains(got.Detail, older) || !strings.Contains(got.Detail, "48 h old") || got.Remedy == "" {
		t.Fatalf("finding: %+v", got)
	}
	if r, err := behind("V1-1710", 49); err != nil || len(r.Findings) != 0 {
		t.Fatalf("below the threshold: %v %+v", err, r.Findings)
	}
	if r, err := Behind(context.Background(), BehindRequest{Root: f.root, Base: "main", Head: "feature", Now: now, Config: testConfig()}); err != nil || len(r.Findings) != 0 {
		t.Fatalf("without a ticket: %v %+v", err, r.Findings)
	}
	for _, bad := range []struct {
		ticket string
		hours  int
	}{{"V1-1710", 0}, {"", 4}, {"-bad", 4}, {"V1 1710", 4}, {"V1-1710", 8761}} {
		if _, err := behind(bad.ticket, bad.hours); err == nil || err.Error() != CodeInvalidArguments {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
}
