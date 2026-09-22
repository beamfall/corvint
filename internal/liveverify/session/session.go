// Package session implements the experimental Go live-test provider's
// foreground session: a bounded-file poll loop that debounces edits,
// computes a current-input identity, and runs the frozen `go test -json`
// invocation (via gorunner.Run, the same contained runner provider.Execute
// uses) once per settled edit. It never replaces provider.Execute's
// authority-gated canonical receipt path and is preview-only, matching
// docs/specs/go-live-test-provider-v0.md's own preview/experimental
// separation: no state this package emits is policy evidence.
package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// State is one point in the session's published state stream.
type State string

const (
	StateIdle           State = "idle"
	StateRunning        State = "running"
	StatePassed         State = "passed"
	StateFailed         State = "failed"
	StateStale          State = "stale"
	StateInfrastructure State = "infrastructure"
	StateCancelled      State = "cancelled"
)

// Event is one state transition. Identity names the current-input identity
// the transition belongs to; a Passed/Failed/Infrastructure event whose
// Identity no longer equals the session's current identity at completion is
// reported as Stale instead, regardless of arrival order.
type Event struct {
	State    State
	Identity string
	Sequence uint64
	Scope    []string
	Detail   string
	// Projection restates this event's own classification in the one shared
	// internal/testvalidity shape the JavaScript provider already emits, so
	// a consumer reads both languages through one representation. It is
	// derived from this event alone and adds no fact Detail does not already
	// carry.
	Projection testvalidity.Projection
	// Tests carries one per-test projection for each test the run's retained
	// `go test -json` stdout reported, failures first then first-seen order,
	// capped at MaxTestProjections (GLTP-V0-051). It is empty for every
	// state whose run did not complete as Passed or Failed.
	Tests []TestProjection
	// TestsOmitted counts reported tests dropped by the MaxTestProjections
	// cap; their per-test state is unknown to a consumer.
	TestsOmitted int
}

// MaxTestProjections bounds Event.Tests so one published event line stays
// well inside a consumer's per-record byte bound.
const MaxTestProjections = 128

// TestProjection is one test's shared testvalidity projection, keyed by the
// `go test -json` Package and Test fields. Action is the last terminal
// action observed for it (pass, fail, skip) or "none" when it never reached
// one; it restates the observation, while Projection is the contract.
type TestProjection struct {
	Package    string                  `json:"package"`
	Name       string                  `json:"name"`
	Action     string                  `json:"action"`
	Projection testvalidity.Projection `json:"projection"`
}

// Config is fully explicit; Run performs no ambient defaulting beyond what is
// documented per field.
type Config struct {
	// Files is the bounded set of absolute, existing regular-file paths this
	// session watches by polling mtime+size, then digesting on a candidate
	// change. It must include go.mod and go.sum when the module has them:
	// the identity is a digest over exactly this set plus ToolchainVersion.
	Files []string
	// TestFiles is the subset of Files (by exact path) that are Go test
	// files. It bounds the affected-scope narrowing in AffectedScope: a
	// change touching only TestFiles can be narrowed to their packages,
	// because editing a test cannot change another package's behavior; any
	// other changed file falls back to Scope.
	TestFiles map[string]bool
	// Scope is the configured full/fallback package pattern list, e.g.
	// []string{"./..."}. It is used for the session's baseline run and for
	// every run whose affected selection cannot be justified.
	Scope []string
	// ModulePath is the module's declared import path (the go.mod `module`
	// line). It is required to express a narrowed affected-package pattern
	// as a plain import path, since gorunner's Packages validator admits
	// dot-relative patterns only in the exact form "./...". An empty
	// ModulePath forces every affected-scope decision to fall back to Scope.
	ModulePath       string
	ToolchainVersion string
	GoExecutable     string
	WorkingDirectory string
	Environment      []gorunner.EnvironmentVariable
	Interval         time.Duration
	Debounce         time.Duration
	Timeout          time.Duration
	OutputLimitBytes int64
	Publish          func(Event)
}

func (cfg Config) validate() error {
	if len(cfg.Files) == 0 || len(cfg.Scope) == 0 || cfg.GoExecutable == "" || cfg.WorkingDirectory == "" {
		return errors.New("session: Config is incomplete")
	}
	if cfg.Interval <= 0 || cfg.Debounce <= 0 || cfg.Timeout <= 0 || cfg.OutputLimitBytes <= 0 {
		return errors.New("session: Config durations and limits must be positive")
	}
	if cfg.Publish == nil {
		return errors.New("session: Config.Publish is required")
	}
	return nil
}

type fileStat struct {
	modTime int64
	size    int64
}

// Session runs the foreground poll/debounce/execute loop described in the
// package doc. It holds no state outside one Run call.
type Session struct {
	cfg Config

	mu              sync.Mutex
	sequence        uint64
	currentIdentity string
	// currentDigests holds the per-file content digests currentIdentity was
	// computed over. It is replaced, never mutated, so a run may keep the
	// map it read (GLTP-V0-052).
	currentDigests  map[string][sha256.Size]byte
	runActive       bool
	activeCancel    context.CancelFunc
	pendingIdentity string
	pendingScope    []string

	runDone chan struct{}
}

// Run executes the session loop until ctx is cancelled (SIGINT/SIGTERM/stdin
// close in the caller) or an unrecoverable Config error is found. On return,
// no run this session started is still executing: shutdown cancels the
// active run's context and waits for gorunner.Run's bounded cleanup so no
// descendant survives the call.
func Run(ctx context.Context, cfg Config) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	s := &Session{cfg: cfg, runDone: make(chan struct{}, 1)}

	// The change baseline is sampled before the identity, so an edit landing
	// between the two is detected instead of absorbed (GLTP-V0-045).
	snapshot, err := s.snapshot()
	if err != nil {
		return err
	}
	identity, digests, err := s.digest(cfg.Files)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.currentIdentity = identity
	s.currentDigests = digests
	s.mu.Unlock()
	s.publish(Event{State: StateIdle, Identity: identity, Sequence: 0})
	s.startRun(ctx, identity, cfg.Scope)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	dirty := map[string]bool{}
	// unrun accumulates every settled change no started run has covered yet,
	// across coalesced settles and settles whose identity failed, so a later
	// narrower batch never drops an earlier fallback (GLTP-V0-047).
	unrun := map[string]bool{}
	var debounceC <-chan time.Time
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			s.cancelAndDrain()
			return ctx.Err()

		case <-ticker.C:
			next, err := s.snapshot()
			if err != nil {
				continue
			}
			changed := diffSnapshot(snapshot, next)
			snapshot = next
			if len(changed) == 0 {
				continue
			}
			for _, path := range changed {
				dirty[path] = true
			}
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(cfg.Debounce)
			debounceC = debounceTimer.C

		case <-debounceC:
			debounceC = nil
			debounceTimer = nil
			for path := range dirty {
				unrun[path] = true
			}
			dirty = map[string]bool{}
			identity, digests, err := s.digest(cfg.Files)
			if err != nil {
				continue
			}
			s.mu.Lock()
			unchanged := identity == s.currentIdentity
			s.mu.Unlock()
			if unchanged {
				continue
			}
			scope := s.affectedScope(unrun)
			s.mu.Lock()
			s.currentIdentity = identity
			s.currentDigests = digests
			if s.runActive {
				s.pendingIdentity = identity
				s.pendingScope = scope
				s.mu.Unlock()
			} else {
				s.mu.Unlock()
				s.startRun(ctx, identity, scope)
				unrun = map[string]bool{}
			}

		case <-s.runDone:
			s.mu.Lock()
			s.runActive = false
			nextIdentity, nextScope := s.pendingIdentity, s.pendingScope
			s.pendingIdentity, s.pendingScope = "", nil
			s.mu.Unlock()
			if nextIdentity != "" {
				s.startRun(ctx, nextIdentity, nextScope)
				unrun = map[string]bool{}
			}
		}
	}
}

// affectedScope narrows to the packages containing changed test files only
// when every changed path is a known test file; any other change (a
// non-test source file, go.mod, go.sum, or an untracked path) cannot be
// proven not to affect other packages, so it falls back to the full
// configured Scope rather than omit coverage silently.
func (s *Session) affectedScope(changed map[string]bool) []string {
	if len(changed) == 0 || s.cfg.ModulePath == "" {
		return s.cfg.Scope
	}
	dirs := map[string]bool{}
	for path := range changed {
		if !s.cfg.TestFiles[path] {
			return s.cfg.Scope
		}
		dirs[filepath.Dir(path)] = true
	}
	patterns := make([]string, 0, len(dirs))
	for dir := range dirs {
		rel, err := filepath.Rel(s.cfg.WorkingDirectory, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			return s.cfg.Scope
		}
		if s.insideNestedModule(dir) {
			return s.cfg.Scope
		}
		if rel == "." {
			patterns = append(patterns, s.cfg.ModulePath)
		} else {
			patterns = append(patterns, s.cfg.ModulePath+"/"+filepath.ToSlash(rel))
		}
	}
	sort.Strings(patterns)
	return patterns
}

// insideNestedModule reports whether dir, or an ancestor strictly below the
// working directory, holds a go.mod: such a package belongs to another module,
// so ModulePath joined with its relative directory names no package.
func (s *Session) insideNestedModule(dir string) bool {
	root := filepath.Clean(s.cfg.WorkingDirectory)
	for current := filepath.Clean(dir); current != root; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return true
		}
		if filepath.Dir(current) == current {
			return false
		}
	}
	return false
}

func (s *Session) startRun(ctx context.Context, identity string, scope []string) {
	s.mu.Lock()
	s.sequence++
	seq := s.sequence
	runCtx, cancel := context.WithCancel(ctx)
	s.activeCancel = cancel
	s.runActive = true
	s.mu.Unlock()

	s.publish(Event{State: StateRunning, Identity: identity, Sequence: seq, Scope: scope})

	go func() {
		defer func() {
			s.mu.Lock()
			s.activeCancel = nil
			s.mu.Unlock()
			select {
			case s.runDone <- struct{}{}:
			default:
			}
		}()

		plan := gorunner.Plan{
			GoExecutable:     s.cfg.GoExecutable,
			WorkingDirectory: s.cfg.WorkingDirectory,
			Environment:      s.cfg.Environment,
			Packages:         scope,
			OutputLimitBytes: s.cfg.OutputLimitBytes,
			RetainOutput:     true,
			Timeout:          s.cfg.Timeout,
		}
		result, runErr := gorunner.Run(runCtx, plan)
		state, detail := classify(result, runErr)
		completed := state == StatePassed || state == StateFailed

		s.mu.Lock()
		current := s.currentIdentity
		digests := s.currentDigests
		s.mu.Unlock()
		if state != StateCancelled && identity != current {
			state = StateStale
		}
		if state != StateCancelled && state != StateStale && !s.filesStillMatch(identity) {
			state = StateStale
		}
		event := Event{State: state, Identity: identity, Sequence: seq, Scope: scope, Detail: detail}
		if completed {
			// A superseded run's content digests are no longer retained, so
			// its per-test entries stay unanchored (GLTP-V0-052).
			var locate func(pkg, name string) string
			if state != StateStale {
				locate = s.sourceLocator(digests)
			}
			event.Tests, event.TestsOmitted = testProjections(result.Stdout.Data, identity, state == StateStale, locate)
		}
		s.publish(event)
	}()
}

// filesStillMatch re-digests the watched files at completion: an edit the
// poll has not seen or the debounce has not settled still makes the result
// stale (GLTP-V0-046). On a mismatch it clears the current identity, so the
// next settle reruns even when it restores the identity this run carried.
func (s *Session) filesStillMatch(identity string) bool {
	onDisk, _, err := s.digest(s.cfg.Files)
	if err == nil && onDisk == identity {
		return true
	}
	s.mu.Lock()
	if s.currentIdentity == identity {
		s.currentIdentity = ""
	}
	s.mu.Unlock()
	return false
}

// classify maps one gorunner.Result to a terminal session state. Cancelled
// takes precedence (an explicit shutdown cancelled the run's context); any
// prelaunch/containment/timeout/output-cap failure, and a go command
// terminated by a signal rather than exiting, is Infrastructure, never
// silently reported as Failed.
func classify(result gorunner.Result, runErr error) (State, string) {
	if result.Cancelled {
		return StateCancelled, ""
	}
	if runErr != nil {
		return StateInfrastructure, runErr.Error()
	}
	if !result.Started {
		return StateInfrastructure, "process did not start"
	}
	if result.TimedOut {
		return StateInfrastructure, "run deadline exceeded"
	}
	if result.OutputLimitExceeded {
		return StateInfrastructure, "output limit exceeded"
	}
	if !result.ProcessCleanupDone {
		return StateInfrastructure, "process cleanup incomplete"
	}
	if !result.Exited {
		return StateInfrastructure, "process terminated by signal"
	}
	if result.ExitCode != 0 {
		return StateFailed, failureDetail(result.Stdout.Data)
	}
	if hasFailEvent(result.Stdout.Data) {
		return StateInfrastructure, "exit status 0 disagrees with a fail event"
	}
	return StatePassed, ""
}

// hasFailEvent reports whether a retained `go test -json` stream holds any
// `fail` action. A TestMain that exits 0 after a failed test makes go test
// exit 0, so a zero exit alone does not state a pass (GLTP-V0-048).
func hasFailEvent(stdout []byte) bool {
	for line := range bytes.Lines(stdout) {
		var event struct{ Action string }
		if json.Unmarshal(line, &event) == nil && event.Action == "fail" {
			return true
		}
	}
	return false
}

// maxFailureDetailTestNames bounds how many distinct failing test names
// failureDetail includes, in first-seen order.
const maxFailureDetailTestNames = 5

// maxFailureDetailBytes bounds the returned detail string; a longer summary
// is truncated with a trailing marker.
const maxFailureDetailBytes = 512

const failureDetailTruncatedMarker = "...[truncated]"

// failureDetail derives a bounded, deterministic failure summary from a
// retained `go test -json` stdout stream: the package FAIL line(s) and up to
// maxFailureDetailTestNames failing test names, in first-seen order. It
// never changes the session-level Event.Detail contract (still a plain
// string) and adds no per-test data structure of its own. When stdout was
// not retained (RetainOutput false, or genuinely empty), it says so instead
// of returning an empty string.
func failureDetail(stdout []byte) string {
	if len(stdout) == 0 {
		return "output-not-retained"
	}
	var parts []string
	seenTest := map[string]bool{}
	testNames := 0
	for line := range bytes.Lines(stdout) {
		var event struct {
			Action string
			Output string
		}
		if err := json.Unmarshal(line, &event); err != nil || event.Action != "output" {
			continue
		}
		line := strings.TrimSpace(event.Output)
		switch {
		case strings.HasPrefix(line, "--- FAIL: "):
			if testNames >= maxFailureDetailTestNames {
				continue
			}
			name := strings.TrimPrefix(line, "--- FAIL: ")
			if idx := strings.IndexByte(name, ' '); idx >= 0 {
				name = name[:idx]
			}
			if name == "" || seenTest[name] {
				continue
			}
			seenTest[name] = true
			testNames++
			parts = append(parts, name)
		case strings.HasPrefix(line, "FAIL\t") || line == "FAIL":
			parts = append(parts, line)
		}
	}
	if len(parts) == 0 {
		return "no-failure-detail-found"
	}
	detail := strings.Join(parts, "; ")
	if len(detail) > maxFailureDetailBytes {
		cut := maxFailureDetailBytes - len(failureDetailTruncatedMarker)
		for cut > 0 && !utf8.RuneStart(detail[cut]) {
			cut--
		}
		detail = detail[:cut] + failureDetailTruncatedMarker
	}
	return detail
}

func (s *Session) cancelAndDrain() {
	s.mu.Lock()
	active, cancel := s.runActive, s.activeCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if active {
		<-s.runDone
	}
}

func (s *Session) publish(event Event) {
	event.Projection = projectionFor(event.State, event.Identity)
	s.cfg.Publish(event)
}

// projectionFor restates one published session state as the shared
// internal/testvalidity.Projection (roadmap IPR-06, LPCV-V0-047), reusing
// that package's types rather than defining a parallel shape. It draws only
// on what this session observed and invokes nothing on provider.Execute's
// authority-gated canonical receipt path, so it does not cross the design
// boundary GLTP-V0-048 draws.
//
// Every axis with no evidence behind it stays explicitly unstated: a session
// run has no TCQ claim, so association and hygiene project UNSUPPORTED, and
// no mutation run is ever attempted here, so the strength axis stays
// NOT_MEASURED no matter how execution reads (LPCV-V0-048). Only a terminal
// run whose identity still matched at completion evidences CURRENT currency;
// Infrastructure and Cancelled produced no test result at all, so their
// freshness stays UNKNOWN rather than claiming currency, and a Stale run
// stays stale. Causes stay inside ExecutionFacts.Cause's frozen vocabulary;
// the free-form failure summary remains on Event.Detail (GLTP-V0-049).
func projectionFor(state State, identity string) testvalidity.Projection {
	return testvalidity.ProjectGoSession(string(state), identity)
}

// testProjections derives one shared testvalidity.Projection per test from a
// completed run's retained `go test -json` stdout (GLTP-V0-051). Only the
// execution and freshness axes carry session evidence: pass and fail project
// PASSED/FAILED, and currency is CURRENT unless the event was superseded,
// when it is STALE. A skip projects SKIPPED (GLTP-V0-052, LPCV-V0-052),
// never PASSED; a test with no terminal action has no outcome, so its
// execution axis abstains as UNSUPPORTED instead of being invented. When
// locate is non-nil and returns a file:line for the test, that anchor leads
// the execution and freshness anchors (GLTP-V0-052).
// Association and hygiene stay UNSUPPORTED (no TCQ claim) and strength stays
// NOT_MEASURED (no mutation run), exactly as the session-level projection.
func testProjections(stdout []byte, identity string, stale bool, locate func(pkg, name string) string) ([]TestProjection, int) {
	actions, order := lastTestActions(stdout)
	failing := make([]TestProjection, 0, len(order))
	others := make([]TestProjection, 0, len(order))
	for _, key := range order {
		test := TestProjection{Package: key[0], Name: key[1], Action: actions[key]}
		anchor := ""
		if locate != nil {
			anchor = locate(test.Package, test.Name)
		}
		test.Projection = testProjectionFor(test.Action, identity, stale, anchor)
		if test.Action == "fail" {
			failing = append(failing, test)
			continue
		}
		others = append(others, test)
	}
	tests := append(failing, others...)
	if len(tests) <= MaxTestProjections {
		return tests, 0
	}
	return tests[:MaxTestProjections], len(tests) - MaxTestProjections
}

var testOutcomes = map[string]string{"pass": "PASSED", "fail": "FAILED", "skip": "SKIPPED"}

func testProjectionFor(action, identity string, stale bool, anchor string) testvalidity.Projection {
	return testvalidity.ProjectGoTest(action, identity, stale, anchor)
}

// lastTestActions returns each (Package, Test) pair's terminal action and the
// pairs in first-seen order. A pair with no observed run action, that never
// reached a terminal after its run, or whose stream reports two different
// terminal actions (a test printing framing lines, or one name in both test
// packages), is "none" (GLTP-V0-051, GLTP-V0-052).
// Package-level events (empty Test) and undecodable lines are ignored.
func lastTestActions(stdout []byte) (map[[2]string]string, [][2]string) {
	actions := map[[2]string]string{}
	ran := map[[2]string]bool{}
	conflicted := map[[2]string]bool{}
	var order [][2]string
	for line := range bytes.Lines(stdout) {
		var event struct {
			Action  string
			Package string
			Test    string
		}
		if err := json.Unmarshal(line, &event); err != nil || event.Test == "" {
			continue
		}
		key := [2]string{event.Package, event.Test}
		if _, seen := actions[key]; !seen {
			actions[key] = "none"
			order = append(order, key)
		}
		if event.Action == "run" {
			ran[key] = true
			continue
		}
		if _, terminal := testOutcomes[event.Action]; !terminal {
			continue
		}
		if !ran[key] {
			continue
		}
		if previous := actions[key]; previous != "none" && previous != event.Action {
			conflicted[key] = true
		}
		actions[key] = event.Action
	}
	for key := range conflicted {
		actions[key] = "none"
	}
	return actions, order
}

// sourceLocator returns the per-event file:line lookup GLTP-V0-052 defines.
// go test -json names a test but never its declaration, and its Output text
// is written by the test itself, so the anchor comes from source instead: the
// package directory's _test.go files, each required to be watched and to
// still hash to the digest the run's identity was computed over, parsed for
// top-level function declarations. Exactly one declaration of the reported
// name yields "<repo-relative path>:<line>"; a subtest, a package outside
// ModulePath, an unwatched or changed or unparseable test file, or zero or
// several declarations yields "", leaving the entry unanchored.
func (s *Session) sourceLocator(digests map[string][sha256.Size]byte) func(pkg, name string) string {
	declarations := map[string]map[string][]string{}
	return func(pkg, name string) string {
		if strings.Contains(name, "/") {
			return ""
		}
		dir := packageDirectory(s.cfg.WorkingDirectory, s.cfg.ModulePath, pkg)
		if dir == "" {
			return ""
		}
		if _, scanned := declarations[dir]; !scanned {
			declarations[dir] = testDeclarations(s.cfg.WorkingDirectory, dir, digests)
		}
		found := declarations[dir][name]
		if len(found) != 1 {
			return ""
		}
		return found[0]
	}
}

// packageDirectory maps an import path inside modulePath to its directory
// under root, or "" when the package is outside the module.
func packageDirectory(root, modulePath, pkg string) string {
	if modulePath == "" {
		return ""
	}
	if pkg == modulePath {
		return root
	}
	rest, inside := strings.CutPrefix(pkg, modulePath+"/")
	if !inside {
		return ""
	}
	return filepath.Join(root, filepath.FromSlash(rest))
}

// testDeclarations indexes every top-level function declared in dir's
// _test.go files by name, as repo-relative "path:line". It returns nil, so
// every lookup abstains, when any such file is unwatched, unreadable,
// changed since digests, or unparseable: a file it could not read might
// declare the same name.
func testDeclarations(root, dir string, digests map[string][sha256.Size]byte) map[string][]string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	declared := map[string][]string{}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		want, watched := digests[path]
		if !watched {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if sha256.Sum256(content) != want {
			return nil
		}
		file, err := parser.ParseFile(fset, path, content, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		for _, decl := range file.Decls {
			function, isFunction := decl.(*ast.FuncDecl)
			if !isFunction || function.Recv != nil {
				continue
			}
			anchor := fmt.Sprintf("%s:%d", filepath.ToSlash(rel), fset.Position(function.Pos()).Line)
			declared[function.Name.Name] = append(declared[function.Name.Name], anchor)
		}
	}
	return declared
}

func (s *Session) snapshot() (map[string]fileStat, error) {
	out := make(map[string]fileStat, len(s.cfg.Files))
	for _, path := range s.cfg.Files {
		info, err := os.Stat(path)
		if err != nil {
			continue // a watched file may be transiently absent mid-edit/save.
		}
		out[path] = fileStat{modTime: info.ModTime().UnixNano(), size: info.Size()}
	}
	return out, nil
}

func diffSnapshot(previous, next map[string]fileStat) []string {
	var changed []string
	for path, stat := range next {
		if old, ok := previous[path]; !ok || old != stat {
			changed = append(changed, path)
		}
	}
	for path := range previous {
		if _, ok := next[path]; !ok {
			changed = append(changed, path)
		}
	}
	return changed
}

// digest computes the current-input identity: a SHA-256 over the sorted,
// path-framed content of every file in files, plus ToolchainVersion. It is
// an internal debounce/staleness identity, not a canonical GLTP wire value.
// It also returns each file's content digest, which binds a source anchor
// to the content the identity names (GLTP-V0-052).
func (s *Session) digest(files []string) (string, map[string][sha256.Size]byte, error) {
	sorted := append([]string(nil), files...)
	sort.Strings(sorted)
	hasher := sha256.New()
	digests := make(map[string][sha256.Size]byte, len(sorted))
	for _, path := range sorted {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		contentDigest := sha256.Sum256(content)
		digests[path] = contentDigest
		io.WriteString(hasher, path)
		hasher.Write([]byte{0})
		hasher.Write(contentDigest[:])
		hasher.Write([]byte{0})
	}
	io.WriteString(hasher, s.cfg.ToolchainVersion)
	return hex.EncodeToString(hasher.Sum(nil)), digests, nil
}
