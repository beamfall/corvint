//go:build darwin || linux

package store_test

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTBSV1532_TicketShowUnderConcurrentWritesNeverOKEmpty (V1-3462): ticket
// show of an existing ticket, read while other writers commit, returns OK
// with exactly one item or a non-OK outcome; it never returns OK with an
// empty items array. The writers are real concurrent store.Mutate creates.
func TestTBSV1532_TicketShowUnderConcurrentWritesNeverOKEmpty(t *testing.T) {
	s := newLeaseStore(t)
	id := s.ticket(t, "subject")
	const writers, writes, readers, reads = 2, 12, 3, 16
	var wg sync.WaitGroup
	errs := make(chan error, writers*writes+readers*reads)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < writes; i++ {
				title := fmt.Sprintf("w%d-%d", w, i)
				report, err := store.Mutate(context.Background(), s.repo, operator(), envelope("create-"+title, "CREATE", "", "", createPayload(title)), now(t))
				if err != nil {
					errs <- fmt.Errorf("writer %s: %v", title, err)
				} else if report.Outcome.Outcome != mutation.OutcomeCompleted && report.Outcome.Outcome != mutation.OutcomeStorageFailed {
					errs <- fmt.Errorf("writer %s: %+v", title, report.Outcome)
				}
			}
		}(w)
	}
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < reads; i++ {
				var stdout bytes.Buffer
				cli.Run(cli.Env{Cwd: s.repo.PrimaryWorktree, Args: []string{"ticket", "show", id}, Stdout: &stdout})
				res, err := wire.DecodeResult(stdout.Bytes())
				switch {
				case err != nil:
					errs <- fmt.Errorf("undecodable ticket show: %v\n%s", err, stdout.Bytes())
				case res.Outcome == wire.OutcomeOK && len(res.Items) != 1:
					errs <- fmt.Errorf("ticket show OK with %d items:\n%s", len(res.Items), stdout.Bytes())
				case res.Outcome != wire.OutcomeOK && len(res.Codes) == 0:
					errs <- fmt.Errorf("non-OK ticket show names no code:\n%s", stdout.Bytes())
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
