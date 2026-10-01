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

const cmockaRunner = "cmocka-xml"

var cmockaName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var cmockaTime = regexp.MustCompile(`^[0-9]+\.[0-9]{3}$`)

func cmockaInventory(target string, expected, selectors []string) error {
	if !cmockaName.MatchString(target) || len(target) > 128 || len(expected) < 1 || len(expected) > 64 || len(selectors) > 1 {
		return fmt.Errorf("cmocka requires one named group and 1..64 expected tests")
	}
	seen := map[string]bool{}
	for _, id := range expected {
		group, name, ok := strings.Cut(id, "::")
		if !ok || group != target || !cmockaName.MatchString(name) || len(name) > 128 || seen[id] {
			return fmt.Errorf("cmocka requires unique literal group::test identities")
		}
		seen[id] = true
	}
	if len(selectors) == 1 && (len(expected) != 1 || selectors[0] != expected[0]) {
		return fmt.Errorf("cmocka selector must be the sole expected literal identity")
	}
	return nil
}
func buildCMocka(r tr.Request) (tr.Invocation, error) {
	if err := cmockaInventory(r.Target, r.ExpectedTests, r.Selectors); err != nil {
		return tr.Invocation{}, err
	}
	if r.Project != "" || r.Config != "" || r.ConfigSha256 != "" || r.Reporter != "" || r.ReporterSha256 != "" || len(r.ReportFiles) > 0 || len(r.Tools) > 0 {
		return tr.Invocation{}, fmt.Errorf("cmocka does not admit project, configuration, reporter or auxiliary tool overrides")
	}
	v := tr.Invocation{Format: cmockaRunner, Argv: []string{}, Environment: map[string]string{"CMOCKA_MESSAGE_OUTPUT": "STANDARD,XML", "CMOCKA_XML_FILE": filepath.Join(r.ReportDir, "cmocka.xml"), "CMOCKA_ERROR_OUTPUT": "STDERR"}, ReportPaths: []string{"cmocka.xml"}, SuccessExitCodes: []int{0}}
	v.FailureExitCodes = failureExits(cmockaRunner)
	if len(r.Selectors) == 1 {
		_, name, _ := strings.Cut(r.Selectors[0], "::")
		v.Environment["CMOCKA_TEST_FILTER"] = name
	}
	v.Phases = []tr.Phase{{Kind: "TEST", Tool: "primary", Argv: []string{}, Environment: v.Environment}}
	v.Argv = nil
	v.Environment = nil
	return v, nil
}

// cmockaNode keeps the exact small native XML grammar closed while permitting
// CMocka's diagnostic CDATA. Generic JUnit decoding loses fixture-error counts.
type cmockaNode struct {
	name     string
	attrs    map[string]string
	text     string
	children []*cmockaNode
}

func cmockaXML(data []byte) (*cmockaNode, error) {
	if len(data) > tr.MaxReportBytes {
		return nil, fmt.Errorf("cmocka XML exceeds bound")
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	var root *cmockaNode
	stack := []*cmockaNode{}
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
			if t.Name.Space != "" || nodes > 260 || len(stack) >= 4 {
				return nil, fmt.Errorf("cmocka XML structure outside profile")
			}
			n := &cmockaNode{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				if a.Name.Space != "" {
					return nil, fmt.Errorf("cmocka XML namespace unsupported")
				}
				if _, ok := n.attrs[a.Name.Local]; ok {
					return nil, fmt.Errorf("duplicate cmocka XML attribute")
				}
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("multiple cmocka XML roots")
				}
				root = n
			} else {
				p := stack[len(stack)-1]
				p.children = append(p.children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("unbalanced cmocka XML")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if len(bytes.TrimSpace(t)) != 0 {
					return nil, fmt.Errorf("trailing cmocka XML text")
				}
			} else {
				stack[len(stack)-1].text += string(t)
			}
		case xml.ProcInst:
			if root != nil || declaration || t.Target != "xml" {
				return nil, fmt.Errorf("cmocka XML instruction unsupported")
			}
			declaration = true
		default:
			return nil, fmt.Errorf("cmocka XML directive/comment unsupported")
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("missing cmocka XML root")
	}
	return root, nil
}
func cmockaShape(n *cmockaNode, name string, attrs ...string) bool {
	if n.name != name || len(n.attrs) != len(attrs) {
		return false
	}
	for _, a := range attrs {
		if _, ok := n.attrs[a]; !ok {
			return false
		}
	}
	return true
}
func cmockaCount(s string) (int, error) {
	n, e := strconv.Atoi(s)
	if e != nil || n < 0 || n > 64 || strconv.Itoa(n) != s {
		return 0, fmt.Errorf("cmocka counter outside0..64")
	}
	return n, nil
}
func parseCMocka(in tr.Input) (tr.Observation, error) {
	o := tr.Observation{Runner: cmockaRunner, RetryInformation: tr.NotReported}
	problem := func(code, detail string) { o.Problems = append(o.Problems, tr.Problem{Code: code, Detail: detail}) }
	reject := func(err error) (tr.Observation, error) { problem("CMOCKA_INVALID_REPORT", err.Error()); return o, err }
	if err := cmockaInventory(in.Target, in.Expected, in.Selectors); err != nil {
		return reject(err)
	}
	data, ok := in.Reports["cmocka.xml"]
	if !ok || len(in.Reports) != 1 {
		return reject(fmt.Errorf("exact cmocka.xml report required"))
	}
	root, err := cmockaXML(data)
	if err != nil {
		return reject(err)
	}
	if !cmockaShape(root, "testsuites") || strings.TrimSpace(root.text) != "" || len(root.children) != 1 {
		return reject(fmt.Errorf("exactly one cmocka suite required"))
	}
	suite := root.children[0]
	if !cmockaShape(suite, "testsuite", "name", "time", "tests", "failures", "errors", "skipped") || strings.TrimSpace(suite.text) != "" || !cmockaTime.MatchString(suite.attrs["time"]) {
		return reject(fmt.Errorf("cmocka suite grammar mismatch"))
	}
	if suite.attrs["name"] != in.Target {
		return reject(fmt.Errorf("cmocka native group disagrees with caller target"))
	}
	counts := map[string]int{}
	for _, key := range []string{"tests", "failures", "errors", "skipped"} {
		n, e := cmockaCount(suite.attrs[key])
		if e != nil {
			return reject(e)
		}
		counts[key] = n
	}
	seen := map[string]bool{}
	expected := map[string]bool{}
	for _, id := range in.Expected {
		expected[id] = true
	}
	failed, skipped := 0, 0
	nativeMessages := map[string]string{}
	for _, c := range suite.children {
		if !cmockaShape(c, "testcase", "name", "time") || !cmockaTime.MatchString(c.attrs["time"]) || strings.TrimSpace(c.text) != "" || !cmockaName.MatchString(c.attrs["name"]) || len(c.attrs["name"]) > 128 || len(c.children) > 1 {
			return reject(fmt.Errorf("cmocka case grammar mismatch"))
		}
		id := in.Target + "::" + c.attrs["name"]
		if seen[id] {
			return reject(fmt.Errorf("duplicate cmocka case"))
		}
		seen[id] = true
		v := tr.Test{ID: id, Name: c.attrs["name"], Suite: in.Target, State: tr.Passed}
		if len(c.children) == 1 {
			child := c.children[0]
			if len(child.children) > 0 {
				return reject(fmt.Errorf("nested cmocka outcome"))
			}
			switch child.name {
			case "skipped":
				if !cmockaShape(child, "skipped") || strings.TrimSpace(child.text) != "" {
					return reject(fmt.Errorf("cmocka skipped grammar mismatch"))
				}
				v.State = tr.Skipped
				skipped++
			case "failure":
				if !cmockaShape(child, "failure") && !cmockaShape(child, "failure", "message") {
					return reject(fmt.Errorf("cmocka failure grammar mismatch"))
				}
				v.State = tr.Failed
				v.FailureKind = tr.Unknown
				failed++
				if len(child.attrs) == 1 {
					if child.attrs["message"] != "Unknown error" || strings.TrimSpace(child.text) != "" {
						return reject(fmt.Errorf("cmocka failure message mismatch"))
					}
					v.Message = child.attrs["message"]
				} else {
					v.Message = child.text
					nativeMessages[id] = child.text
				}
			default:
				return reject(fmt.Errorf("unknown cmocka case outcome"))
			}
		}
		if !expected[id] {
			problem("CMOCKA_SURPLUS_TEST", "native identity outside expected inventory")
		}
		o.Tests = append(o.Tests, v)
	}
	if len(o.Tests) != counts["tests"] || failed != counts["failures"]+counts["errors"] || skipped != counts["skipped"] || failed+skipped > len(o.Tests) {
		problem("CMOCKA_COUNTER_MISMATCH", "native counters disagree with case outcomes")
	}
	if counts["errors"] > 0 {
		problem("CMOCKA_FIXTURE_ERROR", "native errors counter cannot identify ordinary assertion failures")
	}
	if len(o.Tests) == 0 {
		problem("CMOCKA_NO_TESTS", "native suite has no observed tests")
	}
	for id := range expected {
		if !seen[id] {
			problem("CMOCKA_MISSING_TEST", "expected native identity absent")
		}
	}
	if in.ExitCode != counts["failures"]+counts["errors"] {
		problem("CMOCKA_EXIT_MISMATCH", "native exit disagrees with failed/error count")
	}
	// Native STANDARD output exposes group teardown errors omitted from XML and
	// the exit count. Require both original witnesses; unknown output abstains.
	var stdout, stderr strings.Builder
	fmt.Fprintf(&stdout, "[==========] %s: Running %d test(s).\n", in.Target, len(o.Tests))
	for _, v := range o.Tests {
		fmt.Fprintf(&stdout, "[ RUN      ] %s\n", v.Name)
		label := "[       OK ]"
		if v.State == tr.Failed {
			label = "[  FAILED  ]"
		}
		if v.State == tr.Skipped {
			label = "[  SKIPPED ]"
		}
		fmt.Fprintf(&stdout, "%s %s\n", label, v.Name)
		if msg, ok := nativeMessages[v.ID]; ok {
			fmt.Fprintf(&stderr, "[  ERROR   ] --- %s\n", msg)
		}
	}
	fmt.Fprintf(&stdout, "[==========] %s: %d test(s) run.\n", in.Target, len(o.Tests))
	fmt.Fprintf(&stderr, "[  PASSED  ] %d test(s).\n", len(o.Tests)-failed-skipped)
	for _, state := range []string{tr.Skipped, tr.Failed} {
		count, label, terminal := skipped, "[  SKIPPED ]", "SKIPPED"
		if state == tr.Failed {
			count, label, terminal = failed, "[  FAILED  ]", "FAILED"
		}
		if count > 0 {
			fmt.Fprintf(&stderr, "%s %s: %d test(s), listed below:\n", label, in.Target, count)
			for _, v := range o.Tests {
				if v.State == state {
					fmt.Fprintf(&stderr, "%s %s\n", label, v.Name)
				}
			}
			fmt.Fprintf(&stderr, "\n %d %s TEST(S)\n", count, terminal)
		}
	}
	if string(in.Stdout) != stdout.String() || string(in.Stderr) != stderr.String() {
		problem("CMOCKA_STANDARD_MISMATCH", "native STANDARD and XML witnesses conflict or contain unsupported diagnostics")
	}
	if bytes.Contains(in.Stderr, []byte("[  FAILED  ] GROUP SETUP")) || bytes.Contains(in.Stderr, []byte("[  FAILED  ] GROUP TEARDOWN")) {
		problem("CMOCKA_GROUP_FIXTURE_ERROR", "native group fixture diagnostic retained")
	}
	o.Complete = len(o.Problems) == 0
	sort.Slice(o.Tests, func(i, j int) bool { return o.Tests[i].ID < o.Tests[j].ID })
	return o, nil
}
