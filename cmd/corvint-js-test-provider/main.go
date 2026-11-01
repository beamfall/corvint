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
// nonzero after the document is emitted. The experimental (proposed
// PWP-V0-015) `qualify-keep-reporters` runs the
// same external e2e configuration replace-only and then with the project's
// reporters kept, and prints the canonical qualification record
// (PWP-V0-015); it exits zero only when the record is qualified. An e2e
// admission or version refusal (a typed E2ERefusal, decided before anything
// runs) exits with exitAdmissionRefused and writes one stderr line
// "refusal: <code>..." naming the refusal code (PWP-V0-038). `bun` runs the
// bun test profile (LPCV-V0-254); its typed pre-run refusal exits the same way
// with the bun refusal document on stdout (LPCV-V0-258).
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
	"strings"
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

// observeTestRepositoryAtStart names the test repository before a retained
// one-shot run; tests replace it with a fake observer.
var observeTestRepositoryAtStart = jstestprovider.ObserveTestRepositoryAtStart

// observeRetainedStart observes the test repository only when the document is
// retained, so a non-retained stdout document is unchanged (V1-1156).
func observeRetainedStart(ctx context.Context, retain bool, dir string) *jstestprovider.ApplicationRepositoryIdentity {
	if !retain {
		return nil
	}
	return observeTestRepositoryAtStart(ctx, dir)
}

// withRetainedRevision records the start identity in a basic (unprofiled)
// receipt that does not already carry one. Profiled receipts keep their own
// codec rules, and an unavailable observation leaves the revision unknown.
func withRetainedRevision(receipt jstestprovider.Receipt, start *jstestprovider.ApplicationRepositoryIdentity) jstestprovider.Receipt {
	if start != nil && receipt.Profile == "" && receipt.TestRepositoryAtStart == nil {
		receipt.TestRepositoryAtStart = start
	}
	return receipt
}

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

const providerUsage = `Usage: corvint-js-test-provider SUBCOMMAND [flags]

Subcommands:
  unit                    run the bound Vitest adapter and print the receipt document
  bun                     run bun test with the JUnit reporter and print the receipt document;
                          refuses an unqualified bun version or lockfile before running
                          (LPCV-V0-254..262)
  e2e                     run the bound Playwright adapter and print the receipt document;
                          with --external-server the document is the canonical qualified
                          receipt that test-validity --receipt, docs corpus discover-playwright
                          --receipt and a witness-playwright receipt input accept
  qualify-keep-reporters  takes the e2e flags with --external-server (no --keep-reporters,
                          --keep-reporters-qualification or watch); prints the keep-reporters
                          qualification record (PWP-V0-015)
  negate                  generate step-level negative controls for one Playwright test (LPCV-V0-057)
  external-origins        read a project policy and network evidence of 1..16 runs and print the
                          external-origin dependence report; runs no test (PWP-V0-027)
  help, -h, --help        print this usage and exit 0

With e2e --external-server, --test-arg (what Playwright runs) and --test-file (what identity
binds) are independent. When the config uses project dependencies, Playwright also reports the
setup projects' results, so each setup file needs its own --test-file; otherwise the run refuses
report-identity-unknown with a test-file-unbound detail that names each unbound file and its
cause, at most 8 then +N more (PWP-V0-019). Under --sensitive-input-redaction the detail names no
file. No file is bound unless it is passed with --test-file.

--preflight FILE (external mode only) runs a declared corvint-js-preflight/0 document after
readiness and before the runner: http probes GET baseUrl+path until an admitted status (default
2xx; redirects not followed), command probes run a setup or seed argv in --dir and need exit 0,
and storage-state probes check a Playwright storage-state file's required cookies (unexpired for
minValidSeconds) and origins. Every probe has its own timeoutMs inside totalTimeoutMs. The first
failure stops the preflight, never starts Playwright, and writes a receipt whose
infrastructure has phase "preflight", the failing probe and its artefact, and no tests
(PWP-V0-090..093). Example declaration:
  {"schema":"corvint-js-preflight/0","baseUrl":"http://127.0.0.1:3000","totalTimeoutMs":60000,
   "probes":[{"name":"api","kind":"http","path":"/healthz","timeoutMs":10000},
             {"name":"seed","kind":"command","argv":["npm","run","seed"],"timeoutMs":30000},
             {"name":"admin","kind":"storage-state","file":"playwright/.auth/admin.json",
              "requireCookies":["session"],"minValidSeconds":300,"timeoutMs":1000}]}

--server-ready-url readiness differs by mode: without --external-server the owned server is ready
on any status below 500 and redirects are followed; with --external-server only a 2xx answer is
ready and a redirect is not followed (PWP-V0-031).

With --external-server the runner's console is captured to 4 MiB per stream and truncated past
it; the run continues and the report file carries the observation. --runner-output PATH (or -
for this process's stderr) receives every console byte, and the receipt's external.runner
records the exit status, signal and stop signal (PWP-V0-028, PWP-V0-029). On --timeout the
runner first gets SIGINT, then --interrupt-grace (default 10s; 0 sends SIGTERM first) before
SIGTERM and SIGKILL (PWP-V0-030).

--retain also writes the printed document, byte for byte, under .corvint/test-evidence of the
enclosing Git worktree (LPCV-V0-055); a retention that creates .corvint also writes
.corvint/.gitignore (*), otherwise .corvint/test-evidence/.gitignore (*), so Git status stays
clean; it never overwrites an existing non-empty one, repairs an existing empty one and refuses a
non-regular entry (LPCV-V0-084).

Exit status: 0 when the run was observed (a failed test is an observation); 1 when the
infrastructure prevented an observation, retention failed or a keep-reporters record is not
qualified; 2 for a
usage error (and a negate or external-origins refusal); 3 when e2e refuses admission or the
runner version, or bun refuses its version, lockfile or isolation, before running anything, with one stderr line "refusal: <code>..." naming the
refusal code and, without watch, the typed refusal document on stdout (PWP-V0-034, PWP-V0-038).
The flags of each subcommand follow.
`

// usageSink, while printUsage runs, receives each subcommand's flag defaults instead of the
// subcommand parsing its arguments (LPCV-V0-071).
var usageSink io.Writer

// newSubcommandFlags is the subcommand flag set, or a non-exiting one writing to usageSink.
func newSubcommandFlags(name string, handling flag.ErrorHandling) *flag.FlagSet {
	if usageSink == nil {
		return flag.NewFlagSet(name, handling)
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(usageSink)
	return fs
}

// printUsage writes the subcommand summary and the flags of every subcommand that has its own;
// qualify-keep-reporters takes the e2e flags (LPCV-V0-071). It runs no test.
func printUsage(w io.Writer) {
	fmt.Fprint(w, providerUsage)
	usageSink = w
	defer func() { usageSink = nil }()
	for _, show := range []func(){
		func() { _, _, _ = parseUnitConfig([]string{"-h"}) },
		func() { _, _, _ = parseBunConfig([]string{"-h"}) },
		func() { _ = runE2EMode("e2e", []string{"-h"}) },
		func() { _, _ = parseNegate([]string{"-h"}) },
		func() { _, _, _ = parseExternalOrigins([]string{"-h"}) },
	} {
		fmt.Fprintln(w)
		show()
	}
}

// isHelpRequest reports whether a subcommand's flags, before any "--", ask for its usage.
// negate and external-origins otherwise turn flag.ErrHelp into a refusal (V1-1221).
func isHelpRequest(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--":
			return false
		case "-h", "-help", "--help":
			return true
		}
	}
	return false
}

// printSubcommandUsage writes one subcommand's flag usage to w; it runs no test.
func printSubcommandUsage(w io.Writer, show func()) {
	usageSink = w
	defer func() { usageSink = nil }()
	show()
}

func main() {
	if code, handled := companionHelp().Serve(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: corvint-js-test-provider <unit|bun|e2e|negate|qualify-keep-reporters|external-origins> [flags]; corvint-js-test-provider --help for details")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "help", "-h", "-help", "--help":
		printUsage(os.Stdout)
		return
	case "unit":
		err = runUnit(os.Args[2:])
	case "bun":
		err = runBun(os.Args[2:], os.Stdout, os.Stderr)
		if line, refused := bunRefusalLine(err); refused {
			fmt.Fprintln(os.Stderr, line)
			os.Exit(exitAdmissionRefused)
		}
	case "e2e":
		err = runE2E(os.Args[2:])
		if line, refused := admissionRefusalLine(err); refused {
			fmt.Fprintln(os.Stderr, line)
			os.Exit(exitAdmissionRefused)
		}
	case "negate":
		if isHelpRequest(os.Args[2:]) {
			printSubcommandUsage(os.Stdout, func() { _, _ = parseNegate([]string{"-h"}) })
			return
		}
		ctx, cancel := interruptContext()
		err = runNegate(ctx, os.Args[2:], os.Stdout)
		cancel()
		if errors.Is(err, errRefused) {
			os.Exit(2)
		}
	case "qualify-keep-reporters":
		err = runQualifyKeepReporters(os.Args[2:])
	case "external-origins":
		if isHelpRequest(os.Args[2:]) {
			printSubcommandUsage(os.Stdout, func() { _, _, _ = parseExternalOrigins([]string{"-h"}) })
			return
		}
		err = runExternalOrigins(os.Args[2:], os.Stdout, os.Stderr)
		if errors.Is(err, errExternalOriginsRefused) {
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		os.Exit(2)
	}
	if errors.Is(err, errRetention) || errors.Is(err, errNotQualified) {
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "infrastructure failure:", err)
		reportSecretShapedMember(os.Stderr, err)
		os.Exit(1)
	}
}

// exitAdmissionRefused is the e2e exit status for a typed admission or version
// refusal: the run was refused before anything ran, so it is distinct from
// zero (observed, including failed tests), 1 (infrastructure failure) and 2
// (usage error) (PWP-V0-038).
const exitAdmissionRefused = 3

// admissionRefusalLine is the one stderr line for a typed e2e refusal, naming
// its closed code first; line breaks inside a detail are escaped so the line
// stays one line. It reports false for any other error.
func admissionRefusalLine(err error) (string, bool) {
	var refusal *jstestprovider.E2ERefusal
	if !errors.As(err, &refusal) {
		return "", false
	}
	return "refusal: " + strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(err.Error()), true
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
	fs := newSubcommandFlags("unit", flag.ExitOnError)
	configureWatchHelp(fs)
	dir := fs.String("dir", ".", "working directory to run the unit command in (resolved to an absolute path)")
	configFile := fs.String("config", "", "vitest config file path")
	packageJSON := fs.String("package-json", "", "package.json path")
	lockfile := fs.String("lockfile", "", "lockfile path")
	runnerVersion := fs.String("runner-version", "", "pinned vitest version")
	timeout := fs.Duration("timeout", 5*time.Minute, "bound on the unit run")
	retain := fs.Bool("retain", false, "also retain the document under .corvint/test-evidence of the enclosing Git worktree; a basic receipt then records the test repository start revision")
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

	start := observeRetainedStart(ctx, retain, cfg.Dir)
	receipt, err := jstestprovider.RunUnit(ctx, cfg)
	if err != nil {
		return err
	}
	return emit(os.Stdout, os.Stderr, withRetainedRevision(receipt, start), retainFrom(retain, cfg.Dir))
}

// parseBunConfig parses the `bun` subcommand's flags (LPCV-V0-254). Paths are resolved to absolute
// clean paths; --test-file values are both bound and passed to bun test as path selectors.
func parseBunConfig(args []string) (jstestprovider.BunConfig, bool, error) {
	fs := newSubcommandFlags("bun", flag.ExitOnError)
	dir := fs.String("dir", ".", "working directory to run bun test in (resolved to an absolute path)")
	executable := fs.String("bun", "", "bun executable (default: bun on PATH)")
	packageJSON := fs.String("package-json", "", "package.json path")
	lockfile := fs.String("lockfile", "", "text bun.lock path (required; bun.lockb is refused)")
	runnerVersion := fs.String("runner-version", "", "pinned bun version; a different observed version is refused")
	maxConcurrency := fs.Int("max-concurrency", 0, "passed as --max-concurrency when positive; must be 1 with --service")
	namePattern := fs.String("test-name-pattern", "", "passed as --test-name-pattern (example-level selection)")
	timeout := fs.Duration("timeout", 5*time.Minute, "bound on the bun run")
	retain := fs.Bool("retain", false, "also retain the document under .corvint/test-evidence of the enclosing Git worktree")
	var testFiles, envKeys, preload, services stringList
	fs.Var(&testFiles, "test-file", "a test file to bind and run (repeatable, at least one)")
	fs.Var(&envKeys, "env-key", "a declared environment variable name to observe (repeatable)")
	fs.Var(&preload, "preload", "a preload script passed as --preload and bound by digest (repeatable)")
	fs.Var(&services, "service", "a caller-started service the suite depends on, NAME=IDENTITY (repeatable); declares the shared-service isolation profile")
	if err := fs.Parse(args); err != nil {
		return jstestprovider.BunConfig{}, false, err
	}
	resolved := make(map[*string]string)
	for _, p := range []*string{dir, packageJSON, lockfile} {
		r, err := resolvePath(*p)
		if err != nil {
			return jstestprovider.BunConfig{}, false, err
		}
		resolved[p] = r
	}
	resolvedTestFiles := make([]string, len(testFiles))
	for i, f := range testFiles {
		var err error
		if resolvedTestFiles[i], err = resolvePath(f); err != nil {
			return jstestprovider.BunConfig{}, false, err
		}
	}
	return jstestprovider.BunConfig{
		Config: jstestprovider.Config{
			Dir:             resolved[dir],
			TestFiles:       resolvedTestFiles,
			PackageJSON:     resolved[packageJSON],
			Lockfile:        resolved[lockfile],
			RunnerVersion:   *runnerVersion,
			DeclaredEnvKeys: envKeys,
			Timeout:         *timeout,
		},
		Executable:      *executable,
		Preload:         preload,
		MaxConcurrency:  *maxConcurrency,
		TestNamePattern: *namePattern,
		Services:        services,
	}, *retain, nil
}

// runBun runs the bun profile. A typed *BunRefusal writes its document to stdout and is returned
// for main to exit exitAdmissionRefused (LPCV-V0-258).
func runBun(args []string, stdout, stderr io.Writer) error {
	cfg, retain, err := parseBunConfig(args)
	if err != nil {
		return err
	}
	ctx, cancel := interruptContext()
	defer cancel()
	receipt, err := jstestprovider.RunBun(ctx, cfg)
	var refusal *jstestprovider.BunRefusal
	if errors.As(err, &refusal) {
		_, _ = stdout.Write(jstestprovider.EncodeBunRefusal(refusal))
		return err
	}
	if err != nil {
		return err
	}
	return emit(stdout, stderr, receipt, retainFrom(retain, cfg.Dir))
}

// bunRefusalLine is the one stderr line for a typed bun refusal, as admissionRefusalLine.
func bunRefusalLine(err error) (string, bool) {
	var refusal *jstestprovider.BunRefusal
	if !errors.As(err, &refusal) {
		return "", false
	}
	return "refusal: " + strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(err.Error()), true
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
	// A one-shot e2e refusal with a closed code also writes the typed refusal
	// document to stdout (PWP-V0-034); watch and qualify-keep-reporters do not.
	refuse := func(err error) error {
		if mode == "e2e" && watch == nil {
			return writeE2ERefusal(os.Stdout, err)
		}
		return err
	}
	fs := newSubcommandFlags(mode, flag.ExitOnError)
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
	keepReportersQualification := fs.String("keep-reporters-qualification", "", "experimental (proposed PWP-V0-016), with --keep-reporters: a qualified record from qualify-keep-reporters; the receipt carries it and projects only when it matches exactly")
	serverReadyURL := fs.String("server-ready-url", "", "URL polled until ready: owned server, any status < 500 (redirects followed); --external-server, 2xx only (no redirects)")
	preflightPath := fs.String("preflight", "", "external mode: a corvint-js-preflight/0 declaration (base URL, http readiness, setup/seed command and storage-state probes, each with a timeout) run after readiness and before the runner; a failed probe writes a preflight infrastructure receipt with no test verdicts (PWP-V0-090)")
	serverReadyTimeout := fs.Duration("server-ready-timeout", 15*time.Second, "bound on waiting for server readiness")
	timeout := fs.Duration("timeout", 5*time.Minute, "bound on the playwright test command")
	runnerOutputPath := fs.String("runner-output", "", "external mode: write every runner console byte, untruncated, to this file (created or truncated); - writes it to stderr")
	interruptGrace := fs.Duration("interrupt-grace", defaultInterruptGrace, "external mode: on --timeout send SIGINT, then wait this long before SIGTERM and SIGKILL; 0 sends SIGTERM first")
	retain := fs.Bool("retain", false, "also retain the document under .corvint/test-evidence of the enclosing Git worktree; a basic receipt then records the test repository start revision")
	var testFiles stringList
	var envKeys stringList
	var serverArgv stringList
	var testArgv stringList
	var sensitiveActionPatterns stringList
	var sensitiveFields stringList
	fs.Var(&testFiles, "test-file", "a test file to bind identity to (repeatable); with --external-server pass every file Playwright reports a result for, including setup files that project dependencies run, or the receipt refuses test-file-unbound naming up to 8 unbound files (none under --sensitive-input-redaction) (PWP-V0-019)")
	fs.Var(&envKeys, "env-key", "a declared environment variable name to observe (repeatable)")
	fs.Var(&serverArgv, "server-arg", "one token of the app server command, in order (repeatable)")
	fs.Var(&testArgv, "test-arg", "one token of the playwright test command, in order (repeatable); selects what runs and binds no file identity, so with --external-server --test-file is still required for every reported file")
	fs.Var(&sensitiveActionPatterns, "sensitive-action-pattern", "additional case-insensitive input-action title pattern (repeatable; additive to defaults)")
	fs.Var(&sensitiveFields, "sensitive-field", "additional step metadata field to redact (repeatable; additive to defaults)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := admitRunnerControl(fs, *externalServer, *interruptGrace); err != nil {
		return refuse(err)
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

	var preflight *jstestprovider.Preflight
	if *preflightPath != "" {
		if mode != "e2e" {
			return errors.New("qualify-keep-reporters takes no --preflight")
		}
		if preflight, err = jstestprovider.LoadPreflight(*preflightPath); err != nil {
			return refuse(err)
		}
	}

	runnerOutput, closeRunnerOutput, err := openRunnerOutput(*runnerOutputPath, os.Stderr)
	if err != nil {
		return refuse(err)
	}
	defer closeRunnerOutput()

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
		Preflight:                  preflight,
	}
	if *externalServer {
		cfg.InterruptGrace = *interruptGrace
		if runnerOutput != nil {
			cfg.RunnerOutput = runnerOutput
		}
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
	start := observeRetainedStart(ctx, *retain, resolvedDir)
	receipt, err := runE2EReceipt(ctx, cfg)
	if err != nil {
		return refuse(err)
	}
	return emit(os.Stdout, os.Stderr, withRetainedRevision(receipt, start), retainFrom(*retain, resolvedDir))
}

// writeE2ERefusal writes the typed corvint-playwright-e2e-refusal/0 document
// for a pre-run refusal (PWP-V0-034), so stdout is never an empty document. A
// refusal is never retained; e2e then exits exitAdmissionRefused with one
// "refusal: <code>" stderr line (PWP-V0-038).
// Any other failure writes nothing to stdout. It returns err, or, when the
// document write fails or is short, err joined with refusal-output-incomplete
// so stderr keeps the refusal and names the lost document.
func writeE2ERefusal(stdout io.Writer, err error) error {
	var refusal *jstestprovider.E2ERefusal
	if !errors.As(err, &refusal) {
		return err
	}
	data := jstestprovider.EncodeE2ERefusal(refusal)
	n, writeErr := stdout.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		return fmt.Errorf("%w; refusal-output-incomplete: %v", err, writeErr)
	}
	return err
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
			reportSecretShapedMember(stderr, err)
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

// reportSecretShapedMember adds one diagnostic line naming the member a
// qualified-document-secret-shaped refusal matched, as a JSON pointer that never
// carries the matched text (PWP-V3-012). The refusal line itself is unchanged.
func reportSecretShapedMember(stderr io.Writer, err error) {
	var refusal *jstestprovider.QualifiedSecretError
	if !errors.As(err, &refusal) {
		return
	}
	pointer := refusal.Pointer
	if pointer == "" {
		pointer = "(document root)"
	}
	fmt.Fprintf(stderr, "secret-shaped member: %s\n", pointer)
}

// reportProjectLocationUnknown adds one diagnostic line naming, as JSON pointers that never carry
// reported text, the tests a report-identity-unknown refusal rejected for an unknown project,
// browser identity or location (PWP-V3-016). The refusal line itself is unchanged.
func reportProjectLocationUnknown(stderr io.Writer, receipt jstestprovider.Receipt) {
	if receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "report-identity-unknown" {
		return
	}
	if named := jstestprovider.ProjectLocationUnknownTests(receipt); len(named) != 0 {
		fmt.Fprintf(stderr, "project-location-unknown tests: %s\n", strings.Join(named, " "))
	}
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
			reportProjectLocationUnknown(stderr, receipt)
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
