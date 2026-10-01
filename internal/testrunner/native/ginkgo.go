package native

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// ginkgoRunner executes one caller-built `go test -c` binary whose single Go
// wrapper calls Ginkgo v2.33.0 RunSpecs, and reads one native JSON report.
const ginkgoRunner = "ginkgo-v2"

// ginkgoMaxFocusArg bounds the whole `-ginkgo.focus=...` element to the shared
// executor's per-argument limit, below Linux MAX_ARG_STRLEN.
const ginkgoMaxFocusArg = 4096

var ginkgoWrapper = regexp.MustCompile(`^Test[A-Z0-9_][A-Za-z0-9_]*$`)
var ginkgoInteger = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// Exact enum tables of Ginkgo v2.33.0 types. A zero enum marshals as JSON null
// and is refused; strings outside a table are refused.
var ginkgoStates = ginkgoSet("pending", "skipped", "passed", "failed", "aborted", "panicked", "interrupted", "timedout")
var ginkgoNodeTypes = ginkgoSet("Container", "It", "BeforeEach", "JustBeforeEach", "AfterEach", "JustAfterEach", "BeforeAll", "AfterAll", "BeforeSuite", "SynchronizedBeforeSuite", "AfterSuite", "SynchronizedAfterSuite", "ReportBeforeEach", "ReportAfterEach", "ReportBeforeSuite", "ReportAfterSuite", "DeferCleanup", "DeferCleanup (Each)", "DeferCleanup (All)", "DeferCleanup (Suite)")
var ginkgoContexts = ginkgoSet("leaf-node", "top-level", "in-container")
var ginkgoEventTypes = ginkgoSet("By", "By (End)", "Node", "Node (End)", "Repeat", "Retry")
var ginkgoSuiteLevel = ginkgoSet("BeforeSuite", "SynchronizedBeforeSuite", "AfterSuite", "SynchronizedAfterSuite", "ReportBeforeSuite", "ReportAfterSuite", "DeferCleanup (Suite)")

func ginkgoSet(values ...string) map[string]bool {
	m := map[string]bool{}
	for _, v := range values {
		m[v] = true
	}
	return m
}

// ginkgoTarget splits `<GoWrapper>::<SuiteDescription>`.
func ginkgoTarget(target string) (string, string, error) {
	wrapper, desc, ok := strings.Cut(target, "::")
	if !ok || !ginkgoWrapper.MatchString(wrapper) || len(wrapper) > 128 || desc == "" || len(desc) > 1024 || !utf8.ValidString(desc) || strings.ContainsAny(desc, "\x00\r\n") || strings.Contains(desc, "::") || strings.HasSuffix(desc, ":") {
		return "", "", errors.New("ginkgo Target must be GoWrapper::SuiteDescription")
	}
	return wrapper, desc, nil
}

// ginkgoInventory validates the expected inventory and returns the selected
// full texts in selector order.
func ginkgoInventory(target string, expected, selectors []string) ([]string, error) {
	if len(expected) < 1 || len(expected) > tr.MaxTests || len(selectors) > tr.MaxTests {
		return nil, errors.New("ginkgo requires 1..4096 expected tests and at most 4096 selectors")
	}
	seen := map[string]bool{}
	for _, id := range expected {
		full, ok := strings.CutPrefix(id, target+"::")
		if !ok || full == "" || len(full) > 4096 || !utf8.ValidString(full) || strings.ContainsAny(full, "\x00\r\n") || seen[id] {
			return nil, errors.New("ginkgo requires unique Target::FullText identities")
		}
		seen[id] = true
	}
	var texts []string
	chosen := map[string]bool{}
	for _, s := range selectors {
		if !seen[s] || chosen[s] {
			return nil, errors.New("ginkgo selectors must be unique expected identities")
		}
		chosen[s] = true
		texts = append(texts, strings.TrimPrefix(s, target+"::"))
	}
	return texts, nil
}

// ginkgoFocus is the one `-ginkgo.focus` value. Ginkgo matches it, unanchored,
// against SuiteDescription + " " + FullText; the anchors make it exact.
func ginkgoFocus(desc string, texts []string) string {
	q := make([]string, len(texts))
	for i, t := range texts {
		q[i] = regexp.QuoteMeta(t)
	}
	return "^" + regexp.QuoteMeta(desc) + " (" + strings.Join(q, "|") + ")$"
}

func buildGinkgo(r tr.Request) (tr.Invocation, error) {
	wrapper, desc, err := ginkgoTarget(r.Target)
	if err != nil {
		return tr.Invocation{}, err
	}
	texts, err := ginkgoInventory(r.Target, r.ExpectedTests, r.Selectors)
	if err != nil {
		return tr.Invocation{}, err
	}
	if r.Config != "" || r.ConfigSha256 != "" || r.Reporter != "" || r.ReporterSha256 != "" || len(r.ReportFiles) > 0 || len(r.Tools) > 0 {
		return tr.Invocation{}, errors.New("ginkgo does not admit configuration, reporter, report or auxiliary tool overrides")
	}
	if r.Project == "" || !filepath.IsLocal(r.Project) || !strings.HasSuffix(r.Project, "_test.go") || strings.ContainsAny(r.Project, "\x00\r\n") || r.InputFiles[r.Project] == "" {
		return tr.Invocation{}, errors.New("ginkgo requires a pinned literal suite file as Project")
	}
	// Ginkgo reports SuitePath from getcwd, which resolves symlinks. Parsing
	// compares it with the unresolved Root, so the Root must already be resolved.
	if root, e := filepath.EvalSymlinks(r.Root); !filepath.IsAbs(r.Root) || e != nil || root != filepath.Clean(r.Root) {
		return tr.Invocation{}, errors.New("ginkgo requires an absolute symlink-resolved Root")
	}
	v := tr.Invocation{Format: ginkgoRunner, Environment: map[string]string{"GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "GOFLAGS": "", "GOWORK": "off"}, ReportPaths: []string{"ginkgo.json"}, SuccessExitCodes: []int{0}, FailureExitCodes: failureExits(ginkgoRunner)}
	v.Argv = []string{"-test.run=^" + regexp.QuoteMeta(wrapper) + "$", "-test.timeout=0", "-ginkgo.json-report=" + filepath.Join(r.ReportDir, "ginkgo.json"), "-ginkgo.seed=1", "-ginkgo.randomize-all=false", "-ginkgo.fail-fast=false", "-ginkgo.fail-on-pending=false", "-ginkgo.fail-on-empty=true", "-ginkgo.flake-attempts=1", "-ginkgo.dry-run=false", "-ginkgo.timeout=1h", "-ginkgo.grace-period=1s", "-ginkgo.sleep-on-failure=0", "-ginkgo.poll-progress-after=0", "-ginkgo.poll-progress-interval=0", "-ginkgo.output-interceptor-mode=none", "-ginkgo.no-color=true"}
	if len(texts) > 0 {
		focus := "-ginkgo.focus=" + ginkgoFocus(desc, texts)
		if len(focus) > ginkgoMaxFocusArg {
			return tr.Invocation{}, errors.New("ginkgo focus argument exceeds 4096 bytes")
		}
		v.Argv = append(v.Argv, focus)
	}
	return v, nil
}

// ginkgoJSON decodes strict JSON into a tree, refusing duplicate keys, invalid
// UTF-8, excessive depth and trailing content.
func ginkgoJSON(b []byte) (any, error) {
	if !utf8.Valid(b) {
		return nil, errors.New("ginkgo report is not valid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	v, err := ginkgoValue(d, 0)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errors.New("trailing ginkgo report content")
	}
	return v, nil
}

func ginkgoValue(d *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("ginkgo report nesting bound exceeded")
	}
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, _ := k.(string)
			if _, dup := m[key]; dup {
				return nil, fmt.Errorf("duplicate ginkgo report key %q", key)
			}
			if m[key], err = ginkgoValue(d, depth+1); err != nil {
				return nil, err
			}
		}
		_, err = d.Token()
		return m, err
	case json.Delim('['):
		a := []any{}
		for d.More() {
			v, err := ginkgoValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		_, err = d.Token()
		return a, err
	}
	return t, nil
}

func ginkgoObject(v any, name string, required, optional []string) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("ginkgo %s must be an object", name)
	}
	allowed := ginkgoSet(append(append([]string{}, required...), optional...)...)
	for k := range m {
		if !allowed[k] {
			return nil, fmt.Errorf("ginkgo %s has unknown key %q", name, k)
		}
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			return nil, fmt.Errorf("ginkgo %s lacks key %q", name, k)
		}
	}
	return m, nil
}

func ginkgoString(v any, name string) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("ginkgo %s must be a string", name)
	}
	return s, nil
}

func ginkgoBool(v any, name string) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("ginkgo %s must be a boolean", name)
	}
	return b, nil
}

func ginkgoInt(v any, name string) (int64, error) {
	n, ok := v.(json.Number)
	if !ok || !ginkgoInteger.MatchString(n.String()) {
		return 0, fmt.Errorf("ginkgo %s must be an integer", name)
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ginkgo %s must be an integer", name)
	}
	return i, nil
}

// ginkgoEnum admits only an exact table string; JSON null (a zero enum) refuses.
func ginkgoEnum(v any, table map[string]bool, name string) (string, error) {
	s, ok := v.(string)
	if !ok || !table[s] {
		return "", fmt.Errorf("ginkgo %s is not a known enum string", name)
	}
	return s, nil
}

func ginkgoStrings(v any, name string) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	a, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("ginkgo %s must be a string array", name)
	}
	out := make([]string, len(a))
	for i, x := range a {
		s, ok := x.(string)
		if !ok {
			return nil, fmt.Errorf("ginkgo %s must be a string array", name)
		}
		out[i] = s
	}
	return out, nil
}

// ginkgoOpaque checks a diagnostic value only for its JSON kind; the report
// size bound already limits it.
func ginkgoOpaque(v any, name string, kinds ...string) error {
	for _, k := range kinds {
		switch k {
		case "null":
			if v == nil {
				return nil
			}
		case "object":
			if _, ok := v.(map[string]any); ok {
				return nil
			}
		case "array":
			if _, ok := v.([]any); ok {
				return nil
			}
		case "string":
			if _, ok := v.(string); ok {
				return nil
			}
		case "integer":
			if _, err := ginkgoInt(v, name); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("ginkgo %s has an unsupported JSON kind", name)
}

type ginkgoFailure struct {
	message    string
	context    string
	additional int
}

func ginkgoFailureValue(v any, name string) (*ginkgoFailure, error) {
	m, err := ginkgoObject(v, name, []string{"Message", "Location", "TimelineLocation", "FailureNodeLocation", "ProgressReport"}, []string{"ForwardedPanic", "FailureNodeContext", "FailureNodeType", "FailureNodeContainerIndex", "AdditionalFailure"})
	if err != nil {
		return nil, err
	}
	f := &ginkgoFailure{}
	if f.message, err = ginkgoString(m["Message"], name+".Message"); err != nil {
		return nil, err
	}
	for _, k := range []string{"Location", "TimelineLocation", "FailureNodeLocation", "ProgressReport"} {
		if err = ginkgoOpaque(m[k], name+"."+k, "object"); err != nil {
			return nil, err
		}
	}
	if x, ok := m["ForwardedPanic"]; ok {
		if _, err = ginkgoString(x, name+".ForwardedPanic"); err != nil {
			return nil, err
		}
	}
	if x, ok := m["FailureNodeContext"]; ok {
		if f.context, err = ginkgoEnum(x, ginkgoContexts, name+".FailureNodeContext"); err != nil {
			return nil, err
		}
	}
	if x, ok := m["FailureNodeType"]; ok {
		if _, err = ginkgoEnum(x, ginkgoNodeTypes, name+".FailureNodeType"); err != nil {
			return nil, err
		}
	}
	if x, ok := m["FailureNodeContainerIndex"]; ok {
		if _, err = ginkgoInt(x, name+".FailureNodeContainerIndex"); err != nil {
			return nil, err
		}
	}
	if x, ok := m["AdditionalFailure"]; ok {
		if _, err = ginkgoAdditional(x, name+".AdditionalFailure"); err != nil {
			return nil, err
		}
		f.additional++
	}
	return f, nil
}

func ginkgoAdditional(v any, name string) (*ginkgoFailure, error) {
	m, err := ginkgoObject(v, name, []string{"State", "Failure"}, nil)
	if err != nil {
		return nil, err
	}
	if _, err = ginkgoEnum(m["State"], ginkgoStates, name+".State"); err != nil {
		return nil, err
	}
	return ginkgoFailureValue(m["Failure"], name+".Failure")
}

type ginkgoSpec struct {
	node, state, full, message string
	attempts, mustPass, process int64
	failure                     *ginkgoFailure
	additional                  int
	retryEvent                  bool
}

func ginkgoSpecValue(v any) (ginkgoSpec, error) {
	var s ginkgoSpec
	m, err := ginkgoObject(v, "SpecReport", []string{"ContainerHierarchyTexts", "ContainerHierarchyLocations", "ContainerHierarchyLabels", "ContainerHierarchySemVerConstraints", "ContainerHierarchyComponentSemVerConstraints", "LeafNodeType", "LeafNodeLocation", "LeafNodeLabels", "LeafNodeSemVerConstraints", "LeafNodeText", "State", "StartTime", "EndTime", "RunTime", "ParallelProcess", "NumAttempts", "MaxFlakeAttempts", "MaxMustPassRepeatedly"}, []string{"Failure", "CapturedGinkgoWriterOutput", "CapturedStdOutErr", "ReportEntries", "ProgressReports", "AdditionalFailures", "SpecEvents"})
	if err != nil {
		return s, err
	}
	containers, err := ginkgoStrings(m["ContainerHierarchyTexts"], "ContainerHierarchyTexts")
	if err != nil {
		return s, err
	}
	leaf, err := ginkgoString(m["LeafNodeText"], "LeafNodeText")
	if err != nil {
		return s, err
	}
	var parts []string
	for _, t := range append(containers, leaf) {
		if strings.ContainsAny(t, "\x00\r\n") {
			return s, errors.New("ginkgo node text contains NUL, CR or LF")
		}
		if t != "" {
			parts = append(parts, t)
		}
	}
	s.full = strings.Join(parts, " ")
	if s.node, err = ginkgoEnum(m["LeafNodeType"], ginkgoNodeTypes, "LeafNodeType"); err != nil {
		return s, err
	}
	if s.state, err = ginkgoEnum(m["State"], ginkgoStates, "State"); err != nil {
		return s, err
	}
	for k, kinds := range map[string][]string{"ContainerHierarchyLocations": {"null", "array"}, "ContainerHierarchyLabels": {"null", "array"}, "ContainerHierarchySemVerConstraints": {"null", "array"}, "ContainerHierarchyComponentSemVerConstraints": {"null", "array"}, "LeafNodeLocation": {"object"}, "LeafNodeLabels": {"null", "array"}, "LeafNodeSemVerConstraints": {"null", "array"}, "StartTime": {"string"}, "EndTime": {"string"}, "RunTime": {"integer"}, "MaxFlakeAttempts": {"integer"}, "CapturedGinkgoWriterOutput": {"string"}, "CapturedStdOutErr": {"string"}, "ReportEntries": {"array"}, "ProgressReports": {"array"}} {
		if x, ok := m[k]; ok {
			if err = ginkgoOpaque(x, k, kinds...); err != nil {
				return s, err
			}
		}
	}
	if s.process, err = ginkgoInt(m["ParallelProcess"], "ParallelProcess"); err != nil {
		return s, err
	}
	if s.attempts, err = ginkgoInt(m["NumAttempts"], "NumAttempts"); err != nil || s.attempts < 0 {
		return s, errors.New("ginkgo NumAttempts must be a non-negative integer")
	}
	if s.mustPass, err = ginkgoInt(m["MaxMustPassRepeatedly"], "MaxMustPassRepeatedly"); err != nil {
		return s, err
	}
	if x, ok := m["Failure"]; ok {
		if s.failure, err = ginkgoFailureValue(x, "Failure"); err != nil {
			return s, err
		}
		s.message = s.failure.message
		s.additional += s.failure.additional
	}
	if x, ok := m["AdditionalFailures"]; ok {
		a, ok := x.([]any)
		if !ok {
			return s, errors.New("ginkgo AdditionalFailures must be an array")
		}
		for _, f := range a {
			if _, err = ginkgoAdditional(f, "AdditionalFailures"); err != nil {
				return s, err
			}
			s.additional++
		}
	}
	if x, ok := m["SpecEvents"]; ok {
		a, ok := x.([]any)
		if !ok {
			return s, errors.New("ginkgo SpecEvents must be an array")
		}
		for _, e := range a {
			em, err := ginkgoObject(e, "SpecEvent", []string{"SpecEventType", "CodeLocation", "TimelineLocation"}, []string{"Message", "Duration", "NodeType", "Attempt"})
			if err != nil {
				return s, err
			}
			kind, err := ginkgoEnum(em["SpecEventType"], ginkgoEventTypes, "SpecEventType")
			if err != nil {
				return s, err
			}
			s.retryEvent = s.retryEvent || kind == "Retry" || kind == "Repeat"
			for k, kinds := range map[string][]string{"CodeLocation": {"object"}, "TimelineLocation": {"object"}, "Message": {"string"}, "Duration": {"integer"}, "Attempt": {"integer"}} {
				if x, ok := em[k]; ok {
					if err = ginkgoOpaque(x, "SpecEvent."+k, kinds...); err != nil {
						return s, err
					}
				}
			}
			if x, ok := em["NodeType"]; ok {
				if _, err = ginkgoEnum(x, ginkgoNodeTypes, "SpecEvent.NodeType"); err != nil {
					return s, err
				}
			}
		}
	}
	return s, nil
}

// ginkgoConfig compares the native effective SuiteConfig with the fixed argv.
func ginkgoConfig(v any, focus string, problem func(string, string)) error {
	ints := map[string]int64{"RandomSeed": 1, "FlakeAttempts": 1, "MustPassRepeatedly": 0, "PollProgressAfter": 0, "PollProgressInterval": 0, "Timeout": 3600000000000, "GracePeriod": 1000000000, "SleepOnFailure": 0, "ParallelProcess": 1, "ParallelTotal": 1}
	bools := map[string]bool{"RandomizeAllSpecs": false, "FailOnPending": false, "FailOnEmpty": true, "FailFast": false, "DryRun": false, "EmitSpecProgress": false}
	strs := map[string]string{"LabelFilter": "", "SemVerFilter": "", "OutputInterceptorMode": "none", "ParallelHost": ""}
	lists := []string{"FocusStrings", "SkipStrings", "FocusFiles", "SkipFiles", "SourceRoots"}
	var keys []string
	for k := range ints {
		keys = append(keys, k)
	}
	for k := range bools {
		keys = append(keys, k)
	}
	for k := range strs {
		keys = append(keys, k)
	}
	m, err := ginkgoObject(v, "SuiteConfig", append(keys, lists...), nil)
	if err != nil {
		return err
	}
	for k, want := range ints {
		got, err := ginkgoInt(m[k], "SuiteConfig."+k)
		if err != nil {
			return err
		}
		if got != want {
			problem("GINKGO_CONFIG_MISMATCH", k)
		}
	}
	for k, want := range bools {
		got, err := ginkgoBool(m[k], "SuiteConfig."+k)
		if err != nil {
			return err
		}
		if got != want {
			problem("GINKGO_CONFIG_MISMATCH", k)
		}
	}
	for k, want := range strs {
		got, err := ginkgoString(m[k], "SuiteConfig."+k)
		if err != nil {
			return err
		}
		if got != want {
			problem("GINKGO_CONFIG_MISMATCH", k)
		}
	}
	for _, k := range lists {
		got, err := ginkgoStrings(m[k], "SuiteConfig."+k)
		if err != nil {
			return err
		}
		if k == "FocusStrings" && focus != "" {
			if len(got) != 1 || got[0] != focus {
				problem("GINKGO_CONFIG_MISMATCH", k)
			}
		} else if len(got) != 0 {
			problem("GINKGO_CONFIG_MISMATCH", k)
		}
	}
	return nil
}

func parseGinkgo(in tr.Input, b []byte, o *tr.Observation) error {
	problem := func(code, detail string) { o.Problems = append(o.Problems, tr.Problem{Code: code, Detail: detail}) }
	reject := func(err error) error { problem("GINKGO_INVALID_REPORT", err.Error()); return err }
	_, desc, err := ginkgoTarget(in.Target)
	if err != nil {
		return reject(err)
	}
	texts, err := ginkgoInventory(in.Target, in.Expected, in.Selectors)
	if err != nil {
		return reject(err)
	}
	focus := ""
	if len(texts) > 0 {
		focus = ginkgoFocus(desc, texts)
	}
	root, err := ginkgoJSON(b)
	if err != nil {
		return reject(err)
	}
	reports, ok := root.([]any)
	if !ok || len(reports) != 1 {
		return reject(errors.New("ginkgo report must be an array of exactly one suite"))
	}
	rep, err := ginkgoObject(reports[0], "Report", []string{"SuitePath", "SuiteDescription", "SuiteLabels", "SuiteSemVerConstraints", "SuiteComponentSemVerConstraints", "SuiteSucceeded", "SuiteHasProgrammaticFocus", "SpecialSuiteFailureReasons", "PreRunStats", "StartTime", "EndTime", "RunTime", "SuiteConfig", "SpecReports"}, nil)
	if err != nil {
		return reject(err)
	}
	suiteDesc, err := ginkgoString(rep["SuiteDescription"], "SuiteDescription")
	if err != nil {
		return reject(err)
	}
	if suiteDesc != desc {
		problem("GINKGO_TARGET_MISMATCH", "native SuiteDescription differs from Target")
		return errors.New("ginkgo SuiteDescription differs from Target")
	}
	suitePath, err := ginkgoString(rep["SuitePath"], "SuitePath")
	if err != nil {
		return reject(err)
	}
	if in.SourceRoot == "" || suitePath != filepath.Clean(in.SourceRoot) {
		problem("GINKGO_SUITE_PATH_MISMATCH", "native SuitePath differs from the execution Root")
	}
	for k, kinds := range map[string][]string{"SuiteLabels": {"null", "array"}, "SuiteSemVerConstraints": {"null", "array"}, "SuiteComponentSemVerConstraints": {"null", "object"}, "StartTime": {"string"}, "EndTime": {"string"}, "RunTime": {"integer"}} {
		if err = ginkgoOpaque(rep[k], k, kinds...); err != nil {
			return reject(err)
		}
	}
	succeeded, err := ginkgoBool(rep["SuiteSucceeded"], "SuiteSucceeded")
	if err != nil {
		return reject(err)
	}
	programmatic, err := ginkgoBool(rep["SuiteHasProgrammaticFocus"], "SuiteHasProgrammaticFocus")
	if err != nil {
		return reject(err)
	}
	reasons, err := ginkgoStrings(rep["SpecialSuiteFailureReasons"], "SpecialSuiteFailureReasons")
	if err != nil {
		return reject(err)
	}
	stats, err := ginkgoObject(rep["PreRunStats"], "PreRunStats", []string{"TotalSpecs", "SpecsThatWillRun"}, nil)
	if err != nil {
		return reject(err)
	}
	total, err := ginkgoInt(stats["TotalSpecs"], "TotalSpecs")
	if err != nil {
		return reject(err)
	}
	willRun, err := ginkgoInt(stats["SpecsThatWillRun"], "SpecsThatWillRun")
	if err != nil {
		return reject(err)
	}
	if err = ginkgoConfig(rep["SuiteConfig"], focus, problem); err != nil {
		return reject(err)
	}
	var records []any
	if rep["SpecReports"] != nil {
		if records, ok = rep["SpecReports"].([]any); !ok {
			return reject(errors.New("ginkgo SpecReports must be an array"))
		}
	}
	selected := ginkgoSet(texts...)
	expected := ginkgoSet(in.Expected...)
	seen := map[string]bool{}
	nIt, suiteLevel, eligible := 0, 0, 0
	nativeFailure, suiteFailed := false, false
	for _, raw := range records {
		s, err := ginkgoSpecValue(raw)
		if err != nil {
			return reject(err)
		}
		if s.attempts > 1 || s.mustPass > 1 || s.retryEvent {
			problem("GINKGO_RETRY_UNSUPPORTED", s.full)
		}
		if s.process != 1 {
			problem("GINKGO_PARALLEL_UNSUPPORTED", s.full)
		}
		if s.additional > 0 {
			problem("GINKGO_ADDITIONAL_FAILURE", s.full)
		}
		if ginkgoSuiteLevel[s.node] {
			if suiteLevel++; suiteLevel > 64 {
				return reject(errors.New("ginkgo suite-level record bound exceeded"))
			}
			if s.state != "passed" || s.failure != nil {
				suiteFailed = true
				problem("GINKGO_SUITE_PROBLEM", s.node+" "+s.state)
			}
			continue
		}
		if s.node != "It" {
			problem("GINKGO_UNSUPPORTED_NODE", s.node)
			continue
		}
		if nIt++; nIt > tr.MaxTests {
			return reject(errors.New("ginkgo It record bound exceeded"))
		}
		if s.full == "" || len(s.full) > 4096 || seen[s.full] {
			return reject(errors.New("ginkgo full text is empty, too long or duplicated"))
		}
		seen[s.full] = true
		id := in.Target + "::" + s.full
		if !expected[id] {
			problem("GINKGO_INVENTORY_MISMATCH", "native spec outside expected inventory")
		}
		row := tr.Test{ID: id, Name: s.full, Suite: desc, State: tr.Unknown, Message: s.message}
		isEligible := s.state != "pending" && (len(texts) == 0 || selected[s.full])
		if s.state != "passed" && s.state != "skipped" && s.state != "pending" {
			nativeFailure = true
		}
		switch {
		case !isEligible:
			if s.attempts != 0 || (s.state != "pending" && s.state != "skipped") {
				problem("GINKGO_UNSELECTED_EXECUTED", s.full)
			} else if s.failure != nil {
				problem("GINKGO_UNEXPECTED_FAILURE", s.full)
			} else {
				row.State = tr.Skipped
			}
		case s.attempts == 0:
			problem("GINKGO_NOT_ENTERED", s.full)
		case s.attempts > 1:
		case s.state == "passed":
			if s.failure != nil {
				problem("GINKGO_UNEXPECTED_FAILURE", s.full)
			} else {
				row.State = tr.Passed
				row.Attempts = []tr.Attempt{{State: tr.Passed}}
			}
		case s.state == "failed":
			if s.failure == nil || s.failure.context != "leaf-node" {
				problem("GINKGO_HOOK_FAILURE", s.full)
			} else {
				row.State, row.FailureKind = tr.Failed, tr.Unknown
				row.Attempts = []tr.Attempt{{State: tr.Failed, FailureKind: tr.Unknown, Message: s.message}}
			}
		case s.state == "skipped":
			row.State = tr.Skipped
			row.Attempts = []tr.Attempt{{State: tr.Skipped, Message: s.message}}
		default:
			problem("GINKGO_UNRESOLVED_STATE", s.state+" "+s.full)
		}
		if isEligible {
			eligible++
		}
		o.Tests = append(o.Tests, row)
	}
	if int64(nIt) != total || nIt != len(in.Expected) {
		problem("GINKGO_COUNT_MISMATCH", fmt.Sprintf("It records %d, TotalSpecs %d, expected %d", nIt, total, len(in.Expected)))
	}
	if int64(eligible) != willRun {
		problem("GINKGO_COUNT_MISMATCH", fmt.Sprintf("eligible %d, SpecsThatWillRun %d", eligible, willRun))
	}
	for id := range expected {
		if !seen[strings.TrimPrefix(id, in.Target+"::")] {
			problem("GINKGO_INVENTORY_MISMATCH", "expected spec absent from native report")
			break
		}
	}
	if programmatic {
		problem("GINKGO_PROGRAMMATIC_FOCUS", "native suite has programmatic focus")
	}
	// A BeforeSuite Skip() records a reason while the suite can still succeed;
	// any reason keeps the run incomplete regardless of SuiteSucceeded.
	if len(reasons) > 0 {
		problem("GINKGO_SUITE_FAILURE_REASON", strings.Join(reasons, "; "))
	}
	switch in.ExitCode {
	case 0:
		if !succeeded || nativeFailure || programmatic {
			problem("GINKGO_EXIT_SUITE_CONTRADICTION", "exit 0 without a successful unfocused suite")
		}
	case 1:
		if succeeded {
			problem("GINKGO_EXIT_SUITE_CONTRADICTION", "exit 1 with SuiteSucceeded true")
		} else if !nativeFailure && !suiteFailed && len(reasons) == 0 {
			problem("GINKGO_UNEXPLAINED_SUITE_FAILURE", "exit 1 without a failed spec, suite node or reason")
		}
	default:
		problem("GINKGO_EXIT_UNSUPPORTED", strconv.Itoa(in.ExitCode))
	}
	return nil
}
