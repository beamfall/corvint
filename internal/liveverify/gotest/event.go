// Package gotest observes the bounded JSON event stream emitted by Go 1.27's
// `go test -json` mode without retaining test or build output text.
package gotest

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MaxLineBytes   = 1 << 20
	MaxEventBytes  = 16 << 20
	MaxOutputBytes = 8 << 20
	MaxPackages    = 4_096
	MaxTests       = 100_000
	MaxEvents      = 100_000
)

type Failure string

const (
	FailureRead                Failure = "read"
	FailureMissingLF           Failure = "missing_lf"
	FailureEmptyLine           Failure = "empty_line"
	FailureOversizeLine        Failure = "oversize_line"
	FailureTooManyEventBytes   Failure = "too_many_event_bytes"
	FailureTooManyOutputBytes  Failure = "too_many_output_bytes"
	FailureTooManyEvents       Failure = "too_many_events"
	FailureTooManyPackages     Failure = "too_many_packages"
	FailureTooManyTests        Failure = "too_many_tests"
	FailureSink                Failure = "sink"
	FailureMalformedJSON       Failure = "malformed_json"
	FailureDuplicateKey        Failure = "duplicate_key"
	FailureUnknownField        Failure = "unknown_field"
	FailureMissingField        Failure = "missing_field"
	FailureInvalidField        Failure = "invalid_field"
	FailureUnknownAction       Failure = "unknown_action"
	FailureSequenceConflict    Failure = "sequence_conflict"
	FailureMissingTerminal     Failure = "missing_terminal"
	FailureConflictingTerminal Failure = "conflicting_terminal"
	FailureConfig              Failure = "config"
	FailurePackageDiscovery    Failure = "package_discovery"
	FailureArtifactContainment Failure = "artifact_containment"
	FailureBuildMismatch       Failure = "build_mismatch"
	FailureProcessMismatch     Failure = "process_mismatch"
)

type Error struct {
	Failure Failure
	Line    int
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("go-test-json:%s", e.Failure)
	}
	return fmt.Sprintf("go-test-json:%s:line-%d", e.Failure, e.Line)
}

type EventKind string

const (
	TestEvent  EventKind = "test"
	BuildEvent EventKind = "build"
)

type Event struct {
	Sequence           uint64
	Kind               EventKind
	Action             string
	Package            string
	Test               string
	ImportPath         string
	ElapsedNS          int64
	HasElapsed         bool
	RawTime            string
	TimeUTC            time.Time
	HasTime            bool
	OutputType         string
	OutputDigest       string
	OutputBytes        uint64
	HasOutput          bool
	FailedBuild        string
	Attribute          *AttributeDigest
	ArtifactPathSHA256 string
}

type AttributeDigest struct {
	KeySHA256   string
	ValueSHA256 string
}

type TestState struct {
	Name       string
	Status     string
	ElapsedNS  int64
	HasElapsed bool
}

type PackageState struct {
	Name        string
	Status      string
	ElapsedNS   int64
	HasElapsed  bool
	Tests       []TestState
	FailedBuild string
}

type BuildState struct {
	ImportPath string
	Status     string
}

type Observation struct {
	Events            []Event
	Packages          []PackageState
	Builds            []BuildState
	EventCount        uint64
	EventBytes        uint64
	OutputBytes       uint64
	TestCount         uint64
	IncompleteReasons []string
}

// ProcessOutcome is the terminal result of the exact go test process whose
// stdout is supplied to Observe or Stream.
type ProcessOutcome struct {
	Exited   bool
	ExitCode uint32
}

// Config binds runner events to the preceding discovery and to a run-owned
// artifact directory. All fields are mandatory.
type Config struct {
	DiscoveredPackages []string
	RequestedPackages  []string
	ArtifactRoot       string
	Process            ProcessOutcome
}

type artifactRoot struct {
	lexical string
	handle  *os.Root
}

type reducer struct {
	observation Observation
	packages    map[string]int
	tests       map[string]map[string]int
	builds      map[string]int
	identities  map[string]struct{}
	discovered  map[string]struct{}
	requested   map[string]struct{}
	buildRefs   map[string]uint64
	artifacts   artifactRoot
	process     ProcessOutcome
	inputBytes  uint64
	limits      bounds
	retain      bool
	sink        func(Event) error
}

func Observe(r io.Reader, config Config) (Observation, error) {
	return observe(r, config, true, nil, v0Bounds)
}

// Stream emits bounded provisional facts without retaining the event stream.
// Canonical IDs are assigned by the post-WEI layer, not by this decoder.
func Stream(r io.Reader, config Config, sink func(Event) error) (Observation, error) {
	return observe(r, config, false, sink, v0Bounds)
}

type bounds struct {
	lineBytes, eventBytes, outputBytes, packages, tests, events uint64
}

var v0Bounds = bounds{
	lineBytes: MaxLineBytes, eventBytes: MaxEventBytes, outputBytes: MaxOutputBytes,
	packages: MaxPackages, tests: MaxTests, events: MaxEvents,
}

func observe(r io.Reader, config Config, retain bool, sink func(Event) error, limits bounds) (Observation, error) {
	discovered, requested, artifacts, err := validateConfig(config, limits)
	if err != nil {
		return Observation{}, err
	}
	defer func() { _ = artifacts.handle.Close() }()
	red := reducer{
		packages:   make(map[string]int),
		tests:      make(map[string]map[string]int),
		builds:     make(map[string]int),
		identities: make(map[string]struct{}),
		discovered: discovered,
		requested:  requested,
		buildRefs:  make(map[string]uint64),
		artifacts:  artifacts,
		process:    config.Process,
		limits:     limits,
		retain:     retain,
		sink:       sink,
	}
	reader := bufio.NewReaderSize(r, int(limits.lineBytes)+1)
	for lineNo := 1; ; lineNo++ {
		line, err := reader.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			return red.observation, fail(FailureOversizeLine, lineNo, "record exceeds line bound")
		}
		if len(line) != 0 {
			if uint64(len(line)) > limits.lineBytes {
				return red.observation, fail(FailureOversizeLine, lineNo, "record exceeds line bound")
			}
			if line[len(line)-1] != '\n' {
				return red.observation, fail(FailureMissingLF, lineNo, "record is not LF terminated")
			}
			if len(line) == 1 {
				return red.observation, fail(FailureEmptyLine, lineNo, "empty record")
			}
			if red.observation.EventCount == limits.events {
				return red.observation, fail(FailureTooManyEvents, lineNo, "event bound exceeded")
			}
			if uint64(len(line)) > limits.eventBytes-red.inputBytes {
				return red.observation, fail(FailureTooManyEventBytes, lineNo, "event byte bound exceeded")
			}
			red.inputBytes += uint64(len(line))
			if parseErr := red.consume(line, lineNo); parseErr != nil {
				return red.observation, parseErr
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return red.observation, fail(FailureRead, lineNo, "event stream read failed")
		}
	}
	if err := red.complete(); err != nil {
		return red.observation, err
	}
	return red.observation, nil
}

func (r *reducer) consume(line []byte, lineNo int) error {
	decoded, err := decodeRunnerEvent(line[:len(line)-1], lineNo)
	if err != nil {
		return err
	}
	e := decoded.event
	e.Sequence = r.observation.EventCount
	if e.Kind == TestEvent {
		if _, ok := r.discovered[e.Package]; !ok {
			return fail(FailurePackageDiscovery, lineNo, "package absent from discovery")
		}
	} else if _, ok := r.discovered[e.ImportPath]; !ok {
		return fail(FailurePackageDiscovery, lineNo, "build absent from discovery")
	}
	if decoded.path != "" {
		digest, pathErr := containedArtifactDigest(r.artifacts, decoded.path)
		if pathErr != nil {
			return fail(FailureArtifactContainment, lineNo, "artifact path")
		}
		e.ArtifactPathSHA256 = digest
	}
	factBody, factErr := e.CanonicalFactBody()
	if factErr != nil {
		return fail(FailureInvalidField, lineNo, "normalized fact")
	}
	if uint64(len(factBody)) > r.limits.eventBytes-r.observation.EventBytes {
		return fail(FailureTooManyEventBytes, lineNo, "canonical event byte bound exceeded")
	}
	r.observation.EventBytes += uint64(len(factBody))
	if e.OutputBytes > r.limits.outputBytes-r.observation.OutputBytes {
		return fail(FailureTooManyOutputBytes, lineNo, "output byte bound exceeded")
	}
	r.observation.OutputBytes += e.OutputBytes

	switch e.Kind {
	case BuildEvent:
		if err := r.reduceBuild(e, e.HasOutput, decoded.key, decoded.value, decoded.path, lineNo); err != nil {
			return err
		}
	case TestEvent:
		if err := r.reduceTest(e, e.HasOutput, decoded.key, decoded.value, decoded.path, lineNo); err != nil {
			return err
		}
	}
	if r.sink != nil {
		if err := r.sink(e); err != nil {
			return fail(FailureSink, lineNo, "event sink")
		}
	}
	if r.retain {
		r.observation.Events = append(r.observation.Events, e)
	}
	r.observation.EventCount++
	return nil
}

func (r *reducer) reduceBuild(e Event, hasOutput bool, key, value, path string, line int) error {
	if e.ImportPath == "" {
		return fail(FailureMissingField, line, "ImportPath")
	}
	if e.Package != "" || e.Test != "" || e.HasElapsed || e.HasTime || e.OutputType != "" || e.FailedBuild != "" || key != "" || value != "" || path != "" {
		return fail(FailureInvalidField, line, "test-only field on build event")
	}
	idx, ok := r.builds[e.ImportPath]
	if !ok {
		if !r.addIdentity(e.ImportPath) {
			return fail(FailureTooManyPackages, line, "package bound exceeded")
		}
		idx = len(r.observation.Builds)
		r.builds[e.ImportPath] = idx
		r.observation.Builds = append(r.observation.Builds, BuildState{ImportPath: e.ImportPath})
	}
	b := &r.observation.Builds[idx]
	if b.Status == "fail" {
		if e.Action == "build-fail" {
			return fail(FailureConflictingTerminal, line, e.ImportPath)
		}
		return fail(FailureSequenceConflict, line, "build output after failure")
	}
	if e.Action == "build-output" {
		if !hasOutput {
			return fail(FailureMissingField, line, "Output")
		}
		b.Status = "output"
		return nil
	}
	if hasOutput {
		return fail(FailureInvalidField, line, "Output on build-fail")
	}
	b.Status = "fail"
	return nil
}

func (r *reducer) reduceTest(e Event, hasOutput bool, key, value, path string, line int) error {
	if e.Package == "" {
		return fail(FailureMissingField, line, "Package")
	}
	if e.ImportPath != "" {
		return fail(FailureInvalidField, line, "ImportPath on test event")
	}
	pidx, exists := r.packages[e.Package]
	if e.Action == "start" {
		if e.Test != "" || hasOutput || e.HasElapsed || e.FailedBuild != "" || e.OutputType != "" || key != "" || value != "" || path != "" {
			return fail(FailureInvalidField, line, "invalid start event")
		}
		if exists {
			return fail(FailureSequenceConflict, line, "duplicate package start")
		}
		if !r.addIdentity(e.Package) {
			return fail(FailureTooManyPackages, line, "package bound exceeded")
		}
		r.packages[e.Package] = len(r.observation.Packages)
		r.tests[e.Package] = make(map[string]int)
		r.observation.Packages = append(r.observation.Packages, PackageState{Name: e.Package, Status: "running"})
		return nil
	}
	if !exists {
		return fail(FailureSequenceConflict, line, "event before package start")
	}
	pkg := &r.observation.Packages[pidx]
	if pkg.Status != "running" {
		if isTerminal(e.Action) {
			return fail(FailureConflictingTerminal, line, e.Package)
		}
		return fail(FailureSequenceConflict, line, "event after package terminal")
	}
	if e.Action == "output" {
		if !hasOutput {
			return fail(FailureMissingField, line, "Output")
		}
		switch e.OutputType {
		case "", "frame", "error", "error-continue":
		default:
			return fail(FailureInvalidField, line, "OutputType")
		}
		if e.HasElapsed || e.FailedBuild != "" || key != "" || value != "" || path != "" {
			return fail(FailureInvalidField, line, "invalid output event")
		}
		if e.Test != "" {
			idx, ok := r.tests[e.Package][e.Test]
			if !ok {
				var added bool
				idx, added = r.addTest(pkg, e.Package, e.Test, "benchmarking")
				if !added {
					return fail(FailureTooManyTests, line, "test bound exceeded")
				}
			}
			if isTerminal(pkg.Tests[idx].Status) {
				return fail(FailureSequenceConflict, line, "test output after terminal")
			}
		}
		return nil
	}
	if hasOutput || e.OutputType != "" {
		return fail(FailureInvalidField, line, "Output on non-output action")
	}
	if e.Action == "attr" || e.Action == "artifacts" {
		if e.Test == "" {
			return fail(FailureMissingField, line, "Test")
		}
		if e.HasElapsed || e.FailedBuild != "" {
			return fail(FailureInvalidField, line, "result field on metadata action")
		}
		if e.Action == "attr" && path != "" {
			return fail(FailureInvalidField, line, "attr fields")
		}
		if e.Action == "artifacts" && (path == "" || key != "" || value != "") {
			return fail(FailureInvalidField, line, "artifacts fields")
		}
		idx, ok := r.tests[e.Package][e.Test]
		if !ok {
			return fail(FailureSequenceConflict, line, "metadata before test run")
		}
		if isTerminal(pkg.Tests[idx].Status) {
			return fail(FailureSequenceConflict, line, "metadata after test terminal")
		}
		return nil
	}
	if key != "" || value != "" || path != "" {
		return fail(FailureInvalidField, line, "metadata on non-metadata action")
	}
	if e.Test == "" {
		if !isTerminal(e.Action) || e.Action == "bench" {
			return fail(FailureInvalidField, line, "package action")
		}
		if e.FailedBuild != "" && e.Action != "fail" {
			return fail(FailureInvalidField, line, "FailedBuild on non-fail action")
		}
		for _, test := range pkg.Tests {
			if !isTerminal(test.Status) {
				return fail(FailureMissingTerminal, line, e.Package+"/"+test.Name)
			}
		}
		pkg.Status, pkg.ElapsedNS, pkg.HasElapsed, pkg.FailedBuild = e.Action, e.ElapsedNS, e.HasElapsed, e.FailedBuild
		if e.FailedBuild != "" {
			r.buildRefs[e.FailedBuild]++
		}
		return nil
	}
	return r.reduceNamedTest(pkg, e, line)
}

func (r *reducer) reduceNamedTest(pkg *PackageState, e Event, line int) error {
	if e.FailedBuild != "" {
		return fail(FailureInvalidField, line, "FailedBuild on named test action")
	}
	idx, exists := r.tests[e.Package][e.Test]
	if e.Action == "run" {
		if exists {
			return fail(FailureSequenceConflict, line, "duplicate test run")
		}
		if e.HasElapsed || e.FailedBuild != "" {
			return fail(FailureInvalidField, line, "invalid run event")
		}
		if _, added := r.addTest(pkg, e.Package, e.Test, "running"); !added {
			return fail(FailureTooManyTests, line, "test bound exceeded")
		}
		return nil
	}
	if !exists {
		if e.Action != "bench" && e.Action != "fail" {
			return fail(FailureSequenceConflict, line, "test action before run")
		}
		var added bool
		idx, added = r.addTest(pkg, e.Package, e.Test, "benchmarking")
		if !added {
			return fail(FailureTooManyTests, line, "test bound exceeded")
		}
	}
	test := &pkg.Tests[idx]
	switch e.Action {
	case "pause":
		if test.Status != "running" || e.HasElapsed {
			return fail(FailureSequenceConflict, line, "pause requires running test")
		}
		test.Status = "paused"
	case "cont":
		if test.Status != "paused" || e.HasElapsed {
			return fail(FailureSequenceConflict, line, "cont requires paused test")
		}
		test.Status = "running"
	case "pass", "skip":
		if isTerminal(test.Status) {
			return fail(FailureConflictingTerminal, line, e.Package+"/"+e.Test)
		}
		if test.Status != "running" && test.Status != "paused" {
			return fail(FailureSequenceConflict, line, "terminal requires active test")
		}
		test.Status, test.ElapsedNS, test.HasElapsed = e.Action, e.ElapsedNS, e.HasElapsed
	case "fail", "bench":
		if isTerminal(test.Status) {
			return fail(FailureConflictingTerminal, line, e.Package+"/"+e.Test)
		}
		if test.Status != "running" && test.Status != "paused" && test.Status != "benchmarking" {
			return fail(FailureSequenceConflict, line, "terminal requires active test")
		}
		if test.Status == "benchmarking" || e.Action == "bench" {
			r.markIncomplete("BENCHMARK_ACTION")
		}
		test.Status, test.ElapsedNS, test.HasElapsed = e.Action, e.ElapsedNS, e.HasElapsed
	default:
		return fail(FailureInvalidField, line, "named test action")
	}
	return nil
}

func (r *reducer) complete() error {
	if r.observation.EventCount == 0 {
		return fail(FailureMissingTerminal, 0, "empty event stream")
	}
	hasFailure := false
	for _, pkg := range r.observation.Packages {
		for _, test := range pkg.Tests {
			if !isTerminal(test.Status) {
				return fail(FailureMissingTerminal, 0, pkg.Name+"/"+test.Name)
			}
			if test.Status == "fail" {
				hasFailure = true
				if pkg.Status != "fail" {
					return fail(FailureProcessMismatch, 0, "test/package terminal disagreement")
				}
			}
		}
		if _, required := r.requested[pkg.Name]; required && !isTerminal(pkg.Status) {
			return fail(FailureMissingTerminal, 0, pkg.Name)
		}
		if pkg.Status == "fail" {
			hasFailure = true
		}
		if pkg.FailedBuild != "" {
			idx, ok := r.builds[pkg.FailedBuild]
			if !ok || r.observation.Builds[idx].Status != "fail" {
				return fail(FailureBuildMismatch, 0, "failed build lacks build terminal")
			}
		}
	}
	for name := range r.requested {
		idx, ok := r.packages[name]
		if !ok || !isTerminal(r.observation.Packages[idx].Status) {
			return fail(FailureMissingTerminal, 0, "requested package")
		}
	}
	for _, build := range r.observation.Builds {
		if build.Status == "fail" {
			hasFailure = true
			if r.buildRefs[build.ImportPath] == 0 {
				return fail(FailureBuildMismatch, 0, "unreferenced build failure")
			}
		}
	}
	if !r.process.Exited || (r.process.ExitCode == 0 && hasFailure) || (r.process.ExitCode != 0 && !hasFailure) {
		return fail(FailureProcessMismatch, 0, "runner/process disagreement")
	}
	return nil
}

func validateConfig(config Config, limits bounds) (map[string]struct{}, map[string]struct{}, artifactRoot, error) {
	if len(config.DiscoveredPackages) == 0 || len(config.RequestedPackages) == 0 ||
		uint64(len(config.DiscoveredPackages)) > limits.packages || uint64(len(config.RequestedPackages)) > limits.packages {
		return nil, nil, artifactRoot{}, fail(FailureConfig, 0, "package context")
	}
	discovered := make(map[string]struct{}, len(config.DiscoveredPackages))
	for _, name := range config.DiscoveredPackages {
		if name == "" || len(name) > MaxLineBytes || !allValidUTF8(name) {
			return nil, nil, artifactRoot{}, fail(FailureConfig, 0, "empty discovery package")
		}
		if _, exists := discovered[name]; exists {
			return nil, nil, artifactRoot{}, fail(FailureConfig, 0, "duplicate discovery package")
		}
		discovered[name] = struct{}{}
	}
	requested := make(map[string]struct{}, len(config.RequestedPackages))
	for _, name := range config.RequestedPackages {
		if _, exists := discovered[name]; !exists {
			return nil, nil, artifactRoot{}, fail(FailureConfig, 0, "requested package absent from discovery")
		}
		if _, exists := requested[name]; exists {
			return nil, nil, artifactRoot{}, fail(FailureConfig, 0, "duplicate requested package")
		}
		requested[name] = struct{}{}
	}
	root, err := validateArtifactRoot(config.ArtifactRoot)
	if err != nil {
		return nil, nil, artifactRoot{}, fail(FailureConfig, 0, "artifact root")
	}
	return discovered, requested, root, nil
}

func validateArtifactRoot(path string) (artifactRoot, error) {
	if path == "" || len(path) > MaxLineBytes || !allValidUTF8(path) || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return artifactRoot{}, errors.New("invalid artifact root")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return artifactRoot{}, errors.New("invalid artifact root")
	}
	handle, err := os.OpenRoot(path)
	if err != nil {
		return artifactRoot{}, err
	}
	openedInfo, err := handle.Stat(".")
	if err != nil || !os.SameFile(info, openedInfo) {
		_ = handle.Close()
		return artifactRoot{}, errors.New("artifact root identity changed")
	}
	return artifactRoot{lexical: path, handle: handle}, nil
}

func containedArtifactDigest(root artifactRoot, path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", errors.New("invalid artifact path")
	}
	rel, err := filepath.Rel(root.lexical, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("artifact escape")
	}
	logical := filepath.ToSlash(rel)
	if !validLogicalPath(logical) {
		return "", errors.New("invalid logical artifact path")
	}
	current := ""
	if rel != "." {
		for _, component := range strings.Split(rel, string(filepath.Separator)) {
			current = filepath.Join(current, component)
			info, statErr := root.handle.Lstat(current)
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
				return "", errors.New("artifact component")
			}
		}
	}
	info, err := root.handle.Stat(rel)
	if err != nil || !info.IsDir() {
		return "", errors.New("artifact is not a directory")
	}
	finalInfo, err := root.handle.Lstat(rel)
	if err != nil || finalInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, finalInfo) {
		return "", errors.New("artifact path identity changed")
	}
	body := make([]byte, 0, len(logical)+31)
	body = append(body, `{"namespace":"RUN","path":`...)
	body = appendString(body, logical)
	body = append(body, '}')
	return domainDigest("go-logical-path", "go-logical-path/0", body), nil
}

func validLogicalPath(path string) bool {
	if path == "." {
		return true
	}
	if path == "" || path[0] == '/' || strings.Contains(path, `\`) {
		return false
	}
	for _, component := range strings.Split(path, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
		for _, r := range component {
			if r < 0x20 || r == 0x7f {
				return false
			}
		}
	}
	return true
}

func domainDigest(kind, profile string, body []byte) string {
	h := sha256.New()
	var framed [8]byte
	binary.BigEndian.PutUint32(framed[:4], uint32(len(kind)))
	_, _ = h.Write(framed[:4])
	_, _ = h.Write([]byte(kind))
	binary.BigEndian.PutUint32(framed[:4], uint32(len(profile)))
	_, _ = h.Write(framed[:4])
	_, _ = h.Write([]byte(profile))
	binary.BigEndian.PutUint64(framed[:], uint64(len(body)))
	_, _ = h.Write(framed[:])
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func (r *reducer) addIdentity(name string) bool {
	if _, exists := r.identities[name]; exists {
		return true
	}
	if uint64(len(r.identities)) == r.limits.packages {
		return false
	}
	r.identities[name] = struct{}{}
	return true
}

func (r *reducer) addTest(pkg *PackageState, packageName, testName, status string) (int, bool) {
	if r.observation.TestCount == r.limits.tests {
		return 0, false
	}
	idx := len(pkg.Tests)
	r.tests[packageName][testName] = idx
	pkg.Tests = append(pkg.Tests, TestState{Name: testName, Status: status})
	r.observation.TestCount++
	return idx, true
}

func (r *reducer) markIncomplete(reason string) {
	for _, existing := range r.observation.IncompleteReasons {
		if existing == reason {
			return
		}
	}
	r.observation.IncompleteReasons = append(r.observation.IncompleteReasons, reason)
}

func isTerminal(status string) bool {
	switch status {
	case "pass", "fail", "skip", "bench":
		return true
	default:
		return false
	}
}

func fail(class Failure, line int, detail string) *Error {
	return &Error{Failure: class, Line: line}
}

func digestString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
