package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Synthetic runner results qualify receipt composition only. No Execute deadline
// elapses in these cases; GLTP-V0-039(b) remains BLOCKED by the frozen 30 minutes.
func TestDeadlineReceiptComposition(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(*ReceiptInput)
		failure string
	}{
		{"all-passed", func(*ReceiptInput) {}, "TIMEOUT"},
		{"incomplete-stream", func(i *ReceiptInput) { i.Decoder = DecoderTruncated }, "TIMEOUT"},
		{"nonzero-no-failed-terminal", func(i *ReceiptInput) { i.Runner.ExitCode = 1 }, "TIMEOUT"},
		{"provider-failure", func(i *ReceiptInput) { i.ProviderFailure = true; i.Runner.ProcessCleanupDone = false }, "TIMEOUT"},
		{"cancellation", func(i *ReceiptInput) { i.Runner.Cancelled = true }, "TIMEOUT"},
		{"source-drift", func(i *ReceiptInput) {
			i.SourcePostSHA256 = digestForReceipt("drift")
			i.ProviderFailure = true
			i.Runner.Cancelled = true
		}, "STALE"},
		{"toolchain-drift", func(i *ReceiptInput) {
			i.ToolchainPostSHA256 = digestForReceipt("drift")
			i.ProviderFailure = true
			i.Runner.Cancelled = true
		}, "STALE"},
		{"failed-terminals", func(i *ReceiptInput) {
			i.Events[2].Action = "fail"
			i.Events[3].Action = "fail"
			i.Observation.Packages[0].Status = "fail"
			i.Observation.Packages[0].Tests[0].Status = "fail"
			i.Runner.ExitCode = 1
		}, "TIMEOUT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := validReceiptInput(t)
			input.Runner.TimedOut = true
			tc.edit(&input)
			receipt, err := ComposeReceipt(input)
			if err != nil {
				t.Fatal(err)
			}
			var run map[string]any
			if err := json.Unmarshal(receipt.Run, &run); err != nil {
				t.Fatal(err)
			}
			execution := run["execution"].(map[string]any)
			if execution["status"] != "INCOMPLETE" || execution["failureClass"] != tc.failure || execution["limit"] != "RUN_TIME" || execution["cancelled"] != false || execution["timedOut"] != true {
				t.Fatalf("deadline outcome = %v", execution)
			}
			if directory := os.Getenv("CORVINT_DEADLINE_FIXTURES"); directory != "" {
				if err := os.WriteFile(filepath.Join(directory, tc.name+".jsonl"), receipt.Transcript, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestCancellationProviderFailureReceiptComposition(t *testing.T) {
	input := validReceiptInput(t)
	input.Runner.Cancelled = true
	input.ProviderFailure = true
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	var run map[string]any
	if err := json.Unmarshal(receipt.Run, &run); err != nil {
		t.Fatal(err)
	}
	execution := run["execution"].(map[string]any)
	if execution["cancelled"] != true || execution["failureClass"] != "INFRASTRUCTURE" || execution["status"] != "INCOMPLETE" {
		t.Fatalf("cancellation/provider-failure outcome = %v", execution)
	}
	if path := os.Getenv("CORVINT_CANCELLATION_PROVIDER_FAILURE_FIXTURE"); path != "" {
		if err := os.WriteFile(path, receipt.Transcript, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCancellationDrainOutputLimitReceiptComposition(t *testing.T) {
	input := validReceiptInput(t)
	input.Runner.Cancelled = true
	input.Runner.OutputLimitExceeded = true
	input.Decoder = DecoderTruncated
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	var run map[string]any
	if err := json.Unmarshal(receipt.Run, &run); err != nil {
		t.Fatal(err)
	}
	execution := run["execution"].(map[string]any)
	if execution["cancelled"] != true || execution["timedOut"] != false || execution["status"] != "INCOMPLETE" || execution["failureClass"] != "CANCELLATION" || execution["limit"] != nil {
		t.Fatalf("cancellation/output-limit outcome = %v", execution)
	}
	if path := os.Getenv("CORVINT_CANCELLATION_DRAIN_OUTPUT_LIMIT_FIXTURE"); path != "" {
		if err := os.WriteFile(path, receipt.Transcript, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFrozenReceiptResourceAndScopeClaims(t *testing.T) {
	input := validReceiptInput(t)
	receipt, err := ComposeReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	var run map[string]any
	if err := json.Unmarshal(receipt.Run, &run); err != nil {
		t.Fatal(err)
	}
	execution := run["execution"].(map[string]any)
	if execution["status"] != "PASSED" {
		t.Fatalf("unenforced targets withheld pass: %v", execution)
	}
	want := map[string]any{"cpuMilliseconds": nil, "memoryPeakBytes": nil, "openFilesPeak": nil, "processesPeak": nil}
	if !reflect.DeepEqual(execution["resources"], want) {
		t.Fatalf("unobserved resources = %v", execution["resources"])
	}
	scope := run["scope"].(map[string]any)
	if !reflect.DeepEqual(scope["excluded"], []any{}) || scope["conclusion"] != "UNKNOWN" {
		t.Fatalf("scope = %v", scope)
	}
	wantUnknown := []any{"BUILD_CONSTRAINT_VARIANTS", "CROSS_PLATFORM_VARIANTS", "DISCOVERY_INCOMPLETE", "EXTERNAL_MODULE_FRONTIER", "FUZZ_BENCHMARK_FRONTIER", "MANDATORY_GATE_OUTSIDE_RUN", "NESTED_MODULE_FRONTIER", "NETWORK_STATE_UNKNOWN", "NON_GO_TEST_FRONTIER", "NO_AFFECTED_SELECTION_PROOF", "PACKAGE_PATTERN_SEMANTICS", "PARENT_TEST_UNAVAILABLE", "SOURCE_ANCHOR_UNAVAILABLE", "UNOBSERVED_DYNAMIC_SUBTESTS"}
	if !reflect.DeepEqual(scope["unknownReasons"], wantUnknown) {
		t.Fatalf("unknown reasons = %v", scope["unknownReasons"])
	}
}

func TestForgedSourceBindingRefusesBeforeLaunch(t *testing.T) {
	fixture := newFixture(t, false)
	fixture.authority.binding.SourceIdentity = "workspace-source:sha256:" + digestForReceipt("foreign source B")
	transcript, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityUnavailable) || errors.Is(err, ErrAuthorityDrift) {
		t.Fatalf("forged source error = %v", err)
	}
	if transcript.Runner.Started || len(transcript.Receipt.Transcript) != 0 {
		t.Fatal("forged source launched or composed a receipt")
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

type failingDiscoveryAuthority struct {
	*testAuthority
	failure   error
	releaseOK *bool
}
type failingDiscoveryLease struct {
	*testLease
	failure   error
	releaseOK *bool
}

func (a failingDiscoveryAuthority) Acquire(ctx context.Context, r AuthorityRequest) (ExecutionLease, error) {
	lease, err := a.testAuthority.Acquire(ctx, r)
	if err != nil {
		return nil, err
	}
	return failingDiscoveryLease{lease.(*testLease), a.failure, a.releaseOK}, nil
}
func (l failingDiscoveryLease) Discover(context.Context, DiscoveryRequest, io.Writer, io.Writer) (DiscoveryResult, error) {
	return DiscoveryResult{}, l.failure
}
func (l failingDiscoveryLease) Release(ctx context.Context) error {
	err := l.testLease.Release(ctx)
	*l.releaseOK = err == nil
	return err
}
func TestDiscoveryAuthoritySentinelsRemainDistinct(t *testing.T) {
	for _, want := range []error{ErrAuthorityDrift, ErrAuthorityUnavailable} {
		t.Run(want.Error(), func(t *testing.T) {
			fixture := newFixture(t, false)
			released := false
			fixture.config.Authority = failingDiscoveryAuthority{fixture.authority, fmt.Errorf("parent refusal: %w", want), &released}
			transcript, err := Execute(context.Background(), fixture.config)
			if !released {
				t.Fatal("clean release precondition failed")
			}
			other := ErrAuthorityDrift
			if want == ErrAuthorityDrift {
				other = ErrAuthorityUnavailable
			}
			if !errors.Is(err, want) || errors.Is(err, other) {
				t.Fatalf("error chain = %v; want only %v", err, want)
			}
			if transcript.Runner.Started || len(transcript.Receipt.Transcript) != 0 {
				t.Fatal("refusal launched or emitted receipt")
			}
			assertDirectoryEmpty(t, fixture.temporaryParent)
		})
	}
}

func TestPrelaunchRevalidationErrorPreservesUnavailable(t *testing.T) {
	fixture := newFixture(t, false)
	fixture.authority.revalidationErr = fmt.Errorf("parent transport: %w", ErrAuthorityUnavailable)
	transcript, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityUnavailable) || errors.Is(err, ErrAuthorityDrift) {
		t.Fatalf("prelaunch revalidation chain = %v", err)
	}
	if transcript.Runner.Started || len(transcript.Receipt.Transcript) != 0 {
		t.Fatal("prelaunch transport refusal launched or composed receipt")
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}
