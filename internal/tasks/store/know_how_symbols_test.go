package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const symGoV1 = `package sym

// F does f.
func F() int { return 1 }

func G() int { return 2 }

type T struct{}

func (T) M() {}

func init() {}

func init() {}
`

const symPyV1 = "def f():\n    return 1\n\n\ndef g():\n    return 2\n"

func writeCommit(t *testing.T, root, msg string, files map[string]string) string {
	t.Helper()
	for p, body := range files {
		full := filepath.Join(root, p)
		if body == "" {
			gitRun(t, root, "rm", "-q", p)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, root, "add", p)
	}
	gitRun(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", msg)
	return gitOut(t, root, "rev-parse", "HEAD")
}

func symbolAnchors(specs ...string) []ticket.KnowHowAnchor {
	out := make([]ticket.KnowHowAnchor, len(specs))
	for i, s := range specs {
		path, name, _ := strings.Cut(s, "#")
		out[i] = ticket.KnowHowAnchor{Path: path, Symbol: name}
	}
	return out
}

// TestKHNV0016_SymbolPinsReuseTheIndexExtractor: a symbol anchor pins its
// file's blob and the SHA-256 of the one declaration the context index's
// extractor names (a Go method as Receiver.Method, a Python def); a symbol
// that is missing or declared twice, or a file no extractor admits, is
// refused KNOWHOW_UNRESOLVED and nothing is pinned.
func TestKHNV0016_SymbolPinsReuseTheIndexExtractor(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	head := writeCommit(t, root, "base", map[string]string{"sym.go": symGoV1, "lib.py": symPyV1, "notes.txt": "F\n"})
	commit, got, err := store.KnowHowPinAnchors(root, "HEAD", symbolAnchors("lib.py#f", "sym.go", "sym.go#F", "sym.go#T.M"))
	if err != nil || commit != head {
		t.Fatalf("pin: %s %v", commit, err)
	}
	blob := gitOut(t, root, "rev-parse", "HEAD:sym.go")
	if got[1].Blob != blob || got[1].SymbolSha256 != "" || got[2].Blob != blob || got[3].Blob != blob {
		t.Fatalf("blobs: %+v", got)
	}
	if got[2].SymbolSha256 != string(wire.Sum([]byte("// F does f.\nfunc F() int { return 1 }"))) {
		t.Fatalf("F digest is not its declaration text: %+v", got[2])
	}
	if got[0].SymbolSha256 == "" || got[3].SymbolSha256 == "" || got[0].SymbolSha256 == got[3].SymbolSha256 {
		t.Fatalf("digests: %+v", got)
	}
	for name, spec := range map[string]string{
		"missing symbol":   "sym.go#Nope",
		"bare method name": "sym.go#M",
		"declared twice":   "sym.go#init",
		"no extractor":     "notes.txt#F",
		"missing file":     "gone.go#F",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := store.KnowHowPinAnchors(root, "HEAD", symbolAnchors(spec))
			if err == nil || wire.CodeOf(err) != wire.CodeMalformed || !strings.Contains(err.Error(), store.KnowHowUnresolved+":") {
				t.Fatalf("pinned %s: %v", spec, err)
			}
		})
	}
}

// TestKHNV0017_SymbolFreshnessFollowsTheDeclaration: editing another symbol
// in the same file, or moving the pinned one down, leaves a symbol anchor
// CURRENT while the file anchor of that path goes STALE; editing the pinned
// declaration makes it STALE; deleting or renaming it, deleting the file,
// growing it past 1 MiB, or duplicating it reads UNKNOWN, never CURRENT.
func TestKHNV0017_SymbolFreshnessFollowsTheDeclaration(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	writeCommit(t, root, "base", map[string]string{"sym.go": symGoV1, "lib.py": symPyV1})
	_, pins, err := store.KnowHowPinAnchors(root, "HEAD", symbolAnchors("lib.py#f", "lib.py#g", "sym.go", "sym.go#F", "sym.go#G", "sym.go#T.M"))
	if err != nil {
		t.Fatal(err)
	}
	notes := func() []store.KnowHowNote {
		out := make([]store.KnowHowNote, len(pins))
		for i, a := range pins {
			out[i] = store.KnowHowNote{TicketID: "T", Entry: knowHowEntry(int64(i+1), "2026-10-01T00:00:00Z", a)}
		}
		return out
	}
	states := func() string {
		ns := notes()
		store.ResolveKnowHowFreshness(root, ns)
		var s []string
		for _, n := range ns {
			s = append(s, n.Freshness)
		}
		return strings.Join(s, ",")
	}
	if got := states(); got != "CURRENT,CURRENT,CURRENT,CURRENT,CURRENT,CURRENT" {
		t.Fatalf("base: %s", got)
	}

	// Edit G and g; shift F down by inserting a declaration above it.
	editG := strings.Replace(strings.Replace(symGoV1, "return 2", "return 3", 1), "package sym\n", "package sym\n\nvar V = 1\n", 1)
	writeCommit(t, root, "edit other", map[string]string{"sym.go": editG, "lib.py": strings.Replace(symPyV1, "return 2", "return 3", 1)})
	if got := states(); got != "CURRENT,STALE,STALE,CURRENT,STALE,CURRENT" {
		t.Fatalf("after editing G and g: %s", got)
	}

	// Edit F itself.
	writeCommit(t, root, "edit pinned", map[string]string{"sym.go": strings.Replace(editG, "return 1", "return 4", 1)})
	if got := states(); got != "CURRENT,STALE,STALE,STALE,STALE,CURRENT" {
		t.Fatalf("after editing F: %s", got)
	}

	// Rename F, duplicate T.M, and delete lib.py.
	renamed := strings.Replace(strings.Replace(editG, "func F()", "func F2()", 1), "func (T) M() {}\n", "func (T) M() {}\n\nfunc (*T) M() {}\n", 1)
	writeCommit(t, root, "rename", map[string]string{"sym.go": renamed, "lib.py": ""})
	if got := states(); got != "UNKNOWN,UNKNOWN,STALE,UNKNOWN,STALE,UNKNOWN" {
		t.Fatalf("after rename, duplicate and delete: %s", got)
	}

	// A file past the 1 MiB read bound is UNKNOWN even when F is unchanged.
	big := symGoV1 + "\n// " + strings.Repeat("x", 1<<20) + "\n"
	writeCommit(t, root, "big", map[string]string{"sym.go": big})
	if got := states(); !strings.HasSuffix(got, "STALE,UNKNOWN,UNKNOWN,UNKNOWN") {
		t.Fatalf("oversize file: %s", got)
	}
	if _, _, err := store.KnowHowPinAnchors(root, "HEAD", symbolAnchors("sym.go#F")); err == nil || !strings.Contains(err.Error(), store.KnowHowUnresolved) {
		t.Fatalf("pinned in an oversize file: %v", err)
	}

	// No Git: every symbol anchor is UNKNOWN.
	ns := notes()
	store.ResolveKnowHowFreshness("", ns)
	for _, n := range ns {
		if n.Freshness != store.KnowHowUnknown {
			t.Fatalf("no checkout: %s", n.Freshness)
		}
	}
}

// TestKHNV0018_ProjectionShowsEffectivePinsAndProvenance: selection and the
// full projection use a note's latest RECONFIRM pins and name its seq,
// actor, time, attempt and generation; the compact form names the symbol
// but no pins; a note never re-confirmed and without symbols projects as
// before.
func TestKHNV0018_ProjectionShowsEffectivePinsAndProvenance(t *testing.T) {
	blobA, blobB := strings.Repeat("a", 40), strings.Repeat("b", 40)
	d1, d2 := string(wire.Sum([]byte("1"))), string(wire.Sum([]byte("2")))
	add := knowHowEntry(1, "2026-10-01T00:00:00Z", ticket.KnowHowAnchor{Path: "a.go", Blob: blobA, Symbol: "F", SymbolSha256: d1})
	att, gen := "att-7", wire.Size("4")
	re := ticket.KnowHowEntry{Seq: "2", Operation: ticket.KnowHowReconfirm, Note: "1", Commit: strings.Repeat("d", 40), Attempt: &att, Generation: &gen,
		Anchors: []ticket.KnowHowAnchor{{Path: "a.go", Blob: blobB, Symbol: "F", SymbolSha256: d2}},
		ActorID: "ops", ActorRole: "OPERATOR", RecordedAt: "2026-10-05T00:00:00Z"}
	eff := ticket.EffectiveKnowHow([]ticket.KnowHowEntry{add, re})
	n := store.KnowHowNote{TicketID: "T", Entry: eff[0], Freshness: store.KnowHowCurrent, Anchors: []string{store.KnowHowCurrent}}
	full := string(wire.Encode(store.KnowHowNoteValue(n, false)))
	for _, want := range []string{`"blob":"` + blobB, `"symbol":"F"`, `"symbolSha256":"` + d2, `"commit":"dddd`, `"reconfirmed":{"actor":{"id":"ops","role":"OPERATOR"},"attempt":"att-7","generation":"4","recordedAt":"2026-10-05T00:00:00Z","seq":"2"}`, `"recordedAt":"2026-10-01T00:00:00Z"`} {
		if !strings.Contains(full, want) {
			t.Fatalf("full projection lacks %s: %s", want, full)
		}
	}
	compact := string(wire.Encode(store.KnowHowNoteValue(n, true)))
	if !strings.Contains(compact, `"symbol":"F"`) || strings.Contains(compact, "symbolSha256") || strings.Contains(compact, "reconfirmed") || strings.Contains(compact, blobB) {
		t.Fatalf("compact projection: %s", compact)
	}
	plain := store.KnowHowNote{TicketID: "T", Entry: knowHowEntry(1, "2026-10-01T00:00:00Z", ticket.KnowHowAnchor{Path: "a.go", Blob: blobA}),
		Freshness: store.KnowHowCurrent, Anchors: []string{store.KnowHowCurrent}}
	if s := string(wire.Encode(store.KnowHowNoteValue(plain, false))); strings.Contains(s, "symbol") || strings.Contains(s, "reconfirmed") {
		t.Fatalf("a file-anchor note gained keys: %s", s)
	}
}
