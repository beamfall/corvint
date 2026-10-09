package stepnegation

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	maxTraceEntries   = 20000
	maxTraceFileBytes = 256 << 20
	maxResponseBytes  = 1 << 20
)

// Trace is the transient view of one Playwright trace.zip that derivation
// and the pass rules read. It is never retained (LPCV-V0-059, LPCV-V0-069).
type Trace struct {
	Steps      []TraceStep
	Assertions []Assertion
	// Failures lists every other failed test-runner step (pw:api, hook,
	// fixture and any unrecognized category).
	Failures  []Failure
	Responses []Response
}

// TraceStep is one test.step in reporter pre-order.
type TraceStep struct {
	CallID   string
	Title    string
	Ordinal  int
	Parent   int
	Location string
	Failed   bool
	Error    string
}

// Assertion is one expect step.
type Assertion struct {
	CallID     string
	Step       int
	Location   string
	Occurrence int
	Matcher    string
	Negated    bool
	Soft       bool
	Failed     bool
	Error      string
	StartTime  float64
	Frame      *FrameExpect
}

// FrameExpect is the library-side Frame.expect call recorded for an
// assertion: its selector, expression and expected text.
type FrameExpect struct {
	Selector     string
	Expression   string
	ExpectedText []ExpectedText
	IsNot        bool
	// Ordinal is the 1-based order among calls with the same selector and
	// expression.
	Ordinal int
}

// ExpectedText is one recorded expected value.
type ExpectedText struct {
	String              *string `json:"string"`
	RegexSource         *string `json:"regexSource"`
	MatchSubstring      bool    `json:"matchSubstring"`
	NormalizeWhiteSpace bool    `json:"normalizeWhiteSpace"`
	IgnoreCase          bool    `json:"ignoreCase"`
}

// Failure is one failed non-assertion, non-test.step runner step.
type Failure struct {
	CallID string
	Method string
	Step   int
	Error  string
}

// Response is one recorded network response.
type Response struct {
	Method    string
	URL       string
	MimeType  string
	Start     float64
	Completed float64
	// Body is present only for a text or JSON body of at most 1 MiB.
	Body []byte
}

type traceEvent struct {
	Type      string          `json:"type"`
	CallID    string          `json:"callId"`
	ParentID  string          `json:"parentId"`
	StepID    string          `json:"stepId"`
	StartTime float64         `json:"startTime"`
	Class     string          `json:"class"`
	Method    string          `json:"method"`
	Title     string          `json:"title"`
	Params    json.RawMessage `json:"params"`
	Stack     []struct {
		File   string `json:"file"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	} `json:"stack"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Snapshot *struct {
		Request struct {
			Method string `json:"method"`
			URL    string `json:"url"`
		} `json:"request"`
		Response struct {
			Content struct {
				MimeType string `json:"mimeType"`
				File     string `json:"_file"`
			} `json:"content"`
		} `json:"response"`
		MonotonicTime float64 `json:"_monotonicTime"`
		Time          float64 `json:"time"`
	} `json:"snapshot"`
}

type frameParams struct {
	Selector     string         `json:"selector"`
	Expression   string         `json:"expression"`
	ExpectedText []ExpectedText `json:"expectedText"`
	IsNot        bool           `json:"isNot"`
}

var (
	contextTracePattern = regexp.MustCompile(`^[0-9]+-trace\.trace$`)
	networkPattern      = regexp.MustCompile(`^[0-9]+-trace\.network$`)
	expectTitlePattern  = regexp.MustCompile(`^Expect "([^"]+)"`)
)

// ParseTrace reads one trace.zip. root makes stack locations
// worktree-relative; a location outside root keeps its slash path.
func ParseTrace(data []byte, root string) (*Trace, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("trace archive does not open: %w", err)
	}
	if len(archive.File) > maxTraceEntries {
		return nil, errors.New("trace archive has too many entries")
	}
	files := map[string]*zip.File{}
	var contexts, networks []string
	for _, file := range archive.File {
		files[file.Name] = file
		if contextTracePattern.MatchString(file.Name) {
			contexts = append(contexts, file.Name)
		}
		if networkPattern.MatchString(file.Name) {
			networks = append(networks, file.Name)
		}
	}
	sort.Strings(contexts)
	sort.Strings(networks)
	testTrace, ok := files["test.trace"]
	if !ok {
		return nil, errors.New("trace archive has no test.trace")
	}
	events, err := readEvents(testTrace)
	if err != nil {
		return nil, err
	}
	trace := &Trace{}
	trace.buildSteps(events, root)
	if err := trace.attachFrames(files, contexts); err != nil {
		return nil, err
	}
	if err := trace.readResponses(files, networks); err != nil {
		return nil, err
	}
	return trace, nil
}

func readEvents(file *zip.File) ([]traceEvent, error) {
	if file.UncompressedSize64 > maxTraceFileBytes {
		return nil, fmt.Errorf("trace entry %s exceeds its bound", file.Name)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	decoder := json.NewDecoder(io.LimitReader(reader, maxTraceFileBytes))
	var events []traceEvent
	for {
		var event traceEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			return events, nil
		}
		if err != nil {
			return nil, fmt.Errorf("trace entry %s does not decode: %w", file.Name, err)
		}
		events = append(events, event)
	}
}

func (trace *Trace) buildSteps(events []traceEvent, root string) {
	parents := map[string]string{}
	methods := map[string]string{}
	stepOrdinal := map[string]int{}
	assertionIndex := map[string]int{}
	failureIndex := map[string]int{}
	occurrences := map[string]int{}
	var innermost func(callID string) int
	innermost = func(callID string) int {
		for depth := 0; callID != "" && depth < 4096; depth++ {
			if ordinal, ok := stepOrdinal[callID]; ok {
				return ordinal
			}
			callID = parents[callID]
		}
		return 0
	}
	for _, event := range events {
		switch event.Type {
		case "before":
			if event.Class != "Test" {
				continue
			}
			parents[event.CallID] = event.ParentID
			methods[event.CallID] = event.Method
			location := ""
			if len(event.Stack) > 0 {
				location = relativeLocation(root, event.Stack[0].File) + ":" + strconv.Itoa(event.Stack[0].Line) + ":" + strconv.Itoa(event.Stack[0].Column)
			}
			switch event.Method {
			case "test.step":
				ordinal := len(trace.Steps) + 1
				trace.Steps = append(trace.Steps, TraceStep{CallID: event.CallID, Title: event.Title, Ordinal: ordinal, Parent: innermost(event.ParentID), Location: location})
				stepOrdinal[event.CallID] = ordinal
			case "expect":
				occurrences[location]++
				matcher, negated, soft := parseExpectTitle(event.Title)
				assertionIndex[event.CallID] = len(trace.Assertions)
				trace.Assertions = append(trace.Assertions, Assertion{CallID: event.CallID, Step: innermost(event.ParentID), Location: location, Occurrence: occurrences[location], Matcher: matcher, Negated: negated, Soft: soft, StartTime: event.StartTime})
			}
		case "after":
			if event.Error == nil {
				continue
			}
			message := event.Error.Message
			if message == "" {
				message = "error"
			}
			if ordinal, ok := stepOrdinal[event.CallID]; ok {
				trace.Steps[ordinal-1].Failed, trace.Steps[ordinal-1].Error = true, message
				continue
			}
			if index, ok := assertionIndex[event.CallID]; ok {
				trace.Assertions[index].Failed, trace.Assertions[index].Error = true, message
				continue
			}
			if _, ok := methods[event.CallID]; ok {
				if _, seen := failureIndex[event.CallID]; !seen {
					failureIndex[event.CallID] = len(trace.Failures)
					trace.Failures = append(trace.Failures, Failure{CallID: event.CallID, Method: methods[event.CallID], Step: innermost(parents[event.CallID]), Error: message})
				}
			}
		}
	}
}

func parseExpectTitle(title string) (matcher string, negated, soft bool) {
	match := expectTitlePattern.FindStringSubmatch(title)
	if match == nil {
		return "", false, false
	}
	words := strings.Fields(match[1])
	if len(words) == 0 {
		return "", false, false
	}
	for _, word := range words[:len(words)-1] {
		switch word {
		case "not":
			negated = true
		case "soft":
			soft = true
		}
	}
	return words[len(words)-1], negated, soft
}

func relativeLocation(root, file string) string {
	if root != "" {
		if relative, err := filepath.Rel(root, file); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(relative)
		}
	}
	return filepath.ToSlash(file)
}

func (trace *Trace) attachFrames(files map[string]*zip.File, contexts []string) error {
	byStep := map[string]int{}
	for index, assertion := range trace.Assertions {
		byStep[assertion.CallID] = index
	}
	type call struct {
		index  int
		start  float64
		params frameParams
	}
	var calls []call
	for _, name := range contexts {
		events, err := readEvents(files[name])
		if err != nil {
			return err
		}
		for _, event := range events {
			if event.Type != "before" || event.Class != "Frame" || event.Method != "expect" {
				continue
			}
			index, ok := byStep[event.StepID]
			if !ok {
				continue
			}
			var params frameParams
			if json.Unmarshal(event.Params, &params) != nil || params.Selector == "" {
				continue
			}
			calls = append(calls, call{index: index, start: event.StartTime, params: params})
		}
	}
	sort.SliceStable(calls, func(i, j int) bool { return calls[i].start < calls[j].start })
	ordinals := map[string]int{}
	for _, item := range calls {
		if trace.Assertions[item.index].Frame != nil {
			// More than one library call for one assertion: its locator is
			// not recoverable as one call.
			trace.Assertions[item.index].Frame = &FrameExpect{}
			continue
		}
		key := item.params.Selector + "\x00" + item.params.Expression
		ordinals[key]++
		trace.Assertions[item.index].Frame = &FrameExpect{Selector: item.params.Selector, Expression: item.params.Expression, ExpectedText: item.params.ExpectedText, IsNot: item.params.IsNot, Ordinal: ordinals[key]}
	}
	return nil
}

func (trace *Trace) readResponses(files map[string]*zip.File, networks []string) error {
	for _, name := range networks {
		events, err := readEvents(files[name])
		if err != nil {
			return err
		}
		for _, event := range events {
			if event.Type != "resource-snapshot" || event.Snapshot == nil {
				continue
			}
			snapshot := event.Snapshot
			response := Response{Method: snapshot.Request.Method, URL: snapshot.Request.URL, MimeType: snapshot.Response.Content.MimeType, Start: snapshot.MonotonicTime, Completed: snapshot.MonotonicTime + snapshot.Time}
			if snapshot.Time < 0 {
				// Playwright records -1 for a response that never completed.
				response.Completed = math.Inf(1)
			}
			if body := files[snapshot.Response.Content.File]; body != nil && textual(response.MimeType) && body.UncompressedSize64 <= maxResponseBytes {
				reader, err := body.Open()
				if err != nil {
					return err
				}
				data, err := io.ReadAll(io.LimitReader(reader, maxResponseBytes+1))
				reader.Close()
				if err != nil {
					return err
				}
				if len(data) <= maxResponseBytes {
					response.Body = data
				}
			}
			trace.Responses = append(trace.Responses, response)
		}
	}
	sort.SliceStable(trace.Responses, func(i, j int) bool { return trace.Responses[i].Start < trace.Responses[j].Start })
	return nil
}

func textual(mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	return strings.HasPrefix(mimeType, "text/") || mimeType == "application/json" || strings.HasSuffix(mimeType, "+json")
}

// Step returns the step with the given ordinal, or nil.
func (trace *Trace) Step(ordinal int) *TraceStep {
	if ordinal < 1 || ordinal > len(trace.Steps) {
		return nil
	}
	return &trace.Steps[ordinal-1]
}

// StepAssertions returns the assertions whose innermost test.step is ordinal.
func (trace *Trace) StepAssertions(ordinal int) []Assertion {
	var result []Assertion
	for _, assertion := range trace.Assertions {
		if assertion.Step == ordinal {
			result = append(result, assertion)
		}
	}
	return result
}

// FindAssertion returns the assertion at location with the given occurrence.
func (trace *Trace) FindAssertion(location string, occurrence int) *Assertion {
	for index := range trace.Assertions {
		if trace.Assertions[index].Location == location && trace.Assertions[index].Occurrence == occurrence {
			return &trace.Assertions[index]
		}
	}
	return nil
}

// Inventory returns the step inventory with screened titles.
func (trace *Trace) Inventory() Inventory {
	inventory := Inventory{Steps: make([]InventoryStep, 0, len(trace.Steps))}
	counts := map[int]int{}
	for _, assertion := range trace.Assertions {
		if assertion.Step == 0 {
			inventory.AssertionsOutsideSteps++
			continue
		}
		counts[assertion.Step]++
	}
	for _, step := range trace.Steps {
		inventory.Steps = append(inventory.Steps, InventoryStep{Title: Screen(step.Title), Ordinal: step.Ordinal, Assertions: counts[step.Ordinal]})
	}
	return inventory
}
