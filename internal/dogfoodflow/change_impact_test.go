package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestChangeDWCV0756ImpactBesideQueryMatchesTheSequentialRun (V1-4197): with a
// runner safe for concurrent calls, the coordination-time impact command runs
// while the query runs, and the rows, row order and evidence files equal those
// of the sequential run, for a produced impact and for a typed abstention. The
// overlap is proven by a handshake, not by timing: in the concurrent run each
// command waits inside Run until the other has entered it, so the second to
// enter finds the first still running. The minute-long wait only bounds a
// deadlock, when the impact never starts beside the query.
func TestChangeDWCV0756ImpactBesideQueryMatchesTheSequentialRun(t *testing.T) {
	root := t.TempDir()
	for _, impactStatus := range []int{0, 2} {
		run := func(concurrent bool) ([]step, map[string]string) {
			t.Helper()
			evidence := t.TempDir()
			var mu sync.Mutex
			running, overlapped, stalled := 0, false, ""
			queryIn, impactIn := make(chan struct{}), make(chan struct{})
			var queryOnce, impactOnce sync.Once
			steps := Runner{Path: "/bin/corvint", Concurrent: concurrent, Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
				var self, other chan struct{}
				var once *sync.Once
				switch args[0] {
				case "query":
					self, other, once = queryIn, impactIn, &queryOnce
				case "impact":
					self, other, once = impactIn, queryIn, &impactOnce
				default:
					return 1
				}
				mu.Lock()
				running++
				overlapped = overlapped || running > 1
				mu.Unlock()
				once.Do(func() { close(self) })
				if concurrent {
					select {
					case <-other:
					case <-time.After(time.Minute):
						mu.Lock()
						stalled = args[0]
						mu.Unlock()
					}
				}
				mu.Lock()
				running--
				mu.Unlock()
				if args[0] == "query" {
					_, _ = io.WriteString(stdout, "{\"query\":true}\n")
					return 0
				}
				if impactStatus == 2 {
					_, _ = io.WriteString(stderr, "{\"code\": \"unsupported-impact-range\", \"error\": \"x\", \"ok\": false}\n")
					return 2
				}
				_, _ = io.WriteString(stdout, "{\"impact\":true}\n")
				return 0
			}}
			c := &change{evidence: evidence}
			c.flow = flow{ctx: context.Background(), prefix: "dogfood-change", root: root, stderr: &bytes.Buffer{}, base: strings.Repeat("a", 40), target: strings.Repeat("b", 40)}
			c.options.Steps = steps
			impact := c.startImpact()
			c.coordinationQuery("task")
			c.prechangeImpact(impact)
			mu.Lock()
			defer mu.Unlock()
			if stalled != "" {
				t.Fatalf("impact exit %d: %s waited a minute for the other command to start beside it", impactStatus, stalled)
			}
			if concurrent != overlapped {
				t.Fatalf("impact exit %d: concurrent=%v but the commands overlapped=%v", impactStatus, concurrent, overlapped)
			}
			files := map[string]string{}
			entries, _ := os.ReadDir(evidence)
			for _, entry := range entries {
				data, _ := os.ReadFile(filepath.Join(evidence, entry.Name()))
				files[entry.Name()] = string(data)
			}
			return c.rows, files
		}
		sequentialRows, sequentialFiles := run(false)
		concurrentRows, concurrentFiles := run(true)
		if !reflect.DeepEqual(sequentialRows, concurrentRows) || !reflect.DeepEqual(sequentialFiles, concurrentFiles) {
			t.Fatalf("impact exit %d: concurrent run differs:\nrows %v\nvs   %v\nfiles %v\nvs    %v", impactStatus, concurrentRows, sequentialRows, concurrentFiles, sequentialFiles)
		}
	}
}
