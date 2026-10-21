package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0829: a program or supervisor transition the writer did not complete is
// a coded error, so wire.CodeOf keeps the writer's refusal code.
func TestV10829_TransitionRefusalCarriesTheReportCode(t *testing.T) {
	refused := &Report{Kind: "Refused", Detail: "stale", Outcome: mutation.Outcome{Outcome: mutation.OutcomeBlocked, Codes: []string{wire.CodeStalePolicy, wire.CodeFenced}}}
	uncoded := &Report{Kind: "NoChange", Outcome: mutation.Outcome{Outcome: mutation.OutcomeBlocked}}
	for _, c := range []struct {
		name string
		err  error
		code string
	}{
		{"refused report", transitionRefusal("program transition WORKTREE_ADD", refused), wire.CodeStalePolicy},
		{"uncoded report", transitionRefusal("program transition ADMITTED", uncoded), wire.CodeMalformed},
		{"no report", transitionRefusal("program transition ADMITTED", nil), wire.CodeMalformed},
		{"transitionOK refused", transitionOK(refused, nil), wire.CodeStalePolicy},
	} {
		var e *wire.Error
		if !asWire(c.err, &e) || wire.CodeOf(c.err) != c.code {
			t.Fatalf("%s: %T %v, want coded %s", c.name, c.err, c.err, c.code)
		}
	}
	if e := transitionOK(&Report{Kind: "Transaction", Outcome: mutation.Outcome{Outcome: mutation.OutcomeCompleted}}, nil); e != nil {
		t.Fatalf("completed transition refused: %v", e)
	}
}

func asWire(err error, target **wire.Error) bool {
	e, ok := err.(*wire.Error)
	*target = e
	return ok
}

// V1-0829: a recovery boot that cannot be read for an operational reason
// keeps that reason's code, so descriptor exhaustion stays RESOURCE_EXHAUSTED
// and retryable. Missing, invalid or mismatched boot evidence is
// QUIESCENCE_UNPROVED and not retryable.
func TestV10829_RecoveryBootRefusalKeepsOperationalCodes(t *testing.T) {
	dir := t.TempDir()
	missing, missingErr := supervisor.RecoveryBoot(filepath.Join(dir, "absent"), "effect", supervisor.Boot{})
	if missingErr == nil {
		t.Fatalf("absent boot read: %+v", missing)
	}
	invalidDir := filepath.Join(dir, "invalid")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, "boot"), []byte("not a boot"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, invalidErr := supervisor.RecoveryBoot(invalidDir, "effect", supervisor.Boot{})
	if invalidErr == nil {
		t.Fatal("invalid boot decoded")
	}
	for _, c := range []struct {
		name      string
		err       error
		code      string
		retryable bool
	}{
		{"descriptor exhaustion", &fs.PathError{Op: "open", Path: "boot", Err: syscall.EMFILE}, wire.CodeResourceExhausted, true},
		{"file table exhaustion", &fs.PathError{Op: "open", Path: "boot", Err: syscall.ENFILE}, wire.CodeResourceExhausted, true},
		{"marked exhaustion", wire.WithoutRetry(&fs.PathError{Op: "open", Path: "boot", Err: syscall.EMFILE}), wire.CodeResourceExhausted, false},
		{"missing boot", missingErr, wire.CodeQuiescenceUnproved, false},
		{"invalid boot", invalidErr, wire.CodeQuiescenceUnproved, false},
		{"mismatched boot", errors.New("recovery leader differs"), wire.CodeQuiescenceUnproved, false},
	} {
		err := recoveryBootRefusal(c.err)
		retryable := wire.RetryOf(wire.CodeOf(err)).Retryable && !wire.RetryForbidden(err)
		if wire.CodeOf(err) != c.code || retryable != c.retryable || !strings.Contains(err.Error(), "recovery boot unavailable") {
			t.Fatalf("%s: %v (code %s retryable %v), want %s retryable %v", c.name, err, wire.CodeOf(err), retryable, c.code, c.retryable)
		}
	}
}
