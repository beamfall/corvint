//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func descriptorCount() (int, error) {
	path := "/dev/fd"
	if runtime.GOOS == "linux" {
		path = "/proc/self/fd"
	}
	entries, err := os.ReadDir(path)
	return len(entries), err
}

// CAL-V0-221: exercise the complete route's actual merged observation and its
// forced no-watch fallback. Large qualification is opt-in; the default pair
// still catches the old RLIMIT-dependent watch on ordinary development runs.
func TestCALV0221_CompleteRouteBoundedDescriptors(t *testing.T) {
	sizes := []int{100, 2100}
	if os.Getenv("CORVINT_V10906_LARGE") == "1" {
		sizes = []int{1000, 100000}
	}
	for _, size := range sizes {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			repo := historyStoreAt(t, root, 1, 2)
			for seq := 3; seq <= min(size, 2000); seq++ {
				id := fmt.Sprintf("fd-%d", seq)
				p, _ := snapshot.RequestPath(id)
				historyAppend(t, repo, map[string][]byte{p: historyRequest(id, uint64(seq))}, id)
			}
			shards, err := os.ReadDir(filepath.Join(repo.StateDir, "requests"))
			if err != nil {
				t.Fatal(err)
			}
			requests := 0
			for i, shard := range shards {
				if size >= 2000 && (!shard.IsDir() || shard.Name() != fmt.Sprintf("%02x", i)) {
					t.Fatalf("request shard %d: %s", i, shard.Name())
				}
				entries, err := os.ReadDir(filepath.Join(repo.StateDir, "requests", shard.Name()))
				if err != nil {
					t.Fatal(err)
				}
				requests += len(entries)
			}
			if requests != min(size, 2000) || size >= 2000 && len(shards) != 256 {
				t.Fatalf("fixture requests=%d shards=%d", requests, len(shards))
			}
			t.Logf("fixture receipts=%d requests=%d shards=%d", size, requests, len(shards))
			head, err := writerGuards(repo, transaction.Mutate)
			if err != nil {
				t.Fatal(err)
			}
			// Descriptor scaling needs real receipt files, not 100k request
			// projections. Retain the populated request shards, then pad with
			// canonically valid no-op receipts. Publish the fixture head once.
			for seq := head.LastSeq.Uint64() + 1; seq <= uint64(size); seq++ {
				raw := wire.EncodeFile(fixture.ReceiptValue(seq, head.LastReceiptSha256, "MUTATION", head.Generation.Uint64()))
				name, err := snapshot.ReceiptName(seq)
				if err != nil {
					t.Fatal(err)
				}
				historyWrite(t, filepath.Join(repo.StateDir, "receipts", name), raw)
				digest := wire.Sum(raw)
				head.LastSeq, head.LastReceiptSha256 = wire.SizeOf(seq), &digest
			}
			historyWrite(t, filepath.Join(repo.StateDir, "head.json"), wire.EncodeFile(fixture.HeadValue(repo.PrimaryWorktree, head.LastSeq.Uint64(), *head.LastReceiptSha256, head.Generation.Uint64(), head.InitSha256)))
			if head.LastSeq.Uint64() != uint64(size) {
				t.Fatalf("fixture head %s, want %d", head.LastSeq, size)
			}
			dirs := 0
			if err := filepath.WalkDir(repo.StateDir, func(_ string, d os.DirEntry, err error) error {
				if err == nil && d.IsDir() {
					dirs++
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			reader := lockedJournalReader(repo, head)
			// Prove canonical fixture validity before forcing the fallback.
			proof, err := reader.Audit()
			if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
				t.Fatalf("fixture: %v", err)
			}
			for _, fallback := range []bool{false, true} {
				var peaks []int
				for run := 0; run < 5; run++ {
					load, loadErr := exec.Command("uptime").Output()
					if loadErr != nil {
						t.Fatalf("load band unavailable: %v", loadErr)
					}
					before, err := descriptorCount()
					if err != nil {
						t.Fatal(err)
					}
					var peak atomic.Int64
					peak.Store(int64(before))
					stop, done := make(chan struct{}), make(chan error, 1)
					go func() {
						ticker := time.NewTicker(time.Millisecond)
						defer ticker.Stop()
						for {
							select {
							case <-stop:
								done <- nil
								return
							case <-ticker.C:
								n, err := descriptorCount()
								if err != nil {
									done <- err
									return
								}
								if int64(n) > peak.Load() {
									peak.Store(int64(n))
								}
							}
						}
					}()
					func() {
						defer func() {
							close(stop)
							if err := <-done; err != nil {
								t.Error(err)
							}
						}()
						var watch *authority.ChangeGuard
						if !fallback {
							watch, err = authority.WatchChanges(repo)
							if err != nil {
								t.Fatal(err)
							}
							defer watch.Close()
						}
						audit, err := reader.AuditForMutation("fd-absent")
						if err != nil {
							t.Fatal(err)
						}
						var stages []string
						ctx := context.WithValue(context.Background(), mutationStageKey{}, func(s string) { stages = append(stages, s) })
						inv, _, canonical, err := observeMutation(ctx, repo, reader, audit, watch)
						if err != nil || inv == nil || canonical == nil {
							t.Fatalf("observation: %v", err)
						}
						want := "audited,observed"
						if fallback {
							want = "audited,fresh"
						}
						if strings.Join(stages, ",") != want {
							t.Fatalf("route stages %v, want %s", stages, want)
						}
						if canonical.Identity != proof.Identity || canonical.StructuralConsistency != "CONSISTENT" || canonical.ProjectionAgreement != "AGREES" {
							t.Fatal("canonical result differs")
						}
					}()
					after, err := descriptorCount()
					if err != nil {
						t.Fatal(err)
					}
					if after != before {
						t.Fatalf("descriptor leak: %d -> %d", before, after)
					}
					delta := int(peak.Load()) - before
					if delta > 4*dirs+64 {
						t.Fatalf("descriptor bound: %d with %d directories", delta, dirs)
					}
					peaks = append(peaks, int(peak.Load()))
					t.Logf("receipts=%d fallback=%v run=%d baseline=%d sampled_peak=%d delta=%d load=%s", size, fallback, run+1, before, peak.Load(), delta, strings.TrimSpace(string(load)))
				}
				sort.Ints(peaks)
				t.Logf("receipts=%d fallback=%v median_sampled_peak=%d peaks=%v", size, fallback, peaks[2], peaks)
			}
		})
	}
}
