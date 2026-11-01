package cem2proto

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// committedChanges is the repository's sealed CEM corpus. One literal names
// it, so the affected planner bounds this package's corpus reads by path. The
// only other input is Git objects named by those maps' base and target
// commits; they are content-addressed, so no working-tree edit can change
// them. Git runs from this directory and the projection opens the repository
// that holds it, so no test literal names the repository root.
var committedChanges = filepath.FromSlash("../../.corvint/changes")

// corpusRepository is the repository holding committedChanges.
func corpusRepository() string { return filepath.Dir(filepath.Dir(committedChanges)) }

// corpusSampleSize bounds the default round-trip; CEM2PROTO_CORPUS=all
// projects every committed map.
const corpusSampleSize = 16

// notRun refuses a silent pass in CI and records NOT_RUN elsewhere.
func notRun(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("CI") == "true" {
		t.Fatalf(format, args...)
	}
	t.Skipf("NOT_RUN: "+format, args...)
}

type corpusMap struct {
	name, target string
	raw          []byte
}

func readCorpus(t *testing.T) []corpusMap {
	t.Helper()
	entries, err := os.ReadDir(committedChanges)
	if err != nil {
		notRun(t, "committed CEM corpus unreadable: %v", err)
	}
	var out []corpusMap
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".cem.json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(committedChanges, name))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, corpusMap{name: name, target: strings.TrimSuffix(name, ".cem.json"), raw: raw})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	if len(out) == 0 {
		notRun(t, "no committed CEM maps under %s", committedChanges)
	}
	return out
}

func hasCommit(oid string) bool {
	return exec.Command("git", "-C", committedChanges, "cat-file", "-e", oid+"^{commit}").Run() == nil
}

// CEM2-V0-005, -021: committed maps project back byte-identically from Git.
func TestCEM2V0005CorpusRoundTrip(t *testing.T) {
	corpus := readCorpus(t)
	sample := corpus
	if os.Getenv("CEM2PROTO_CORPUS") != "all" && len(corpus) > corpusSampleSize {
		sample = nil
		for i := 0; i < corpusSampleSize; i++ {
			sample = append(sample, corpus[i*len(corpus)/corpusSampleSize])
		}
	}
	var projected, abstained, unavailable int
	var elapsed time.Duration
	for _, c := range sample {
		stable, err := wire.ParseStable(c.raw)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		header, body, err := Encode(stable, false)
		var typed *Error
		if errors.As(err, &typed) && typed.Unsupported() {
			abstained++
			t.Logf("%s: encoder abstained: %v", c.name, err)
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !hasCommit(stable.Change.BaseRevision) || !hasCommit(c.target) {
			unavailable++
			t.Logf("%s: base or target objects absent; abstained", c.name)
			continue
		}
		start := time.Now()
		src, err := NewGitSource(corpusRepository())
		if err != nil {
			t.Fatal(err)
		}
		got, err := Project(context.Background(), src, c.target, header, body)
		elapsed += time.Since(start)
		if err != nil {
			t.Fatalf("%s: projection refused: %v", c.name, err)
		}
		if !bytes.Equal(got, c.raw) {
			t.Fatalf("%s: projection is not byte-identical (%d vs %d bytes)", c.name, len(got), len(c.raw))
		}
		projected++
	}
	if projected == 0 {
		notRun(t, "no committed map was projectable (abstained=%d unavailable=%d)", abstained, unavailable)
	}
	t.Logf("corpus round-trip: sample=%d of %d projected=%d abstained=%d unavailable=%d total=%s mean=%s",
		len(sample), len(corpus), projected, abstained, unavailable, elapsed.Round(time.Millisecond), (elapsed / time.Duration(projected)).Round(time.Millisecond))
}

func gzipLen(t *testing.T, data []byte) int {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Len()
}

// CEM2-V0-022: V1-3760 sizes on every committed map, without Git. The 2.0
// raw and gzip columns count header plus body; the header line also reports
// the bodies alone. gzip is Go's default level.
func TestCEM2V0022CorpusMeasurements(t *testing.T) {
	corpus := readCorpus(t)
	type form struct {
		raw, gz int
		all     bytes.Buffer
	}
	var v1, derived, pinned form
	var headers, derivedBodies, pinnedBodies, encoded, abstained int
	var sizes []int
	for _, c := range corpus {
		stable, err := wire.ParseStable(c.raw)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		dh, db, err := Encode(stable, false)
		var typed *Error
		if errors.As(err, &typed) && typed.Unsupported() {
			abstained++
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		ph, pb, err := Encode(stable, true)
		if err != nil {
			t.Fatalf("%s: pinned: %v", c.name, err)
		}
		encoded++
		headers += len(dh)
		derivedBodies += len(db)
		pinnedBodies += len(pb)
		sizes = append(sizes, len(c.raw))
		for _, entry := range []struct {
			f    *form
			data []byte
		}{{&v1, c.raw}, {&derived, append(append([]byte(nil), dh...), db...)}, {&pinned, append(append([]byte(nil), ph...), pb...)}} {
			entry.f.raw += len(entry.data)
			entry.f.gz += gzipLen(t, entry.data)
			entry.f.all.Write(entry.data)
		}
	}
	if encoded == 0 {
		notRun(t, "no committed map encoded (abstained=%d)", abstained)
	}
	sort.Ints(sizes)
	pct := func(n int) float64 { return 100 * float64(n) / float64(v1.raw) }
	t.Logf("corpus: maps=%d encoded=%d abstained=%d median1.0=%d B", len(corpus), encoded, abstained, sizes[len(sizes)/2])
	t.Logf("1.0 raw=%d gzip-per-file=%d (%.1f%%) gzip-whole=%d (%.1f%%)", v1.raw, v1.gz, pct(v1.gz), gzipLen(t, v1.all.Bytes()), pct(gzipLen(t, v1.all.Bytes())))
	t.Logf("2.0 derived raw=%d (%.1f%%) gzip-per-file=%d (%.1f%%) gzip-whole=%d (%.1f%%)", derived.raw, pct(derived.raw), derived.gz, pct(derived.gz), gzipLen(t, derived.all.Bytes()), pct(gzipLen(t, derived.all.Bytes())))
	t.Logf("2.0 pinned raw=%d (%.1f%%) gzip-per-file=%d (%.1f%%) gzip-whole=%d (%.1f%%)", pinned.raw, pct(pinned.raw), pinned.gz, pct(pinned.gz), gzipLen(t, pinned.all.Bytes()), pct(gzipLen(t, pinned.all.Bytes())))
	t.Logf("header mean=%.1f B total=%d; bodies alone: derived=%d (%.1f%%) pinned=%d (%.1f%%)", float64(headers)/float64(encoded), headers, derivedBodies, pct(derivedBodies), pinnedBodies, pct(pinnedBodies))
	if derived.raw >= v1.raw || pinned.raw >= v1.raw {
		t.Fatal("the 2.0 forms must be smaller than cem/1.0 on the committed corpus")
	}
}
