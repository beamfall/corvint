package contextindex

import (
	"runtime"
	"sync"
)

// forEachAnchor runs visit for 0..count-1 on every CPU.
func forEachAnchor(count int, visit func(int)) {
	next := make(chan int)
	var workers sync.WaitGroup
	for range runtime.NumCPU() {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for position := range next {
				visit(position)
			}
		}()
	}
	for position := range count {
		next <- position
	}
	close(next)
	workers.Wait()
}
