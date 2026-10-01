package dynamic

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"io"
	"strings"
	"unicode"
)

// Reject ambiguous objects before encoding/json applies last-key/case-fold wins.
func uniqueJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 128 {
			return fmt.Errorf("JSON depth bound exceeded")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				s, ok := k.(string)
				if !ok {
					return fmt.Errorf("invalid JSON key")
				}
				s = strings.Map(func(r rune) rune {
					min := r
					for q := unicode.SimpleFold(r); q != r; q = unicode.SimpleFold(q) {
						if q < min {
							min = q
						}
					}
					return min
				}, s)
				if seen[s] {
					return fmt.Errorf("duplicate or case-aliased JSON key %q", s)
				}
				seen[s] = true
				if e = value(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e := value(depth + 1); e != nil {
					return e
				}
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
		_, e = d.Token()
		return e
	}
	if e := value(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

// The admitted JUnit dialects share this closed structure. Text diagnostics are
// allowed only inside their native containers, never as unrecognized outcomes.
func closedXML(b []byte) error {
	d := xml.NewDecoder(bytes.NewReader(b))
	stack := []string{}
	roots := 0
	children := map[string]string{"": " testsuite testsuites ", "testsuites": " testsuite properties ", "testsuite": " testsuite testcase properties system-out system-err ", "testcase": " failure error skipped properties system-out system-err ", "properties": " property "}
	attrs := map[string]string{
		"testsuites": " name tests failures errors skipped disabled assertions time ",
		"testsuite":  " name tests failures errors skipped disabled assertions time timestamp hostname file id package ",
		"testcase":   " name classname time file line col assertions ",
		"failure":    " message type ", "error": " message type ", "skipped": " message type ", "property": " name value ",
	}
	for {
		t, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		switch x := t.(type) {
		case xml.StartElement:
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			} else {
				roots++
			}
			if roots > 1 || len(stack) > 128 || x.Name.Space != "" || !strings.Contains(children[parent], " "+x.Name.Local+" ") {
				return fmt.Errorf("unsupported XML element %s/%s", parent, x.Name.Local)
			}
			seen := map[string]bool{}
			for _, a := range x.Attr {
				if a.Name.Space != "" || seen[a.Name.Local] || !strings.Contains(attrs[x.Name.Local], " "+a.Name.Local+" ") {
					return fmt.Errorf("unsupported or duplicate XML attribute %s", a.Name.Local)
				}
				seen[a.Name.Local] = true
			}
			stack = append(stack, x.Name.Local)
		case xml.EndElement:
			if len(stack) == 0 {
				return fmt.Errorf("invalid XML end")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if strings.TrimSpace(string(x)) != "" {
				if len(stack) == 0 {
					return fmt.Errorf("text outside XML root")
				}
				switch stack[len(stack)-1] {
				case "failure", "error", "skipped", "system-out", "system-err", "property":
				default:
					return fmt.Errorf("unexpected XML text")
				}
			}
		case xml.Directive:
			return fmt.Errorf("XML directives unsupported")
		case xml.ProcInst:
			if x.Target != "xml" || roots != 0 {
				return fmt.Errorf("unsupported XML processing instruction")
			}
		}
	}
	if roots != 1 || len(stack) != 0 {
		return fmt.Errorf("incomplete XML document")
	}
	return nil
}

func validateAttempts(t tr.Test, o *tr.Observation) {
	if len(t.Attempts) == 0 {
		problem(o, "missing-attempt", t.ID)
		return
	}
	last := t.Attempts[len(t.Attempts)-1].State
	if (t.State == tr.Flaky && (last != tr.Passed || len(t.Attempts) < 2)) || (t.State != tr.Flaky && t.State != last) {
		problem(o, "attempt-outcome-conflict", t.ID)
	}
	for _, a := range t.Attempts[:len(t.Attempts)-1] {
		if a.State != tr.Failed && a.State != tr.TimedOut {
			problem(o, "invalid-prior-attempt", t.ID)
		}
	}
}

func nativeError(b json.RawMessage) bool {
	s := strings.TrimSpace(string(b))
	return s != "" && s != "null" && s != "{}"
}
