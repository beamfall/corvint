// SPDX-License-Identifier: Apache-2.0

package main

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

// TestTCXV0_ToolIndexMatchesJournaled: the CLI rewrites the index of a
// journaled store after a real `ticket archive`; the tool run on a
// journal-free copy of the same intent tree writes the same entries, and the
// same header but for the capture time and so its checksum (TCX-V0-988).
func TestTCXV0_ToolIndexMatchesJournaled(t *testing.T) {
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
	var out bytes.Buffer
	cli.Run(cli.Env{Cwd: src.Root, Args: []string{"ticket", "archive", "--target", "B", "--expected-revision", "1", "--request-id", "x", "--payload-stdin"}, Stdin: strings.NewReader(`{"reason":"gone"}`), Stdout: &out, Stderr: io.Discard})
	if res, err := wire.DecodeResult(out.Bytes()); err != nil || res.Outcome != wire.OutcomeOK {
		t.Fatalf("archive: %s", out.Bytes())
	}
	journaled, err := os.ReadFile(intent.ClosedIndexPath(src.StateDir))
	if err != nil {
		t.Fatalf("the CLI did not write the index: %v", err)
	}

	dst := fixture.TempRepo(t)
	copyTree(t, src.IntentDir, dst.IntentDir)
	// B was archived just now, so the CLI left it out of the racy window; a
	// future time keeps it out of the copy's index whatever the test's pace.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dst.IntentDir, intent.TicketsDir, "B.json"), future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst.StateDir); !os.IsNotExist(err) {
		t.Fatalf("copy has a state directory: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{dst.Root}, &stdout, &stderr); code != 0 {
		t.Fatalf("tool exit %d: %s", code, stderr.Bytes())
	}
	path := intent.ClosedIndexPath(dst.StateDir)
	if strings.TrimSpace(stdout.String()) != path {
		t.Fatalf("tool printed %q, want %q", stdout.String(), path)
	}
	built, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	jh, jb, _ := bytes.Cut(journaled, []byte("\n"))
	bh, bb, _ := bytes.Cut(built, []byte("\n"))
	jf, bf := strings.Split(string(jh), "\t"), strings.Split(string(bh), "\t")
	if !bytes.Equal(jb, bb) || len(jf) != 5 || len(bf) != 5 || jf[0] != bf[0] || jf[1] != bf[1] || jf[2] != bf[2] || jf[4] != bf[4] {
		t.Fatalf("tool index differs from the journaled one:\n%s\n---\n%s", journaled, built)
	}
	if !bytes.Contains(jb, []byte("tickets/A.json\t")) || !bytes.Contains(jb, []byte("tickets/C.json\t")) || bytes.Contains(jb, []byte("tickets/B.json\t")) {
		t.Fatalf("unexpected entries:\n%s", jb)
	}

	empty := t.TempDir()
	if code := run([]string{empty}, io.Discard, io.Discard); code != 1 {
		t.Fatalf("a directory without a store exited %d", code)
	}
	if code := run(nil, io.Discard, io.Discard); code != 2 {
		t.Fatalf("no arguments exited %d", code)
	}
}

// copyTree copies the regular files under from to to, keeping each file's
// modification time so the copy's stat keys match the source's.
func copyTree(t *testing.T, from, to string) {
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
