//go:build darwin || linux

package journal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// precheckStore is a resume base over refreshStore with every base file and
// shard settled at once (resumeQuiescence zero) and one tail request. The
// base also checks one evidence blob as if it were a bound segment, so the
// segment path of PSR-V0-225 runs on this unsegmented fixture.
func precheckStore(t *testing.T, quiescence time.Duration) (*fixture.Repo, Reader, *ResumeBase, string) {
	t.Helper()
	saved := resumeQuiescence
	resumeQuiescence = 0
	t.Cleanup(func() { resumeQuiescence = saved })
	repo, r := refreshStore(t)
	base := resumeBase(t, r)
	blob := "evidence/" + string(wire.Sum(fixture.Ticket("A").Encode()))
	if f, ok := base.files[blob]; !ok || !f.quiet {
		t.Fatalf("base blob %s: %+v", blob, f)
	}
	base.segments = append(base.segments, blob)
	appendRequest(t, repo, "tail-1")
	resumeQuiescence = quiescence
	return repo, r, base, blob
}

// gatedReads counts, while f runs, the request shard listings and the reads
// of file p.
func gatedReads(t *testing.T, p string, f func()) (shards, reads int) {
	t.Helper()
	savedNames, savedRead := afterReadNames, afterNativeRead
	t.Cleanup(func() { afterReadNames, afterNativeRead = savedNames, savedRead })
	afterReadNames = func(dir string) {
		if strings.HasPrefix(dir, "requests/") {
			shards++
		}
	}
	afterNativeRead = func(read string) {
		if read == p {
			reads++
		}
	}
	f()
	afterReadNames, afterNativeRead = savedNames, savedRead
	return shards, reads
}

// TestPSRV0225_PrecheckMovesBaseReadsOutOfTheGate covers PSR-V0-225: after
// PrecheckResume, the resumed audit and the recheck before commit list no
// request shard whose stamps are unchanged and re-hash no settled segment
// the precheck re-hashed; they stat every file instead and still accept an
// untouched journal. Without the precheck each lists every shard and the
// resumed audit re-hashes the segment. A precheck that trusted nothing,
// because no shard had settled when it began, leaves the gated work as it
// was.
func TestPSRV0225_PrecheckMovesBaseReadsOutOfTheGate(t *testing.T) {
	_, r, base, blob := precheckStore(t, 0)
	var res *Result
	shards, reads := gatedReads(t, blob, func() {
		var err error
		if res, _, err = r.AuditForResume(base, ""); err != nil || res.Mode != ModeWriter {
			t.Fatalf("resumed audit: %+v %v", res, err)
		}
		if decline, cleanup := r.RecheckResume(base, res); decline != nil || cleanup != nil {
			t.Fatalf("recheck: decline %v cleanup %v", decline, cleanup)
		}
	})
	if shards == 0 || reads != 1 {
		t.Fatalf("without a precheck: %d shard listings, %d segment reads; want every shard listed and one re-hash", shards, reads)
	}
	audited, _ := gatedReads(t, blob, func() {
		if _, _, err := r.AuditForResume(base, ""); err != nil {
			t.Fatal(err)
		}
	})

	if err := r.PrecheckResume(base); err != nil {
		t.Fatal(err)
	}
	if base.pre == nil || len(base.pre.shards) == 0 || !base.pre.rehashed(blob) {
		t.Fatalf("precheck trusted nothing: %+v", base.pre)
	}
	shards, reads = gatedReads(t, blob, func() {
		var err error
		if res, _, err = r.AuditForResume(base, ""); err != nil || res.Mode != ModeWriter {
			t.Fatalf("resumed audit after the precheck: %+v %v", res, err)
		}
		if decline, cleanup := r.RecheckResume(base, res); decline != nil || cleanup != nil {
			t.Fatalf("recheck after the precheck: decline %v cleanup %v", decline, cleanup)
		}
	})
	if shards != 0 || reads != 0 {
		t.Fatalf("after the precheck: %d shard listings, %d segment reads under the gate; want none", shards, reads)
	}

	resumeQuiescence = time.Hour
	if err := r.PrecheckResume(base); err != nil {
		t.Fatal(err)
	}
	if len(base.pre.shards) != 0 {
		t.Fatalf("precheck trusted %d unsettled shards", len(base.pre.shards))
	}
	shards, _ = gatedReads(t, blob, func() {
		if _, _, err := r.AuditForResume(base, ""); err != nil {
			t.Fatal(err)
		}
	})
	if audited == 0 || shards != audited {
		t.Fatalf("unsettled shards: %d listings in the resumed audit, want %d", shards, audited)
	}
}

// TestPSRV0225_PrecheckNeverAdmitsAStaleBase covers PSR-V0-225's failure
// modes: a change made after the precheck, to a shard it trusted or to a
// segment it re-hashed, is seen under the gate by stamps and declines the
// resumed audit, or the recheck before commit when it follows that audit,
// exactly as without a precheck. A precheck that read a changed base
// trusts nothing it could not verify, and a failed close of its read
// lifetime is cleanup with nothing recorded.
func TestPSRV0225_PrecheckNeverAdmitsAStaleBase(t *testing.T) {
	request, _ := snapshot.RequestPath("refresh-2")
	shard := filepath.Dir(request)
	changes := []struct {
		name   string
		change func(t *testing.T, repo *fixture.Repo, blob string)
	}{
		{"stray-request-in-trusted-shard", func(t *testing.T, repo *fixture.Repo, _ string) {
			fixture.Write(t, filepath.Join(repo.StateDir, shard, "stray.json"), []byte("{}\n"))
		}},
		{"stray-request-shard-mtime-restored", func(t *testing.T, repo *fixture.Repo, _ string) {
			dir := filepath.Join(repo.StateDir, shard)
			info, err := os.Stat(dir)
			if err != nil {
				t.Fatal(err)
			}
			fixture.Write(t, filepath.Join(dir, "stray.json"), []byte("{}\n"))
			if err := os.Chtimes(dir, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
		}},
		{"removed-request", func(t *testing.T, repo *fixture.Repo, _ string) { remove(t, filepath.Join(repo.StateDir, request)) }},
		{"rewritten-request", func(t *testing.T, repo *fixture.Repo, _ string) {
			rewriteKeepingStamps(t, filepath.Join(repo.StateDir, request))
		}},
		{"stray-request-in-new-shard", func(t *testing.T, repo *fixture.Repo, _ string) {
			fixture.Write(t, filepath.Join(repo.StateDir, "requests", "zz", "stray.json"), []byte("{}\n"))
		}},
		{"rewritten-segment", func(t *testing.T, repo *fixture.Repo, blob string) {
			rewriteKeepingStamps(t, filepath.Join(repo.StateDir, blob))
		}},
	}
	for _, tc := range changes {
		t.Run("after-precheck/"+tc.name, func(t *testing.T) {
			repo, r, base, blob := precheckStore(t, 0)
			if err := r.PrecheckResume(base); err != nil || base.pre == nil {
				t.Fatalf("precheck: %+v %v", base.pre, err)
			}
			tc.change(t, repo, blob)
			if res, _, err := r.AuditForResume(base, ""); err == nil || res != nil {
				t.Fatalf("resumed audit accepted a base changed after the precheck: %+v", res)
			}
		})
		t.Run("after-audit/"+tc.name, func(t *testing.T) {
			repo, r, base, blob := precheckStore(t, 0)
			if err := r.PrecheckResume(base); err != nil || base.pre == nil {
				t.Fatalf("precheck: %+v %v", base.pre, err)
			}
			res, _, err := r.AuditForResume(base, "")
			if err != nil {
				t.Fatal(err)
			}
			tc.change(t, repo, blob)
			if decline, cleanup := r.RecheckResume(base, res); decline == nil || cleanup != nil {
				t.Fatalf("recheck over a change after the audit: decline %v cleanup %v", decline, cleanup)
			}
		})
	}
	t.Run("before-precheck/rewritten-segment", func(t *testing.T) {
		repo, r, base, blob := precheckStore(t, 0)
		rewriteKeepingStamps(t, filepath.Join(repo.StateDir, blob))
		if err := r.PrecheckResume(base); err != nil {
			t.Fatal(err)
		}
		if base.pre.rehashed(blob) {
			t.Fatal("precheck trusted a segment whose bytes differ")
		}
		if res, _, err := r.AuditForResume(base, ""); err == nil || res != nil {
			t.Fatalf("resumed audit accepted a rewritten segment: %+v", res)
		}
	})
	t.Run("during-recheck/requests-replaced", func(t *testing.T) {
		repo, r, base, _ := precheckStore(t, 0)
		if err := r.PrecheckResume(base); err != nil || base.pre == nil {
			t.Fatalf("precheck: %+v %v", base.pre, err)
		}
		res, _, err := r.AuditForResume(base, "")
		if err != nil {
			t.Fatal(err)
		}
		// Once the recheck has listed requests/ through its handle, rename
		// that directory away and put a copy with a stray in its place: the
		// pinned handles still see the old tree, the pathname does not.
		requests := filepath.Join(repo.StateDir, "requests")
		saved := afterReadNames
		t.Cleanup(func() { afterReadNames = saved })
		afterReadNames = func(dir string) {
			if dir != "requests" {
				return
			}
			afterReadNames = saved
			if err := os.Rename(requests, requests+".old"); err != nil {
				t.Fatal(err)
			}
			copyDir(t, requests+".old", requests)
			fixture.Write(t, filepath.Join(repo.StateDir, shard, "stray.json"), []byte("{}\n"))
		}
		if decline, cleanup := r.RecheckResume(base, res); decline == nil || cleanup != nil {
			t.Fatalf("recheck over requests/ replaced during it: decline %v cleanup %v", decline, cleanup)
		}
	})
	t.Run("failed-close", func(t *testing.T) {
		_, r, base, _ := precheckStore(t, 0)
		saved := closeReadFile
		t.Cleanup(func() { closeReadFile = saved })
		closeReadFile = func(f *os.File) error { return errors.Join(saved(f), errors.New("injected close failure")) }
		if err := r.PrecheckResume(base); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || base.pre != nil {
			t.Fatalf("precheck over a failed close: %v, recorded %+v", err, base.pre)
		}
	})
}

// copyDir copies the regular files and directories under src to dst.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
