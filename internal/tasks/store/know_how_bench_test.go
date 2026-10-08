package store_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// knowHowBenchRepo commits 32 Go files of 40 functions each, then edits
// every file's first function in a second commit, so every pinned blob
// differs from HEAD. It returns the root, the paths and each file's
// first-commit blob.
func knowHowBenchRepo(b *testing.B) (string, []string, []string) {
	b.Helper()
	root := b.TempDir()
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			b.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	write := func(edit bool) []string {
		var paths []string
		for f := range 32 {
			var src strings.Builder
			fmt.Fprintf(&src, "package p%d\n", f)
			for fn := range 40 {
				body := "return 1"
				if edit && fn == 0 {
					body = "return 2"
				}
				fmt.Fprintf(&src, "\n// F%d documents itself.\nfunc F%d() int {\n\t%s\n}\n", fn, fn, body)
			}
			p := fmt.Sprintf("pkg/f%02d.go", f)
			if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
				b.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, p), []byte(src.String()), 0o644); err != nil {
				b.Fatal(err)
			}
			paths = append(paths, p)
		}
		return paths
	}
	paths := write(false)
	git("add", ".")
	git("commit", "-q", "-m", "base")
	blobs := make([]string, len(paths))
	for i, p := range paths {
		blobs[i] = git("rev-parse", "HEAD:"+p)
	}
	write(true)
	git("commit", "-q", "-am", "edit")
	return root, paths, blobs
}

// BenchmarkKnowHowFreshness32FileAnchors is the KHN-V0-005 read at the cap:
// 32 notes of 4 file anchors over 32 changed files (all STALE).
func BenchmarkKnowHowFreshness32FileAnchors(b *testing.B) {
	root, paths, blobs := knowHowBenchRepo(b)
	notes := make([]store.KnowHowNote, 32)
	for n := range notes {
		var anchors []ticket.KnowHowAnchor
		for j := range 4 {
			f := (n + j*8) % 32
			anchors = append(anchors, ticket.KnowHowAnchor{Path: paths[f], Blob: blobs[f]})
		}
		notes[n] = store.KnowHowNote{TicketID: "T", Entry: knowHowEntry(int64(n+1), "2026-10-07T00:00:00Z", anchors...)}
	}
	b.ResetTimer()
	for b.Loop() {
		store.ResolveKnowHowFreshness(root, notes)
	}
}

// BenchmarkKnowHowFreshness32SymbolAnchors is the KHN-V0-009 worst case at
// the cap: 32 notes of 4 symbol anchors, pinned at the base commit, whose
// files all changed elsewhere, so every blob differs and every declaration
// is re-extracted (all CURRENT).
func BenchmarkKnowHowFreshness32SymbolAnchors(b *testing.B) {
	root, paths, _ := knowHowBenchRepo(b)
	notes := make([]store.KnowHowNote, 32)
	for n := range notes {
		var anchors []ticket.KnowHowAnchor
		for j := range 4 {
			anchors = append(anchors, ticket.KnowHowAnchor{Path: paths[n], Symbol: fmt.Sprintf("F%d", 1+j*9)})
		}
		_, pinned, err := store.KnowHowPinAnchors(root, "HEAD~1", anchors)
		if err != nil {
			b.Fatal(err)
		}
		notes[n] = store.KnowHowNote{TicketID: "T", Entry: knowHowEntry(int64(n+1), "2026-10-07T00:00:00Z", pinned...)}
	}
	store.ResolveKnowHowFreshness(root, notes)
	if notes[0].Freshness != store.KnowHowCurrent {
		b.Fatalf("symbol anchors read %s", notes[0].Freshness)
	}
	b.ResetTimer()
	for b.Loop() {
		store.ResolveKnowHowFreshness(root, notes)
	}
}
