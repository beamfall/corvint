// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"sync"
)

// targetWorkers bounds independent targets, not either target's independent pair.
// The override changes scheduling only; it never enters a build environment.
func targetWorkers(targets int) int {
	workers := runtime.NumCPU()
	if override, err := strconv.Atoi(os.Getenv("CORVINT_ARCHIVE_WORKERS")); err == nil && override > 0 {
		workers = override
	}
	return max(1, min(workers, targets))
}

type targetResult[T any] struct {
	value T
	err   error
}

// runTargetWorkers returns the serial prefix through the first terminal error.
// A failure cancels only higher indices: earlier targets must finish to establish
// which error the serial loop would have returned. All workers and their owned
// subprocesses must retire before the caller can collect results or clean scratch.
func runTargetWorkers[T any](ctx context.Context, count int, run func(context.Context, int) (T, error)) []targetResult[T] {
	results := make([]targetResult[T], count)
	contexts := make([]context.Context, count)
	cancels := make([]context.CancelFunc, count)
	for index := range contexts {
		contexts[index], cancels[index] = context.WithCancel(ctx)
		defer cancels[index]()
	}
	var mutex sync.Mutex
	next, limit := 0, count
	var workers sync.WaitGroup
	for worker := 0; worker < targetWorkers(count); worker++ {
		workers.Go(func() {
			for {
				mutex.Lock()
				if next >= limit {
					mutex.Unlock()
					return
				}
				index := next
				next++
				mutex.Unlock()
				value, err := run(contexts[index], index)
				mutex.Lock()
				results[index] = targetResult[T]{value: value, err: err}
				if err != nil && index+1 < limit {
					limit = index + 1
					for later := limit; later < count; later++ {
						cancels[later]()
					}
				}
				mutex.Unlock()
			}
		})
	}
	workers.Wait()
	for index, result := range results {
		if result.err != nil {
			return results[:index+1]
		}
	}
	return results
}
