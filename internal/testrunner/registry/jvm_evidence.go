// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"encoding/xml"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov/v8"
	"github.com/Beamfall/corvint/internal/nativelinecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// JVMCapture holds only core-captured artifacts for an attributed test tree.
// Key binds artifacts to the checkout/configuration/target; Tests is the full
// tree attribution set. The host must use the same union for every test when
// there are no qualified in-process test markers. Process is the existing
// native collector lifecycle observation, including every descendant/image.
// JaCoCo exec data requires the existing classfile-bound XML export before
// decoding; raw probes alone cannot resolve source lines.
type JVMCapture struct {
	Key          tr.InventoryKey
	Tests        []string
	TreeComplete bool
	Processes    []nativelinecov.Process
	// Regions comes from the source dialect at the captured blob, never from
	// JaCoCo hits or method start lines. An unavailable dialect stays absent.
	Regions testevidence.Component[testevidence.RegionsFile]
}

func jvmMissing(reason string) testevidence.Native {
	return testevidence.Native{
		Executed: testevidence.Missing[testevidence.ExecutedFile](reason),
		Regions:  testevidence.Missing[testevidence.RegionsFile](reason),
		Loaded:   testevidence.Missing[testevidence.Blob](reason),
	}
}

func jvmEvidence(s JVMSourceSet, c JVMCapture) testevidence.Native {
	n := jvmMissing("jvm-native-not-collected")
	n.Loaded = testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: append([]testevidence.Blob{}, s.Classpath...)}
	sort.Slice(n.Loaded.Items, func(i, j int) bool {
		a, b := n.Loaded.Items[i], n.Loaded.Items[j]
		if a.Path.Repository != b.Path.Repository {
			return a.Path.Repository < b.Path.Repository
		}
		return a.Path.Lexical < b.Path.Lexical
	})
	if c.Regions.State == "" {
		n.Regions = testevidence.Missing[testevidence.RegionsFile]("jvm-region-dialect-unavailable")
	} else {
		n.Regions = c.Regions
		check := testevidence.New("validation", "validation", "")
		check.Regions = n.Regions
		valid := check.ValidateComponents() == nil
		for _, region := range n.Regions.Items {
			found := false
			for _, source := range s.Sources {
				found = found || source == region.Blob
			}
			valid = valid && found
		}
		if !valid {
			n.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Incomplete, Reason: "jvm-regions-malformed-or-unbound"}
		}
	}
	fail := func(reason string) testevidence.Native {
		n.Executed = testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Incomplete, Reason: reason}
		return n
	}
	if !c.TreeComplete || len(c.Processes) == 0 || len(c.Processes) > tr.MaxTests {
		return fail("jvm-process-tree-incomplete-or-capped")
	}
	processes := map[int]nativelinecov.Process{}
	roots := 0
	for _, process := range c.Processes {
		if process.ID <= 0 || process.Parent < 0 || process.ID == process.Parent || processes[process.ID].ID != 0 || !process.Exited || !process.Instrumented || len(process.Reports) == 0 || len(process.Reports) > tr.MaxReports {
			return fail("jvm-process-coverage-incomplete")
		}
		if process.Parent == 0 {
			roots++
		}
		processes[process.ID] = process
	}
	if roots != 1 {
		return fail("jvm-process-root-divergent")
	}
	for _, process := range c.Processes {
		seen := map[int]bool{}
		for process.Parent != 0 {
			if seen[process.ID] || processes[process.Parent].ID == 0 {
				return fail("jvm-process-parent-unresolved")
			}
			seen[process.ID] = true
			process = processes[process.Parent]
		}
	}
	mapping := make(map[string]string, len(s.Sources))
	byPath := map[string]testevidence.Blob{}
	for name, blob := range s.Sources {
		if old, exists := byPath[blob.Path.Lexical]; exists && old != blob {
			return fail("jvm-source-mapping-divergent")
		}
		mapping[name], byPath[blob.Path.Lexical] = blob.Path.Lexical, blob
	}
	lines := v8.Lines{}
	bytes := 0
	for _, process := range c.Processes {
		for _, report := range process.Reports {
			bytes += len(report.Data)
			if bytes > testevidence.MaxRecordBytes {
				return fail("jvm-coverage-cap")
			}
			if len(report.Data) > nativelinecov.MaxReportBytes {
				return fail("jvm-coverage-report-cap")
			}
			if report.Format != "jacoco" {
				return fail("jvm-jacoco-xml-export-required")
			}
			if !jvmJaCoCoSchema(report.Data) {
				return fail("jvm-jacoco-unknown-or-malformed-xml")
			}
			decoded, err := nativelinecov.Decode(report, mapping)
			if err != nil {
				return fail("jvm-jacoco-malformed-or-unmapped")
			}
			lines.Merge(decoded)
		}
	}
	paths := make([]string, 0, len(lines))
	for name := range lines {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	n.Executed = testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present, Items: []testevidence.ExecutedFile{}}
	for _, name := range paths {
		file := testevidence.ExecutedFile{Blob: byPath[name], Executed: []testevidence.Range{}, Unexecuted: []testevidence.Range{}}
		lineNumbers := make([]int, 0, len(lines[name]))
		for line := range lines[name] {
			lineNumbers = append(lineNumbers, line)
		}
		sort.Ints(lineNumbers)
		for _, line := range lineNumbers {
			span := testevidence.Range{Start: int32(line), StartColumn: 1, End: int32(line + 1), EndColumn: 1}
			if lines[name][line] {
				file.Executed = append(file.Executed, span)
			} else {
				file.Unexecuted = append(file.Unexecuted, span)
			}
		}
		n.Executed.Items = append(n.Executed.Items, file)
	}
	return n
}

// The shared legacy line decoder intentionally ignores non-line elements. The
// selection piece first checks the closed native JaCoCo vocabulary so a new or
// unknown XML construct cannot silently lose coverage edges.
func jvmJaCoCoSchema(raw []byte) bool {
	if len(raw) == 0 || len(raw) > nativelinecov.MaxReportBytes {
		return false
	}
	attributes := map[string]string{
		"report": " name ", "sessioninfo": " id start dump ", "group": " name ",
		"package": " name ", "class": " name sourcefilename ", "method": " name desc line ",
		"sourcefile": " name ", "line": " nr mi ci mb cb ", "counter": " type missed covered ",
	}
	parents := map[string]string{
		"report": "  ", "sessioninfo": " report ", "group": " report group ",
		"package": " report group ", "class": " package ", "method": " class ",
		"sourcefile": " package ", "line": " sourcefile ", "counter": " report group package class method sourcefile ",
	}
	required := map[string]string{
		"report": "name", "sessioninfo": "id start dump", "group": "name", "package": "name", "class": "name",
		"method": "name desc", "sourcefile": "name", "line": "nr mi ci mb cb", "counter": "type missed covered",
	}
	d := xml.NewDecoder(bytes.NewReader(raw))
	stack := []string{}
	nodes, roots := 0, 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			return roots == 1 && len(stack) == 0
		}
		if err != nil {
			return false
		}
		switch t := token.(type) {
		case xml.StartElement:
			nodes++
			parent := ""
			if len(stack) != 0 {
				parent = stack[len(stack)-1]
			} else {
				roots++
			}
			if roots != 1 || nodes > 100000 || len(stack) >= 32 || t.Name.Space != "" || attributes[t.Name.Local] == "" || !strings.Contains(parents[t.Name.Local], " "+parent+" ") {
				return false
			}
			seen := map[string]bool{}
			for _, a := range t.Attr {
				if a.Name.Space != "" || seen[a.Name.Local] || !strings.Contains(attributes[t.Name.Local], " "+a.Name.Local+" ") {
					return false
				}
				seen[a.Name.Local] = true
				if strings.Contains(" nr mi ci mb cb missed covered start dump ", " "+a.Name.Local+" ") || t.Name.Local == "method" && a.Name.Local == "line" {
					n, err := strconv.ParseUint(a.Value, 10, 63)
					if err != nil || strings.HasPrefix(a.Value, "+") || (a.Name.Local == "nr" || a.Name.Local == "line") && n == 0 {
						return false
					}
				}
				if t.Name.Local == "counter" && a.Name.Local == "type" {
					switch a.Value {
					case "INSTRUCTION", "BRANCH", "LINE", "COMPLEXITY", "METHOD", "CLASS":
					default:
						return false
					}
				}
			}
			for _, name := range strings.Fields(required[t.Name.Local]) {
				if !seen[name] {
					return false
				}
			}
			stack = append(stack, t.Name.Local)
		case xml.EndElement:
			if len(stack) == 0 {
				return false
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(bytes.TrimSpace(t)) != 0 {
				return false
			}
		case xml.Directive:
			if roots != 0 || !strings.HasPrefix(string(t), "DOCTYPE report ") || strings.Contains(string(t), "[") {
				return false
			}
		case xml.ProcInst:
			if roots != 0 || t.Target != "xml" {
				return false
			}
		case xml.Comment:
		default:
			return false
		}
	}
}
