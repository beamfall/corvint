// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// ASD-V0-2231: cancellation must close the pipes, because a context alone
// does not interrupt io.Pipe.Write or a JSON decoder waiting for a response.
func boundedCorpusTransport(t *testing.T, limit time.Duration, serve func(context.Context, io.Reader, io.Writer) error) (*io.PipeWriter, *io.PipeReader) {
	t.Helper()
	input, clientInput := io.Pipe()
	clientOutput, output := io.Pipe()
	ctx, cancel := context.WithTimeout(t.Context(), limit)
	closePipes := func(err error) {
		_ = input.CloseWithError(err)
		_ = output.CloseWithError(err)
	}
	stop := context.AfterFunc(ctx, func() { closePipes(ctx.Err()) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := serve(ctx, input, output)
		if err == nil {
			err = io.EOF
		}
		closePipes(err)
	}()
	t.Cleanup(func() {
		cancel()
		stop()
		closePipes(context.Canceled)
		_ = clientInput.Close()
		_ = clientOutput.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("MCP cancellation left server running after 5s cleanup bound")
		}
	})
	return clientInput, clientOutput
}

func TestASDV02231CorpusWindowsRefusal(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows reader refusal")
	}
	_, err := doccorpus.ReadFile(t.TempDir(), "missing.json")
	var refusal *doccorpus.Error
	if !errors.As(err, &refusal) || refusal.Code != doccorpus.PlatformUnsupportedCode {
		t.Fatalf("Windows reader must refuse before opening input: %v", err)
	}
	// The transport's startup-refusal regression also runs on Windows and
	// proves that this unsupported reader cannot strand the client pipes.
}

func TestASDV02231CorpusTransportBound(t *testing.T) {
	t.Run("startup_refusal", func(t *testing.T) {
		root := t.TempDir()
		input, output := boundedCorpusTransport(t, time.Second, func(ctx context.Context, in io.Reader, out io.Writer) error {
			var stderr bytes.Buffer
			code := run(ctx, []string{"--root", root, "--artifact", "missing.json"}, in, out, &stderr)
			if code != 2 || stderr.String() != "corvint-corpus-mcp: repository unavailable\n" {
				return fmt.Errorf("unexpected startup result %d: %s", code, &stderr)
			}
			return errors.New("observed bounded repository refusal")
		})
		if _, err := io.WriteString(input, "{}\n"); err == nil || err.Error() != "observed bounded repository refusal" {
			t.Fatalf("write after startup refusal: %v", err)
		}
		if _, err := output.Read(make([]byte, 1)); err == nil || err.Error() != "observed bounded repository refusal" {
			t.Fatalf("read after startup refusal: %v", err)
		}
	})
	for _, operation := range []string{"write", "read"} {
		t.Run("blocked_"+operation, func(t *testing.T) {
			input, output := boundedCorpusTransport(t, 100*time.Millisecond, func(ctx context.Context, in io.Reader, _ io.Writer) error {
				if operation == "read" {
					if _, err := io.ReadFull(in, make([]byte, 1)); err != nil {
						return err
					}
				}
				<-ctx.Done()
				return ctx.Err()
			})
			_, err := input.Write([]byte("x"))
			if operation == "read" {
				if err != nil {
					t.Fatal(err)
				}
				_, err = output.Read(make([]byte, 1))
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("blocked %s: %v, want deadline exceeded", operation, err)
			}
		})
	}
}
