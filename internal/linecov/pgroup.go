// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// RunIsolated runs cmd as the collector runs a test binary: in a process
// group of its own, which a cancelled context kills whole, with a bounded
// wait for output pipes a descendant still holds, and with the group's
// remaining members killed once cmd returns. Where Unix process groups do not
// exist only cmd itself is killed, as exec.CommandContext does.
func RunIsolated(cmd *exec.Cmd) error {
	return RunIsolatedRSS(cmd, 0)
}

// RSSLimitError is a run the RSS watchdog killed: the resident memory of its
// process group, Observed bytes, passed Limit (GLS-V0-058).
type RSSLimitError struct {
	Limit, Observed int64
}

func (e *RSSLimitError) Error() string {
	return fmt.Sprintf("rss-limit-exceeded: process group resident memory %d MiB passed the %d MiB limit; the group was killed", e.Observed>>20, e.Limit>>20)
}

// RSSWatchdogEnforced reports whether this platform can measure a process
// group's resident memory. Where it cannot, RunIsolatedRSS runs without the
// watchdog and only the command's context deadline bounds the run.
func RSSWatchdogEnforced() bool { return groupRSSSupported }

// rssPollInterval is how often the watchdog samples the group.
const rssPollInterval = 50 * time.Millisecond

// RunIsolatedRSS runs cmd as RunIsolated does, under a watchdog that samples
// the resident memory of cmd's whole process group and, once it exceeds
// maxRSS bytes, kills the group and returns *RSSLimitError rather than the
// command's own exit error (GLS-V0-058). maxRSS <= 0, or a platform where
// RSSWatchdogEnforced is false, runs without the watchdog.
func RunIsolatedRSS(cmd *exec.Cmd, maxRSS int64) error {
	isolate(cmd)
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return err
	}
	var over *RSSLimitError
	var wg sync.WaitGroup
	done := make(chan struct{})
	if maxRSS > 0 && groupRSSSupported {
		pgid := cmd.Process.Pid
		wg.Add(1)
		go func() {
			defer wg.Done()
			tick := time.NewTicker(rssPollInterval)
			defer tick.Stop()
			for {
				// The leader is not reaped before done closes, so its pid,
				// the group id, cannot be reused while this loop samples it.
				if rss := groupRSS(pgid); rss > maxRSS {
					over = &RSSLimitError{Limit: maxRSS, Observed: rss}
					killGroup(pgid)
					return
				}
				select {
				case <-done:
					return
				case <-tick.C:
				}
			}
		}()
	}
	err := cmd.Wait()
	close(done)
	wg.Wait()
	cleanup(cmd)
	if over != nil {
		return over
	}
	return err
}
