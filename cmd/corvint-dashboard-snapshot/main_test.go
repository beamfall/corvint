package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/dashboard/model"
)

const (
	testBase   = "0123456789abcdef0123456789abcdef01234567"
	testTarget = "89abcdef0123456789abcdef0123456789abcdef"
	testTime   = "2026-08-23T12:34:56.123456789Z"
)

func TestParseArgumentsAcceptsExactContract(t *testing.T) {
	root := t.TempDir()
	arguments := []string{
		"snapshot", "--source", "local-trace-v1=.context-corvint/traces",
		"--root", root, "--source", "query-envelope-v1=receipts/query.json",
		"--cem", ".corvint/change.cem.json", "--ocm", ".corvint/change.ocm.json",
		"--cem-ocm-profile", "cem/0.2+ocm/0.1",
		"--expected-base", testBase, "--target", testTarget,
		"--conformance", "--generated-at", testTime,
	}
	got, err := parseArguments(arguments)
	if err != nil {
		t.Fatal(err)
	}
	want := options{
		Root: root,
		Sources: []sourceOption{
			{AdapterID: "local-trace-v1", RelativePath: ".context-corvint/traces"},
			{AdapterID: "query-envelope-v1", RelativePath: "receipts/query.json"},
		},
		Bundle: &bundleOption{
			CEM: ".corvint/change.cem.json", OCM: ".corvint/change.ocm.json",
			Profile:      "cem/0.2+ocm/0.1",
			ExpectedBase: testBase, Target: testTarget,
		},
		GeneratedAt: testTime,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %#v, want %#v", got, want)
	}
}

func TestParseArgumentsDoesNotAcceptAmbientClockInjection(t *testing.T) {
	t.Setenv("CORVINT_DASHBOARD_GENERATED_AT", testTime)
	t.Setenv("SOURCE_DATE_EPOCH", "1787488496")
	parsed, err := parseArguments([]string{"snapshot", "--root", t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.GeneratedAt != "" {
		t.Fatalf("generatedAt = %q, want process clock", parsed.GeneratedAt)
	}
}

func TestPathArgumentsUseExact4096ByteCap(t *testing.T) {
	rootPrefix := filepath.VolumeName(t.TempDir()) + string(os.PathSeparator)
	rootAtLimit := rootPrefix + strings.Repeat("a", maxArgumentBytes-len(rootPrefix))
	rootOverLimit := rootAtLimit + "a"
	if _, err := parseArguments([]string{"snapshot", "--root", rootAtLimit}); err != nil {
		t.Fatalf("rejected %d-byte root: %v", len(rootAtLimit), err)
	}
	if _, err := parseArguments([]string{"snapshot", "--root", rootOverLimit}); err == nil {
		t.Fatalf("accepted %d-byte root", len(rootOverLimit))
	}
	pathAtLimit := strings.Repeat("a", maxArgumentBytes)
	pathOverLimit := pathAtLimit + "a"
	if _, ok := parseSource("local-trace-v1=" + pathAtLimit); !ok {
		t.Fatalf("rejected %d-byte relative path", len(pathAtLimit))
	}
	if _, ok := parseSource("local-trace-v1=" + pathOverLimit); ok {
		t.Fatalf("accepted %d-byte relative path", len(pathOverLimit))
	}

	bundleArguments := func(cem, ocm string) []string {
		return []string{
			"snapshot", "--root", string(os.PathSeparator),
			"--cem", cem, "--ocm", ocm,
			"--cem-ocm-profile", "cem/0.1+ocm/0.1",
			"--expected-base", testBase, "--target", testTarget,
		}
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "cem at limit", args: bundleArguments(pathAtLimit, "ocm.json")},
		{name: "ocm at limit", args: bundleArguments("cem.json", pathAtLimit)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseArguments(test.args); err != nil {
				t.Fatalf("rejected 4096-byte path: %v", err)
			}
		})
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "cem over limit", args: bundleArguments(pathOverLimit, "ocm.json")},
		{name: "ocm over limit", args: bundleArguments("cem.json", pathOverLimit)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseArguments(test.args); err == nil {
				t.Fatal("accepted 4097-byte path")
			}
		})
	}
}

func TestSourceAdapterIDUsesIndependent128ByteCap(t *testing.T) {
	pathAtLimit := strings.Repeat("p", maxArgumentBytes)
	adapterAtLimit := strings.Repeat("a", maxAdapterIDBytes)
	if _, ok := parseSource(adapterAtLimit + "=" + pathAtLimit); !ok {
		t.Fatal("rejected independently bounded adapter and relative path")
	}
	if _, ok := parseSource(adapterAtLimit + "a=path"); ok {
		t.Fatal("accepted 129-byte adapter ID")
	}
	pathWithEquals := strings.Repeat("p", maxArgumentBytes/2) + "=" + strings.Repeat("p", maxArgumentBytes/2-1)
	parsed, ok := parseSource("local-trace-v1=" + pathWithEquals)
	if !ok || parsed.RelativePath != pathWithEquals {
		t.Fatal("rejected 4096-byte relative path containing equals")
	}
}

func TestParseArgumentsRejectsInvalidAndHostileInputs(t *testing.T) {
	root := t.TempDir()
	valid := []string{"snapshot", "--root", root}
	tests := map[string][]string{
		"empty":                       nil,
		"wrong subcommand":            {"serve", "--root", root},
		"missing root":                {"snapshot"},
		"relative root":               {"snapshot", "--root", "."},
		"unclean root":                {"snapshot", "--root", root + "/."},
		"duplicate root":              append(append([]string{}, valid...), "--root", root),
		"unknown flag":                append(append([]string{}, valid...), "--bogus", "x"),
		"flag without value":          append(append([]string{}, valid...), "--source"),
		"bare argument":               append(append([]string{}, valid...), "hostile"),
		"source without equals":       append(append([]string{}, valid...), "--source", "local-trace-v1"),
		"source bad adapter":          append(append([]string{}, valid...), "--source", "Local_Trace=ok"),
		"source absolute":             append(append([]string{}, valid...), "--source", "local-trace-v1=/tmp/x"),
		"source traversal":            append(append([]string{}, valid...), "--source", "local-trace-v1=a/../b"),
		"source backslash":            append(append([]string{}, valid...), "--source", `local-trace-v1=a\b`),
		"source control":              append(append([]string{}, valid...), "--source", "local-trace-v1=a\nb"),
		"bundle one flag":             append(append([]string{}, valid...), "--cem", "cem.json"),
		"bundle three flags":          append(append([]string{}, valid...), "--cem", "cem.json", "--ocm", "ocm.json", "--target", testTarget),
		"bundle four without profile": append(append([]string{}, valid...), "--cem", "cem.json", "--ocm", "ocm.json", "--expected-base", testBase, "--target", testTarget),
		"unsupported bundle profile":  append(append([]string{}, valid...), "--cem", "cem.json", "--ocm", "ocm.json", "--cem-ocm-profile", "cem/9+ocm/9", "--expected-base", testBase, "--target", testTarget),
		"uppercase object":            append(append([]string{}, valid...), "--expected-base", strings.ToUpper(testBase)),
		"generated ordinary":          append(append([]string{}, valid...), "--generated-at", testTime),
		"duplicate conformance":       append(append([]string{}, valid...), "--conformance", "--conformance", "--generated-at", testTime),
		"duplicate generated":         append(append([]string{}, valid...), "--conformance", "--generated-at", testTime, "--generated-at", testTime),
		"conformance without time":    append(append([]string{}, valid...), "--conformance"),
		"generated offset":            append(append([]string{}, valid...), "--conformance", "--generated-at", "2026-08-23T12:34:56.123456789+00:00"),
		"generated short fraction":    append(append([]string{}, valid...), "--conformance", "--generated-at", "2026-08-23T12:34:56.123Z"),
		"generated impossible":        append(append([]string{}, valid...), "--conformance", "--generated-at", "2026-02-30T12:34:56.123456789Z"),
	}
	duplicateSource := append(append([]string{}, valid...), "--source", "local-trace-v1=a")
	tests["duplicate source"] = append(duplicateSource, "--source", "local-trace-v1=a")
	for name, arguments := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseArguments(arguments); err == nil {
				t.Fatal("accepted invalid arguments")
			}
		})
	}
}

func TestRunContextWritesExactSnapshotOnlyOnSuccess(t *testing.T) {
	root := t.TempDir()
	want := validSnapshot(t)
	var stdout, stderr bytes.Buffer
	exit := runContext(context.Background(), []string{"snapshot", "--root", root}, &stdout, &stderr,
		func(_ context.Context, request compileRequest) ([]byte, error) {
			if request.Root != root {
				t.Fatalf("root = %q, want %q", request.Root, root)
			}
			return append([]byte(nil), want...), nil
		})
	if exit != 0 || !bytes.Equal(stdout.Bytes(), want) || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.Bytes(), stderr.Bytes())
	}
}

func TestRunContextFatalErrorsHaveNoStdoutAndCanonicalStderr(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "repository", err: &dashboardError{code: errorRepository}, code: errorRepository},
		{name: "resource", err: &dashboardError{code: errorResource}, code: errorResource},
		{name: "interrupted", err: context.Canceled, code: errorInterrupted},
		{name: "unknown", err: errors.New("hostile /secret/path"), code: errorInternal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := runContext(context.Background(), []string{"snapshot", "--root", root}, &stdout, &stderr,
				func(context.Context, compileRequest) ([]byte, error) { return nil, test.err })
			want := `{"code":"` + test.code + `","profile":"corvint-dashboard-error/0"}` + "\n"
			if exit != 2 || stdout.Len() != 0 || stderr.String() != want {
				t.Fatalf("exit=%d stdout=%q stderr=%q want=%q", exit, stdout.Bytes(), stderr.Bytes(), want)
			}
		})
	}
}

func TestRunContextRejectsInvalidCompilerOutput(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		body []byte
		code string
	}{
		{name: "empty", code: errorInternal},
		{name: "canonical-looking object without snapshot proof", body: []byte("{}\n"), code: errorInternal},
		{name: "missing newline", body: []byte("{}"), code: errorInternal},
		{name: "oversized", body: append(make([]byte, maxSnapshotBytes), '\n'), code: errorResource},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := runContext(context.Background(), []string{"snapshot", "--root", root}, &stdout, &stderr,
				func(context.Context, compileRequest) ([]byte, error) { return test.body, nil })
			if exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), test.code) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.Bytes(), stderr.Bytes())
			}
		})
	}
}

func TestRunContextContainsCompilerPanic(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	exit := runContext(context.Background(), []string{"snapshot", "--root", root}, &stdout, &stderr,
		func(context.Context, compileRequest) ([]byte, error) { panic("hostile /secret/path") })
	want := `{"code":"DASHBOARD_INTERNAL_ERROR","profile":"corvint-dashboard-error/0"}` + "\n"
	if exit != 2 || stdout.Len() != 0 || stderr.String() != want {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.Bytes(), stderr.Bytes())
	}
}

type writeAction struct {
	written  int
	retained int
	err      error
	panic    bool
}

type retainingWriter struct {
	actions []writeAction
	output  bytes.Buffer
}

func (writer *retainingWriter) Write(body []byte) (int, error) {
	if len(writer.actions) == 0 {
		writer.output.Write(body)
		return len(body), nil
	}
	action := writer.actions[0]
	writer.actions = writer.actions[1:]
	if action.retained > 0 && action.retained <= len(body) {
		writer.output.Write(body[:action.retained])
	}
	if action.panic {
		panic("hostile writer /secret/path")
	}
	if action.retained == 0 && action.written > 0 && action.written <= len(body) {
		writer.output.Write(body[:action.written])
	}
	return action.written, action.err
}

func TestRunContextLoopsOnPositiveShortWrites(t *testing.T) {
	root := t.TempDir()
	body := validSnapshot(t)
	writer := &retainingWriter{actions: []writeAction{{written: 1}, {written: 1}}}
	var stderr bytes.Buffer
	exit := runContext(context.Background(), []string{"snapshot", "--root", root}, writer, &stderr,
		func(context.Context, compileRequest) ([]byte, error) { return body, nil })
	if exit != 0 || !bytes.Equal(writer.output.Bytes(), body) || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, writer.output.Bytes(), stderr.Bytes())
	}
}

func TestRunContextOutputWriteFailuresAreStableAndRetainOnlyAcceptedPrefix(t *testing.T) {
	writeFailure := errors.New("hostile /secret/path")
	body := validSnapshot(t)
	tests := []struct {
		name    string
		actions []writeAction
		prefix  string
	}{
		{name: "immediate error", actions: []writeAction{{err: writeFailure}}},
		{name: "partial then error", actions: []writeAction{{written: 1}, {written: 1, err: writeFailure}}, prefix: string(body[:2])},
		{name: "full length with error", actions: []writeAction{{written: len(body), err: writeFailure}}, prefix: string(body)},
		{name: "zero progress", actions: []writeAction{{written: 0}}},
		{name: "negative count", actions: []writeAction{{written: -1, retained: 1}}, prefix: string(body[:1])},
		{name: "oversized count", actions: []writeAction{{written: len(body) + 1, retained: 1}}, prefix: string(body[:1])},
		{name: "panic before output", actions: []writeAction{{panic: true}}},
		{name: "panic after retaining", actions: []writeAction{{retained: 1, panic: true}}, prefix: string(body[:1])},
		{name: "panic after retaining full length", actions: []writeAction{{retained: len(body), panic: true}}, prefix: string(body)},
		{name: "partial then panic", actions: []writeAction{{written: 1}, {panic: true}}, prefix: string(body[:1])},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writer := &retainingWriter{actions: append([]writeAction(nil), test.actions...)}
			var stderr bytes.Buffer
			exit := runContext(context.Background(), []string{"snapshot", "--root", t.TempDir()}, writer, &stderr,
				func(context.Context, compileRequest) ([]byte, error) { return body, nil })
			wantError := `{"code":"OUTPUT_WRITE_FAILED","profile":"corvint-dashboard-error/0"}` + "\n"
			if exit != 2 || writer.output.String() != test.prefix || stderr.String() != wantError {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, writer.output.Bytes(), stderr.Bytes())
			}
			if strings.Contains(stderr.String(), "secret") {
				t.Fatalf("stderr disclosed writer error: %q", stderr.String())
			}
		})
	}
}

func TestWriteSnapshotRejectsFailureAtEveryAcceptedPrefix(t *testing.T) {
	body := []byte("{\"ok\":true}\n")
	failure := errors.New("hostile /secret/path")
	for accepted := 0; accepted <= len(body); accepted++ {
		t.Run("error-prefix-"+strconv.Itoa(accepted), func(t *testing.T) {
			actions := prefixFailureActions(len(body), accepted, writeAction{err: failure})
			writer := &retainingWriter{actions: actions}
			if writeSnapshot(writer, body) {
				t.Fatal("write succeeded")
			}
			if got, want := writer.output.Bytes(), body[:accepted]; !bytes.Equal(got, want) {
				t.Fatalf("retained=%q, want=%q", got, want)
			}
		})
		t.Run("panic-prefix-"+strconv.Itoa(accepted), func(t *testing.T) {
			actions := prefixFailureActions(len(body), accepted, writeAction{panic: true})
			writer := &retainingWriter{actions: actions}
			if writeSnapshot(writer, body) {
				t.Fatal("write succeeded")
			}
			if got, want := writer.output.Bytes(), body[:accepted]; !bytes.Equal(got, want) {
				t.Fatalf("retained=%q, want=%q", got, want)
			}
		})
	}
}

func prefixFailureActions(total, accepted int, failure writeAction) []writeAction {
	if accepted == total {
		failure.written = total
		failure.retained = total
		return []writeAction{failure}
	}
	if accepted == 0 {
		return []writeAction{failure}
	}
	return []writeAction{{written: accepted}, failure}
}

func TestRunContextInvalidArgumentsDoNotInvokeCompilerOrEchoInput(t *testing.T) {
	var called atomic.Bool
	var stdout, stderr bytes.Buffer
	hostile := "../../secret-π\nTOKEN"
	exit := runContext(context.Background(), []string{"snapshot", "--root", hostile}, &stdout, &stderr,
		func(context.Context, compileRequest) ([]byte, error) {
			called.Store(true)
			return nil, nil
		})
	if exit != 2 || called.Load() || stdout.Len() != 0 {
		t.Fatalf("exit=%d called=%v stdout=%q", exit, called.Load(), stdout.Bytes())
	}
	if strings.Contains(stderr.String(), "secret") || strings.Contains(stderr.String(), "TOKEN") {
		t.Fatalf("stderr disclosed hostile argument: %q", stderr.String())
	}
}

func TestRunContextCancellationStopsCompilation(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan int, 1)
	var stdout, stderr bytes.Buffer
	go func() {
		done <- runContext(ctx, []string{"snapshot", "--root", root}, &stdout, &stderr,
			func(ctx context.Context, _ compileRequest) ([]byte, error) {
				close(started)
				<-ctx.Done()
				return nil, ctx.Err()
			})
	}()
	<-started
	cancel()
	select {
	case exit := <-done:
		if exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), errorInterrupted) {
			t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.Bytes(), stderr.Bytes())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not stop after cancellation")
	}
}

func TestRunContextDeterministicAndReadOnly(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "marker")
	if err := os.WriteFile(marker, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(marker)
	if err != nil {
		t.Fatal(err)
	}
	body := validSnapshot(t)
	compile := func(context.Context, compileRequest) ([]byte, error) { return body, nil }
	outputs := make([]string, 2)
	for index := range outputs {
		var stdout, stderr bytes.Buffer
		if exit := runContext(context.Background(), []string{"snapshot", "--root", root}, &stdout, &stderr, compile); exit != 0 {
			t.Fatalf("run %d exit=%d stderr=%q", index, exit, stderr.String())
		}
		outputs[index] = stdout.String()
	}
	after, err := os.Stat(marker)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if outputs[0] != outputs[1] || string(contents) != "unchanged" || !os.SameFile(before, after) || before.ModTime() != after.ModTime() {
		t.Fatalf("outputs=%q marker=%q before=%v after=%v", outputs, contents, before.ModTime(), after.ModTime())
	}
}

func TestSupportedPlatformAllowlist(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		if !supportedPlatform(goos) {
			t.Fatalf("qualified platform %q rejected", goos)
		}
	}
	for _, goos := range []string{"freebsd", "openbsd", "plan9", "js"} {
		if supportedPlatform(goos) {
			t.Fatalf("unqualified platform %q accepted", goos)
		}
	}
}

func validSnapshot(t testing.TB) []byte {
	t.Helper()
	_, encoded, err := model.Compile(model.Input{
		GeneratedAt: testTime,
		Observation: model.ObservationInput{
			ClockSource: model.ClockCaller,
			Start:       testTime,
			End:         testTime,
			ScanState:   model.ScanComplete,
		},
		Repository: model.Repository{WorktreeState: model.WorktreeUnknown},
		Registry:   validRegistry(),
	})
	if err != nil {
		t.Fatalf("compile valid snapshot fixture: %v", err)
	}
	return encoded
}

func validRegistry() []model.AdapterRegistration {
	defaultTrace := ".context-corvint/traces"
	return []model.AdapterRegistration{
		{AdapterID: "beamfall-shadow-v0", DeliveryStage: model.DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "BEAMFALL_SHADOW", VerifierID: "unsupported"},
		{AdapterID: "cem-ocm-bundle-v0", AcceptedProfiles: []string{"cem/0.1+ocm/0.1", "cem/0.2+ocm/0.1"}, DeliveryStage: model.DeliveryNotStarted, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "CEM_OCM_BUNDLE", VerifierID: "go-cem-ocm-bundle-v0"},
		{AdapterID: "frontier-usage-v0", DeliveryStage: model.DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "FRONTIER_RECEIPT", VerifierID: "unsupported"},
		{AdapterID: "go-live-usage-v0", DeliveryStage: model.DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "GO_LIVE_RECEIPT", VerifierID: "unsupported"},
		{AdapterID: "harness-usage-v0", DeliveryStage: model.DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "HARNESS_RECEIPT", VerifierID: "unsupported"},
		{AdapterID: "head-spec-index-v0", DeliveryStage: model.DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "HEAD_SPEC_INDEX", VerifierID: "unsupported"},
		{AdapterID: "impact-envelope-v1", DeliveryStage: model.DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "IMPACT_ENVELOPE", VerifierID: "unsupported"},
		{AdapterID: "local-trace-v1", AcceptedProfiles: []string{"corvint-local-trace/1"}, DefaultLocation: &defaultTrace, DeliveryStage: model.DeliveryNotStarted, IssueCodes: []string{"OBSERVATION_TIME_UNKNOWN", "REPOSITORY_OBJECT_UNAVAILABLE", "SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_INVALID_IDENTITY", "SOURCE_INVALID_SCHEMA", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_NOT_PRESENT", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK", "STORE_CHANGED", "TRACE_ANCESTRY_BOUND", "TRACE_STORE_BOUND", "UNSUPPORTED_OBJECT_ALTERNATES", "VERIFIER_REJECTED"}, MaxBytes: "16777216", SourceKind: "LOCAL_TRACE_STORE", VerifierID: "go-local-trace-v1"},
		{AdapterID: "pulse-dogfood-v0", DeliveryStage: model.DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "PULSE_RECEIPT", VerifierID: "unsupported"},
		{AdapterID: "query-envelope-v1", DeliveryStage: model.DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"}, MaxBytes: "0", SourceKind: "QUERY_ENVELOPE", VerifierID: "unsupported"},
		{AdapterID: "stable-read-v0", AcceptedProfiles: []string{"dashboard-stable-read/0"}, DeliveryStage: model.DeliveryNotStarted, IssueCodes: []string{"SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_NOT_PRESENT", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK"}, MaxBytes: "16777216", SourceKind: "INTERNAL", VerifierID: "go-stable-read-v0"},
	}
}

func TestCLIInvalidArgumentsBlackBox(t *testing.T) {
	stdout, stderr := runCLIHelper(t, []string{"snapshot", "--root", "../../hostile"}, nil)
	want := `{"code":"DASHBOARD_INVALID_ARGUMENT","profile":"corvint-dashboard-error/0"}` + "\n"
	if len(stdout) != 0 || string(stderr) != want {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCLIClockContractBlackBox(t *testing.T) {
	root := string(os.PathSeparator)
	internal := `{"code":"DASHBOARD_INTERNAL_ERROR","profile":"corvint-dashboard-error/0"}` + "\n"
	invalid := `{"code":"DASHBOARD_INVALID_ARGUMENT","profile":"corvint-dashboard-error/0"}` + "\n"
	tests := []struct {
		name   string
		args   []string
		env    []string
		stderr string
	}{
		{
			name:   "paired conformance clock",
			args:   []string{"snapshot", "--conformance", "--generated-at", testTime, "--root", root},
			stderr: internal,
		},
		{
			name:   "conformance alone",
			args:   []string{"snapshot", "--conformance", "--root", root},
			stderr: invalid,
		},
		{
			name:   "generated at alone",
			args:   []string{"snapshot", "--generated-at", testTime, "--root", root},
			stderr: invalid,
		},
		{
			name:   "ambient values do not inject caller clock",
			args:   []string{"snapshot", "--root", root},
			env:    []string{"CORVINT_DASHBOARD_GENERATED_AT=" + testTime, "SOURCE_DATE_EPOCH=1787488496"},
			stderr: internal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := runCLIHelper(t, test.args, test.env)
			if len(stdout) != 0 || string(stderr) != test.stderr {
				t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func runCLIHelper(t *testing.T, arguments, environment []string) ([]byte, []byte) {
	t.Helper()
	commandArguments := append([]string{"-test.run=TestCLIHelperProcess", "--"}, arguments...)
	command := exec.Command(os.Args[0], commandArguments...)
	command.Env = append(append(os.Environ(), "CORVINT_DASHBOARD_TEST_HELPER=1"), environment...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("error=%v", err)
	}
	return stdout.Bytes(), stderr.Bytes()
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("CORVINT_DASHBOARD_TEST_HELPER") != "1" {
		return
	}
	separator := 0
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index + 1
			break
		}
	}
	if separator == 0 {
		os.Exit(97)
	}
	arguments := os.Args[separator:]
	expectedGeneratedAt := ""
	for index, argument := range arguments {
		if argument == "--generated-at" && index+1 < len(arguments) {
			expectedGeneratedAt = arguments[index+1]
		}
	}
	exit := runContext(context.Background(), arguments, os.Stdout, os.Stderr,
		func(_ context.Context, request compileRequest) ([]byte, error) {
			if request.GeneratedAt != expectedGeneratedAt {
				return nil, &dashboardError{code: errorResource}
			}
			return nil, &dashboardError{code: errorInternal}
		})
	os.Exit(exit)
}
