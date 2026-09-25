package appflows

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Run-evidence ingest formats (AFU-V1-012).
const (
	FormatPlaywrightJSON = "playwright-json"
	FormatJUnitXML       = "junit-xml"
	FormatGoTestJSON     = "go-test-json"
)

// RunHeader is the run context a test report does not carry, declared by the caller.
type RunHeader struct {
	RunID               string
	RunnerVersion       string
	Source              RunSource
	BuildArtifactDigest string
	Environment         RunDigestRef
	Fixture             RunDigestRef
	Cleanup             string
	Controls            []RunControl
}

// RunControl declares that TestKey is a negative control of Subject and must show Expected.
type RunControl struct {
	Subject  string
	TestKey  string
	Expected string
}

// Ingested is one ingest result. When a bound is exceeded Incomplete names it and Records is empty,
// so a bounded ingest is never a truncated success (AFU-V1-037).
type Ingested struct {
	Records    []TestRunEvidence
	Incomplete string
}

type boundError string

func (e boundError) Error() string { return "run evidence exceeds " + string(e) }

type runTest struct {
	key      string
	project  string
	attempts []RunAttempt
}

// add keeps one more attempt, enforcing the attempt and link bounds before it is kept (AFU-V1-037).
func (t *runTest) add(a RunAttempt) error {
	if len(t.attempts) >= maxRunAttempts {
		return boundError(BoundAttempts)
	}
	if len(a.AssertionAnchors)+len(a.Attachments) > maxRunLinks {
		return boundError(BoundLinks)
	}
	t.attempts = append(t.attempts, a)
	return nil
}

func appendTest(tests []*runTest, t *runTest) ([]*runTest, error) {
	if len(tests) >= maxRunRecords {
		return nil, boundError(BoundRecords)
	}
	return append(tests, t), nil
}

type runAdapter struct {
	runner string
	parse  func([]byte) ([]*runTest, error)
}

var runAdapters = map[string]runAdapter{
	FormatPlaywrightJSON: {"playwright", parsePlaywrightRun},
	FormatJUnitXML:       {"junit", parseJUnitRun},
	FormatGoTestJSON:     {"go-test", parseGoTestRun},
}

// IngestRunEvidence turns one test report into INGESTED test-run-evidence/0 records, one per test,
// keeping every attempt in order (AFU-V1-011..014, AFU-V1-037, AFU-V1-038).
func IngestRunEvidence(format string, raw []byte, header RunHeader) (Ingested, error) {
	adapter, ok := runAdapters[format]
	if !ok {
		return Ingested{}, errors.New("unsupported run-evidence format " + format)
	}
	if len(raw) > MaxBytes {
		return Ingested{Incomplete: BoundBytes}, nil
	}
	tests, err := adapter.parse(raw)
	var bound boundError
	if errors.As(err, &bound) {
		return Ingested{Incomplete: string(bound)}, nil
	}
	if err != nil {
		return Ingested{}, err
	}
	records, err := runRecords(adapter.runner, tests, header)
	if errors.As(err, &bound) {
		return Ingested{Incomplete: string(bound)}, nil
	}
	return Ingested{Records: records}, err
}

func runRecords(runner string, tests []*runTest, header RunHeader) ([]TestRunEvidence, error) {
	results := map[string]string{}
	for _, t := range tests {
		id := t.key + "\x00" + t.project
		if _, dup := results[id]; dup {
			return nil, fmt.Errorf("test %q repeats in one report", t.key)
		}
		t.attempts = finishAttempts(t.attempts)
		results[id] = Classify(t.attempts)
	}
	records := []TestRunEvidence{}
	for _, t := range tests {
		if len(t.attempts) == 0 {
			continue
		}
		r := TestRunEvidence{Schema: RunEvidenceSchema, Authority: AuthorityIngested, RunID: header.RunID, Runner: RunRunner{Name: runner, Version: header.RunnerVersion},
			Source: header.Source, BuildArtifactDigest: header.BuildArtifactDigest, Environment: header.Environment, Fixture: header.Fixture,
			TestKey: t.key, Project: t.project, Attempts: t.attempts, Cleanup: header.Cleanup, NegativeControls: controlsFor(t, header.Controls, results)}
		if _, err := EncodeRunEvidence(r); err != nil {
			return nil, fmt.Errorf("test %q: %w", t.key, err)
		}
		records = append(records, r)
	}
	return records, nil
}

func controlsFor(t *runTest, controls []RunControl, results map[string]string) []NegativeControl {
	out := []NegativeControl{}
	for _, c := range controls {
		if c.Subject != t.key {
			continue
		}
		observed, ok := results[c.TestKey+"\x00"+t.project]
		if !ok {
			observed = "not-run"
		}
		out = append(out, NegativeControl{TestKey: c.TestKey, Expected: c.Expected, Observed: observed})
	}
	return out
}

// Secret hygiene (AFU-V1-038) fails safe rather than parsing headers: a line that mentions a sensitive
// name anywhere, in any case or quoting, is replaced whole; a request or response body marker anywhere
// in a line keeps only the text before it and ends the detail; and an attachment that may hold
// cookies, credentials, a request or a response is dropped. EncodeRunEvidence then applies the
// product secret screen before any write.
const droppedMarker = "[dropped by run-evidence hygiene]"

var (
	// sensitiveLine matches the names inside any word, so `access_token`, `csrfToken` and `sessionId` count.
	sensitiveLine       = regexp.MustCompile(`(?i)cookie|authori[sz]ation|bearer|token|secret|passw(or)?d|api[-_]?key|csrf|session|credential|\bbasic\s+[a-z0-9+/]{4,}`)
	bodyMarker          = regexp.MustCompile(`(?i)\b(request|response)(\s*(body|text|data|payload))?\s*:`)
	sensitiveAttachment = regexp.MustCompile(`(?i)cookie|authori[sz]ation|token|secret|passw(or)?d|session|credential|request|response|body|storage-?state`)
)

func finishAttempts(attempts []RunAttempt) []RunAttempt {
	out := []RunAttempt{}
	for i, a := range attempts {
		a.Ordinal = i + 1
		a.Failure = strings.TrimSpace(scrubFailure(a.Failure))
		a.Attachments = keptAttachments(a.Attachments)
		if a.AssertionAnchors == nil {
			a.AssertionAnchors = []RunAnchor{}
		}
		out = append(out, a)
	}
	return out
}

func scrubFailure(text string) string {
	kept := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(strings.ToValidUTF8(text, "\uFFFD"), "\x00", ""), "\n") {
		body := bodyMarker.FindStringIndex(line)
		if body != nil {
			line = line[:body[0]]
		}
		if sensitiveLine.MatchString(line) {
			line = droppedMarker
		}
		if body != nil {
			return strings.Join(append(kept, strings.TrimSuffix(line, droppedMarker)+droppedMarker), "\n")
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// keptAttachments keeps path pointers only: inline content has no path and is never read.
func keptAttachments(in []RunAttachment) []RunAttachment {
	kept := []RunAttachment{}
	for _, a := range in {
		if a.Path == "" || sensitiveAttachment.MatchString(a.Name+" "+path.Base(a.Path)) || strings.HasSuffix(strings.ToLower(a.Path), ".har") {
			continue
		}
		kept = append(kept, a)
	}
	return kept
}

// jsonDepthBounded refuses nesting deeper than maxRunDepth token by token, before any decode (AFU-V1-037).
func jsonDepthBounded(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	depth := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errors.New("run report is not valid JSON")
		}
		switch tok {
		case json.Delim('{'), json.Delim('['):
			depth++
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
		if depth > maxRunDepth {
			return boundError(BoundTraversal)
		}
	}
}

func durationMS(ms float64) int64 {
	if ms < 0 || math.IsNaN(ms) || ms > math.MaxInt32 {
		return 0
	}
	return int64(math.Round(ms))
}

type playwrightResult struct {
	Status      string            `json:"status"`
	Duration    float64           `json:"duration"`
	Error       *playwrightError  `json:"error"`
	Errors      []playwrightError `json:"errors"`
	Attachments []RunAttachment   `json:"attachments"`
}

type playwrightError struct {
	Message  string `json:"message"`
	Location *struct {
		File string `json:"file"`
		Line int    `json:"line"`
	} `json:"location"`
}

// parsePlaywrightRun reads a Playwright JSON report: one test per spec and project, one attempt per result.
func parsePlaywrightRun(raw []byte) ([]*runTest, error) {
	if err := jsonDepthBounded(raw); err != nil {
		return nil, err
	}
	var doc struct {
		Suites []playwrightSuite `json:"suites"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, errors.New("invalid Playwright JSON report")
	}
	tests := []*runTest{}
	var err error
	for _, spec := range collectSpecs(doc.Suites, nil) {
		for _, pt := range spec.Tests {
			t := &runTest{key: spec.File + " > " + spec.Title, project: pt.ProjectName}
			for _, a := range pt.Annotations {
				if a.Type == "corvint-test-key" && a.Description != "" {
					t.key = a.Description
				}
			}
			if tests, err = appendTest(tests, t); err != nil {
				return nil, err
			}
			for i, result := range pt.Results {
				unexpected := pt.Status == "unexpected" && i == len(pt.Results)-1
				if err = t.add(playwrightAttempt(result, pt.ExpectedStatus, unexpected)); err != nil {
					return nil, err
				}
			}
		}
	}
	return tests, nil
}

// playwrightAttempt keeps one result. A passed result is recorded as failed when Playwright expected
// another status (`test.fail()`) or marked the test `unexpected`, so it never classifies as passed.
func playwrightAttempt(r playwrightResult, expected string, unexpected bool) RunAttempt {
	errs := r.Errors
	if len(errs) == 0 && r.Error != nil {
		errs = []playwrightError{*r.Error}
	}
	a := RunAttempt{Outcome: r.Status, DurationMS: durationMS(r.Duration), AssertionAnchors: []RunAnchor{}, Attachments: r.Attachments}
	messages := []string{}
	for _, e := range errs {
		messages = append(messages, e.Message)
		if e.Location != nil {
			a.AssertionAnchors = append(a.AssertionAnchors, RunAnchor{Path: e.Location.File, Line: e.Location.Line})
		}
	}
	a.Failure = strings.Join(messages, "\n")
	if r.Status == "passed" && expected != "" && expected != "passed" {
		a.Outcome, a.Failure = "failed", "passed, but the test expects "+expected
	}
	if a.Outcome == "passed" && unexpected {
		a.Outcome, a.Failure = "failed", "passed, but Playwright reported the test unexpected"
	}
	return a
}

// junitAttemptElements are the testcase children that each record one attempt. `flaky*` are earlier
// failed runs of a test whose last run passed; `rerun*` are later failed runs after the first failure.
var junitAttemptElements = map[string]string{"failure": "failed", "error": "failed", "skipped": "skipped",
	"flakyFailure": "failed", "flakyError": "failed", "rerunFailure": "failed", "rerunError": "failed"}

var junitAttachment = regexp.MustCompile(`\[\[ATTACHMENT\|([^\]\r\n]+)\]\]`)

// junitRunPayload is the byte preflight mirrored from TCQ-V0-026: after an optional BOM and one XML
// declaration, no declaration, DTD or entity markup is accepted.
func junitRunPayload(raw []byte) ([]byte, error) {
	payload := bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	if bytes.HasPrefix(payload, []byte("<?xml ")) {
		end := bytes.Index(payload, []byte("?>"))
		if end < 0 {
			return nil, errors.New("JUnit report has an unterminated XML declaration")
		}
		payload = payload[end+2:]
	}
	for _, marker := range []string{"<?", "<!DOCTYPE", "<!ENTITY"} {
		if bytes.Contains(payload, []byte(marker)) {
			return nil, errors.New("JUnit report carries a declaration, DTD or entity")
		}
	}
	if !utf8.Valid(payload) {
		return nil, errors.New("JUnit report is not UTF-8")
	}
	return payload, nil
}

// junitCase collects one testcase's attempts in report order. primary is the index of the attempt the
// testcase itself describes (failure, error, skipped, or the final pass), which takes the testcase
// time and the testcase-level attachments.
type junitCase struct {
	test        *runTest
	seconds     float64
	attempts    []RunAttempt
	primary     int
	attachments []RunAttachment
}

type junitRunWalk struct {
	depth   int
	tests   []*runTest
	c       *junitCase
	element string
	attempt RunAttempt
	detail  strings.Builder
	out     strings.Builder
	into    *strings.Builder
}

// parseJUnitRun walks a JUnit XML report once; the depth bound is checked before each descent.
func parseJUnitRun(raw []byte) ([]*runTest, error) {
	payload, err := junitRunPayload(raw)
	if err != nil {
		return nil, err
	}
	d := xml.NewDecoder(bytes.NewReader(payload))
	d.Strict = true
	w := &junitRunWalk{tests: []*runTest{}}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return w.tests, nil
		}
		if err != nil {
			return nil, errors.New("invalid JUnit XML report")
		}
		if err = w.token(tok); err != nil {
			return nil, err
		}
	}
}

func (w *junitRunWalk) token(tok xml.Token) error {
	switch t := tok.(type) {
	case xml.StartElement:
		if w.depth >= maxRunDepth {
			return boundError(BoundTraversal)
		}
		w.depth++
		return w.start(t)
	case xml.EndElement:
		w.depth--
		return w.end(t.Name.Local)
	case xml.CharData:
		if w.into != nil {
			w.into.Write(t)
		}
	}
	return nil
}

// start routes character data: failure and error text and stackTrace go to the attempt detail,
// system-out to attachment markers, everything else is not kept.
func (w *junitRunWalk) start(t xml.StartElement) error {
	name := t.Name.Local
	w.into = nil
	switch {
	case name == "testcase" && w.c == nil:
		return w.openCase(t)
	case w.c == nil:
	case w.element == "" && junitAttemptElements[name] != "":
		return w.openAttempt(t)
	case name == "system-out":
		w.out.Reset()
		w.into = &w.out
	case w.element != "" && name == "stackTrace":
		w.detail.WriteString("\n")
		w.into = &w.detail
	}
	return nil
}

func (w *junitRunWalk) openCase(t xml.StartElement) error {
	key := junitAttr(t, "name")
	if class := junitAttr(t, "classname"); class != "" {
		key = class + " > " + key
	}
	w.c = &junitCase{test: &runTest{key: key}, seconds: junitSeconds(t), primary: -1}
	var err error
	w.tests, err = appendTest(w.tests, w.c.test)
	return err
}

func (w *junitRunWalk) openAttempt(t xml.StartElement) error {
	if len(w.c.attempts) >= maxRunAttempts {
		return boundError(BoundAttempts)
	}
	w.element = t.Name.Local
	w.attempt = RunAttempt{Outcome: junitAttemptElements[w.element], DurationMS: durationMS(junitSeconds(t) * 1000), Attachments: []RunAttachment{}}
	w.detail.Reset()
	w.detail.WriteString(junitAttr(t, "message"))
	if w.element == "failure" || w.element == "error" {
		w.detail.WriteString("\n")
		w.into = &w.detail
	}
	return nil
}

// end resumes failure and error text after a nested element, so text on both sides of it is kept.
func (w *junitRunWalk) end(name string) error {
	w.into = nil
	if name != w.element && (w.element == "failure" || w.element == "error") {
		w.detail.WriteString("\n")
		w.into = &w.detail
	}
	switch {
	case w.c == nil:
	case name == "system-out" && w.element != "":
		w.attempt.Attachments = append(w.attempt.Attachments, junitAttachments(w.out.String())...)
	case name == "system-out":
		w.c.attachments = append(w.c.attachments, junitAttachments(w.out.String())...)
	case name == w.element:
		return w.closeAttempt()
	case name == "testcase" && w.element == "":
		return w.closeCase()
	}
	return nil
}

func (w *junitRunWalk) closeAttempt() error {
	a := w.attempt
	if a.Outcome == "failed" {
		a.Failure = w.detail.String()
	}
	primary := w.element == "failure" || w.element == "error" || w.element == "skipped"
	w.element = ""
	if primary && w.c.primary >= 0 {
		return fmt.Errorf("testcase %q carries more than one result", w.c.test.key)
	}
	if primary {
		w.c.primary = len(w.c.attempts)
		a.DurationMS = durationMS(w.c.seconds * 1000)
	}
	w.c.attempts = append(w.c.attempts, a)
	return nil
}

// closeCase appends the final passed attempt when the testcase names no result of its own, then keeps
// every attempt under the attempt and link bounds.
func (w *junitRunWalk) closeCase() error {
	c := w.c
	w.c = nil
	if c.primary < 0 {
		c.primary = len(c.attempts)
		c.attempts = append(c.attempts, RunAttempt{Outcome: "passed", DurationMS: durationMS(c.seconds * 1000), Attachments: []RunAttachment{}})
	}
	c.attempts[c.primary].Attachments = append(c.attempts[c.primary].Attachments, c.attachments...)
	for _, a := range c.attempts {
		if err := c.test.add(a); err != nil {
			return err
		}
	}
	return nil
}

func junitAttr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func junitSeconds(t xml.StartElement) float64 {
	seconds, err := strconv.ParseFloat(junitAttr(t, "time"), 64)
	if err != nil {
		return 0
	}
	return seconds
}

func junitAttachments(out string) []RunAttachment {
	kept := []RunAttachment{}
	for _, m := range junitAttachment.FindAllStringSubmatch(out, -1) {
		kept = append(kept, RunAttachment{Name: path.Base(m[1]), Path: m[1]})
	}
	return kept
}

type goTestEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

type goTestOpen struct {
	pkg    string
	test   *runTest
	output strings.Builder
}

var (
	goTestOutcomes = map[string]string{"pass": "passed", "fail": "failed", "skip": "skipped"}
	goTestAnchor   = regexp.MustCompile(`(?m)^\s+([\w./-]+_test\.go):(\d+): `)
)

const goTestTimeout = "panic: test timed out after "

// parseGoTestRun reads a `go test -json` event stream. Each run of a test (for example under -count)
// is one attempt; a test still running when the stream ends timed out if its package hit the test
// timeout, and was interrupted otherwise.
func parseGoTestRun(raw []byte) ([]*runTest, error) {
	if err := jsonDepthBounded(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	tests := []*runTest{}
	byKey := map[string]*runTest{}
	open := map[string]*goTestOpen{}
	timedOut := map[string]bool{}
	for {
		var e goTestEvent
		err := d.Decode(&e)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("invalid go test -json stream")
		}
		if strings.Contains(e.Output, goTestTimeout) {
			timedOut[e.Package] = true
		}
		if e.Test == "" {
			continue
		}
		key := e.Package + " > " + e.Test
		if e.Action == "run" {
			if tests, err = goTestRun(tests, byKey, open, e.Package, key); err != nil {
				return nil, err
			}
			continue
		}
		o := open[key]
		if o == nil {
			continue
		}
		if e.Action == "output" {
			o.output.WriteString(e.Output)
			continue
		}
		outcome := goTestOutcomes[e.Action]
		if outcome == "" {
			continue
		}
		delete(open, key)
		if err = o.test.add(goTestAttempt(outcome, e.Elapsed*1000, o.output.String())); err != nil {
			return nil, err
		}
	}
	return tests, goTestUnfinished(tests, open, timedOut)
}

func goTestRun(tests []*runTest, byKey map[string]*runTest, open map[string]*goTestOpen, pkg, key string) ([]*runTest, error) {
	if open[key] != nil {
		return nil, fmt.Errorf("test %q starts again before it ends", key)
	}
	t := byKey[key]
	if t == nil {
		t = &runTest{key: key}
		var err error
		if tests, err = appendTest(tests, t); err != nil {
			return nil, err
		}
		byKey[key] = t
	}
	open[key] = &goTestOpen{pkg: pkg, test: t}
	return tests, nil
}

func goTestUnfinished(tests []*runTest, open map[string]*goTestOpen, timedOut map[string]bool) error {
	for _, t := range tests {
		o := open[t.key]
		if o == nil {
			continue
		}
		outcome := "interrupted"
		if timedOut[o.pkg] {
			outcome = "timedOut"
		}
		if err := t.add(goTestAttempt(outcome, 0, o.output.String())); err != nil {
			return err
		}
	}
	return nil
}

// goTestAttempt keeps the output of an attempt that did not pass as its failure detail, with each
// `file_test.go:line:` report as an assertion anchor.
func goTestAttempt(outcome string, ms float64, output string) RunAttempt {
	a := RunAttempt{Outcome: outcome, DurationMS: durationMS(ms), AssertionAnchors: []RunAnchor{}, Attachments: []RunAttachment{}}
	if outcome == "passed" || outcome == "skipped" {
		return a
	}
	a.Failure = output
	for _, m := range goTestAnchor.FindAllStringSubmatch(output, -1) {
		line, _ := strconv.Atoi(m[2])
		a.AssertionAnchors = append(a.AssertionAnchors, RunAnchor{Path: m[1], Line: line})
	}
	return a
}
