//go:build darwin || linux

package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// fakeBatchGit puts a `git` first on PATH that runs batch for
// `cat-file --batch` and execs the real Git otherwise. batch reads the
// request on stdin; REAL names the real Git.
func fakeBatchGit(t *testing.T, batch string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nREAL=" + real + "\nif [ \"$3\" = cat-file ] && [ \"$4\" = --batch ]; then\n" + batch + "\nfi\nexec \"$REAL\" \"$@\"\n"
	if err := fixture.WriteExecutable(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// blobReadCheckout commits small.go and big.go (past
// knowHowSymbolMaxBlobBytes), pins a symbol anchor in each at that commit,
// then edits both so freshness must read their HEAD blobs.
func blobReadCheckout(t *testing.T) (root, bigOid string, notes []KnowHowNote) {
	t.Helper()
	root = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir, c.Env = root, gitEnvironment()
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(small, big string) {
		t.Helper()
		for name, body := range map[string]string{"small.go": small, "big.go": big} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		git("add", "-A")
		git("-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "c")
	}
	git("init", "-q", "-b", "main")
	commit("package p\n\nfunc S() int { return 1 }\n", "package p\n\nfunc B() int { return 1 }\n")
	_, pins, err := KnowHowPinAnchors(root, "HEAD", []ticket.KnowHowAnchor{{Path: "small.go", Symbol: "S"}, {Path: "big.go", Symbol: "B"}})
	if err != nil {
		t.Fatal(err)
	}
	commit("package p\n\nvar V = 1\n\nfunc S() int { return 1 }\n",
		"package p\n\nfunc B() int { return 1 }\n\n// "+strings.Repeat("x", knowHowSymbolMaxBlobBytes)+"\n")
	for i, a := range pins {
		notes = append(notes, KnowHowNote{TicketID: "T", Entry: ticket.KnowHowEntry{Seq: wire.CountOf(int64(i + 1)), Operation: ticket.KnowHowAdd,
			Text: "note", Anchors: []ticket.KnowHowAnchor{a}, Routes: []string{}, Commit: strings.Repeat("c", 40), ActorID: "russell",
			ActorRole: "OWNER", RecordedAt: "2026-10-01T00:00:00Z"}})
	}
	return root, git("rev-parse", "HEAD:big.go"), notes
}

func freshnessOf(notes []KnowHowNote) string {
	var s []string
	for _, n := range notes {
		s = append(s, n.Freshness)
	}
	return strings.Join(s, ",")
}

// TestV13019_OversizedSymbolBlobsAreNeverRequested: a symbol anchor whose
// HEAD blob is larger than knowHowSymbolMaxBlobBytes by its --batch-check
// size reads UNKNOWN without its content ever being requested from
// `git cat-file --batch`, while a small changed blob is still read; pinning a
// symbol in the oversized file is refused KNOWHOW_UNRESOLVED, again without
// requesting it (V1-3019, KHN-V0-017).
func TestV13019_OversizedSymbolBlobsAreNeverRequested(t *testing.T) {
	root, bigOid, notes := blobReadCheckout(t)
	log := filepath.Join(t.TempDir(), "requested")
	fakeBatchGit(t, "  tee -a "+log+" | \"$REAL\" \"$@\"\n  exit $?")
	ResolveKnowHowFreshness(root, notes)
	if got := freshnessOf(notes); got != "CURRENT,UNKNOWN" {
		t.Fatalf("freshness: %s", got)
	}
	if _, _, err := KnowHowPinAnchors(root, "HEAD", []ticket.KnowHowAnchor{{Path: "big.go", Symbol: "B"}}); err == nil || !strings.Contains(err.Error(), KnowHowUnresolved) {
		t.Fatalf("pin in an oversized file: %v", err)
	}
	requested, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(requested), bigOid) {
		t.Fatalf("the oversized blob %s was requested:\n%s", bigOid, requested)
	}
	if !strings.Contains(string(requested), "\n") {
		t.Fatal("the small changed blob was not read")
	}
}

// TestV13019_BlobReadEndsAtItsDeadline: a `git cat-file --batch` that never
// answers is killed at knowHowBlobReadBound, and its symbol anchors read
// UNKNOWN, as for any failed read (V1-3019, KHN-V0-017).
func TestV13019_BlobReadEndsAtItsDeadline(t *testing.T) {
	root, _, notes := blobReadCheckout(t)
	fakeBatchGit(t, "  exec /bin/sleep 600")
	bound := knowHowBlobReadBound
	knowHowBlobReadBound = time.Second
	t.Cleanup(func() { knowHowBlobReadBound = bound })
	started := time.Now()
	ResolveKnowHowFreshness(root, notes)
	if took := time.Since(started); took > 20*time.Second {
		t.Fatalf("the blob read took %s", took)
	}
	if got := freshnessOf(notes); got != "UNKNOWN,UNKNOWN" {
		t.Fatalf("freshness: %s", got)
	}
}
