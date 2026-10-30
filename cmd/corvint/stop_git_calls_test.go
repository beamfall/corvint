package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/localcompletion"
)

// LCP-V0-019: the enrolled Stop keeps both repository observations and private
// policy reads, but lists content only once.
func TestEnrolledStopSpawnsBoundedGit(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh for the git shim")
	}
	root, key := handoffRepository(t)
	path, spawns := countingGit(t)
	t.Setenv("PATH", path[len("PATH="):])
	// This pins subprocess count, not latency. Use the existing test deadline
	// override so a loaded host cannot turn a count assertion into a timeout.
	ctx := context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return time.Minute })
	var stdout, stderr bytes.Buffer
	code := runLocalCompletionEvent(ctx, root, dogfoodEventArguments("stop"),
		strings.NewReader(`{"stopHookActive":false,"sessionIdSha256":"`+key+`"}`), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Stop exit %d: %s", code, stderr.String())
	}
	trace, err := os.ReadFile(filepath.Join(strings.TrimPrefix(strings.SplitN(path, string(os.PathListSeparator), 2)[0], "PATH="), "spawns.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("LCP-V0-019 enrolled Stop Git trace:\n%s", trace)
	if got := spawns(); got != 22 {
		t.Fatalf("LCP-V0-019: enrolled Stop spawned %d Git processes, want 22", got)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"decision":"block"`)) || !bytes.Contains(stdout.Bytes(), []byte(`"lifecycle":"active"`)) {
		t.Fatalf("enrolled incomplete Stop result changed: %s", stdout.String())
	}
}

// LCP-V0-019: the event's actual re-read dispatch falls back when either part
// of the closing identity mismatches; matching facts save four Git processes.
func TestDogfoodEventObservedMismatchFallsBack(t *testing.T) {
	root, key := handoffRepository(t)
	ctx := context.Background()
	want, observed, err := localcompletion.EvaluateObserved(ctx, root, key)
	if err != nil {
		t.Fatal(err)
	}
	target := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD^{tree}"))
	path, spawns := countingGit(t)
	t.Setenv("PATH", path[len("PATH="):])
	for _, test := range []struct {
		name, target, tree string
		wantCalls          int
	}{
		{"match", target, tree, 4},
		{"target mismatch", tree, tree, 8},
		{"tree mismatch", target, target, 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := observed.Matches(test.target, test.tree); got != (test.wantCalls == 4) {
				t.Fatalf("Matches = %v", got)
			}
			got, err := localEventReevaluate(ctx, root, key, observed, gokernel.Repository{CommitRevision: test.target, TreeRevision: test.tree})
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("re-read: %#v, %v; want %#v", got, err, want)
			}
			if calls := spawns(); calls != test.wantCalls {
				t.Fatalf("re-read spawns = %d, want %d", calls, test.wantCalls)
			}
		})
	}
}
