package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// maxJobs bounds --jobs: each worker runs one corvint process at a time over
// its own temporary snapshot copy.
const maxJobs = 16

// validateJobs refuses a --jobs outside 1..maxJobs, and parallel runs that
// measure latency or write the ordinal-ordered context capture (CEP-V0-756).
func validateJobs(configuration options) error {
	if configuration.jobs < 1 || configuration.jobs > maxJobs {
		return fmt.Errorf("--jobs %d is outside 1..%d", configuration.jobs, maxJobs)
	}
	if configuration.jobs > 1 && (configuration.snapshotLatency || configuration.contextPackets != "") {
		return errors.New("--jobs above 1 refuses --snapshot-latency and --context-packets, which need one sample at a time")
	}
	return nil
}

// judgeSamples judges every sample and returns the reports in sample order.
// With --jobs 1 it is the serial loop over one workspace set. Above 1, the
// samples of one snapshot (repo and base commit) form one group judged
// serially in sample order on its own workspace set, so a snapshot copy is
// never shared, and at most --jobs groups run at once (CEP-V0-756). The
// first error cancels the rest and is returned.
func judgeSamples(ctx context.Context, configuration options, samples []sample, judgeOne func(context.Context, *workspaces, int, sample) (sampleReport, error)) ([]sampleReport, error) {
	reports := make([]sampleReport, len(samples))
	if configuration.jobs <= 1 {
		space := newWorkspaces()
		defer space.close()
		for ordinal, item := range samples {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			report, err := judgeOne(ctx, space, ordinal, item)
			if err != nil {
				return nil, err
			}
			reports[ordinal] = report
		}
		return reports, nil
	}
	var order []string
	groups := map[string][]int{}
	for ordinal, item := range samples {
		key := item.Repo + "@" + item.BaseCommit
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], ordinal)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wait     sync.WaitGroup
		lock     sync.Mutex
		firstErr error
	)
	slots := make(chan struct{}, configuration.jobs)
	for _, key := range order {
		ordinals := groups[key]
		wait.Add(1)
		slots <- struct{}{}
		go func() {
			defer wait.Done()
			defer func() { <-slots }()
			space := newWorkspaces()
			defer space.close()
			for _, ordinal := range ordinals {
				err := ctx.Err()
				if err == nil {
					reports[ordinal], err = judgeOne(ctx, space, ordinal, samples[ordinal])
				}
				if err != nil {
					lock.Lock()
					if firstErr == nil {
						firstErr = err
						cancel()
					}
					lock.Unlock()
					return
				}
			}
		}()
	}
	wait.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return reports, ctx.Err()
}
