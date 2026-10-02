package native

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

type trxError struct {
	Message    string `xml:"Message"`
	StackTrace string `xml:"StackTrace"`
}

// This is the qualified TRX subset, not a permissive XML transport. Unknown
// structure and namespace aliases must not disappear during Go XML projection.
func checkNativeTRXXML(b []byte) error {
	type rule struct {
		attrs, children string
		repeat, text    bool
	}
	grammar := map[string]rule{
		"TestRun":        {"id name", "Times TestSettings Results TestDefinitions TestEntries TestLists ResultSummary", false, false},
		"Times":          {"creation queuing start finish", "", false, false},
		"TestSettings":   {"id name", "Deployment", false, false},
		"Deployment":     {"runDeploymentRoot", "", false, false},
		"Results":        {"", "UnitTestResult", true, false},
		"UnitTestResult": {"executionId testId testName computerName duration startTime endTime testType outcome testListId relativeResultsDirectory", "Output", false, false},
		"Output":         {"", "ErrorInfo StdOut StdErr", false, false},
		"ErrorInfo":      {"", "Message StackTrace", false, false},
		"Message":        {"", "", false, true}, "StackTrace": {"", "", false, true}, "StdOut": {"", "", false, true}, "StdErr": {"", "", false, true}, "Text": {"", "", false, true},
		"TestDefinitions": {"", "UnitTest", true, false},
		"UnitTest":        {"id name storage", "Execution TestMethod", false, false},
		"Execution":       {"id", "", false, false},
		"TestMethod":      {"codeBase adapterTypeName className name", "", false, false},
		"TestEntries":     {"", "TestEntry", true, false},
		"TestEntry":       {"testId executionId testListId", "", false, false},
		"TestLists":       {"", "TestList", true, false},
		"TestList":        {"name id", "", false, false},
		"ResultSummary":   {"outcome", "Counters RunInfos Output", false, false},
		"Counters":        {"total executed passed failed error timeout aborted inconclusive passedButRunAborted notRunnable notExecuted disconnected warning completed inProgress pending", "", false, false},
		"RunInfos":        {"", "RunInfo", true, false},
		"RunInfo":         {"computerName outcome timestamp", "Text", false, false},
	}
	contains := func(words, word string) bool {
		for _, v := range strings.Fields(words) {
			if v == word {
				return true
			}
		}
		return false
	}
	type frame struct {
		name     string
		children map[string]bool
	}
	var stack []frame
	roots := 0
	decoder := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})))
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			if roots != 1 {
				return errors.New("native TRX root inventory")
			}
			return nil
		}
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.Directive:
			return errors.New("native TRX XML directives unsupported")
		case xml.StartElement:
			if v.Name.Space != "http://microsoft.com/schemas/VisualStudio/TeamTest/2010" {
				return errors.New("native TRX element namespace mismatch")
			}
			admitted, ok := grammar[v.Name.Local]
			if !ok {
				return errors.New("unknown native TRX element")
			}
			attrs := map[string]bool{}
			for _, a := range v.Attr {
				// The captured documents use one default namespace declaration on TestRun.
				if a.Name.Space == "" && a.Name.Local == "xmlns" && len(stack) == 0 && a.Value == v.Name.Space {
					continue
				}
				if a.Name.Space != "" || !contains(admitted.attrs, a.Name.Local) || attrs[a.Name.Local] {
					return errors.New("unknown, namespaced or duplicate native TRX attribute")
				}
				attrs[a.Name.Local] = true
			}
			if len(stack) == 0 {
				roots++
				if roots != 1 || v.Name.Local != "TestRun" {
					return errors.New("native TRX root inventory")
				}
			} else {
				p := &stack[len(stack)-1]
				parent := grammar[p.name]
				if !contains(parent.children, v.Name.Local) {
					return errors.New("unknown native TRX child element")
				}
				if !parent.repeat && p.children[v.Name.Local] {
					return errors.New("duplicate native TRX singleton element")
				}
				p.children[v.Name.Local] = true
			}
			stack = append(stack, frame{v.Name.Local, map[string]bool{}})
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(bytes.TrimSpace(v)) > 0 && (len(stack) == 0 || !grammar[stack[len(stack)-1].name].text) {
				return errors.New("unexpected native TRX structural text")
			}
		}
	}
}
