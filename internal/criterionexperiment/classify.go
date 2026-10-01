package criterionexperiment

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Classify is deliberately conservative: only a complete named test run and
// one package terminal event can yield pass or expected-failure.
func Classify(raw []byte, exit int, complete bool, test, assertion, expectedPackage string) string {
	if !utf8.Valid(raw) || !complete || len(raw) == 0 || len(raw) > 1<<20 || raw[len(raw)-1] != '\n' {
		return "infrastructure-failed"
	}
	run, terminal, pkgTerminal, starts := 0, 0, 0, 0
	targetStatus, pkgStatus, pkg := "", "", ""
	marked := false
	wrongDiagnostic := false
	diagnosticCount := 0
	bad := false
	for _, line := range bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n")) {
		var e struct {
			Time        string
			Action      string
			Package     string
			Test        string
			Elapsed     float64
			Output      string
			OutputType  string
			FailedBuild string
		}
		if !eventKeys(line) {
			return "infrastructure-failed"
		}
		d := json.NewDecoder(bytes.NewReader(line))
		d.DisallowUnknownFields()
		if d.Decode(&e) != nil {
			return "infrastructure-failed"
		}
		if pkgTerminal > 0 {
			bad = true
		}
		if e.Test != "" && e.Test != test {
			bad = true
		}
		if e.Package == "" || e.Package != expectedPackage || e.FailedBuild != "" {
			return "infrastructure-failed"
		}
		if pkg == "" {
			pkg = e.Package
		}
		if pkg != e.Package {
			return "infrastructure-failed"
		}
		if strings.Contains(e.Output, "panic:") || strings.Contains(e.Output, "fatal error:") || strings.Contains(e.Output, "[build failed]") {
			bad = true
		}
		switch e.Action {
		case "start":
			starts++
			if e.Test != "" || run > 0 || pkgTerminal > 0 {
				bad = true
			}
		case "run":
			if e.Test != test || terminal > 0 {
				bad = true
			} else {
				run++
			}
		case "output":
			if e.OutputType == "error" {
				diagnosticCount++
				match := diagnosticLine.FindStringSubmatch(e.Output)
				if e.Test != test || run != 1 || terminal != 0 || len(match) != 2 {
					wrongDiagnostic = true
					break
				}
				message := match[1]
				if message == assertion || strings.HasPrefix(message, assertion+": ") {
					marked = true
				} else {
					wrongDiagnostic = true
				}
			}
		case "pass", "fail":
			if e.Test == "" {
				if terminal != 1 {
					bad = true
				}
				pkgTerminal++
				pkgStatus = e.Action
			} else if e.Test == test {
				terminal++
				targetStatus = e.Action
				if run != 1 {
					bad = true
				}
			} else {
				bad = true
			}
		default:
			bad = true // skips, benchmark, pause/cont and subtest events unsupported
		}
	}
	if bad || starts != 1 || run != 1 || terminal != 1 || pkgTerminal != 1 {
		return "invalid-control"
	}
	if exit == 0 && targetStatus == "pass" && pkgStatus == "pass" && diagnosticCount == 0 {
		return "pass"
	}
	if exit == 1 && targetStatus == "fail" && pkgStatus == "fail" && marked && !wrongDiagnostic && diagnosticCount == 1 {
		return "expected-failure"
	}
	return "invalid-control"
}

// Go event elapsed values are decimals, so use JSON tokens for duplicate and
// exact-key checks rather than the integer-only CEM document parser.
func eventKeys(line []byte) bool {
	d := json.NewDecoder(bytes.NewReader(line))
	tok, e := d.Token()
	if e != nil || tok != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	allowed := map[string]bool{"Time": true, "Action": true, "Package": true, "Test": true, "Elapsed": true, "Output": true, "FailedBuild": true, "OutputType": true}
	for d.More() {
		tok, e = d.Token()
		if e != nil {
			return false
		}
		key, ok := tok.(string)
		if !ok || seen[key] || !allowed[key] {
			return false
		}
		seen[key] = true
		var value any
		if d.Decode(&value) != nil {
			return false
		}
	}
	tok, e = d.Token()
	if e != nil || tok != json.Delim('}') {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}

// Source paths in this profile exclude colons; the first source delimiter must
// end the prefix so a location embedded in the assertion remains message text.
var diagnosticLine = regexp.MustCompile(`^[ \t]+[^:\r\n]+\.go:[0-9]+: ([^\r\n]*)\n$`)
