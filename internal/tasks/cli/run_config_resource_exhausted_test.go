package cli_test

import (
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

// CAL-V0-215: configuration I/O fails before either runner creates its results
// directory. Open and read faults must both survive to the caller's envelope.
func TestCALV0215_RunConfigResourceExhausted(t *testing.T) {
	r, id := obligationBatchRepo(t)
	config := rbConfig(t, []string{"/usr/bin/true"}, nil)
	if _, _, err := cli.ReadRunConfigForTest(config); err != nil {
		t.Fatalf("fixture config: %v", err)
	}
	if x := atm(t, r.Root, nil, "receipt", "audit"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("fixture audit: %s", x.stdout)
	}
	out := filepath.Join(r.Root, "results")
	before := fixture.TreeSnapshot(t, r.Root)
	for _, errno := range []error{syscall.EMFILE, syscall.ENFILE} {
		for _, boundary := range []string{"open", "read"} {
			for _, verb := range []string{"run-batch", "qualify"} {
				t.Run(verb+"/"+boundary+"/"+errno.Error(), func(t *testing.T) {
					fired, closed := 0, 0
					restore := cli.SetRunConfigOpenForTest(func(file string) (io.ReadCloser, error) {
						if file != config {
							t.Fatalf("config path: %s", file)
						}
						fault := &os.PathError{Op: boundary, Path: file, Err: errno}
						if boundary == "open" {
							fired++
							return nil, fault
						}
						return configFaultReader{read: func([]byte) (int, error) { fired++; return 0, fault }, close: func() error { closed++; return nil }}, nil
					})
					t.Cleanup(restore)
					// Verify the original errno remains available to enclosing callers.
					if _, _, err := cli.ReadRunConfigForTest(config); !errors.Is(err, errno) {
						t.Fatalf("errno lost: %v", err)
					}
					fired, closed = 0, 0
					args := []string{verb}
					if verb == "run-batch" {
						args = append(args, id)
					} else {
						args = append(args, "--base", "HEAD")
					}
					args = append(args, "--config", config, "--out", out, "--request-id", "pressure", "--spec", "e2e/batch.spec.ts")
					x := atm(t, r.Root, nil, args...)
					restore()
					wantClosed := 0
					if boundary == "read" {
						wantClosed = 1
					}
					if fired != 1 || closed != wantClosed {
						t.Fatalf("fired=%d closed=%d", fired, closed)
					}
					if x.res.Outcome != wire.OutcomeError || !reflect.DeepEqual(x.res.Codes, []string{wire.CodeResourceExhausted}) || !strings.Contains(string(x.stdout), `"retryable":true`) || !strings.Contains(string(x.stdout), `"mutation":null`) {
						t.Fatalf("result: %s", x.stdout)
					}
					if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, r.Root)) {
						t.Fatal("configuration failure changed filesystem")
					}
					if _, err := os.Stat(out); !os.IsNotExist(err) {
						t.Fatalf("results directory exists: %v", err)
					}
				})
			}
		}
	}
}

type configFaultReader struct {
	read  func([]byte) (int, error)
	close func() error
}

func (r configFaultReader) Read(p []byte) (int, error) { return r.read(p) }
func (r configFaultReader) Close() error               { return r.close() }
