package contextindex

import (
	"bytes"
	"runtime"
	"sync"
	"testing"
)

// parityReceipt is one cell of the task/budget matrix: the receipt bytes a
// single construction emitted for that pair, its state, and the error that
// stood in for them.
type parityReceipt struct {
	task   string
	budget int
	bytes  []byte
	state  string
	err    error
}

// emitParityReceipts emits the whole contrasting task/budget matrix from one
// index. Every cell is independent -- EvalQuery reads the index and the
// repository and writes neither, and each of its receipts costs two Git
// processes -- so the cells are emitted concurrently and the matrix returns in
// its declared order regardless of the order they finished in. The
// construction switches the parity tests flip are package globals, so a whole
// matrix is emitted under one construction and never overlaps another.
func emitParityReceipts(index *Index) []parityReceipt {
	receipts := make([]parityReceipt, 0, len(evalContextParityTasks)*len(evalContextParityBudgets))
	for _, task := range evalContextParityTasks {
		for _, budget := range evalContextParityBudgets {
			receipts = append(receipts, parityReceipt{task: task, budget: budget})
		}
	}
	admitted := make(chan struct{}, 2*runtime.NumCPU())
	var pending sync.WaitGroup
	for position := range receipts {
		pending.Add(1)
		go func(cell *parityReceipt) {
			defer pending.Done()
			admitted <- struct{}{}
			defer func() { <-admitted }()
			cell.bytes, cell.state, cell.err = evalContextReceipt(index, cell.task, cell.budget)
		}(&receipts[position])
	}
	pending.Wait()
	return receipts
}

// compareParityReceipts compares two matrices emitted from the same task and
// budget order byte for byte, and returns how many pairs were compared and the
// states the expected construction reached. A pair either side failed to emit
// is skipped exactly as it was when the two receipts were emitted one after the
// other.
func compareParityReceipts(t *testing.T, repository, expectedLabel, actualLabel string, expected, actual []parityReceipt) (int, map[string]int) {
	t.Helper()
	compared, states := 0, map[string]int{}
	for position, want := range expected {
		got := actual[position]
		if want.err != nil || got.err != nil {
			t.Logf("skipping %q at budget %d in %s: %v / %v", want.task, want.budget, repository, want.err, got.err)
			continue
		}
		compared++
		states[want.state]++
		if !bytes.Equal(want.bytes, got.bytes) {
			t.Fatalf("receipt diverged in %s for %q at budget %d:\n %s = %s\n %s = %s",
				repository, want.task, want.budget, expectedLabel, want.bytes, actualLabel, got.bytes)
		}
	}
	return compared, states
}
