package cli_test

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0738_RebuildClosedIndexMatchesJournaled pins V1-3485: the CLI
// rewrites the index of a journaled store after a real `ticket archive`;
// `doctor --rebuild-closed-index` run on a journal-free copy of the same
// intent tree writes the same entries, and the same header but for the capture
// time and so its checksum (TCL-V0-738, TCX-V0-988). A directory with no store
// and an invalid tree refuse with the read's own code and write nothing.
func TestTCLV0738_RebuildClosedIndexMatchesJournaled(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	src := fixture.TempRepo(t)
	fixture.WriteWritableState(t, src)
	reason := "done"
	var recs []*ticket.Record
	for _, local := range []string{"A", "B", "C"} {
		rec := fixture.Ticket(local)
		if local != "B" {
			rec.Status = ticket.StatusCompleted
			rec.Completion = &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
		}
		recs = append(recs, rec)
	}
	recs[2].Title = `quoted "title"`
	fixture.WriteIntent(t, src, recs...)
	posts := map[string][]byte{}
	for _, rec := range recs {
		posts["intent/tickets/"+rec.TicketID.Local+".json"] = rec.Encode()
	}
	fixture.CommitPosts(t, src, "MUTATION", "", posts)
	old := time.Now().Add(-time.Hour)
	for _, rec := range recs {
		if err := os.Chtimes(filepath.Join(src.IntentDir, intent.TicketsDir, rec.TicketID.Local+".json"), old, old); err != nil {
			t.Fatal(err)
		}
	}
	run := func(cwd string, stdin string, args ...string) *wire.Result {
		t.Helper()
		var out bytes.Buffer
		cli.Run(cli.Env{Cwd: cwd, Args: args, Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: io.Discard})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out.Bytes())
		}
		return res
	}
	if res := run(src.Root, `{"reason":"gone"}`, "ticket", "archive", "--target", "B", "--expected-revision", "1", "--request-id", "x", "--payload-stdin"); res.Outcome != wire.OutcomeOK {
		t.Fatalf("archive: %+v", res)
	}
	journaled, err := os.ReadFile(intent.ClosedIndexPath(src.StateDir))
	if err != nil {
		t.Fatalf("the CLI did not write the index: %v", err)
	}

	dst := fixture.TempRepo(t)
	copyIntentTree(t, src.IntentDir, dst.IntentDir)
	// B was archived just now, so the CLI left it out of the racy window; a
	// future time keeps it out of the copy's index whatever the test's pace.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dst.IntentDir, intent.TicketsDir, "B.json"), future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst.StateDir); !os.IsNotExist(err) {
		t.Fatalf("copy has a state directory: %v", err)
	}
	res := run(dst.Root, "", "doctor", "--rebuild-closed-index")
	if res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
		t.Fatalf("rebuild: %+v", res)
	}
	path := intent.ClosedIndexPath(dst.StateDir)
	if got, _ := res.Items[0].Obj.Get("path"); got.Str != path {
		t.Fatalf("rebuild reported path %q, want %q", got.Str, path)
	}
	if got, _ := res.Items[0].Obj.Get("journal"); got.Str != "ABSENT" {
		t.Fatalf("journal = %q, want ABSENT", got.Str)
	}
	built, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := res.Items[0].Obj.Get("indexSha256"); got.Str != string(wire.Sum(built)) {
		t.Fatalf("indexSha256 %q does not match the written file", got.Str)
	}
	jh, jb, _ := bytes.Cut(journaled, []byte("\n"))
	bh, bb, _ := bytes.Cut(built, []byte("\n"))
	jf, bf := strings.Split(string(jh), "\t"), strings.Split(string(bh), "\t")
	if !bytes.Equal(jb, bb) || len(jf) != 5 || len(bf) != 5 || jf[0] != bf[0] || jf[1] != bf[1] || jf[2] != bf[2] || jf[4] != bf[4] {
		t.Fatalf("rebuilt index differs from the journaled one:\n%s\n---\n%s", journaled, built)
	}
	if !bytes.Contains(jb, []byte("tickets/A.json\t")) || !bytes.Contains(jb, []byte("tickets/C.json\t")) || bytes.Contains(jb, []byte("tickets/B.json\t")) {
		t.Fatalf("unexpected entries:\n%s", jb)
	}

	// The journaled store rebuilds too and reports its journal.
	if res := run(src.Root, "", "doctor", "--rebuild-closed-index"); res.Outcome != wire.OutcomeOK {
		t.Fatalf("journaled rebuild: %+v", res)
	} else if got, _ := res.Items[0].Obj.Get("journal"); got.Str != "PRESENT" {
		t.Fatalf("journaled journal = %q", got.Str)
	}

	// No store: refused with the resolve error, nothing written.
	empty := fixture.TempRepo(t)
	if res := run(empty.Root, "", "doctor", "--rebuild-closed-index"); res.Outcome == wire.OutcomeOK || len(res.Codes) == 0 {
		t.Fatalf("a repository without a store rebuilt: %+v", res)
	}
	if _, err := os.Stat(intent.ClosedIndexPath(empty.StateDir)); !os.IsNotExist(err) {
		t.Fatalf("a refused rebuild wrote an index: %v", err)
	}

	// An invalid tree: refused with the load's code, nothing written.
	bad := fixture.TempRepo(t)
	copyIntentTree(t, src.IntentDir, bad.IntentDir)
	fixture.Write(t, filepath.Join(bad.IntentDir, intent.TicketsDir, "A.json"), []byte("{not json"))
	res = run(bad.Root, "", "doctor", "--rebuild-closed-index")
	if res.Outcome == wire.OutcomeOK || len(res.Codes) == 0 {
		t.Fatalf("an invalid tree rebuilt: %+v", res)
	}
	if _, err := os.Stat(intent.ClosedIndexPath(bad.StateDir)); !os.IsNotExist(err) {
		t.Fatalf("a refused rebuild wrote an index: %v", err)
	}

	// The flag stands alone.
	if res := run(dst.Root, "", "doctor", "--rebuild-closed-index", "--refresh"); res.Outcome == wire.OutcomeOK {
		t.Fatalf("--rebuild-closed-index with another flag was accepted")
	}
}

// copyIntentTree copies the regular files under from to to, keeping each
// file's modification time so the copy's stat keys match the source's.
func copyIntentTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dest := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, raw, 0o644); err != nil {
			return err
		}
		return os.Chtimes(dest, info.ModTime(), info.ModTime())
	})
	if err != nil {
		t.Fatal(err)
	}
}
