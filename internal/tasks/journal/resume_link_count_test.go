//go:build darwin || linux

package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// TestPSRV0247_UnsettledBaseFileSurvivesALinkCountChange covers PSR-V0-247
// (proposed). A writer link-publishes a receipt from its staging slot and
// unlinks the slot only after it publishes the head, so a base audit can
// capture the newest receipt while it still has a second name. Unlinking
// that name changes the receipt's change time and nothing else. A base file
// whose stamps had not settled is re-read against its digest anyway, so the
// resumed audit and the recheck before commit accept it. An unsettled base
// request rewritten in place to the same size and modification time, or a
// settled receipt whose change time moved, still declines: the resume stays
// bound to the bytes the base audit read.
func TestPSRV0247_UnsettledBaseFileSurvivesALinkCountChange(t *testing.T) {
	saved := resumeQuiescence
	t.Cleanup(func() { resumeQuiescence = saved })
	for _, tc := range []struct {
		name       string
		quiescence time.Duration
		rewrite    bool
		want       string
	}{
		{"unsettled-link-count", time.Hour, false, ""},
		{"unsettled-rewritten-in-place", time.Hour, true, "base file bytes changed since the base audit"},
		{"settled-link-count", 0, false, "base file changed since the base audit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resumeQuiescence = tc.quiescence
			repo, r := refreshStore(t)
			name, err := snapshot.ReceiptName(8)
			if err != nil {
				t.Fatal(err)
			}
			receipt := filepath.Join(repo.StateDir, "receipts", name)
			peer := filepath.Join(t.TempDir(), "staging-slot")
			if err := os.Link(receipt, peer); err != nil {
				t.Skipf("no hard link beside the store: %v", err)
			}
			base := resumeBase(t, r)
			if f := base.files["receipts/"+name]; f.info == nil || f.quiet != (tc.quiescence == 0) {
				t.Fatalf("base receipt %s: %+v", name, f)
			}
			// Coarse kernel clocks: let the change time move.
			time.Sleep(20 * time.Millisecond)
			if err := os.Remove(peer); err != nil {
				t.Fatal(err)
			}
			if tc.rewrite {
				request, _ := snapshot.RequestPath("refresh-2")
				rewriteInPlace(t, filepath.Join(repo.StateDir, request))
			}
			appendRequest(t, repo, "tail-1")
			res, _, err := r.AuditForResume(base, "")
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("resumed audit: %+v %v, want %q", res, err, tc.want)
				}
				return
			}
			if err != nil || res.Mode != ModeWriter {
				t.Fatalf("resumed audit: %+v %v", res, err)
			}
			if decline, cleanup := r.RecheckResume(base, res); decline != nil || cleanup != nil {
				t.Fatalf("recheck: decline %v cleanup %v", decline, cleanup)
			}
		})
	}
}

// rewriteInPlace flips one byte of p in place and restores its modification
// time, so only its bytes and change time differ.
func rewriteInPlace(t *testing.T, p string) {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	i := strings.LastIndexByte(string(raw), '0')
	if i < 0 {
		t.Fatalf("no digit to flip in %s", p)
	}
	raw[i] = '1'
	f, err := os.OpenFile(p, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt(raw, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}
