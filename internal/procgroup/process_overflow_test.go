package procgroup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRunProcessTruncateOverflowPolicy(t *testing.T) {
	for _, streams := range []string{"stdout", "stderr", "both"} {
		for _, code := range []int{0, 23} {
			t.Run(streams+"/"+strconv.Itoa(code), func(t *testing.T) {
				directory := t.TempDir()
				complete := filepath.Join(directory, "complete")
				spec := processHelperSpec(t, directory, "bounded-output",
					"PROCESS_STREAMS="+streams, "PROCESS_EXIT="+strconv.Itoa(code),
					"PROCESS_COMPLETE="+complete)
				spec.OutputLimit, spec.StderrLimit = 3, 5
				spec.OverflowPolicy = OverflowTruncate
				result := Run(context.Background(), spec)
				if result.Err != nil || result.ExitStatus != code || !result.ExitObserved ||
					!result.WaitCompleted || !result.PipesDrained || !result.OwnedProcessGroupCleanup {
					t.Fatalf("truncation lost the completed invocation: %+v", result)
				}
				wantOut, wantErr := "", ""
				if streams != "stderr" {
					wantOut = "ooo"
				}
				if streams != "stdout" {
					wantErr = "eeeee"
				}
				if string(result.Stdout) != wantOut || string(result.Stderr) != wantErr {
					t.Fatalf("retained prefixes: stdout=%q stderr=%q", result.Stdout, result.Stderr)
				}
				if !result.OutputOverflow || result.StdoutOverflow != (wantOut != "") ||
					result.StderrOverflow != (wantErr != "") {
					t.Fatalf("overflow evidence was lost: %+v", result)
				}
				if _, err := os.Stat(complete); err != nil {
					t.Fatalf("truncation stopped the producer before completion: %v", err)
				}
			})
		}
	}
}

func TestRunProcessDefaultOverflowPolicyStillTerminates(t *testing.T) {
	for _, streams := range []string{"stdout", "stderr"} {
		t.Run("CRR-V0-005 "+streams, func(t *testing.T) {
			directory := t.TempDir()
			complete := filepath.Join(directory, "complete")
			spec := processHelperSpec(t, directory, "bounded-output",
				"PROCESS_STREAMS="+streams, "PROCESS_COMPLETE="+complete, "PROCESS_HOLD_AFTER_OUTPUT=1")
			spec.OutputLimit = 3 // Zero StderrLimit inherits this cap.
			result := Run(context.Background(), spec)
			assertProcessErrorCode(t, result.Err, "process-output-overflow")
			if !result.OutputOverflow || !result.OwnedProcessGroupCleanup ||
				!result.WaitCompleted || result.TimedOut || result.Cancelled {
				t.Fatalf("default policy did not terminate on overflow: %+v", result)
			}
			if streams == "stdout" && string(result.Stdout) != "ooo" ||
				streams == "stderr" && string(result.Stderr) != "eee" {
				t.Fatalf("default cap changed: stdout=%q stderr=%q", result.Stdout, result.Stderr)
			}
			if _, err := os.Stat(complete); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("overflowing producer completed: %v", err)
			}
		})
	}
}

func TestRunProcessRejectsInvalidOverflowPolicyBeforeSpawn(t *testing.T) {
	spec := processHelperSpec(t, t.TempDir(), "fast")
	spec.OverflowPolicy = OverflowPolicy(99)
	result := Run(context.Background(), spec)
	assertProcessErrorCode(t, result.Err, "process-spec-invalid")
	if result.Started {
		t.Fatal("invalid policy started a process")
	}
	spec.OverflowPolicy = OverflowFail
	spec.StderrLimit = -1
	result = Run(context.Background(), spec)
	assertProcessErrorCode(t, result.Err, "process-spec-invalid")
	if result.Started {
		t.Fatal("invalid stderr cap started a process")
	}
}

func TestTruncateCaptureConsumesCrossingWrite(t *testing.T) {
	notify := make(chan struct{}, 1)
	capture := newProcessCapture(3, notify, OverflowTruncate)
	for _, chunk := range []string{"oo", "oo", strings.Repeat("o", 5)} {
		if n, err := capture.Write([]byte(chunk)); n != len(chunk) || err != nil {
			t.Fatalf("write consumed %d/%d bytes: %v", n, len(chunk), err)
		}
	}
	body, exceeded := capture.result()
	if string(body) != "ooo" || !exceeded {
		t.Fatalf("capture = %q, %v", body, exceeded)
	}
	select {
	case <-notify:
		t.Fatal("truncation requested process termination")
	default:
	}
}
