package platform

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

type xmlNode struct {
	name     string
	attr     map[string]string
	text     string
	children []*xmlNode
}

func readXML(b []byte) (*xmlNode, error) { return readXMLWithSchema(b, false) }

func readXMLWithSchema(b []byte, surefire bool) (*xmlNode, error) {
	if len(b) == 0 || len(b) > tr.MaxReportBytes {
		return nil, fmt.Errorf("missing or oversized XML")
	}
	d := xml.NewDecoder(bytes.NewReader(b))
	var root *xmlNode
	stack := []*xmlNode{}
	nodes := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			nodes++
			if len(stack) >= 32 || nodes > 100000 || t.Name.Space != "" {
				return nil, fmt.Errorf("unsupported XML namespace or bounds")
			}
			n := &xmlNode{name: t.Name.Local, attr: map[string]string{}}
			seenAttributes := map[xml.Name]bool{}
			for _, a := range t.Attr {
				if seenAttributes[a.Name] {
					return nil, fmt.Errorf("duplicate XML attribute")
				}
				seenAttributes[a.Name] = true
				if surefire && len(stack) == 0 && t.Name.Local == "testsuite" {
					if a.Name.Space == "xmlns" && a.Name.Local == "xsi" && a.Value == "http://www.w3.org/2001/XMLSchema-instance" {
						continue
					}
					if a.Name.Space == "http://www.w3.org/2001/XMLSchema-instance" && a.Name.Local == "noNamespaceSchemaLocation" && a.Value == "https://maven.apache.org/surefire/maven-surefire-plugin/xsd/surefire-test-report.xsd" {
						continue
					}
				}
				if a.Name.Space != "" {
					return nil, fmt.Errorf("unsupported XML attribute namespace")
				}
				if _, ok := n.attr[a.Name.Local]; ok {
					return nil, fmt.Errorf("duplicate XML attribute")
				}
				n.attr[a.Name.Local] = a.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("multiple XML documents")
				}
				root = n
			} else {
				p := stack[len(stack)-1]
				p.children = append(p.children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(t)) != "" {
					return nil, fmt.Errorf("content outside XML root")
				}
			} else {
				stack[len(stack)-1].text += string(t)
			}
		case xml.Directive:
			return nil, fmt.Errorf("XML directives are unsupported")
		case xml.ProcInst:
			if t.Target != "xml" || root != nil {
				return nil, fmt.Errorf("unsupported XML processing instruction")
			}
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("incomplete XML")
	}
	return root, nil
}

func count(n *xmlNode, key string) (int, error) {
	s, ok := n.attr[key]
	if !ok || s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, fmt.Errorf("missing or noncanonical %s count", key)
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 || v > tr.MaxTests*tr.MaxAttempts {
		return 0, fmt.Errorf("invalid %s count", key)
	}
	return v, nil
}

func child(n *xmlNode, name string) (*xmlNode, error) {
	var found *xmlNode
	for _, c := range n.children {
		if c.name == name {
			if found != nil {
				return nil, fmt.Errorf("duplicate %s element", name)
			}
			found = c
		}
	}
	return found, nil
}

func xmlReport(in tr.Input, name string) (*xmlNode, error) {
	b, ok := in.Reports[name]
	if !ok || len(in.Reports) != 1 {
		return nil, fmt.Errorf("expected exactly native report %s", name)
	}
	return readXML(b)
}

func parseJUnit(in tr.Input, o *tr.Observation) error {
	engine, title := "junit-jupiter", "JUnit Jupiter"
	if in.Runner == "junit4" || in.Runner == "robolectric" {
		engine, title = "junit-vintage", "JUnit Vintage"
	} else if in.Runner == "kotest" {
		engine, title = "kotest", "Kotest"
	}
	n, err := xmlReport(in, "TEST-"+engine+".xml")
	if err != nil {
		return err
	}
	if n.name != "testsuite" || n.attr["name"] != title {
		return fmt.Errorf("report is not the selected JUnit Platform engine")
	}
	total, err := count(n, "tests")
	if err != nil {
		return err
	}
	wanted := map[string]int{}
	for _, key := range []string{"skipped", "failures", "errors"} {
		wanted[key], err = count(n, key)
		if err != nil {
			return err
		}
	}
	actual := map[string]int{}
	for _, c := range n.children {
		switch c.name {
		case "properties", "system-out", "system-err":
			continue
		case "testcase":
		default:
			return fmt.Errorf("unsupported JUnit suite event %s", c.name)
		}
		name, class := c.attr["name"], c.attr["classname"]
		if name == "" || class == "" {
			return fmt.Errorf("JUnit testcase identity missing")
		}
		state, kind, message, terminal := tr.Passed, "", "", 0
		for _, x := range c.children {
			switch x.name {
			case "failure", "error", "skipped":
				terminal++
				message = x.attr["message"]
				if message == "" {
					message = strings.TrimSpace(x.text)
				}
				switch x.name {
				case "failure":
					state, kind = tr.Failed, tr.Assertion
					actual["failures"]++
				case "error":
					state, kind = tr.Failed, tr.Infrastructure
					actual["errors"]++
				case "skipped":
					state = tr.Skipped
					actual["skipped"]++
				}
			case "system-out", "system-err":
			default:
				return fmt.Errorf("unsupported JUnit testcase event %s", x.name)
			}
		}
		if terminal > 1 {
			return fmt.Errorf("contradictory JUnit outcomes")
		}
		out, err := child(c, "system-out")
		if err != nil || out == nil {
			return fmt.Errorf("JUnit Platform unique identity missing")
		}
		id := ""
		for _, line := range strings.Split(out.text, "\n") {
			if strings.HasPrefix(line, "unique-id: ") {
				if id != "" {
					return fmt.Errorf("ambiguous native unique ID")
				}
				id = strings.TrimPrefix(line, "unique-id: ")
			}
		}
		if !strings.HasPrefix(id, "[engine:"+engine+"]/") {
			return fmt.Errorf("wrong engine or missing JUnit unique ID")
		}
		o.Tests = append(o.Tests, test(id, name, "", class, state, kind, message))
	}
	if total != len(o.Tests) {
		return fmt.Errorf("JUnit testcase count mismatch")
	}
	for k, v := range wanted {
		if actual[k] != v {
			return fmt.Errorf("JUnit %s count mismatch", k)
		}
	}
	failed := actual["failures"]+actual["errors"] > 0
	if (failed && in.ExitCode != 1) || (!failed && in.ExitCode != 0) {
		problem(o, "EXIT_CONTRADICTION", "JUnit report and actual exit disagree")
	}
	o.Complete = len(o.Problems) == 0
	return nil
}

func parseTestNG(in tr.Input, o *tr.Observation) error {
	n, err := xmlReport(in, "testng-results.xml")
	if err != nil {
		return err
	}
	if n.name != "testng-results" {
		return fmt.Errorf("not a TestNG native report")
	}
	counts := map[string]int{}
	for _, key := range []string{"total", "passed", "failed", "skipped", "ignored"} {
		counts[key], err = count(n, key)
		if err != nil {
			return err
		}
	}
	actual := map[string]int{}
	for _, suite := range n.children {
		if suite.name == "reporter-output" {
			continue
		}
		if suite.name != "suite" || suite.attr["name"] == "" || suite.attr["finished-at"] == "" {
			return fmt.Errorf("incomplete TestNG suite")
		}
		for _, group := range suite.children {
			if group.name == "groups" {
				continue
			}
			if group.name != "test" || group.attr["name"] == "" || group.attr["finished-at"] == "" {
				return fmt.Errorf("incomplete TestNG test group")
			}
			for _, class := range group.children {
				if class.name != "class" || class.attr["name"] == "" {
					return fmt.Errorf("invalid TestNG class")
				}
				for _, m := range class.children {
					if m.name != "test-method" || m.attr["name"] == "" || m.attr["finished-at"] == "" {
						return fmt.Errorf("incomplete TestNG method")
					}
					if m.attr["is-config"] == "true" {
						if m.attr["status"] != "PASS" {
							problem(o, "CONFIGURATION_FAILED", "a TestNG configuration method did not pass")
						}
						continue
					}
					if m.attr["retried"] != "" || m.attr["is-retried"] != "" {
						return fmt.Errorf("TestNG retry record requires qualified invocation identity")
					}
					ex, err := child(m, "exception")
					if err != nil {
						return err
					}
					state, kind, message := tr.Unknown, "", ""
					if ex != nil {
						msg, err := child(ex, "message")
						if err != nil {
							return err
						}
						if msg != nil {
							message = strings.TrimSpace(msg.text)
						}
					}
					switch m.attr["status"] {
					case "PASS":
						if ex != nil {
							return fmt.Errorf("TestNG pass carries exception")
						}
						state = tr.Passed
						actual["passed"]++
					case "FAIL":
						if ex == nil || ex.attr["class"] == "" {
							return fmt.Errorf("TestNG failure missing exception")
						}
						state, kind = tr.Failed, tr.Infrastructure
						if ex.attr["class"] == "java.lang.AssertionError" || ex.attr["class"] == "org.opentest4j.AssertionFailedError" {
							kind = tr.Assertion
						}
						actual["failed"]++
					case "SKIP":
						state = tr.Skipped
						actual["skipped"]++
					default:
						return fmt.Errorf("unknown TestNG status")
					}
					for _, c := range m.children {
						if c.name != "exception" && c.name != "reporter-output" {
							return fmt.Errorf("TestNG parameterized/extended method is not qualified")
						}
					}
					id := suite.attr["name"] + "/" + group.attr["name"] + "/" + class.attr["name"] + "#" + m.attr["name"]
					o.Tests = append(o.Tests, test(id, m.attr["name"], "", class.attr["name"], state, kind, message))
				}
			}
		}
	}
	if counts["ignored"] != 0 {
		problem(o, "IGNORED_TESTS", "TestNG ignored tests lack individual reported identities")
	}
	if len(o.Tests) != counts["total"] {
		return fmt.Errorf("TestNG total count mismatch")
	}
	for _, key := range []string{"passed", "failed", "skipped"} {
		if actual[key] != counts[key] {
			return fmt.Errorf("TestNG %s count mismatch", key)
		}
	}
	// TestNG uses a documented bitmask: failures=1, skips=2.
	wantExit := 0
	if actual["failed"] != 0 {
		wantExit |= 1
	}
	if actual["skipped"] != 0 {
		wantExit |= 2
	}
	if in.ExitCode != wantExit {
		problem(o, "EXIT_CONTRADICTION", "TestNG exit bitmask and report disagree")
	}
	o.Complete = len(o.Problems) == 0
	return nil
}
