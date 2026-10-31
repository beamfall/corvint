package wire

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
)

// CAL-V0-215: errno identity, not message text, determines exhaustion.
func TestCALV0215_ResourceExhausted(t *testing.T) {
	for _, errno := range []error{syscall.EMFILE, syscall.ENFILE} {
		for _, err := range []error{errno, &os.PathError{Op: "open", Path: "head.json", Err: errno}, &os.SyscallError{Syscall: "open", Err: errno}, fmt.Errorf("read: %w", &os.PathError{Op: "open", Path: "head.json", Err: errno})} {
			if got := CodeOf(err); got != CodeResourceExhausted || !IsCode(got) {
				t.Fatalf("CodeOf(%v) = %s", err, got)
			}
		}
	}
	r := RetryOf(CodeResourceExhausted)
	if !r.Retryable || strings.TrimSpace(r.Condition) == "" {
		t.Fatalf("classification: %+v", r)
	}
	if retry, present := ResultRetryable(OutcomeError, []string{CodeResourceExhausted}); !retry || !present {
		t.Fatal("exhaustion is not retryable")
	}
	if retry, present := ResultRetryable(OutcomeError, []string{CodeResourceExhausted, CodeFenced}); retry || !present {
		t.Fatal("exhaustion overrode fencing")
	}
	if CodeOf(nil) != "" || CodeOf(errors.New(syscall.EMFILE.Error())) != CodeMalformed || CodeOf(syscall.EACCES) != CodeMalformed {
		t.Fatal("unrelated errors changed")
	}
	typed := Errorf(CodeUnsupportedFilesystem, "open", "explicit refusal")
	if CodeOf(errors.Join(typed, syscall.EMFILE)) != CodeUnsupportedFilesystem {
		t.Fatal("explicit code lost precedence")
	}
}

// CAL-V0-215: post-effect marks survive raw errno and wrapper classification.
func TestCALV0215_WithoutRetry(t *testing.T) {
	for _, errno := range []error{syscall.EMFILE, syscall.ENFILE} {
		original := fmt.Errorf("read: %w", &os.PathError{Op: "open", Path: "head.json", Err: errno})
		for _, err := range []error{errno, original, Errorf(CodeResourceExhausted, "open", "pressure")} {
			marked := fmt.Errorf("after program: %w", WithoutRetry(err))
			if CodeOf(marked) != CodeResourceExhausted || !RetryForbidden(marked) || RetryForbidden(err) {
				t.Fatalf("lost mark or mutated original: %v", marked)
			}
			if errors.Is(err, errno) && !errors.Is(marked, errno) {
				t.Fatal("lost errno identity")
			}
			raw, e := (&Result{Command: []string{"run"}, Outcome: OutcomeError, Codes: []string{CodeOf(marked)}, NotRetryable: RetryForbidden(marked)}).Encode()
			if e != nil || !strings.Contains(string(raw), `"retryable":false`) {
				t.Fatalf("envelope: %s %v", raw, e)
			}
		}
	}
}

// PSR-V0-901: a host out of disk space or inotify watches (ENOSPC) is resource
// exhaustion, not a malformed request; explicit codes keep precedence.
func TestPSRV0901_NoSpaceIsResourceExhausted(t *testing.T) {
	for _, err := range []error{syscall.ENOSPC, &os.PathError{Op: "inotify_add_watch", Path: "tickets", Err: syscall.ENOSPC}, &os.SyscallError{Syscall: "write", Err: syscall.ENOSPC}, fmt.Errorf("guard: %w", &os.PathError{Op: "write", Path: "head.json", Err: syscall.ENOSPC})} {
		if got := CodeOf(err); got != CodeResourceExhausted {
			t.Fatalf("CodeOf(%v) = %s", err, got)
		}
	}
	if CodeOf(errors.New(syscall.ENOSPC.Error())) != CodeMalformed {
		t.Fatal("message text classified as exhaustion")
	}
	if CodeOf(errors.Join(Errorf(CodeUnsupportedFilesystem, "open", "explicit refusal"), syscall.ENOSPC)) != CodeUnsupportedFilesystem {
		t.Fatal("explicit code lost precedence")
	}
	if !RetryForbidden(fmt.Errorf("after program: %w", WithoutRetry(syscall.ENOSPC))) {
		t.Fatal("post-effect mark lost")
	}
}

// TestErrorfWrapsCause: a %w operand in Errorf's format is reachable through
// errors.Is and errors.As, the message reads as %v would, and the code is the
// outer one (V1-2878).
func TestErrorfWrapsCause(t *testing.T) {
	cause := syscall.EROFS
	err := Errorf(CodeUnsupportedFilesystem, "/repo/.git", "directory sync failed: %w", cause)
	if !errors.Is(err, syscall.EROFS) {
		t.Fatalf("errors.Is did not reach the cause: %v", err)
	}
	if want := "UNSUPPORTED_FILESYSTEM: /repo/.git: directory sync failed: " + cause.Error(); err.Error() != want {
		t.Fatalf("message %q, want %q", err.Error(), want)
	}
	inner := Errorf(CodeAdjudication, "", "inner")
	outer := Errorf(CodeUnsupportedFilesystem, "", "%w and %w", inner, syscall.ENOSPC)
	if CodeOf(outer) != CodeUnsupportedFilesystem || !errors.Is(outer, inner) || !errors.Is(outer, syscall.ENOSPC) {
		t.Fatalf("multi-%%w: code %s, is inner %v", CodeOf(outer), errors.Is(outer, inner))
	}
	if plain := Errorf(CodeAdjudication, "", "no cause %v", syscall.EIO); errors.Is(plain, syscall.EIO) || plain.Unwrap() != nil {
		t.Fatal("a plain operand must not be wrapped")
	}
}
