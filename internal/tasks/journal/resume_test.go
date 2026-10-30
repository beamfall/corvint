//go:build darwin || linux

package journal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestPSRV0050_PrefixFallback covers PSR-V0-050 (proposed). On a journal
// that holds still a PrefixFallback audit is the complete audit in one
// attempt. When a writer appends during every capture, the complete attempt
// moves once and the prefix walk settles at the head it captured, instead
// of four complete attempts that all move. A prefix walk that refuses on an
// unchanged observation is ErrPrefixDeclined, never a refusal, and the
// complete audit decides that refusal.
func TestPSRV0050_PrefixFallback(t *testing.T) {
	repo, r := refreshStore(t)
	r.PrefixFallback = true
	captures := 0
	r.afterCapture = func() { captures++ }
	quiet, _, err := r.AuditForWriteObserved()
	if err != nil || quiet.Mode != ModeFull || captures != 1 {
		t.Fatalf("quiescent fallback audit: mode %v captures %d err %v", quiet, captures, err)
	}

	captures = 0
	r.afterCapture = func() {
		captures++
		appendRequest(t, repo, fmt.Sprintf("racing-%d", captures))
	}
	moving, obs, err := r.AuditForWriteObserved()
	if err != nil || moving.Mode != ModePrefix || captures != 2 {
		t.Fatalf("fallback under appends: %+v captures %d err %v", moving, captures, err)
	}
	if _, err := moving.ResumeBase(obs); err != nil {
		t.Fatalf("prefix result yields no resume base: %v", err)
	}

	path, _ := snapshot.RequestPath("refresh-2")
	fixture.Write(t, filepath.Join(repo.StateDir, path), []byte("{}\n"))
	plain := r
	plain.PrefixFallback, plain.afterCapture = false, nil
	_, wantErr := plain.AuditForWrite()
	if wantErr == nil || wire.CodeOf(wantErr) == wire.CodeSnapshotMoved {
		t.Fatalf("complete audit over the fault: %v", wantErr)
	}
	once := false
	r.afterCapture = func() {
		if !once {
			once = true
			appendRequest(t, repo, "racing-once")
		}
	}
	if _, _, err := r.AuditForWriteObserved(); !errors.Is(err, ErrPrefixDeclined) || wire.CodeOf(err) != wire.CodeSnapshotMoved {
		t.Fatalf("prefix walk over the fault: %v, want ErrPrefixDeclined", err)
	}
	r.afterCapture = nil
	if _, _, err := r.AuditForWriteObserved(); wire.CodeOf(err) != wire.CodeOf(wantErr) {
		t.Fatalf("held fallback audit: %v, want the complete audit's %v", err, wantErr)
	}
}

// resumeBase is a resume base from a settled complete audit of repo.
func resumeBase(t *testing.T, r Reader) *ResumeBase {
	t.Helper()
	r.PrefixFallback = true
	res, obs, err := r.AuditForWriteObserved()
	if err != nil || res.Mode != ModeFull {
		t.Fatalf("base audit: %+v %v", res, err)
	}
	base, err := res.ResumeBase(obs)
	if err != nil {
		t.Fatal(err)
	}
	return base
}

// TestPSRV0051_ResumeBaseOnlyFromSettledAudit covers PSR-V0-051 (proposed):
// a resume base comes only from a settled complete or prefix audit with its
// own physical observation, never from a tail or writer audit.
func TestPSRV0051_ResumeBaseOnlyFromSettledAudit(t *testing.T) {
	_, r := refreshStore(t)
	base := resumeBase(t, r)
	if base.Seq() != 8 || base.WriterCheckpoint() == nil || base.WriterCheckpoint().FullSeq != 8 {
		t.Fatalf("base seq %d checkpoint %+v", base.Seq(), base.WriterCheckpoint())
	}
	writer, obs, err := r.AuditForResume(base, "")
	if err != nil || writer.Mode != ModeWriter {
		t.Fatalf("resumed audit: %+v %v", writer, err)
	}
	if _, err := writer.ResumeBase(obs); err == nil {
		t.Fatal("a writer audit yielded a resume base")
	}
	full, _, err := r.AuditForWriteObserved()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := full.ResumeBase(PhysicalObservation{}); err == nil {
		t.Fatal("a result without its observation yielded a resume base")
	}
	if _, _, err := r.AuditForResume(nil, ""); err == nil {
		t.Fatal("an absent base resumed")
	}
}

// TestPSRV0052_ResumeDeclinesOnChangedBase covers PSR-V0-052 (proposed). A
// resumed audit walks only the tail the writer appended after the base, and
// declines whenever a base receipt, request, evidence blob or pinned file
// differs from what the base audit read, a base request is gone, or a
// request file appears that neither the base nor the tail accounts for. It
// runs once with every base file settled (stamps decide) and once with none
// settled (bytes are re-read).
func TestPSRV0052_ResumeDeclinesOnChangedBase(t *testing.T) {
	request, _ := snapshot.RequestPath("refresh-2")
	second, _ := snapshot.ReceiptName(2)
	blob := "evidence/" + string(wire.Sum(fixture.Ticket("A").Encode()))
	cases := []struct {
		name   string
		change func(t *testing.T, repo *fixture.Repo)
		refuse bool // the complete audit refuses the changed journal
	}{
		{"untouched", func(*testing.T, *fixture.Repo) {}, false},
		{"rewritten-receipt", func(t *testing.T, repo *fixture.Repo) {
			rewriteKeepingStamps(t, filepath.Join(repo.StateDir, "receipts", second))
		}, true},
		{"rewritten-request", func(t *testing.T, repo *fixture.Repo) { rewriteKeepingStamps(t, filepath.Join(repo.StateDir, request)) }, true},
		{"rewritten-blob", func(t *testing.T, repo *fixture.Repo) { rewriteKeepingStamps(t, filepath.Join(repo.StateDir, blob)) }, true},
		{"removed-request", func(t *testing.T, repo *fixture.Repo) { remove(t, filepath.Join(repo.StateDir, request)) }, true},
		{"stray-request", func(t *testing.T, repo *fixture.Repo) {
			fixture.Write(t, filepath.Join(repo.StateDir, filepath.Dir(request), "stray.json"), []byte("{}\n"))
		}, true},
	}
	for _, quiescence := range []time.Duration{0, time.Hour} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/quiescence=%s", tc.name, quiescence), func(t *testing.T) {
				saved := resumeQuiescence
				resumeQuiescence = quiescence
				t.Cleanup(func() { resumeQuiescence = saved })
				repo, r := refreshStore(t)
				base := resumeBase(t, r)
				appendRequest(t, repo, "tail-1")
				appendRequest(t, repo, "tail-2")
				tc.change(t, repo)
				res, _, err := r.AuditForResume(base, "")
				if tc.name == "untouched" {
					if err != nil || res.Mode != ModeWriter || res.LastSeq.Uint64() != base.Seq()+2 {
						t.Fatalf("resumed audit over the tail: %+v %v", res, err)
					}
					return
				}
				if err == nil || res != nil {
					t.Fatalf("resumed audit accepted a changed base: %+v", res)
				}
				if _, full := r.AuditForWrite(); tc.refuse && (full == nil || wire.CodeOf(full) == wire.CodeSnapshotMoved) {
					t.Fatalf("complete audit over the change: %v", full)
				}
			})
		}
	}
}

// rewriteKeepingStamps flips one byte of p and restores its modification
// time, so only the change time and the bytes record the rewrite.
func rewriteKeepingStamps(t *testing.T, p string) {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	raw := read(t, p)
	raw[len(raw)/2] ^= 1
	if err := os.WriteFile(p, raw, info.Mode()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

// TestPSRV0057_ResumeCheckCloseFailureIsCleanup covers PSR-V0-057
// (proposed): the base check reads through its own read lifetime, and a
// failed close in that lifetime is the audit's cleanup failure
// (UNSUPPORTED_FILESYSTEM, PhysicalObservation.Cleanup set), which terminates
// the command as CAL-V0-116 requires, never a decline the caller falls back
// from. The resumed audit lists receipts/ at its first capture, in the base
// check, and at its second capture; the close of the check's listing fails.
func TestPSRV0057_ResumeCheckCloseFailureIsCleanup(t *testing.T) {
	repo, r := refreshStore(t)
	base := resumeBase(t, r)
	appendRequest(t, repo, "tail-1")
	savedClose, savedNames := closeReadFile, afterReadNames
	t.Cleanup(func() { closeReadFile, afterReadNames = savedClose, savedNames })
	listings, armed, failed := 0, false, false
	afterReadNames = func(p string) {
		if p == "receipts" {
			listings++
			armed = listings == 2
		}
	}
	closeReadFile = func(f *os.File) error {
		err := savedClose(f)
		if armed {
			armed, failed = false, true
			return errors.Join(err, errors.New("injected close failure"))
		}
		return err
	}
	res, obs, err := r.AuditForResume(base, "")
	if !failed || listings != 3 {
		t.Fatalf("receipts listed %d times, close failed %v", listings, failed)
	}
	if res != nil || obs.Cleanup == nil || wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("resumed audit over a failed close: %+v cleanup %v err %v", res, obs.Cleanup, err)
	}
}

// TestPSRV0056_RecheckResumeSeesChangeAfterAudit covers PSR-V0-056
// (proposed): a base file changed, a request shard made, or a tail receipt
// or tail request file rewritten (PSR-V0-116), after the resumed audit
// passed is a decline at the recheck before commit, and a
// failed close of the recheck's read lifetime is cleanup (PSR-V0-057).
func TestPSRV0056_RecheckResumeSeesChangeAfterAudit(t *testing.T) {
	second, _ := snapshot.ReceiptName(2)
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, repo *fixture.Repo)
	}{
		{"untouched", func(*testing.T, *fixture.Repo) {}},
		{"rewritten-receipt", func(t *testing.T, repo *fixture.Repo) {
			rewriteKeepingStamps(t, filepath.Join(repo.StateDir, "receipts", second))
		}},
		{"stray-request-in-new-shard", func(t *testing.T, repo *fixture.Repo) {
			fixture.Write(t, filepath.Join(repo.StateDir, "requests", "zz", "stray.json"), []byte("{}\n"))
		}},
		// PSR-V0-116 (proposed): the tail the resumed walk read is reread too.
		{"rewritten-tail-receipt", func(t *testing.T, repo *fixture.Repo) {
			h, err := snapshot.DecodeHead(read(t, filepath.Join(repo.StateDir, "head.json")))
			if err != nil {
				t.Fatal(err)
			}
			name, _ := snapshot.ReceiptName(h.LastSeq.Uint64())
			rewriteKeepingStamps(t, filepath.Join(repo.StateDir, "receipts", name))
		}},
		{"rewritten-tail-request", func(t *testing.T, repo *fixture.Repo) {
			path, _ := snapshot.RequestPath("tail-1")
			rewriteKeepingStamps(t, filepath.Join(repo.StateDir, path))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, r := refreshStore(t)
			base := resumeBase(t, r)
			appendRequest(t, repo, "tail-1")
			res, _, err := r.AuditForResume(base, "")
			if err != nil || res.Mode != ModeWriter {
				t.Fatalf("resumed audit: %+v %v", res, err)
			}
			tc.change(t, repo)
			decline, cleanup := r.RecheckResume(base, res)
			if cleanup != nil || (decline == nil) != (tc.name == "untouched") {
				t.Fatalf("recheck: decline %v cleanup %v", decline, cleanup)
			}
		})
	}
	t.Run("failed-close", func(t *testing.T) {
		repo, r := refreshStore(t)
		base := resumeBase(t, r)
		appendRequest(t, repo, "tail-1")
		res, _, err := r.AuditForResume(base, "")
		if err != nil {
			t.Fatal(err)
		}
		saved := closeReadFile
		t.Cleanup(func() { closeReadFile = saved })
		closeReadFile = func(f *os.File) error { return errors.Join(saved(f), errors.New("injected close failure")) }
		decline, cleanup := r.RecheckResume(base, res)
		if decline != nil || wire.CodeOf(cleanup) != wire.CodeUnsupportedFilesystem {
			t.Fatalf("recheck over a failed close: decline %v cleanup %v", decline, cleanup)
		}
	})
}

// TestPSRV0151_ResumedWalksCountEachAttempt covers PSR-V0-151 (V1-1626): a
// resumed audit charges one resumed walk per attempt it ran, so a
// SNAPSHOT_MOVED retry is a second walk with its own wall time, and an
// absent base, which runs no attempt, charges none. Resumed walks never
// reach the complete or prefix columns.
func TestPSRV0151_ResumedWalksCountEachAttempt(t *testing.T) {
	repo, r := refreshStore(t)
	base := resumeBase(t, r)
	_, settled, err := r.AuditForResume(base, "")
	if err != nil || settled.Walks.Resumed != 1 || settled.Walks.ResumedTime <= 0 {
		t.Fatalf("settled resume: walks %+v err %v", settled.Walks, err)
	}
	captures := 0
	r.afterCapture = func() {
		if captures++; captures == 1 {
			appendRequest(t, repo, "resume-retry")
		}
	}
	res, moved, err := r.AuditForResume(base, "")
	if err != nil || res.Mode != ModeWriter || captures != 2 {
		t.Fatalf("resume over one append: %+v captures %d err %v", res, captures, err)
	}
	w := moved.Walks
	if w.Resumed != 2 || w.ResumedTime <= 0 || w.Complete != 0 || w.Prefix != 0 || w.CompleteTime != 0 || w.PrefixTime != 0 {
		t.Fatalf("resume with a SNAPSHOT_MOVED retry: walks %+v, want 2 resumed", w)
	}
	r.afterCapture = nil
	if _, absent, err := r.AuditForResume(nil, ""); err == nil || absent.Walks != (WalkStats{}) {
		t.Fatalf("absent base: walks %+v err %v", absent.Walks, err)
	}
}
