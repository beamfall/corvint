package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func TestEmitQualifiedRetainsCanonicalUnknowns(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	r := jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, Kind: "e2e", External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown"}, Infrastructure: &jstestprovider.InfrastructureFailure{Reason: "server-not-ready", Detail: "fixture"}}
	var stdout, stderr bytes.Buffer
	if err := emit(&stdout, &stderr, r, root); err == nil {
		t.Fatal("infrastructure exit became success")
	}
	want, err := jstestprovider.EncodeQualified(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatal("stdout is not canonical")
	}
	files, err := filepath.Glob(filepath.Join(root, ".corvint", "test-evidence", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("retained %v %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || !bytes.Equal(data, want) {
		t.Fatal("retained bytes differ")
	}
	if _, err = testvaliditydoc.Decode(data); err != nil {
		t.Fatal(err)
	}
}

func TestE2ERequiresCompleteTypedApplicationAttestationFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--app-attestation-command", `["/bin/echo"]`},
		{"--app-attestation-config", "attestation.json"},
		{"--app-attestation-command", `{}`, "--app-attestation-config", "attestation.json"},
	} {
		if err := runE2E(args); err == nil || !strings.Contains(err.Error(), "app-attestation-command and app-attestation-config") {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
	if err := runE2E([]string{"--app-attestation-command", `["/bin/false"]`, "--app-attestation-config", "/missing"}); err == nil || !strings.Contains(err.Error(), "application-attestation-requires-external-server") {
		t.Fatalf("attestation without external mode err=%v", err)
	}
}

func TestSensitiveInputPolicyRequiresExplicitProfileSelection(t *testing.T) {
	t.Run("PWP-V2-001 explicit profile selection", func(t *testing.T) {
		if err := runE2E([]string{"--sensitive-action-pattern", "set secret"}); err == nil || !strings.Contains(err.Error(), "require --sensitive-input-redaction") {
			t.Fatalf("implicit sensitive-input profile selection err=%v", err)
		}
		if err := runE2E([]string{"--sensitive-input-redaction"}); err == nil || !strings.Contains(err.Error(), "sensitive-input-redaction-requires-external-server") {
			t.Fatalf("non-external sensitive-input selection err=%v", err)
		}
	})
}

// PWP-V0-010: the CLI option reaches RunE2E, which refuses it outside
// external-server mode.
func TestKeepReportersFlagRequiresExternalServer(t *testing.T) {
	if err := runE2E([]string{"--keep-reporters"}); err == nil || !strings.Contains(err.Error(), "keep-reporters-unsupported-mode") {
		t.Fatalf("keep-reporters without external mode err=%v", err)
	}
}

// PWP-V0-015, PWP-V0-018: qualify-keep-reporters runs the control run replace-only, runs
// the keep run only after a complete control run, prints a canonical record
// and never reports an incomplete pair as qualified.
func TestQualifyKeepReportersRunOrderAndNotRun(t *testing.T) {
	saved := runE2EReceipt
	t.Cleanup(func() { runE2EReceipt = saved })
	for name, test := range map[string]struct {
		control   jstestprovider.Receipt
		controlEr error
		modes     []bool
		reason    string
	}{
		"playwright-missing": {controlEr: errors.New("runner-unavailable"), modes: []bool{false}, reason: "keep-reporters-control-run-incomplete"},
		"control-partial":    {control: jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, External: &jstestprovider.ExternalLifecycle{}, Cancelled: true}, modes: []bool{false}, reason: "keep-reporters-control-run-incomplete"},
		"reporter-refused":   {control: jstestprovider.Receipt{Profile: jstestprovider.ExternalProfile, External: &jstestprovider.ExternalLifecycle{}}, modes: []bool{false, true}, reason: "keep-reporters-keep-run-incomplete"},
	} {
		t.Run(name, func(t *testing.T) {
			var modes []bool
			runE2EReceipt = func(_ context.Context, cfg jstestprovider.E2EConfig) (jstestprovider.Receipt, error) {
				modes = append(modes, cfg.KeepReporters)
				if cfg.KeepReportersQualification != nil {
					t.Fatal("qualification run carried a record")
				}
				if !cfg.KeepReporters {
					return test.control, test.controlEr
				}
				return jstestprovider.Receipt{}, errors.New("project-reporters-invalid")
			}
			var stdout, stderr bytes.Buffer
			err := qualifyKeepReporters(context.Background(), jstestprovider.E2EConfig{ExternalServer: true, KeepReporters: true}, &stdout, &stderr, "")
			if !errors.Is(err, errNotQualified) || fmt.Sprint(modes) != fmt.Sprint(test.modes) {
				t.Fatalf("err=%v modes=%v", err, modes)
			}
			record, decodeErr := jstestprovider.DecodeKeepReportersQualification(stdout.Bytes())
			if decodeErr != nil || record.Verdict != jstestprovider.KeepReportersNotRun || !strings.Contains(strings.Join(record.Reasons, ","), test.reason) {
				t.Fatalf("record %+v %v\n%s", record, decodeErr, stdout.Bytes())
			}
			if !strings.Contains(stderr.String(), "keep-reporters not-run") {
				t.Fatalf("stderr %q", stderr.String())
			}
		})
	}
}

// PWP-V0-015, PWP-V0-016: the subcommand needs external-server mode and
// selects both reporter modes itself; e2e accepts only a qualified record.
func TestKeepReportersQualificationFlags(t *testing.T) {
	saved := runE2EReceipt
	t.Cleanup(func() { runE2EReceipt = saved })
	runE2EReceipt = func(context.Context, jstestprovider.E2EConfig) (jstestprovider.Receipt, error) {
		t.Fatal("a refused invocation started a run")
		return jstestprovider.Receipt{}, nil
	}
	for _, args := range [][]string{nil, {"--external-server", "--keep-reporters"}, {"--external-server", "--watch", "a.spec.ts", "--foreground", "--experimental", "--trusted-local"}} {
		if err := runQualifyKeepReporters(args); err == nil || !strings.Contains(err.Error(), "external-server only") {
			t.Fatalf("%v: %v", args, err)
		}
	}
	dir := t.TempDir()
	notRun, err := jstestprovider.EncodeKeepReportersQualification(jstestprovider.QualifyKeepReporters(jstestprovider.Receipt{}, errors.New("x"), jstestprovider.Receipt{}, errors.New("x")))
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		body string
		want string
	}{
		"not-qualified": {string(notRun), "keep-reporters-qualification-not-qualified"},
		"invalid":       {"{}\n", "keep-reporters-qualification-invalid"},
	} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(test.body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := runE2E([]string{"--external-server", "--keep-reporters", "--keep-reporters-qualification", path}); err == nil || err.Error() != test.want {
			t.Fatalf("%s: %v", name, err)
		}
		if err := runQualifyKeepReporters([]string{"--external-server", "--keep-reporters-qualification", path}); err == nil || !strings.Contains(err.Error(), "takes no --keep-reporters-qualification") {
			t.Fatalf("%s qualify: %v", name, err)
		}
	}
	if err := runE2E([]string{"--external-server", "--keep-reporters-qualification", filepath.Join(dir, "missing.json")}); err == nil {
		t.Fatal("missing record accepted")
	}
}

// TestParseUnitConfig_RelativeDirResolvedAbsolute confirms the unit
// subcommand's default --dir "." (and any other relative --dir) is resolved
// to an absolute clean path before it reaches procgroup, which rejects a
// relative Spec.Dir.
func TestParseUnitConfig_RelativeDirResolvedAbsolute(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cfg, _, err := parseUnitConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(cfg.Dir) {
		t.Fatalf("want absolute Dir, got %q", cfg.Dir)
	}
	if cfg.Dir != wd {
		t.Fatalf("want Dir=%q (cwd), got %q", wd, cfg.Dir)
	}
}

// LPCV-V0-055: --retain writes the stdout document's exact bytes into
// .corvint/test-evidence of the enclosing worktree, prunes only its own names
// to the newest 32, and refuses a symlinked location while still emitting.
func TestEmitRetainsStdoutBytesPrunesAndRefusesSymlink(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, ".corvint", "test-evidence")
	for _, dir := range []string{filepath.Join(root, ".git"), filepath.Join(root, "web"), evidence} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for index := range 40 {
		name := filepath.Join(evidence, fmt.Sprintf("corvint-js-test-provider-%019d-%016x.json", index+1, index))
		if err := os.WriteFile(name, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(evidence, "operator.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, retain, err := parseUnitConfig([]string{"--dir", filepath.Join(root, "web"), "--retain"})
	if err != nil || !retain {
		t.Fatalf("retain=%v err=%v", retain, err)
	}
	receipt := jstestprovider.Receipt{Kind: "unit", Tests: []jstestprovider.TestOutcome{{Name: "adds", State: jstestprovider.StatePassed}}}
	var stdout, stderr bytes.Buffer
	if err := emit(&stdout, &stderr, receipt, cfg.Dir); err != nil || stderr.Len() != 0 {
		t.Fatalf("err=%v stderr=%s", err, stderr.String())
	}
	var plain bytes.Buffer
	if err := emit(&plain, &stderr, receipt, ""); err != nil || !bytes.Equal(plain.Bytes(), stdout.Bytes()) {
		t.Fatalf("err=%v: without --retain the stdout bytes changed", err)
	}
	retained, _ := filepath.Glob(filepath.Join(evidence, "corvint-js-test-provider-*.json"))
	sort.Strings(retained)
	newest := retained[len(retained)-1]
	data, err := os.ReadFile(newest)
	info, statErr := os.Stat(newest)
	if err != nil || statErr != nil || !bytes.Equal(data, stdout.Bytes()) || info.Mode().Perm() != 0o600 || len(retained) != 32 {
		t.Fatalf("retained=%d mode=%v equal=%v err=%v", len(retained), info, bytes.Equal(data, stdout.Bytes()), err)
	}
	if _, err := os.Stat(filepath.Join(evidence, "operator.json")); err != nil {
		t.Fatalf("pruned a foreign entry: %v", err)
	}

	linked, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(linked, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".corvint"), filepath.Join(linked, ".corvint")); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := emit(&stdout, &stderr, receipt, linked); !errors.Is(err, errRetention) || !bytes.Equal(stdout.Bytes(), plain.Bytes()) || !strings.Contains(stderr.String(), "retention failure") {
		t.Fatalf("err=%v stdout=%d stderr=%s", err, stdout.Len(), stderr.String())
	}
	if after, _ := filepath.Glob(filepath.Join(evidence, "corvint-js-test-provider-*.json")); len(after) != 32 {
		t.Fatalf("a refused retention wrote through the symlink: %d entries", len(after))
	}
}

// LPCVD-V0-005 (V1-1156): a retained basic receipt carries the start identity;
// a non-retained run is not observed, and profiled or already-identified
// receipts are left to their own codec rules.
func TestRetainedBasicReceiptCarriesStartRevision(t *testing.T) {
	saved := observeTestRepositoryAtStart
	t.Cleanup(func() { observeTestRepositoryAtStart = saved })
	start := &jstestprovider.ApplicationRepositoryIdentity{Revision: strings.Repeat("a", 40), DirtyState: "clean"}
	calls := 0
	observeTestRepositoryAtStart = func(context.Context, string) *jstestprovider.ApplicationRepositoryIdentity {
		calls++
		return start
	}
	if got := observeRetainedStart(context.Background(), false, "/x"); got != nil || calls != 0 {
		t.Fatalf("non-retained run observed: %+v calls=%d", got, calls)
	}
	observed := observeRetainedStart(context.Background(), true, "/x")
	if observed != start || calls != 1 {
		t.Fatalf("retained run observed=%+v calls=%d", observed, calls)
	}
	if got := withRetainedRevision(jstestprovider.Receipt{Kind: "unit"}, observed); got.TestRepositoryAtStart != start {
		t.Fatalf("basic receipt=%+v", got.TestRepositoryAtStart)
	}
	if got := withRetainedRevision(jstestprovider.Receipt{Kind: "unit"}, nil); got.TestRepositoryAtStart != nil {
		t.Fatalf("unavailable observation produced %+v", got.TestRepositoryAtStart)
	}
	if got := withRetainedRevision(jstestprovider.Receipt{Kind: "e2e", Profile: jstestprovider.ExternalProfile}, observed); got.TestRepositoryAtStart != nil {
		t.Fatalf("profiled receipt changed: %+v", got.TestRepositoryAtStart)
	}
	own := &jstestprovider.ApplicationRepositoryIdentity{Revision: strings.Repeat("b", 40)}
	if got := withRetainedRevision(jstestprovider.Receipt{Kind: "unit", TestRepositoryAtStart: own}, observed); got.TestRepositoryAtStart != own {
		t.Fatalf("existing identity replaced: %+v", got.TestRepositoryAtStart)
	}
}

// PWP-V0-034: a version refusal writes one typed refusal document to stdout,
// not an empty one, and names the qualified runner versions; other failures
// still write nothing to stdout.
func TestPWPV0034E2EVersionRefusalWritesTypedDocument(t *testing.T) {
	dir := t.TempDir()
	run := func(args []string) (string, error) {
		t.Helper()
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		saved := os.Stdout
		os.Stdout = writer
		runErr := runE2E(args)
		os.Stdout = saved
		writer.Close()
		var out bytes.Buffer
		if _, err = out.ReadFrom(reader); err != nil {
			t.Fatal(err)
		}
		reader.Close()
		return out.String(), runErr
	}
	args := []string{"--dir", dir, "--config", filepath.Join(dir, "playwright.config.cjs"), "--runner-version", "1.62.0", "--external-server", "--app-identity", "fixture", "--server-ready-url", "http://127.0.0.1:1", "--test-file", filepath.Join(dir, "a.spec.cjs")}
	out, err := run(args)
	if err == nil || !strings.HasPrefix(err.Error(), "external-playwright-version-unqualified") {
		t.Fatalf("err %v", err)
	}
	want := `{"schema":"corvint-playwright-e2e-refusal/0","code":"external-playwright-version-unqualified","message":"external-playwright-version-unqualified: qualified runner versions: 1.60.0, 1.63.0, 1.64.0","qualifiedRunnerVersions":["1.60.0","1.63.0","1.64.0"],"runtimeTuple":{"profile":"corvint-playwright-external/0","field":"runner","admitted":["1.60.0","1.63.0","1.64.0"]}}` + "\n"
	if out != want {
		t.Fatalf("stdout\n got %q\nwant %q", out, want)
	}
	saved := runE2EReceipt
	t.Cleanup(func() { runE2EReceipt = saved })
	runE2EReceipt = func(context.Context, jstestprovider.E2EConfig) (jstestprovider.Receipt, error) {
		return jstestprovider.Receipt{}, errors.New("runner-unavailable")
	}
	args[5] = "1.64.0"
	if out, err = run(args); err == nil || out != "" {
		t.Fatalf("non-refusal failure wrote %q (%v)", out, err)
	}
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// it wrote.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = writer
	runErr := fn()
	os.Stdout = saved
	writer.Close()
	var out bytes.Buffer
	if _, err = out.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	reader.Close()
	return out.String(), runErr
}

func TestPWPV0034CLIAdmissionRefusalWritesTypedDocument(t *testing.T) {
	versions := `,"qualifiedRunnerVersions":["1.60.0","1.63.0","1.64.0"]}` + "\n"
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"--external-server", "--interrupt-grace=-1s"}, "interrupt-grace-invalid"},
		{[]string{"--interrupt-grace=1s"}, "runner-control-requires-external-server"},
	} {
		out, err := captureStdout(t, func() error { return runE2E(tc.args) })
		if err == nil || err.Error() != tc.code {
			t.Fatalf("%v: err %v", tc.args, err)
		}
		want := `{"schema":"corvint-playwright-e2e-refusal/0","code":"` + tc.code + `","message":"` + tc.code + `"` + versions
		if out != want {
			t.Fatalf("%v: stdout\n got %q\nwant %q", tc.args, out, want)
		}
	}
	missing := filepath.Join(t.TempDir(), "missing", "runner.log")
	out, err := captureStdout(t, func() error { return runE2E([]string{"--external-server", "--runner-output", missing}) })
	if err == nil || !strings.HasPrefix(err.Error(), "runner-output-unavailable: ") || !strings.HasPrefix(out, `{"schema":"corvint-playwright-e2e-refusal/0","code":"runner-output-unavailable","message":"runner-output-unavailable: `) || !strings.HasSuffix(out, versions) {
		t.Fatalf("runner-output refusal: stdout %q err %v", out, err)
	}
	// qualify-keep-reporters keeps an empty stdout on the same refusal.
	if out, err = captureStdout(t, func() error {
		return runQualifyKeepReporters([]string{"--external-server", "--interrupt-grace=-1s"})
	}); err == nil || err.Error() != "interrupt-grace-invalid" || out != "" {
		t.Fatalf("qualify-keep-reporters wrote %q (%v)", out, err)
	}
}

type refusalSink struct {
	n   int
	err error
}

func (s refusalSink) Write(p []byte) (int, error) {
	if s.n < 0 {
		return len(p), nil
	}
	return s.n, s.err
}

func TestPWPV0034RefusalWriteFailureKeepsDiagnostic(t *testing.T) {
	refusal := &jstestprovider.E2ERefusal{Code: "interrupt-grace-invalid"}
	if err := writeE2ERefusal(refusalSink{n: -1}, refusal); err != refusal {
		t.Fatalf("complete write changed the error: %v", err)
	}
	for _, sink := range []refusalSink{{n: 0, err: errors.New("broken pipe")}, {n: 5}} {
		err := writeE2ERefusal(sink, refusal)
		var typed *jstestprovider.E2ERefusal
		if !errors.As(err, &typed) || typed != refusal || !strings.HasPrefix(err.Error(), "interrupt-grace-invalid; refusal-output-incomplete: ") {
			t.Fatalf("sink %+v: err %v", sink, err)
		}
	}
	plain := errors.New("runner-unavailable")
	if err := writeE2ERefusal(refusalSink{n: 0, err: errors.New("broken pipe")}, plain); err != plain {
		t.Fatalf("non-refusal error changed: %v", err)
	}
}

// PWP-V0-038: an e2e admission or version refusal exits 3 with exactly one
// stderr line naming the refusal code, distinct from an infrastructure failure
// (1) and a usage error (2); the typed document stays on stdout.
func TestPWPV0038E2ERefusalExitStatusAndStderrLine(t *testing.T) {
	dir := t.TempDir()
	code, stdout, stderr := runProviderMain(t, "e2e", "--dir", dir, "--config", filepath.Join(dir, "playwright.config.cjs"), "--runner-version", "1.62.0", "--external-server", "--app-identity", "fixture", "--server-ready-url", "http://127.0.0.1:1", "--test-file", filepath.Join(dir, "a.spec.cjs"))
	if code != 3 {
		t.Fatalf("version refusal exited %d", code)
	}
	if stderr != "refusal: external-playwright-version-unqualified: qualified runner versions: 1.60.0, 1.63.0, 1.64.0\n" {
		t.Fatalf("stderr %q", stderr)
	}
	if !strings.HasPrefix(stdout, `{"schema":"corvint-playwright-e2e-refusal/0","code":"external-playwright-version-unqualified"`) {
		t.Fatalf("stdout %q", stdout)
	}
	if code, _, stderr = runProviderMain(t, "e2e", "--interrupt-grace=1s"); code != 3 || stderr != "refusal: runner-control-requires-external-server\n" {
		t.Fatalf("admission refusal exited %d stderr %q", code, stderr)
	}
	if code, _, _ = runProviderMain(t, "e2e", "--no-such-flag"); code != 2 {
		t.Fatalf("usage error exited %d", code)
	}
	if line, ok := admissionRefusalLine(errors.New("runner-unavailable")); ok || line != "" {
		t.Fatalf("plain error classified as refusal: %q", line)
	}
	if line, _ := admissionRefusalLine(&jstestprovider.E2ERefusal{Code: "report-identity-unknown", Detail: "a\nb\rc"}); line != `refusal: report-identity-unknown: a\nb\rc` {
		t.Fatalf("multi-line detail not escaped: %q", line)
	}
}

// PWP-V0-038: a watch e2e request refused before anything runs reports only
// the refusal; no watch status line precedes it.
func TestPWPV0038WatchRefusalWritesNoStatusLine(t *testing.T) {
	dir := t.TempDir()
	cfg := jstestprovider.E2EConfig{Config: jstestprovider.Config{Dir: dir, ConfigFile: filepath.Join(dir, "playwright.config.cjs"), RunnerVersion: "1.62.0", TestFiles: []string{filepath.Join(dir, "a.spec.cjs")}}, ExternalServer: true, AppIdentity: "fixture", ServerReadyURL: "http://127.0.0.1:1"}
	var stdout, stderr strings.Builder
	err := runE2EWatch(context.Background(), cfg, true, &watchOptions{paths: []string{"a.spec.cjs"}}, &stdout, &stderr)
	var refusal *jstestprovider.E2ERefusal
	if !errors.As(err, &refusal) || refusal.Code != "external-playwright-version-unqualified" {
		t.Fatalf("watch refusal %v", err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("watch wrote before refusing: stdout %q stderr %q", stdout.String(), stderr.String())
	}
}
