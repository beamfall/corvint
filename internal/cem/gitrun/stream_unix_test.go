//go:build unix

package gitrun

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

type refusingStream struct{}

func (refusingStream) Write(p []byte) (int, error) {
	return 0, cemcode.New(cemcode.RepositoryObjectUnavailable, "stream refused")
}

func TestRunStreamProcessFailureParity(t *testing.T) {
	for _, tc := range []struct{ name, script string }{
		{"success", "printf hello"},
		{"exit", "printf 'doom\\nsecond line\\n' >&2; exit 3"},
		{"stderr-bound", "head -c 100000 /dev/zero >&2"},
		{"timeout", "sleep 30"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := shell()
			options.PerOpTimeout = 100 * time.Millisecond
			want, wantErr := Run(context.Background(), NewDefaultBudget(), options, "-c", tc.script)
			var got bytes.Buffer
			gotErr := RunStream(context.Background(), NewDefaultBudget(), options, &got, "-c", tc.script)
			if cemcode.CodeOf(gotErr) != cemcode.CodeOf(wantErr) {
				t.Fatalf("stream=%v buffered=%v", gotErr, wantErr)
			}
			if gotErr == nil && !bytes.Equal(got.Bytes(), want) {
				t.Fatalf("stdout %q != %q", got.Bytes(), want)
			}
			if tc.name == "exit" && gotErr.Error() != wantErr.Error() {
				t.Fatalf("exit error bytes changed: %v / %v", gotErr, wantErr)
			}
		})
	}
}

func TestRunStreamContainsDescendants(t *testing.T) {
	for _, mode := range []string{"consumer-refusal", "cancellation", "timeout", "normal-exit"} {
		t.Run(mode, func(t *testing.T) {
			options := shell()
			options.Dir = t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var sink io.Writer = io.Discard
			want := cemcode.GitTimeout
			script := "sleep 30 >/dev/null 2>&1 & child=$!; trap 'kill $child 2>/dev/null' EXIT INT TERM; echo $child > grandchild; printf ready; wait"
			if mode == "consumer-refusal" {
				sink = refusingStream{}
				want = cemcode.RepositoryObjectUnavailable
			}
			if mode == "cancellation" {
				want = cemcode.GitCancelled
				go func() { time.Sleep(150 * time.Millisecond); cancel() }()
			}
			if mode == "timeout" {
				options.PerOpTimeout = 200 * time.Millisecond
			}
			if mode == "normal-exit" {
				script = "sleep 30 >/dev/null 2>&1 & echo $! > grandchild"
			}
			err := RunStream(ctx, NewDefaultBudget(), options, sink, "-c", script)
			if mode == "normal-exit" {
				if err != nil {
					t.Fatal(err)
				}
			} else if cemcode.CodeOf(err) != want {
				t.Fatalf("got %v, want %s", err, want)
			}
			raw, readErr := os.ReadFile(options.Dir + "/grandchild")
			if readErr != nil {
				t.Fatal(readErr)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
			if err != nil {
				t.Fatal(err)
			}
			waitForDeath(t, pid)
		})
	}
}

func TestRunStreamFastExitKeepsConsumerRefusal(t *testing.T) {
	for _, exit := range []string{"0", "3"} {
		for range 16 {
			err := RunStream(context.Background(), NewDefaultBudget(), shell(), refusingStream{}, "-c", "printf x; exit "+exit)
			if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
				t.Fatalf("exit %s raced consumer error: %v", exit, err)
			}
		}
	}
}

func TestRunStreamAdmissionAndCancellationParity(t *testing.T) {
	for _, mode := range []string{"start", "budget", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			options := shell()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			operations := 1
			if mode == "start" {
				options.Binary = "/nonexistent-cem-binary"
			}
			if mode == "budget" {
				operations = 0
			}
			if mode == "cancel" {
				cancel()
			}
			_, want := Run(ctx, NewBudget(operations, time.Minute), options, "-c", "sleep 30")
			got := RunStream(ctx, NewBudget(operations, time.Minute), options, io.Discard, "-c", "sleep 30")
			if got == nil || want == nil || got.Error() != want.Error() {
				t.Fatalf("stream=%v buffered=%v", got, want)
			}
		})
	}
}
