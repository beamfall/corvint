package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/session"
	"github.com/Beamfall/corvint/internal/testevidence"
)

const (
	defaultSessionInterval = 500 * time.Millisecond
	defaultSessionDebounce = 300 * time.Millisecond
	maxWatchedFiles        = 20_000
)

// retainedStates are the completed event states testvaliditydoc.Discover
// projects; only these are retained (LPCV-V0-055).
var retainedStates = map[session.State]bool{session.StatePassed: true, session.StateFailed: true, session.StateStale: true}

// sessionRetention retains completed event lines under .corvint/test-evidence
// of root. A failure is reported on stderr and makes the session exit 1.
type sessionRetention struct {
	root   string
	stderr io.Writer
	failed atomic.Bool
}

func (retention *sessionRetention) keep(state session.State, line []byte) {
	if retention == nil || !retainedStates[state] {
		return
	}
	if _, err := testevidence.Retain(retention.root, "corvint-go-test-provider", line); err != nil {
		fmt.Fprintln(retention.stderr, "retention failure:", err)
		retention.failed.Store(true)
	}
}

var moduleDirectivePattern = regexp.MustCompile(`(?m)^module\s+(\S+)\s*(?://.*)?$`)

// stringListFlag collects a repeatable -flag value1 -flag value2 ... flag.
type stringListFlag struct{ values []string }

func (s *stringListFlag) String() string { return strings.Join(s.values, ",") }
func (s *stringListFlag) Set(value string) error {
	s.values = append(s.values, value)
	return nil
}

// runSessionCommand implements the `session --foreground` subcommand: an
// explicitly enabled, bounded-file foreground live-test loop built on
// internal/liveverify/session. It prints one JSON line per state transition
// and exits cleanly on SIGINT/SIGTERM/stdin close, leaving no descendant
// process running. It is experimental and carries no qualification
// authority: see docs/specs/go-live-test-provider-v0.md section 14. With
// --retain each completed event line is also retained (LPCV-V0-055).
func runSessionCommand(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("corvint-go-test-provider session", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	experimental := flags.Bool("experimental", false, "enable the experimental provider")
	trusted := flags.Bool("trusted-local", false, "confirm an explicit trusted-local action")
	foreground := flags.Bool("foreground", false, "run the session in the foreground")
	bundlePath := flags.String("authority-bundle", "", "pinned live parent-verifier attachment")
	interval := flags.Duration("interval", defaultSessionInterval, "file-watch poll interval")
	debounce := flags.Duration("debounce", defaultSessionDebounce, "edit debounce window")
	retain := flags.Bool("retain", false, "also retain each completed event under .corvint/test-evidence of the repository root")
	var scopeFlag, watchFlag stringListFlag
	flags.Var(&scopeFlag, "scope", "package pattern to test (repeatable); defaults to the bundle's packages")
	flags.Var(&watchFlag, "watch", "file to watch (repeatable); defaults to every .go file under the repository root")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "ARGUMENTS_REJECTED")
		return 2
	}
	if !*experimental {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "EXPERIMENT_DISABLED")
		return 2
	}
	if !*trusted {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "EXPLICIT_TRUSTED_LOCAL_ACTION_REQUIRED")
		return 2
	}
	if !*foreground {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "FOREGROUND_REQUIRED")
		return 2
	}
	if !supportedProviderHost() {
		writeDiagnostic(stdout, "UNSUPPORTED_PLATFORM", "EXECUTION", "UNSUPPORTED_PLATFORM")
		return 2
	}
	if *bundlePath == "" {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "AUTHORITY_BUNDLE_REQUIRED")
		return 2
	}
	bundle, err := readBundle(*bundlePath)
	if err != nil {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "AUTHORITY_BUNDLE_REJECTED")
		return 2
	}

	var retention *sessionRetention
	if *retain {
		retention = &sessionRetention{root: bundle.RepositoryRoot, stderr: stderr}
	}
	cfg, err := buildSessionConfig(bundle, scopeFlag.values, watchFlag.values, *interval, *debounce, stdout, retention)
	if err != nil {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "SESSION_CONFIG_REJECTED")
		return 2
	}

	ctx, cancel := providerContext()
	defer cancel()
	ctx, cancelStdin := context.WithCancel(ctx)
	defer cancelStdin()
	go func() {
		_, _ = io.Copy(io.Discard, bufio.NewReader(stdin))
		cancelStdin()
	}()

	runErr := session.Run(ctx, cfg)
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		writeDiagnostic(stdout, "EXECUTION_EXIT", "EXECUTION", "SESSION_TERMINATED")
		return 1
	}
	if retention != nil && retention.failed.Load() {
		return 1
	}
	return 0
}

// buildSessionConfig turns one pinned authority bundle plus session-only
// flags into a session.Config. It performs no filesystem write other than a
// non-nil retention's, and adds no ambient default beyond the documented
// interval/debounce constants.
func buildSessionConfig(bundle authorityBundle, scope, watch []string, interval, debounce time.Duration, stdout io.Writer, retention *sessionRetention) (session.Config, error) {
	sortedScope := append([]string(nil), scope...)
	if len(sortedScope) == 0 {
		sortedScope = append([]string(nil), bundle.Packages...)
	}
	sort.Strings(sortedScope)

	files, testFiles, err := resolveWatchedFiles(bundle.RepositoryRoot, watch)
	if err != nil {
		return session.Config{}, err
	}

	modulePath := readModulePath(filepath.Join(bundle.RepositoryRoot, "go.mod"))
	toolchainVersion := readToolchainVersion(bundle.GoExecutable)

	outputLimit := bundle.OutputLimitBytes
	if outputLimit <= 0 || outputLimit > gorunner.MaxOutputBytes {
		outputLimit = gorunner.MaxOutputBytes
	}
	timeout := time.Duration(bundle.TimeoutMilliseconds) * time.Millisecond
	if timeout <= 0 || timeout > gorunner.MaxRunTime {
		timeout = gorunner.MaxRunTime
	}

	home := os.Getenv("HOME")
	if home == "" {
		home = bundle.TemporaryParent
	}
	environment := []gorunner.EnvironmentVariable{
		{Name: "GOCACHE", Value: filepath.Join(bundle.TemporaryParent, "session-gocache")},
		// HOME is inherited, so without GOENV=off a `go env -w` file there
		// (GOFLAGS=-run=..., say) would reach every run (GLTP-V0-048).
		{Name: "GOENV", Value: "off"},
		{Name: "GOPATH", Value: filepath.Join(bundle.TemporaryParent, "session-gopath")},
		{Name: "GOTOOLCHAIN", Value: "local"},
		{Name: "HOME", Value: home},
	}

	publish := func(event session.Event) {
		var line bytes.Buffer
		err := json.NewEncoder(&line).Encode(map[string]any{
			"profile":  "corvint-go-live-session-event/0",
			"state":    string(event.State),
			"identity": event.Identity,
			"sequence": event.Sequence,
			"scope":    event.Scope,
			"detail":   event.Detail,
			// One shared internal/testvalidity.Projection per event, the same
			// shape corvint-js-test-provider emits (GLTP-V0-050).
			"projection": event.Projection,
			// Per-test projections from the run's retained go test -json
			// stream, capped, with the omitted count (GLTP-V0-051).
			"testProjections":        event.Tests,
			"testProjectionsOmitted": event.TestsOmitted,
		})
		if err != nil {
			return
		}
		_, _ = stdout.Write(line.Bytes())
		retention.keep(event.State, line.Bytes())
	}

	return session.Config{
		Files:            files,
		TestFiles:        testFiles,
		Scope:            sortedScope,
		ModulePath:       modulePath,
		ToolchainVersion: toolchainVersion,
		GoExecutable:     bundle.GoExecutable,
		WorkingDirectory: bundle.RepositoryRoot,
		Environment:      environment,
		Interval:         interval,
		Debounce:         debounce,
		Timeout:          timeout,
		OutputLimitBytes: outputLimit,
		Publish:          publish,
	}, nil
}

// resolveWatchedFiles returns the bounded set of absolute file paths to
// watch and the subset that are Go test files. Explicit watch entries are
// resolved relative to repositoryRoot; an empty watch list walks
// repositoryRoot for every .go file (skipping dot directories), always
// including go.mod and go.sum when present, since the session identity is
// a digest over exactly this set.
func resolveWatchedFiles(repositoryRoot string, watch []string) ([]string, map[string]bool, error) {
	var files []string
	if len(watch) > 0 {
		for _, entry := range watch {
			path := entry
			if !filepath.IsAbs(path) {
				path = filepath.Join(repositoryRoot, path)
			}
			path = filepath.Clean(path)
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				return nil, nil, fmt.Errorf("watch entry %q is not a regular file", entry)
			}
			files = append(files, path)
		}
	} else {
		err := filepath.WalkDir(repositoryRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if path != repositoryRoot && strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum" {
				files = append(files, path)
			}
			if len(files) > maxWatchedFiles {
				return fmt.Errorf("watched file set exceeds %d; pass -watch explicitly", maxWatchedFiles)
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	if len(files) == 0 {
		return nil, nil, errors.New("no files to watch")
	}
	sort.Strings(files)
	testFiles := make(map[string]bool)
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			testFiles[path] = true
		}
	}
	return files, testFiles, nil
}

func readModulePath(goModPath string) string {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	match := moduleDirectivePattern.FindSubmatch(content)
	if match == nil {
		return ""
	}
	path := string(match[1])
	if !strings.HasPrefix(path, `"`) {
		return path
	}
	// go.mod admits an interpreted-string module path; an unparseable one
	// yields "" so GLTP-V0-047 falls back to the full scope.
	unquoted, err := strconv.Unquote(path)
	if err != nil {
		return ""
	}
	return unquoted
}

func readToolchainVersion(goExecutable string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, goExecutable, "version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
