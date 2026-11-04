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
// of the sequential run, for a produced impact and for a typed abstention.
func TestChangeDWCV0756ImpactBesideQueryMatchesTheSequentialRun(t *testing.T) {
	root := t.TempDir()
	for _, impactStatus := range []int{0, 2} {
		run := func(concurrent bool) ([]step, map[string]string, time.Duration) {
			t.Helper()
			evidence := t.TempDir()
			var mu sync.Mutex
			running, overlapped := 0, false
			steps := Runner{Path: "/bin/corvint", Concurrent: concurrent, Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
				mu.Lock()
				running++
				overlapped = overlapped || running > 1
				mu.Unlock()
				time.Sleep(100 * time.Millisecond)
				mu.Lock()
				running--
				mu.Unlock()
				switch args[0] {
				case "query":
					_, _ = io.WriteString(stdout, "{\"query\":true}\n")
					return 0
				case "impact":
					if impactStatus == 2 {
						_, _ = io.WriteString(stderr, "{\"code\": \"unsupported-impact-range\", \"error\": \"x\", \"ok\": false}\n")
						return 2
					}
					_, _ = io.WriteString(stdout, "{\"impact\":true}\n")
					return 0
				}
				return 1
			}}
			c := &change{evidence: evidence}
			c.flow = flow{ctx: context.Background(), prefix: "dogfood-change", root: root, stderr: &bytes.Buffer{}, base: strings.Repeat("a", 40), target: strings.Repeat("b", 40)}
			c.options.Steps = steps
			began := time.Now()
			impact := c.startImpact()
			c.coordinationQuery("task")
			c.prechangeImpact(impact)
			elapsed := time.Since(began)
			if concurrent != overlapped {
				t.Fatalf("concurrent=%v but the commands overlapped=%v", concurrent, overlapped)
			}
			files := map[string]string{}
			entries, _ := os.ReadDir(evidence)
			for _, entry := range entries {
				data, _ := os.ReadFile(filepath.Join(evidence, entry.Name()))
				files[entry.Name()] = string(data)
			}
			return c.rows, files, elapsed
		}
		sequentialRows, sequentialFiles, _ := run(false)
		concurrentRows, concurrentFiles, elapsed := run(true)
		if !reflect.DeepEqual(sequentialRows, concurrentRows) || !reflect.DeepEqual(sequentialFiles, concurrentFiles) {
			t.Fatalf("impact exit %d: concurrent run differs:\nrows %v\nvs   %v\nfiles %v\nvs    %v", impactStatus, concurrentRows, sequentialRows, concurrentFiles, sequentialFiles)
		}
		if elapsed >= 200*time.Millisecond {
			t.Errorf("impact exit %d: the concurrent query and impact took %s, as long as two sequential commands", impactStatus, elapsed)
		}
	}
}
