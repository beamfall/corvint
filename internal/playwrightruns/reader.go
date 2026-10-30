// Package playwrightruns is the shared, read-only Playwright run-evidence reader behind
// `corvint runs stability` and `corvint runs compare` (docs/specs/playwright-run-evidence-v0.md,
// PWR-V0). It reads Playwright json-reporter documents, keys every test by its stable identity
// (project, file, title path) and keeps only what the two verdicts need. Failure text passes the
// same run-evidence hygiene and secret screen as the provider receipt (internal/runhygiene,
// internal/secretscreen). Nothing here writes a file, runs a process or opens a network connection.
package playwrightruns

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/runhygiene"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Bounds (PWR-V0-001). A report beyond any bound is refused whole, never truncated.
const (
	MaxReportBytes     = 64 << 20
	MaxTests           = 50000
	MaxResultsPerTest  = 100
	MaxErrorsPerResult = 16
	maxLineBytes       = 240
	// MaxShardTotal and MaxRepeatEach bound the declared config.shard.total and
	// config.projects[].repeatEach (PWR-V0-016, PWR-V0-011).
	MaxShardTotal = 1024
	MaxRepeatEach = 1024
)

// Assertion origins (PWR-V0-007).
const (
	OriginTestFile = "TEST_FILE"
	OriginSupport  = "SUPPORT"
	OriginUnknown  = "UNKNOWN"
)

// Report refusal reasons (PWR-V0-001).
var (
	ErrUnreadable = errors.New("REPORT_UNREADABLE")
	ErrTooLarge   = errors.New("REPORT_BOUND_EXCEEDED")
	ErrShape      = errors.New("REPORT_NOT_PLAYWRIGHT_JSON")
)

// Identity is a test's stable identity: project, file relative to config.rootDir, and the
// describe titles followed by the test title (PWR-V0-001).
type Identity struct {
	Project   string   `json:"project"`
	File      string   `json:"file"`
	TitlePath []string `json:"titlePath"`
}

// Key is an unambiguous map key for the identity.
func (i Identity) Key() string {
	raw, _ := json.Marshal([]any{i.Project, i.File, i.TitlePath})
	return string(raw)
}

// Inputs are the identity inputs a run declares under config.metadata.corvintRunIdentity
// (PWR-V0-002). An absent value is empty; it is never inferred.
type Inputs struct {
	ApplicationRevision string            `json:"applicationRevision"`
	ConfigDigest        string            `json:"configDigest"`
	TestFileBlobs       map[string]string `json:"testFileBlobs"`
	// ServedArtifactDigest names the build the run served; "" is unknown (PWR-V0-055).
	ServedArtifactDigest string `json:"servedArtifactDigest,omitempty"`
	// TestClosureDigests and TestClosureRule are the caller's per-test closure claim (PWR-V0-068);
	// a dropped value is absent.
	TestClosureDigests map[string]string `json:"testClosureDigests,omitempty"`
	TestClosureRule    string            `json:"testClosureRule,omitempty"`
	// conflicts keeps, per normalized file, the sorted blobs that conflicting spellings declared,
	// so two runs with different conflicting declarations still compare as different inputs.
	conflicts map[string][]string
}

// Assertion is one failed assertion or error of one result (PWR-V0-007). Matcher is empty when
// the error names no expect matcher.
type Assertion struct {
	Origin  string `json:"origin"`
	Matcher string `json:"matcher"`
	File    string `json:"file"`
	Line    int    `json:"line"`
}

// Result is one attempt of one test.
type Result struct {
	Retry       int
	Status      string // passed | failed | timedOut | skipped | interrupted
	WorkerIndex int    // -1 when the report omits it
	DurationMS  int64
	FirstError  string // screened first line of the first error, "" when none
	Failures    []Assertion
	// StartTime is the attempt's startTime; HasStartTime is false when it is absent or not
	// RFC 3339 (OUE-V0-050).
	StartTime    time.Time
	HasStartTime bool
	// HTTPFailures are the HTTP 5xx responses this attempt's errors name, in report order
	// (OUE-V0-050).
	HTTPFailures []HTTPFailure
}

// Test is one test of one project in one report.
type Test struct {
	Identity
	Status       string // Playwright aggregate: expected | unexpected | flaky | skipped
	Executed     bool   // false when no result ran: none recorded, or a skip the test did not expect
	ExpectedFail bool   // test.fail(): a "fail" annotation or expectedStatus "failed"
	Results      []Result
	// Repeat is this occurrence's 0-based position among the occurrences of its identity in the
	// report (or the shard set it was merged into), in report order (PWR-V0-011).
	Repeat int
	// line and column are the spec's source location; repeats of one test share it.
	line, column int
}

// Shard is a report's config.shard: the 1-based shard index and the shard count (PWR-V0-016).
type Shard struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

// Run is one admitted report.
type Run struct {
	Sha256             string
	Version            string
	Workers            int
	Projects           []string // sorted distinct project names seen on tests
	ConfiguredProjects []string // sorted distinct config.projects names
	Inputs             Inputs
	InputsDeclared     bool
	ReportErrors       int // top-level report errors, such as a spec file that failed to load
	Tests              []Test
	Ambiguous          map[string]bool // identity keys whose occurrences repeatEach does not explain
	// Shard is config.shard; nil when the report is not one shard of a sharded run.
	Shard *Shard
	// RepeatEach is config.projects[].repeatEach by project name, kept only when it is above 1 and
	// every configured entry of that name agrees (PWR-V0-011).
	RepeatEach map[string]int
	// TestRevision and TestRootPath are the test-suite revision and the repository-relative path of
	// config.rootDir declared under corvintRunIdentity; Category is config.metadata.corvintRunCategory
	// (PWR-V0-033, PWR-V0-037). Each is "" when undeclared and is never inferred.
	TestRevision string
	TestRootPath string
	Category     string
	RootDir      string // config.rootDir as a cleaned slash path, "" when absent
	// StartTime is stats.startTime; HasStartTime is false when it is absent or not RFC 3339.
	StartTime    time.Time
	HasStartTime bool
	// IdentitySource is EMBEDDED, SIDECAR or NOT_DECLARED once the caller asked for it
	// (MarkIdentitySource, ApplySidecar), else "" and never emitted (PWR-V0-067).
	IdentitySource string
	// identityEmbedded is true when config.metadata carries a corvintRunIdentity member, even a
	// malformed one: a sidecar then may only repeat it (PWR-V0-066).
	identityEmbedded bool
}

// repeats is how many repeat observations of one test of project the run declares; 1 unless
// config.projects declares an agreed repeatEach above 1.
func (run *Run) repeats(project string) int {
	if n := run.RepeatEach[project]; n > 1 {
		return n
	}
	return 1
}

// resolveRepeats numbers the occurrences of each identity in report order and records as ambiguous
// every identity whose several occurrences are not exactly the declared repeatEach occurrences of
// one reported spec location (PWR-V0-011, PWR-V0-012). A spec without a line (Playwright lines
// start at 1) has no location to share.
func (run *Run) resolveRepeats() {
	type tally struct {
		count, line, column int
		project             string
		moved               bool
	}
	run.Ambiguous = map[string]bool{}
	seen := map[string]*tally{}
	for i := range run.Tests {
		t := &run.Tests[i]
		key := t.Key()
		s := seen[key]
		if s == nil {
			s = &tally{line: t.line, column: t.column, project: t.Project}
			seen[key] = s
		} else if s.line != t.line || s.column != t.column {
			s.moved = true
		}
		t.Repeat = s.count
		s.count++
	}
	for key, s := range seen {
		if s.count > 1 && (s.moved || s.line < 1 || s.count != run.repeats(s.project)) {
			run.Ambiguous[key] = true
		}
	}
}

type jsonReport struct {
	Config *struct {
		Version  string          `json:"version"`
		RootDir  string          `json:"rootDir"`
		Workers  int             `json:"workers"`
		Metadata json.RawMessage `json:"metadata"`
		Shard    *Shard          `json:"shard"`
		Projects []struct {
			Name       string `json:"name"`
			RepeatEach *int   `json:"repeatEach"`
		} `json:"projects"`
	} `json:"config"`
	Errors []json.RawMessage `json:"errors"`
	Suites *[]jsonSuite      `json:"suites"`
	// Stats is decoded leniently by statsStartTime: an ill-typed member is absent (PWR-V0-033).
	Stats json.RawMessage `json:"stats"`
}

// statsStartTime is stats.startTime when stats is an object whose startTime is an RFC 3339 string.
func statsStartTime(raw json.RawMessage) (time.Time, bool) {
	var stats struct {
		StartTime json.RawMessage `json:"startTime"`
	}
	var text string
	if json.Unmarshal(raw, &stats) != nil || json.Unmarshal(stats.StartTime, &text) != nil {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return time.Time{}, false
	}
	return at.UTC(), true
}

type jsonSuite struct {
	Title  string      `json:"title"`
	File   string      `json:"file"`
	Specs  []jsonSpec  `json:"specs"`
	Suites []jsonSuite `json:"suites"`
}

type jsonSpec struct {
	Title  string     `json:"title"`
	File   string     `json:"file"`
	Line   int        `json:"line"`
	Column int        `json:"column"`
	Tests  []jsonTest `json:"tests"`
}

type jsonTest struct {
	ProjectName    *string `json:"projectName"`
	ExpectedStatus string  `json:"expectedStatus"`
	Status         string  `json:"status"`
	Annotations    []struct {
		Type string `json:"type"`
	} `json:"annotations"`
	Results []jsonResult `json:"results"`
}

type jsonResult struct {
	Retry       int             `json:"retry"`
	Status      string          `json:"status"`
	WorkerIndex *int            `json:"workerIndex"`
	Duration    float64         `json:"duration"`
	StartTime   json.RawMessage `json:"startTime"`
	Error       *jsonError      `json:"error"`
	Errors      []jsonError     `json:"errors"`
}

type jsonError struct {
	Message  string `json:"message"`
	Location *struct {
		File string `json:"file"`
		Line int    `json:"line"`
	} `json:"location"`
}

var (
	resultStatuses = map[string]bool{"passed": true, "failed": true, "timedOut": true, "skipped": true, "interrupted": true}
	testStatuses   = map[string]bool{"expected": true, "unexpected": true, "flaky": true, "skipped": true}
	ansi           = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
	// matcherCall finds the expect matcher a Playwright assertion message names, such as
	// `expect(locator).toHaveText(expected)`, `expect(received).not.toBe(expected)` or
	// `expect(received).rejects.toThrow()`; promise and negation modifiers stay part of the name, and
	// the name runs to the next space, dot or parenthesis, so any JavaScript identifier stays whole.
	matcherCall = regexp.MustCompile(`expect(?:\.soft|\.poll)?\((?:[^()]|\([^()]*\))*\)\.((?:(?:not|resolves|rejects)\.)*[^\s().]+)`)
)

// ReadFile reads and parses one report. Only a regular file is read, without blocking on a FIFO,
// and at most MaxReportBytes+1 bytes are taken from it.
func ReadFile(name string) (*Run, error) {
	f, err := openNonblocking(name)
	if err != nil {
		return nil, ErrUnreadable
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrUnreadable
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxReportBytes+1))
	if err != nil {
		return nil, ErrUnreadable
	}
	return Parse(raw)
}

// Parse admits one json-reporter document (PWR-V0-001): one JSON object with config.version and
// a suites array, known statuses, and every test carrying a project and file.
func Parse(raw []byte) (*Run, error) {
	if len(raw) > MaxReportBytes {
		return nil, ErrTooLarge
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var doc jsonReport
	if err := dec.Decode(&doc); err != nil {
		return nil, ErrShape
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, ErrShape
	}
	if doc.Config == nil || doc.Config.Version == "" || doc.Suites == nil {
		return nil, ErrShape
	}
	if sh := doc.Config.Shard; sh != nil && (sh.Total < 1 || sh.Current < 1 || sh.Current > sh.Total) {
		return nil, ErrShape
	}
	if sh := doc.Config.Shard; sh != nil && sh.Total > MaxShardTotal {
		return nil, ErrTooLarge
	}
	sum := sha256.Sum256(raw)
	run := &Run{Sha256: hex.EncodeToString(sum[:]), Version: doc.Config.Version, Workers: doc.Config.Workers,
		ReportErrors: len(doc.Errors), Ambiguous: map[string]bool{}, Tests: []Test{}}
	run.Inputs, run.InputsDeclared = declaredInputs(doc.Config.Metadata)
	run.TestRevision, run.TestRootPath, run.Category = declaredRunFacts(doc.Config.Metadata)
	run.identityEmbedded = hasIdentityMember(doc.Config.Metadata)
	if doc.Config.RootDir != "" {
		run.RootDir = slashPath(doc.Config.RootDir)
	}
	run.StartTime, run.HasStartTime = statsStartTime(doc.Stats)
	run.Shard = doc.Config.Shard
	run.ConfiguredProjects = []string{}
	configured := map[string]bool{}
	repeatEach := map[string]map[int]bool{}
	for _, p := range doc.Config.Projects {
		if !configured[p.Name] {
			configured[p.Name] = true
			run.ConfiguredProjects = append(run.ConfiguredProjects, p.Name)
			repeatEach[p.Name] = map[int]bool{}
		}
		n := 1
		if p.RepeatEach != nil && *p.RepeatEach > MaxRepeatEach {
			return nil, ErrTooLarge
		}
		if p.RepeatEach != nil && *p.RepeatEach > 1 {
			n = *p.RepeatEach
		}
		repeatEach[p.Name][n] = true
	}
	sort.Strings(run.ConfiguredProjects)
	// A merge-reports output lists a project once per merged shard; entries that disagree leave
	// the project at one repeat, so its duplicates stay ambiguous.
	for name, values := range repeatEach {
		if len(values) != 1 {
			continue
		}
		for n := range values {
			if n > 1 {
				if run.RepeatEach == nil {
					run.RepeatEach = map[string]int{}
				}
				run.RepeatEach[name] = n
			}
		}
	}
	r := reader{run: run, root: doc.Config.RootDir, projects: map[string]bool{}}
	for _, s := range *doc.Suites {
		if err := r.suite(s, nil, true); err != nil {
			return nil, err
		}
	}
	run.resolveRepeats()
	for p := range r.projects {
		run.Projects = append(run.Projects, p)
	}
	sort.Strings(run.Projects)
	return run, nil
}

type reader struct {
	run      *Run
	root     string
	projects map[string]bool
}

// suite walks one suite. A top-level suite is the file suite, whose title is the file path and is
// not part of the title path; nested suites are describe blocks.
func (r *reader) suite(s jsonSuite, titles []string, top bool) error {
	if !top {
		titles = append(append([]string{}, titles...), s.Title)
	}
	for _, spec := range s.Specs {
		file := spec.File
		if file == "" {
			file = s.File
		}
		for _, t := range spec.Tests {
			if err := r.test(file, append(append([]string{}, titles...), spec.Title), spec, t); err != nil {
				return err
			}
		}
	}
	for _, sub := range s.Suites {
		if err := r.suite(sub, titles, false); err != nil {
			return err
		}
	}
	return nil
}

func (r *reader) test(file string, titles []string, spec jsonSpec, t jsonTest) error {
	if len(r.run.Tests) >= MaxTests || len(t.Results) > MaxResultsPerTest {
		return ErrTooLarge
	}
	if file == "" || t.ProjectName == nil || !testStatuses[t.Status] {
		return ErrShape
	}
	id := Identity{Project: *t.ProjectName, File: slashPath(file), TitlePath: titles}
	r.projects[id.Project] = true
	test := Test{Identity: id, Status: t.Status, ExpectedFail: t.ExpectedStatus == "failed", Results: []Result{}, line: spec.Line, column: spec.Column}
	for _, a := range t.Annotations {
		test.ExpectedFail = test.ExpectedFail || a.Type == "fail"
	}
	for _, res := range t.Results {
		if !resultStatuses[res.Status] || res.Retry < 0 {
			return ErrShape
		}
		out := Result{Retry: res.Retry, Status: res.Status, WorkerIndex: -1, DurationMS: durationMS(res.Duration), Failures: []Assertion{}}
		if res.WorkerIndex != nil {
			out.WorkerIndex = *res.WorkerIndex
		}
		var started string
		if json.Unmarshal(res.StartTime, &started) == nil {
			if at, err := time.Parse(time.RFC3339Nano, started); err == nil {
				out.StartTime, out.HasStartTime = at, true
			}
		}
		errs := res.Errors
		if len(errs) == 0 && res.Error != nil {
			errs = []jsonError{*res.Error}
		}
		if len(errs) > MaxErrorsPerResult {
			return ErrTooLarge
		}
		for i, e := range errs {
			if i == 0 {
				out.FirstError = FirstLine(e.Message)
			}
			if f, ok := httpFailure(e.Message); ok {
				out.HTTPFailures = append(out.HTTPFailures, f)
			}
			out.Failures = append(out.Failures, r.assertion(id.File, e))
		}
		test.Results = append(test.Results, out)
	}
	// Playwright records a test it never started (after a setup or serial-suite failure) as a
	// skipped result while expectedStatus stays "passed"; only an expected skip is an outcome.
	test.Executed = len(test.Results) > 0 && (test.Results[len(test.Results)-1].Status != "skipped" || t.ExpectedStatus == "skipped")
	r.run.Tests = append(r.run.Tests, test)
	return nil
}

// assertion classifies one error by where it was raised (PWR-V0-007): the test's own file, another
// source file (a page object, helper or fixture), or unknown when the error has no location.
func (r *reader) assertion(testFile string, e jsonError) Assertion {
	a := Assertion{Origin: OriginUnknown}
	if m := matcherCall.FindStringSubmatch(ansi.ReplaceAllString(e.Message, "")); m != nil {
		a.Matcher = m[1]
	}
	if e.Location == nil || e.Location.File == "" {
		return a
	}
	file := r.relative(e.Location.File)
	a.Origin, a.File, a.Line = OriginSupport, file, e.Location.Line
	if file == testFile {
		a.Origin = OriginTestFile
	}
	return a
}

// relative maps an error location under config.rootDir to the rootDir-relative slash path spec
// files use; any other path is kept as reported.
func (r *reader) relative(file string) string {
	file = slashPath(file)
	root := strings.TrimSuffix(slashPath(r.root), "/")
	if r.root != "" && strings.HasPrefix(file, root+"/") {
		return path.Clean(strings.TrimPrefix(file, root+"/"))
	}
	return file
}

// screenText applies the product secret screen to one emitted string.
func screenText(s string) string {
	out, _ := secretscreen.Screen(s)
	return out
}

// presented is the identity as emitted: every part secret-screened. Pools and matches stay keyed
// by the unscreened identity, so screening never merges two tests.
func (i Identity) presented() Identity {
	out := Identity{Project: screenText(i.Project), File: screenText(i.File), TitlePath: make([]string, len(i.TitlePath))}
	for n, title := range i.TitlePath {
		out.TitlePath[n] = screenText(title)
	}
	return out
}

// slashPath normalizes a reported path by the report's syntax rather than the reading host's:
// a Windows-produced report uses backslashes, which filepath.ToSlash leaves alone on Unix.
func slashPath(p string) string {
	return path.Clean(strings.ReplaceAll(p, `\`, "/"))
}

// FirstLine is the screened first non-empty line of an error message: ANSI escapes removed, the
// run-evidence hygiene and the product secret screen applied, and at most 240 bytes kept.
func FirstLine(message string) string {
	for _, line := range strings.Split(ansi.ReplaceAllString(message, ""), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line, _ = secretscreen.Screen(runhygiene.ScrubFailure(line))
		for len(line) > maxLineBytes {
			line = line[:maxLineBytes]
			for !utf8.ValidString(line) {
				line = line[:len(line)-1]
			}
		}
		return line
	}
	return ""
}

// declaredInputs reads config.metadata.corvintRunIdentity. Any other metadata is ignored; a
// malformed declaration counts as absent.
func declaredInputs(raw json.RawMessage) (Inputs, bool) {
	var meta struct {
		Identity *struct {
			ApplicationRevision  string            `json:"applicationRevision"`
			ConfigDigest         string            `json:"configDigest"`
			TestFileBlobs        map[string]string `json:"testFileBlobs"`
			ServedArtifactDigest string            `json:"servedArtifactDigest"`
		} `json:"corvintRunIdentity"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &meta) != nil || meta.Identity == nil {
		return Inputs{TestFileBlobs: map[string]string{}}, false
	}
	in := Inputs{ApplicationRevision: meta.Identity.ApplicationRevision, ConfigDigest: meta.Identity.ConfigDigest, TestFileBlobs: map[string]string{},
		ServedArtifactDigest: meta.Identity.ServedArtifactDigest}
	declared := map[string]map[string]bool{}
	for file, blob := range meta.Identity.TestFileBlobs {
		key := slashPath(file)
		if declared[key] == nil {
			declared[key] = map[string]bool{}
		}
		declared[key][blob] = true
		in.TestFileBlobs[key] = blob
	}
	// Two spellings of one file that declare different blobs are ambiguous: the file's blob is
	// dropped, so its tests abstain with IDENTITY_INPUT_MISSING (PWR-V0-002).
	for key, blobs := range declared {
		if len(blobs) < 2 {
			continue
		}
		delete(in.TestFileBlobs, key)
		if in.conflicts == nil {
			in.conflicts = map[string][]string{}
		}
		for blob := range blobs {
			in.conflicts[key] = append(in.conflicts[key], blob)
		}
		sort.Strings(in.conflicts[key])
	}
	in.TestClosureDigests, in.TestClosureRule = declaredClosure(raw)
	return in, true
}

// ClosureRuleDeclared is the caller's unverified closure claim (PWR-V0-068), and
// ClosureRuleComputed the closure `corvint runs identity --compute-closures` computed from the
// config's static TypeScript graph (PWR-V0-072). No other rule is admitted.
const (
	ClosureRuleDeclared = "DECLARED"
	ClosureRuleComputed = "corvint-test-closure/0"
)

// ValidClosureDigest reports whether v is sha256: and 64 lowercase hex digits (PWR-V0-068).
func ValidClosureDigest(v string) bool {
	hexPart, ok := strings.CutPrefix(v, "sha256:")
	return ok && validHex64(hexPart)
}

func validHex64(v string) bool {
	if len(v) != 64 {
		return false
	}
	for _, c := range v {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}

// declaredClosure reads corvintRunIdentity.testClosureDigests and .testClosureRule on their own,
// so a malformed closure member never hides the other inputs (PWR-V0-068). A digest of another
// form, a file whose spellings declare different digests and a rule other than DECLARED or
// corvint-test-closure/0 are dropped; nil and "" are absent.
func declaredClosure(raw json.RawMessage) (map[string]string, string) {
	var meta struct {
		Identity struct {
			Digests json.RawMessage `json:"testClosureDigests"`
			Rule    json.RawMessage `json:"testClosureRule"`
		} `json:"corvintRunIdentity"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		return nil, ""
	}
	rule := ""
	if json.Unmarshal(meta.Identity.Rule, &rule) != nil || rule != ClosureRuleDeclared && rule != ClosureRuleComputed {
		rule = ""
	}
	var declared map[string]json.RawMessage
	if json.Unmarshal(meta.Identity.Digests, &declared) != nil || len(declared) == 0 {
		return nil, rule
	}
	out, dropped := map[string]string{}, map[string]bool{}
	for file, value := range declared {
		key := slashPath(file)
		digest := ""
		if json.Unmarshal(value, &digest) != nil || !ValidClosureDigest(digest) {
			dropped[key] = true
			continue
		}
		if prior, seen := out[key]; seen && prior != digest {
			dropped[key] = true
		}
		out[key] = digest
	}
	for key := range dropped {
		delete(out, key)
	}
	if len(out) == 0 {
		return nil, rule
	}
	return out, rule
}

// hasIdentityMember reports whether config.metadata carries a corvintRunIdentity member.
func hasIdentityMember(raw json.RawMessage) bool {
	var parts map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &parts) != nil {
		return false
	}
	_, ok := parts["corvintRunIdentity"]
	return ok
}

// declaredRunFacts reads the reuse facts a run declares (PWR-V0-033, PWR-V0-037):
// corvintRunIdentity.testRevision and .testRootPath, and corvintRunCategory. A malformed value
// counts as absent.
func declaredRunFacts(raw json.RawMessage) (testRevision, testRootPath, category string) {
	var meta struct {
		Identity *struct {
			TestRevision string `json:"testRevision"`
			TestRootPath string `json:"testRootPath"`
		} `json:"corvintRunIdentity"`
		Category string `json:"corvintRunCategory"`
	}
	if len(raw) == 0 {
		return "", "", ""
	}
	if json.Unmarshal(raw, &meta) != nil {
		// One malformed member must not hide the others: read each alone.
		var parts map[string]json.RawMessage
		if json.Unmarshal(raw, &parts) != nil {
			return "", "", ""
		}
		_ = json.Unmarshal(parts["corvintRunIdentity"], &meta.Identity)
		_ = json.Unmarshal(parts["corvintRunCategory"], &meta.Category)
	}
	if meta.Identity != nil {
		testRevision, testRootPath = meta.Identity.TestRevision, meta.Identity.TestRootPath
	}
	return testRevision, testRootPath, meta.Category
}

func durationMS(ms float64) int64 {
	if ms < 0 || math.IsNaN(ms) || ms > math.MaxInt32 {
		return 0
	}
	return int64(math.Round(ms))
}
