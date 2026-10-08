// Command corvint-js-test-provider is the experimental IPR-08 JS/TS live-test
// provider CLI: `unit` runs the bound Vitest adapter, `e2e` runs the bound
// Playwright adapter (internal/jstestprovider). Each subcommand prints the
// receipt plus its testvalidity projection(s) as JSON on stdout. Exit is
// nonzero only for an infrastructure failure that prevented a real
// observation (a failed test is itself a successful observation and exits
// zero), matching the roadmap acceptance line that a real assertion/browser
// failure must surface, not abort the run. With --retain the same document
// bytes are also retained under .corvint/test-evidence of the enclosing Git
// worktree (LPCV-V0-055); a retention failure is reported on stderr and exits
// nonzero after the document is emitted. `qualify-keep-reporters` runs the
// same external e2e configuration replace-only and then with the project's
// reporters kept, and prints the canonical qualification record
// (PWP-V0-015); it exits zero only when the record is qualified.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const producer = "corvint-js-test-provider"

// errRetention marks a retention failure already reported on stderr.
var errRetention = errors.New("retention failed")

// errNotQualified marks a keep-reporters qualification record that was
// printed but is not qualified; its reasons are already on stderr.
var errNotQualified = errors.New("keep-reporters not qualified")

// runE2EReceipt is the e2e entry point; tests replace it with a fake runner.
var runE2EReceipt = jstestprovider.RunE2E

type stringList []string

func (s *stringList) String() string { return fmt.Sprintf("%v", []string(*s)) }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type output struct {
	Receipt         jstestprovider.Receipt  `json:"receipt"`
	TestProjections []testProjection        `json:"testProjections"`
	RunProjection   testvalidity.Projection `json:"runProjection"`
}

type testProjection struct {
	Name       string                        `json:"name"`
	State      jstestprovider.ExecutionState `json:"state"`
	Projection testvalidity.Projection       `json:"projection"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: corvint-js-test-provider <unit|e2e|qualify-keep-reporters> [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "unit":
		err = runUnit(os.Args[2:])
	case "e2e":
		err = runE2E(os.Args[2:])
	case "qualify-keep-reporters":
		err = runQualifyKeepReporters(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		os.Exit(2)
	}
	if errors.Is(err, errRetention) || errors.Is(err, errNotQualified) {
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "infrastructure failure:", err)
		os.Exit(1)
	}
}

func interruptContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// resolvePath makes a relative --dir/--config/--test-file value absolute and
// clean, matching procgroup's requirement that Spec.Dir be an absolute clean
// path (the "." default otherwise made every run reject at the boundary).
// An empty input is left empty so optional flags stay optional.
func resolvePath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// parseUnitConfig parses the `unit` subcommand's flags into a UnitConfig,
// resolving --dir/--config/--test-file to absolute clean paths so a relative
// "." default (or any relative value) satisfies procgroup's requirement that
// Spec.Dir be absolute. The bool reports --retain.
func parseUnitConfig(args []string) (jstestprovider.UnitConfig, bool, error) {
	fs := flag.NewFlagSet("unit", flag.ExitOnError)
	configureWatchHelp(fs)
	dir := fs.String("dir", ".", "working directory to run the unit command in (resolved to an absolute path)")
	configFile := fs.String("config", "", "vitest config file path")
	packageJSON := fs.String("package-json", "", "package.json path")
	lockfile := fs.String("lockfile", "", "lockfile path")
	runnerVersion := fs.String("runner-version", "", "pinned vitest version")
	timeout := fs.Duration("timeout", 5*time.Minute, "bound on the unit run")
	retain := fs.Bool("retain", false, "also retain the document under .corvint/test-evidence of the enclosing Git worktree")
	var testFiles stringList
	var envKeys stringList
	fs.Var(&testFiles, "test-file", "a test file to bind identity to (repeatable)")
	fs.Var(&envKeys, "env-key", "a declared environment variable name to observe (repeatable)")
	if err := fs.Parse(args); err != nil {
		return jstestprovider.UnitConfig{}, false, err
	}

	resolvedDir, err := resolvePath(*dir)
	if err != nil {
		return jstestprovider.UnitConfig{}, false, err
	}
	resolvedConfig, err := resolvePath(*configFile)
	if err != nil {
		return jstestprovider.UnitConfig{}, false, err
	}
	resolvedTestFiles := make(stringList, len(testFiles))
	for i, f := range testFiles {
		if resolvedTestFiles[i], err = resolvePath(f); err != nil {
			return jstestprovider.UnitConfig{}, false, err
		}
	}

	return jstestprovider.UnitConfig{
		Config: jstestprovider.Config{
			Dir:             resolvedDir,
			ConfigFile:      resolvedConfig,
			TestFiles:       resolvedTestFiles,
			PackageJSON:     *packageJSON,
			Lockfile:        *lockfile,
			RunnerName:      "vitest",
			RunnerVersion:   *runnerVersion,
			DeclaredEnvKeys: envKeys,
			Timeout:         *timeout,
		},
	}, *retain, nil
}

func runUnit(args []string) error {
	filtered, watch, err := splitWatchOptions(args)
	if err != nil {
		return err
	}
	cfg, retain, err := parseUnitConfig(filtered)
	if err != nil {
		return err
	}

	ctx, cancel := interruptContext()
	defer cancel()

	if watch != nil {
		return runUnitWatch(ctx, cfg, retain, watch, os.Stdout, os.Stderr)
	}

	receipt, err := jstestprovider.RunUnit(ctx, cfg)
	if err != nil {
		return err
	}
	return emit(os.Stdout, os.Stderr, receipt, retainFrom(retain, cfg.Dir))
}

func runE2E(args []string) error {
	return runE2EMode("e2e", args)
}

// runQualifyKeepReporters takes the external e2e flags and runs the
// two-run keep-reporters qualification (PWP-V0-015).
func runQualifyKeepReporters(args []string) error {
	return runE2EMode("qualify-keep-reporters", args)
}

func runE2EMode(mode string, args []string) error {
	filtered, watch, err := splitWatchOptions(args)
	if err != nil {
		return err
	}
	args = filtered
	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	configureWatchHelp(fs)
	dir := fs.String("dir", ".", "working directory to run server and test commands in (resolved to an absolute path)")
	configFile := fs.String("config", "", "playwright config file path")
	packageJSON := fs.String("package-json", "", "package.json path")
	lockfile := fs.String("lockfile", "", "lockfile path")
	runnerVersion := fs.String("runner-version", "", "pinned @playwright/test version")
	appBuildDir := fs.String("app-build-dir", "", "served app build directory; omit for an explicit unknown app-build identity")
	externalServer := fs.Bool("external-server", false, "observe an externally managed server; never start or stop it; test-arg supplies only Playwright selectors/options")
	appIdentity := fs.String("app-identity", "", "declared external application identity (not proof of served content)")
	appAttestationCommand := fs.String("app-attestation-command", "", "JSON argv for the typed application-attestation provider; canonical config is supplied on stdin")
	appAttestationConfig := fs.String("app-attestation-config", "", "canonical application-attestation expectation config")
	appAttestationTimeout := fs.Duration("app-attestation-timeout", 5*time.Second, "bound on each application-attestation provider observation")
	sensitiveInputRedaction := fs.Bool("sensitive-input-redaction", false, "select the /2 profile and retain only redacted browser input-action steps")
	retainAttemptDetails := fs.Bool("retain-attempt-details", false, "select the /3 profile and retain each attempt's detail")
	keepReporters := fs.Bool("keep-reporters", false, "external mode: keep the project's reporters after the provider reporter instead of replacing them; the receipt binds the kept entries and records their effects as unknown")
	keepReportersQualification := fs.String("keep-reporters-qualification", "", "with --keep-reporters: a qualified record from qualify-keep-reporters; the receipt carries it and projects only when it matches exactly")
	serverReadyURL := fs.String("server-ready-url", "", "URL polled until it answers with status < 500")
	serverReadyTimeout := fs.Duration("server-ready-timeout", 15*time.Second, "bound on waiting for server readiness")
	timeout := fs.Duration("timeout", 5*time.Minute, "bound on the playwright test command")
	retain := fs.Bool("retain", false, "also retain the document under .corvint/test-evidence of the enclosing Git worktree")
	var testFiles stringList
	var envKeys stringList
	var serverArgv stringList
	var testArgv stringList
	var sensitiveActionPatterns stringList
	var sensitiveFields stringList
	fs.Var(&testFiles, "test-file", "a test file to bind identity to (repeatable)")
	fs.Var(&envKeys, "env-key", "a declared environment variable name to observe (repeatable)")
	fs.Var(&serverArgv, "server-arg", "one token of the app server command, in order (repeatable)")
	fs.Var(&testArgv, "test-arg", "one token of the playwright test command, in order (repeatable)")
	fs.Var(&sensitiveActionPatterns, "sensitive-action-pattern", "additional case-insensitive input-action title pattern (repeatable; additive to defaults)")
	fs.Var(&sensitiveFields, "sensitive-field", "additional step metadata field to redact (repeatable; additive to defaults)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	resolvedDir, err := resolvePath(*dir)
	if err != nil {
		return err
	}
	resolvedConfig, err := resolvePath(*configFile)
	if err != nil {
		return err
	}
	resolvedTestFiles := make(stringList, len(testFiles))
	for i, f := range testFiles {
		if resolvedTestFiles[i], err = resolvePath(f); err != nil {
			return err
		}
	}
	resolvedAttestationConfig, err := resolvePath(*appAttestationConfig)
	if err != nil {
		return err
	}
	var attestationProvider *jstestprovider.ApplicationAttestationProvider
	if *appAttestationCommand != "" || resolvedAttestationConfig != "" {
		var command []string
		if *appAttestationCommand == "" || resolvedAttestationConfig == "" || json.Unmarshal([]byte(*appAttestationCommand), &command) != nil || len(command) == 0 {
			return errors.New("app-attestation-command and app-attestation-config must name a nonempty JSON argv and config")
		}
		attestationProvider = &jstestprovider.ApplicationAttestationProvider{Argv: command, ConfigFile: resolvedAttestationConfig, Timeout: *appAttestationTimeout}
	}
	var sensitivePolicy *jstestprovider.SensitiveInputPolicy
	if *sensitiveInputRedaction || len(sensitiveActionPatterns) != 0 || len(sensitiveFields) != 0 {
		if !*sensitiveInputRedaction {
			return errors.New("sensitive policy additions require --sensitive-input-redaction")
		}
		sensitivePolicy = &jstestprovider.SensitiveInputPolicy{AdditionalActionPatterns: sensitiveActionPatterns, AdditionalSensitiveFields: sensitiveFields}
	}

	var qualification *jstestprovider.KeepReportersQualification
	if *keepReportersQualification != "" {
		if mode != "e2e" {
			return errors.New("qualify-keep-reporters takes no --keep-reporters-qualification")
		}
		record, err := readKeepReportersQualification(*keepReportersQualification)
		if err != nil {
			return err
		}
		qualification = &record
	}

	ctx, cancel := interruptContext()
	defer cancel()

	cfg := jstestprovider.E2EConfig{
		Config: jstestprovider.Config{
			Dir:             resolvedDir,
			TestFiles:       resolvedTestFiles,
			ConfigFile:      resolvedConfig,
			PackageJSON:     *packageJSON,
			Lockfile:        *lockfile,
			RunnerName:      "playwright",
			RunnerVersion:   *runnerVersion,
			DeclaredEnvKeys: envKeys,
			Timeout:         *timeout,
		},
		ServerArgv:                 serverArgv,
		ExternalServer:             *externalServer,
		AppIdentity:                *appIdentity,
		ServerReadyURL:             *serverReadyURL,
		ServerReadyLimit:           *serverReadyTimeout,
		AppBuildDir:                *appBuildDir,
		TestArgv:                   testArgv,
		ApplicationAttestation:     attestationProvider,
		SensitiveInputPolicy:       sensitivePolicy,
		RetainAttemptDetails:       *retainAttemptDetails,
		KeepReporters:              *keepReporters,
		KeepReportersQualification: qualification,
	}
	if mode == "qualify-keep-reporters" {
		if watch != nil || *keepReporters || !*externalServer {
			return errors.New("qualify-keep-reporters is one-shot, external-server only and selects both reporter modes itself")
		}
		return qualifyKeepReporters(ctx, cfg, os.Stdout, os.Stderr, retainFrom(*retain, resolvedDir))
	}
	if watch != nil {
		if attestationProvider != nil {
			return errors.New("application attestation is one-shot only")
		}
		return runE2EWatch(ctx, cfg, *retain, watch, os.Stdout, os.Stderr)
	}
	receipt, err := runE2EReceipt(ctx, cfg)
	if err != nil {
		return err
	}
	return emit(os.Stdout, os.Stderr, receipt, retainFrom(*retain, resolvedDir))
}

// readKeepReportersQualification reads and strictly decodes a qualified
// record (PWP-V0-016).
func readKeepReportersQualification(path string) (jstestprovider.KeepReportersQualification, error) {
	file, err := os.Open(path)
	if err != nil {
		return jstestprovider.KeepReportersQualification{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, jstestprovider.MaxKeepReportersQualificationSize+1))
	if err != nil {
		return jstestprovider.KeepReportersQualification{}, err
	}
	record, err := jstestprovider.DecodeKeepReportersQualification(data)
	if err == nil && record.Verdict != jstestprovider.KeepReportersQualified {
		err = errors.New("keep-reporters-qualification-not-qualified")
	}
	return record, err
}

// qualifyKeepReporters runs the control (replace-only) run and, only when it
// completed, the keep-reporters run of the same configuration, then prints
// the canonical record (PWP-V0-015). An incomplete run is reported, never
// retried or inferred; with --retain each produced receipt is retained so the
// record's digests name retained bytes.
func qualifyKeepReporters(ctx context.Context, cfg jstestprovider.E2EConfig, stdout, stderr io.Writer, retainFrom string) error {
	cfg.KeepReporters, cfg.KeepReportersQualification = false, nil
	control, controlErr := runE2EReceipt(ctx, cfg)
	keep, keepErr := jstestprovider.Receipt{}, errors.New("keep-reporters-keep-run-skipped")
	if controlErr == nil && control.Infrastructure == nil && !control.Cancelled && ctx.Err() == nil {
		cfg.KeepReporters = true
		keep, keepErr = runE2EReceipt(ctx, cfg)
	}
	retained := error(nil)
	for _, run := range []struct {
		name    string
		receipt jstestprovider.Receipt
		err     error
	}{{"control", control, controlErr}, {"keep", keep, keepErr}} {
		if run.err != nil {
			fmt.Fprintf(stderr, "%s run: %v\n", run.name, run.err)
			continue
		}
		if run.receipt.Infrastructure != nil {
			fmt.Fprintf(stderr, "%s run: %s: %s\n", run.name, run.receipt.Infrastructure.Reason, run.receipt.Infrastructure.Detail)
		}
		if retainFrom == "" {
			continue
		}
		data, err := jstestprovider.EncodeQualified(run.receipt)
		if err != nil {
			fmt.Fprintf(stderr, "%s run: %v\n", run.name, err)
			continue
		}
		if err := retainDocument(stderr, retainFrom, data); err != nil {
			retained = err
		}
	}
	record := jstestprovider.QualifyKeepReporters(control, controlErr, keep, keepErr)
	data, err := jstestprovider.EncodeKeepReportersQualification(record)
	if err != nil {
		return err
	}
	if _, err := stdout.Write(data); err != nil {
		return err
	}
	if retained != nil {
		return retained
	}
	if record.Verdict != jstestprovider.KeepReportersQualified {
		fmt.Fprintf(stderr, "keep-reporters %s: %v\n", record.Verdict, record.Reasons)
		return errNotQualified
	}
	return nil
}

// retainFrom names the directory whose enclosing worktree retains the
// document, or "" when --retain was not given.
func retainFrom(retain bool, dir string) string {
	if !retain {
		return ""
	}
	return dir
}

// emit writes the document to stdout and, when retainFrom is not empty,
// retains the same bytes (LPCV-V0-055).
func emit(stdout, stderr io.Writer, receipt jstestprovider.Receipt, retainFrom string) error {
	if receipt.Profile == jstestprovider.ExternalProfile || receipt.Profile == jstestprovider.AttestedExternalProfile || receipt.Profile == jstestprovider.SensitiveExternalProfile || receipt.Profile == jstestprovider.AttemptExternalProfile {
		data, err := jstestprovider.EncodeQualified(receipt)
		if err != nil {
			return err
		}
		if _, err = stdout.Write(data); err != nil {
			return err
		}
		retained := retainDocument(stderr, retainFrom, data)
		if receipt.Infrastructure != nil {
			return fmt.Errorf("%s: %s", receipt.Infrastructure.Reason, receipt.Infrastructure.Detail)
		}
		return retained
	}
	out := output{Receipt: receipt, RunProjection: jstestprovider.ReceiptRunProjection(receipt)}
	for _, test := range receipt.Tests {
		out.TestProjections = append(out.TestProjections, testProjection{
			Name:       test.Name,
			State:      test.State,
			Projection: jstestprovider.ToTestProjection(test),
		})
	}
	var document bytes.Buffer
	enc := json.NewEncoder(&document)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}
	if _, err := stdout.Write(document.Bytes()); err != nil {
		return err
	}
	retained := retainDocument(stderr, retainFrom, document.Bytes())
	// An infrastructure failure is the only case that exits nonzero; the CLI
	// still emits the receipt above so the caller has the observation even
	// when it is incomplete.
	if receipt.Infrastructure != nil {
		return fmt.Errorf("%s: %s", receipt.Infrastructure.Reason, receipt.Infrastructure.Detail)
	}
	return retained
}

func retainDocument(stderr io.Writer, from string, document []byte) error {
	if from == "" {
		return nil
	}
	worktree, err := enclosingWorktree(from)
	if err == nil {
		_, err = testevidence.Retain(worktree, producer, document)
	}
	if err != nil {
		fmt.Fprintln(stderr, "retention failure:", err)
		return errRetention
	}
	return nil
}

// enclosingWorktree is the nearest symlink-resolved ancestor of dir (dir
// included) holding a .git entry, the root corvint test-validity --discover
// is pointed at.
func enclosingWorktree(dir string) (string, error) {
	current, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("no Git worktree encloses --dir")
		}
		current = parent
	}
}
