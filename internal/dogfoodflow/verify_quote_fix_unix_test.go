// SPDX-License-Identifier: Apache-2.0

//go:build unix

package dogfoodflow

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestDCWV0403QuoteFixReadsTheVerifyFileOnce: the hint is built from one read
// of the verify file. A FIFO serves the refused two-line file to the first
// open and an empty file to every later one, so a second read would index a
// line that is no longer there and panic.
func TestDCWV0403QuoteFixReadsTheVerifyFileOnce(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "verify")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("mkfifo:", err)
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		if file, err := os.OpenFile(fifo, os.O_WRONLY, 0); err == nil {
			_, _ = io.WriteString(file, "go test -run TestA ./pre\ngo test -run TestA|TestB ./pre\n")
			file.Close()
		}
		// Any later reader gets end of file at once.
		for {
			select {
			case <-stop:
				return
			default:
			}
			if file, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				file.Close()
			}
			time.Sleep(time.Millisecond)
		}
	}()
	fix := quotedRunFix(fifo)
	close(stop)
	<-done
	if !strings.Contains(fix, " line 2: quote the -run alternation as one word: go test -run 'TestA|TestB' ./pre ") {
		t.Fatalf("fix = %q", fix)
	}
}
