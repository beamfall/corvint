package native

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// boostRunner reads the Boost.Test JUNIT log sink (TRE-V0-036..039). The
// detailed XML report omits disabled cases and the XML log carries no per-case
// status, so only the JUnit sink retains a per-case identity for every state.
const boostRunner = "boost-test-junit"
const boostReport = "boost-junit.xml"

// boostFailureExit is boost::exit_test_failure. exit_exception_failure (200)
// means an aborted run, a setup error or an empty filter and stays unadmitted.
const boostFailureExit = 201

// boostMaxNodes bounds the decoded tree; Boost writes one failure element per
// failed assertion, so a case may carry several outcome elements.
const boostMaxNodes = 65536

var boostSegment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var boostTime = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(e[-+][0-9]+)?$`)

// boostPath returns the suite/case path of a Target::suite/.../case identity.
func boostPath(target, id string) (string, bool) {
	prefix, path, ok := strings.Cut(id, "::")
	if !ok || prefix != target {
		return "", false
	}
	segments := strings.Split(path, "/")
	if len(segments) < 1 || len(segments) > 8 {
		return "", false
	}
	for _, s := range segments {
		if !boostSegment.MatchString(s) || len(s) > 128 {
			return "", false
		}
	}
	return path, true
}

func boostInventory(target string, expected, selectors []string) error {
	if !boostSegment.MatchString(target) || len(target) > 128 || len(expected) < 1 || len(expected) > tr.MaxTests || len(selectors) > len(expected) {
		return fmt.Errorf("boost.test requires one named master suite and 1..%d expected tests", tr.MaxTests)
	}
	seen := map[string]bool{}
	for _, id := range expected {
		if _, ok := boostPath(target, id); !ok || seen[id] {
			return fmt.Errorf("boost.test requires unique Target::suite/case identities")
		}
		seen[id] = true
	}
	chosen := map[string]bool{}
	for _, s := range selectors {
		if !seen[s] || chosen[s] {
			return fmt.Errorf("boost.test selectors must be unique expected identities")
		}
		chosen[s] = true
	}
	return nil
}

func buildBoostTest(r tr.Request) (tr.Invocation, error) {
	if err := boostInventory(r.Target, r.ExpectedTests, r.Selectors); err != nil {
		return tr.Invocation{}, err
	}
	if r.Project != "" || r.Config != "" || r.ConfigSha256 != "" || r.Reporter != "" || r.ReporterSha256 != "" || len(r.ReportFiles) > 0 || len(r.Tools) > 0 {
		return tr.Invocation{}, fmt.Errorf("boost.test does not admit project, configuration, reporter or auxiliary tool overrides")
	}
	argv := []string{"--log_format=JUNIT", "--log_level=error", "--log_sink=" + filepath.Join(r.ReportDir, boostReport), "--report_level=no", "--random=0", "--result_code=yes", "--build_info=no", "--color_output=no", "--show_progress=no", "--catch_system_errors=yes", "--auto_start_dbg=no"}
	if len(r.Selectors) > 0 {
		paths := make([]string, 0, len(r.Selectors))
		for _, s := range r.Selectors {
			p, _ := boostPath(r.Target, s)
			paths = append(paths, p)
		}
		// One colon-joined filter: repeated --run_test arguments are a native setup error.
		filter := "--run_test=" + strings.Join(paths, ":")
		if len(filter) > 4096 {
			return tr.Invocation{}, fmt.Errorf("boost.test selector filter exceeds one argument bound")
		}
		argv = append(argv, filter)
	}
	for _, a := range argv {
		if len(a) > 4096 {
			return tr.Invocation{}, fmt.Errorf("boost.test argument exceeds bound")
		}
	}
	return tr.Invocation{Format: boostRunner, Argv: argv, Environment: map[string]string{}, ReportPaths: []string{boostReport}, SuccessExitCodes: []int{0}, FailureExitCodes: failureExits(boostRunner)}, nil
}

type boostNode struct {
	name     string
	attrs    map[string]string
	text     string
	children []*boostNode
}

// boostXML decodes the closed JUnit sink grammar: suite, case and outcome
// elements only, with bounded nodes and no namespaces, directives or comments.
func boostXML(data []byte) (*boostNode, error) {
	if len(data) > tr.MaxReportBytes {
		return nil, fmt.Errorf("boost.test XML exceeds bound")
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	var root *boostNode
	stack := []*boostNode{}
	nodes := 0
	declaration := false
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			nodes++
			if t.Name.Space != "" || nodes > boostMaxNodes || len(stack) >= 3 {
				return nil, fmt.Errorf("boost.test XML structure outside profile")
			}
			n := &boostNode{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				if a.Name.Space != "" {
					return nil, fmt.Errorf("boost.test XML namespace unsupported")
				}
				if _, ok := n.attrs[a.Name.Local]; ok {
					return nil, fmt.Errorf("duplicate boost.test XML attribute")
				}
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("multiple boost.test XML roots")
				}
				root = n
			} else {
				p := stack[len(stack)-1]
				p.children = append(p.children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("unbalanced boost.test XML")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if len(bytes.TrimSpace(t)) != 0 {
					return nil, fmt.Errorf("trailing boost.test XML text")
				}
			} else {
				stack[len(stack)-1].text += string(t)
			}
		case xml.ProcInst:
			if root != nil || declaration || t.Target != "xml" {
				return nil, fmt.Errorf("boost.test XML instruction unsupported")
			}
			declaration = true
		default:
			return nil, fmt.Errorf("boost.test XML directive/comment unsupported")
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("missing boost.test XML root")
	}
	return root, nil
}

func boostShape(n *boostNode, name string, required []string, optional ...string) bool {
	if n.name != name {
		return false
	}
	allowed := map[string]bool{}
	for _, a := range optional {
		allowed[a] = true
	}
	for _, a := range required {
		if _, ok := n.attrs[a]; !ok {
			return false
		}
		allowed[a] = true
	}
	for a := range n.attrs {
		if !allowed[a] {
			return false
		}
	}
	return true
}

func boostCount(s string) (int, error) {
	n, e := strconv.Atoi(s)
	if e != nil || n < 0 || n > tr.MaxTests || strconv.Itoa(n) != s {
		return 0, fmt.Errorf("boost.test counter outside 0..%d", tr.MaxTests)
	}
	return n, nil
}

// boostPseudo reports Boost's synthetic rows for suite fixtures and global
// timing; they carry no test identity and cannot be retained as cases.
func boostPseudo(name string) bool {
	return strings.HasSuffix(name, "-setup-teardown") || strings.HasSuffix(name, "-timed-execution")
}

func parseBoostTest(in tr.Input) (tr.Observation, error) {
	o := tr.Observation{Runner: boostRunner, RetryInformation: tr.NotReported}
	problem := func(code, detail string) { o.Problems = append(o.Problems, tr.Problem{Code: code, Detail: detail}) }
	reject := func(err error) (tr.Observation, error) { problem("BOOST_INVALID_REPORT", err.Error()); return o, err }
	if err := boostInventory(in.Target, in.Expected, in.Selectors); err != nil {
		return reject(err)
	}
	data, ok := in.Reports[boostReport]
	if !ok || len(in.Reports) != 1 {
		return reject(fmt.Errorf("exact %s report required", boostReport))
	}
	root, err := boostXML(data)
	if err != nil {
		return reject(err)
	}
	if !boostShape(root, "testsuite", []string{"tests", "skipped", "errors", "failures", "id", "name", "time"}) || root.attrs["id"] != "0" || !boostTime.MatchString(root.attrs["time"]) || strings.TrimSpace(root.text) != "" {
		return reject(fmt.Errorf("boost.test suite grammar mismatch"))
	}
	if root.attrs["name"] != in.Target {
		return reject(fmt.Errorf("boost.test master suite disagrees with caller target"))
	}
	counts := map[string]int{}
	for _, key := range []string{"tests", "skipped", "errors", "failures"} {
		n, e := boostCount(root.attrs[key])
		if e != nil {
			return reject(e)
		}
		counts[key] = n
	}
	expected := map[string]bool{}
	for _, id := range in.Expected {
		expected[id] = true
	}
	selected := map[string]bool{}
	for _, id := range in.Selectors {
		selected[id] = true
	}
	seen := map[string]bool{}
	ran, skipped, failed, errored := 0, 0, 0, 0
	for _, c := range root.children {
		if c.name == "system-out" || c.name == "system-err" {
			if len(c.attrs) != 0 || len(c.children) != 0 {
				return reject(fmt.Errorf("boost.test suite log grammar mismatch"))
			}
			problem("BOOST_RUNNER_LOG", "native suite-level log retained outside any case")
			continue
		}
		if !boostShape(c, "testcase", []string{"assertions", "name", "time"}, "classname") || strings.TrimSpace(c.text) != "" || !boostTime.MatchString(c.attrs["time"]) {
			return reject(fmt.Errorf("boost.test case grammar mismatch"))
		}
		if n, e := strconv.ParseUint(c.attrs["assertions"], 10, 32); e != nil || strconv.FormatUint(n, 10) != c.attrs["assertions"] {
			return reject(fmt.Errorf("boost.test assertion counter mismatch"))
		}
		outcomes := []*boostNode{}
		skip := false
		for _, child := range c.children {
			if len(child.children) != 0 {
				return reject(fmt.Errorf("nested boost.test outcome"))
			}
			switch child.name {
			case "system-out", "system-err":
				if len(child.attrs) != 0 {
					return reject(fmt.Errorf("boost.test case log grammar mismatch"))
				}
			case "skipped":
				if skip || len(outcomes) != 0 || len(child.attrs) != 0 || strings.TrimSpace(child.text) != "" {
					return reject(fmt.Errorf("boost.test skipped grammar mismatch"))
				}
				skip = true
			case "failure", "error":
				if skip || !boostShape(child, child.name, []string{"message", "type"}) {
					return reject(fmt.Errorf("boost.test outcome grammar mismatch"))
				}
				outcomes = append(outcomes, child)
			default:
				return reject(fmt.Errorf("unknown boost.test case element"))
			}
		}
		if boostPseudo(c.attrs["name"]) {
			if _, ok := c.attrs["classname"]; ok || len(outcomes) == 0 {
				return reject(fmt.Errorf("boost.test synthetic row grammar mismatch"))
			}
			problem("BOOST_SUITE_FIXTURE_FAILURE", "native suite fixture or global row reported "+outcomes[0].attrs["type"])
			continue
		}
		path := []string{}
		if cls, ok := c.attrs["classname"]; ok {
			path = strings.Split(cls, ".")
		}
		path = append(path, c.attrs["name"])
		id := in.Target + "::" + strings.Join(path, "/")
		if _, ok := boostPath(in.Target, id); !ok {
			return reject(fmt.Errorf("boost.test case identity outside the suite/case grammar"))
		}
		if seen[id] {
			return reject(fmt.Errorf("duplicate boost.test case"))
		}
		seen[id] = true
		suite := in.Target
		if len(path) > 1 {
			suite += "::" + strings.Join(path[:len(path)-1], "/")
		}
		v := tr.Test{ID: id, Name: c.attrs["name"], Suite: suite, State: tr.Passed}
		switch {
		case skip:
			v.State = tr.Skipped
			skipped++
		case len(outcomes) > 0:
			// Boost counts a case as aborted (native errors) after an uncaught
			// error entry or a fatal (REQUIRE-level) assertion, and as an
			// ordinary failure (native failures) otherwise.
			v.State = tr.Failed
			v.FailureKind = tr.Assertion
			aborted := false
			messages := []string{}
			for _, out := range outcomes {
				messages = append(messages, out.text)
				t := out.attrs["type"]
				switch {
				case out.name == "error":
					aborted = true
					v.FailureKind = tr.Unknown
					problem("BOOST_ERROR_ENTRY", "native case aborted: "+t)
				case t == "fatal error":
					aborted = true
				case t != "assertion error":
					v.FailureKind = tr.Unknown
					problem("BOOST_FAILURE_TYPE", "native failure type is not an ordinary assertion: "+t)
				}
			}
			v.Message = strings.Join(messages, "\n")
			failed++
			ran++
			if aborted {
				errored++
			}
		default:
			ran++
		}
		if !expected[id] {
			problem("BOOST_SURPLUS_TEST", "native identity outside expected inventory")
		}
		if len(selected) > 0 && !selected[id] && v.State != tr.Skipped {
			problem("BOOST_UNSELECTED_EXECUTED", "native case executed outside the selector filter")
		}
		o.Tests = append(o.Tests, v)
	}
	if len(o.Tests) == 0 {
		problem("BOOST_NO_TESTS", "native suite has no observed tests")
	}
	for id := range expected {
		if !seen[id] {
			problem("BOOST_MISSING_TEST", "expected native identity absent")
		}
	}
	if counts["tests"] != ran || counts["skipped"] != skipped || counts["errors"] != errored || counts["failures"] != failed-errored {
		problem("BOOST_COUNT_MISMATCH", "native counters disagree with case outcomes")
	}
	switch in.ExitCode {
	case 0:
		if failed > 0 {
			problem("BOOST_EXIT_REPORT_CONTRADICTION", "successful exit with failed native cases")
		}
	case boostFailureExit:
		if failed == 0 {
			problem("BOOST_EXIT_REPORT_CONTRADICTION", "test-failure exit without a failed native case")
		}
	default:
		problem("BOOST_EXIT_UNSUPPORTED", "native exit outside the admitted 0/201 profile")
	}
	o.Complete = len(o.Problems) == 0
	sort.Slice(o.Tests, func(i, j int) bool { return o.Tests[i].ID < o.Tests[j].ID })
	return o, nil
}
