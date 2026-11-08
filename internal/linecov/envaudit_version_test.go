// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// GLS-V0-017, GLS-V0-018: an old completeness claim cannot exclude the
// original GOCOVERDIR fixture, even with identical format and runner. An
// unknown audit version also fails closed, through both persisted readers.
func TestEnvAuditVersionAdmission(t *testing.T) {
	requireGo(t)
	t.Setenv("GOCOVERDIR", "")
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"go.mod": fixture["go.mod"], "cov/cov.go": fixture["cov/cov.go"], "cov/cov_test.go": fixture["cov/cov_test.go"],
	})
	git(t, dir, "init", "-q")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "base")
	base := git(t, dir, "rev-parse", "HEAD")
	outcomes, err := Collect(ctx, dir, CollectOptions{Module: "example.com/m", Packages: []string{"cov"}, Work: t.TempDir(), Parallel: 1, Timeout: 5 * time.Minute, TestTimeout: DefaultTestTimeout})
	if err != nil {
		t.Fatal(err)
	}
	var records []Record
	for _, o := range outcomes {
		if o.Record == nil {
			t.Fatalf("%s: %s", o.Name, o.Divergent)
		}
		r := *o.Record
		// Reproduce the old collector: inherited collector variables were
		// omitted, but every record still claimed EnvObserved.
		r.Env = nil
		for _, b := range o.Record.Env {
			if !EnvCollectorModified(b.Name) {
				r.Env = append(r.Env, b)
			}
		}
		records = append(records, r)
	}
	ix, err := Build("example.com/m", base, "fixture-runner", map[string]string{"cov/cov.go": git(t, dir, "rev-parse", base+":cov/cov.go")}, records)
	if err != nil || !ix.EnvBound {
		t.Fatalf("old completeness claim: %v %v", ix, err)
	}
	writeTree(t, dir, map[string]string{"cov/cov.go": strings.Replace(fixture["cov/cov.go"], "return 1", "return 2", 1)})
	git(t, dir, "commit", "-qam", "mutant")
	head := git(t, dir, "rev-parse", "HEAD")
	diff, err := Diff(ctx, dir, base, head)
	if err != nil {
		t.Fatal(err)
	}
	heads := map[string][]string{"cov": {"TestDir", "TestFlags"}}
	input := PlanInput{Base: base, Head: head, Change: diff, ReadDrift: Reader(ctx, dir, base), ReadChange: Reader(ctx, dir, base),
		Units: []Unit{{ID: "go:example.com/m/cov", Dir: "cov", Witness: "cov/cov.go"}},
		Heads: func([]string) (map[string][]string, error) { return heads, nil }, Environ: os.Environ()}
	// Establish the counterexample: trusting the old claim would omit the
	// assertion that actually fails outside collection.
	unsafe := ix.Plan(input)
	if unsafe.State != StateNarrowed || len(unsafe.Units) != 1 || unsafe.Units[0].Mode != Compile {
		t.Fatalf("fixture did not reproduce the old exclusion: %+v", unsafe)
	}
	if !failingTests(t, dir, []string{"cov"})["cov.TestDir"] {
		t.Fatal("GOCOVERDIR fixture did not fail under the ordinary runner")
	}
	for _, version := range []string{"absent", "os-only", "unknown", "duplicate", "misplaced", "current"} {
		t.Run(version, func(t *testing.T) {
			data := auditVersionBytes(t, ix, func(e *extension) {
				switch version {
				case "absent":
					e.Env = e.Env[:len(e.Env)-1]
				case "os-only":
					e.Env[len(e.Env)-1].Value = "2"
				case "unknown":
					e.Env[len(e.Env)-1].Value = "future-audit"
				case "duplicate":
					e.Env = append(e.Env, envAuditMarker)
				case "misplaced":
					e.Env = append(e.Env, EnvBinding{Name: "AFTER_MARKER", Value: EnvUnset})
				}
			})
			file := filepath.Join(t.TempDir(), "index")
			if err := os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			for _, lazy := range []bool{false, true} {
				var got *Index
				if lazy {
					got, err = Open(file)
				} else {
					got, err = Decode(bytes.NewReader(data))
				}
				if err != nil {
					t.Fatal(err)
				}
				defer got.Close()
				if got.EnvBound != (version == "current") {
					t.Fatalf("lazy=%v EnvBound=%v", lazy, got.EnvBound)
				}
				if version == "current" {
					continue // only the exact supported marker admits the claim
				}
				assertUnbound := func(got *Index) {
					t.Helper()
					if sel := got.Plan(input); sel.State != StateFallback || sel.Reason != ReasonEnvironmentUnbound {
						t.Fatalf("old/unknown audit narrowed: %+v", sel)
					}
					plans, reason := got.Narrow(os.Environ(), []string{"cov"}, heads, Effect{}, got.Analyze(diff, Reader(ctx, dir, base)))
					if !strings.HasPrefix(reason, ReasonEnvironmentUnbound) || plans["cov"].Mode != Whole {
						t.Fatalf("old/unknown audit omitted TestDir: %+v %q", plans, reason)
					}
				}
				assertUnbound(got)
				merged, err := Merge(got)
				if err != nil {
					t.Fatal(err)
				}
				var rewritten bytes.Buffer
				if err := merged.Encode(&rewritten); err != nil {
					t.Fatal(err)
				}
				got, err = Decode(&rewritten)
				if err != nil {
					t.Fatal(err)
				}
				assertUnbound(got) // merging/rewriting is not recollection
			}
		})
	}
	for _, format := range []string{FormatV2, FormatV3} {
		ix.Format = format
		var data bytes.Buffer
		if err := ix.Encode(&data); err != nil {
			t.Fatal(err)
		}
		got, err := Decode(&data)
		if err != nil || got.EnvBound {
			t.Fatalf("legacy %s admitted: %v %v", format, got, err)
		}
	}
}

// Rewrite only the gob extension and its enclosing length/checksum, retaining
// a valid /4 file. This reproduces an old writer's bytes without giving the
// production encoder a way to stamp an obsolete audit version.
func auditVersionBytes(t *testing.T, ix *Index, edit func(*extension)) []byte {
	t.Helper()
	var original bytes.Buffer
	if err := ix.Encode(&original); err != nil {
		t.Fatal(err)
	}
	e, _ := extensionOf(ix)
	var before, after bytes.Buffer
	if err := gob.NewEncoder(&before).Encode(e); err != nil {
		t.Fatal(err)
	}
	edit(&e)
	if err := gob.NewEncoder(&after).Encode(e); err != nil {
		t.Fatal(err)
	}
	data := original.Bytes()
	start := len(magicV4) + 8
	n := int(binary.LittleEndian.Uint64(data[len(magicV4):start]))
	h := data[start : start+n]
	tail := append(binary.AppendUvarint(nil, uint64(before.Len())), before.Bytes()...)
	if !bytes.HasSuffix(h, tail) {
		t.Fatal("extension is not the header tail")
	}
	h = append([]byte(nil), h[:len(h)-len(tail)]...)
	h = binary.AppendUvarint(h, uint64(after.Len()))
	h = append(h, after.Bytes()...)
	out := append([]byte(magicV4), make([]byte, 8)...)
	binary.LittleEndian.PutUint64(out[len(magicV4):], uint64(len(h)))
	out = append(out, h...)
	sum := sha256.Sum256(h)
	out = append(out, sum[:]...)
	return append(out, data[start+n+sha256.Size:]...)
}
