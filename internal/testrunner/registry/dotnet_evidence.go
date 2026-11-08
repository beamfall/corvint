// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov/v8"
	"github.com/Beamfall/corvint/internal/nativelinecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func (p *dotnetPlugin) source(name string) (DotnetSource, error) {
	s, ok := p.c.Sources[name]
	if !ok || !dotnetPath(name) || len(s.Data) > testrunner.MaxReportBytes || s.Blob.Path.Repository != p.c.Repository || s.Blob.Path.Lexical != name || s.Blob.Path.Resolved != name || s.Blob.LexicalOID != "" && s.Blob.LexicalOID != s.Blob.OID {
		return DotnetSource{}, fmt.Errorf("dotnet-source-unresolved:%s", name)
	}
	header := []byte(fmt.Sprintf("blob %d\x00", len(s.Data)))
	var digest string
	switch len(s.Blob.OID) {
	case 40:
		h := sha1.New()
		h.Write(header)
		h.Write(s.Data)
		digest = hex.EncodeToString(h.Sum(nil))
	case 64:
		h := sha256.New()
		h.Write(header)
		h.Write(s.Data)
		digest = hex.EncodeToString(h.Sum(nil))
	}
	if digest == "" || digest != s.Blob.OID {
		return DotnetSource{}, fmt.Errorf("dotnet-source-blob-mismatch:%s", name)
	}
	return s, nil
}

func (p *dotnetPlugin) Evidence(ctx context.Context, r EvidenceRequest) ([]NativeEvidence, error) {
	if err := p.check(ctx, r.Runner); err != nil {
		return nil, err
	}
	if len(r.Artifacts) == 0 || len(r.Artifacts) > testrunner.MaxReports || len(p.c.Collections) == 0 || len(p.c.Collections) > testrunner.MaxTests {
		return nil, fmt.Errorf("dotnet-evidence-artifacts-missing-or-capped")
	}
	requested := map[string]bool{}
	for _, handle := range r.Artifacts {
		if !dotnetText(handle) || requested[handle] {
			return nil, fmt.Errorf("dotnet-evidence-artifact-invalid")
		}
		if _, ok := p.c.Artifacts[handle]; !ok {
			return nil, fmt.Errorf("dotnet-evidence-artifact-unavailable")
		}
		requested[handle] = true
	}
	type collected struct {
		lines  v8.Lines
		reason string
	}
	byTest := map[string]*collected{}
	used := map[string]bool{}
	for _, collection := range p.c.Collections {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(collection.Tests) == 0 || len(collection.Tests) > testrunner.MaxTests || len(collection.Artifacts) == 0 || len(collection.Artifacts) > testrunner.MaxReports {
			return nil, fmt.Errorf("dotnet-evidence-collection-invalid")
		}
		lines := v8.Lines{}
		reason := ""
		if !collection.Complete {
			reason = "dotnet-coverage-capture-incomplete"
		}
		seen := map[string]bool{}
		for _, handle := range collection.Artifacts {
			if !requested[handle] || seen[handle] {
				reason = "dotnet-coverage-artifact-missing-or-duplicate"
				continue
			}
			seen[handle], used[handle] = true, true
			data := p.c.Artifacts[handle]
			if err := dotnetCobertura(data); err != nil {
				reason = err.Error()
				continue
			}
			decoded, err := nativelinecov.Decode(nativelinecov.Report{Format: "cobertura", Data: data}, p.c.SourceNames)
			if err != nil {
				reason = "dotnet-coverage-unparseable:" + err.Error()
				continue
			}
			lines.Merge(decoded)
		}
		seen = map[string]bool{}
		for _, id := range collection.Tests {
			if !dotnetText(id) || seen[id] || p.c.Bindings[id] == "" {
				return nil, fmt.Errorf("dotnet-evidence-test-unbound")
			}
			if _, err := p.source(p.c.Bindings[id]); err != nil {
				return nil, err
			}
			seen[id] = true
			if byTest[id] == nil {
				byTest[id] = &collected{lines: v8.Lines{}}
			}
			byTest[id].lines.Merge(lines)
			if reason != "" {
				byTest[id].reason = reason
			}
		}
		if len(byTest) > testrunner.MaxTests {
			return nil, fmt.Errorf("dotnet-evidence-test-cap")
		}
	}
	if len(used) != len(requested) {
		return nil, fmt.Errorf("dotnet-evidence-artifact-unattributed")
	}
	// Bindings is the core's inventory-to-source map, including tests for which
	// no collection arrived. Keep those identities visible and incomplete.
	for id := range p.c.Bindings {
		if byTest[id] == nil {
			if !dotnetText(id) || len(byTest) >= testrunner.MaxTests {
				return nil, fmt.Errorf("dotnet-evidence-test-unbound")
			}
			byTest[id] = &collected{lines: v8.Lines{}, reason: "dotnet-coverage-test-not-collected"}
		}
	}
	loaded := p.loaded(ctx)
	regions := p.regions(loaded)
	ids := make([]string, 0, len(byTest))
	for id := range byTest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]NativeEvidence, 0, len(ids))
	for _, id := range ids {
		capture := byTest[id]
		executed := testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present, Items: []testevidence.ExecutedFile{}}
		paths := make([]string, 0, len(capture.lines))
		for name := range capture.lines {
			paths = append(paths, name)
		}
		sort.Strings(paths)
		for _, name := range paths {
			source, err := p.source(name)
			if err != nil {
				capture.reason = err.Error()
				continue
			}
			file := testevidence.ExecutedFile{Blob: source.Blob, Executed: []testevidence.Range{}, Unexecuted: []testevidence.Range{}}
			numbers := make([]int, 0, len(capture.lines[name]))
			for n := range capture.lines[name] {
				numbers = append(numbers, n)
			}
			sort.Ints(numbers)
			for _, n := range numbers {
				if n > bytes.Count(source.Data, []byte{'\n'})+1 {
					capture.reason = "dotnet-coverage-line-outside-source"
					continue
				}
				span := testevidence.Range{Start: int32(n), StartColumn: 1, End: int32(n + 1), EndColumn: 1}
				if capture.lines[name][n] {
					file.Executed = append(file.Executed, span)
				} else {
					file.Unexecuted = append(file.Unexecuted, span)
				}
			}
			executed.Items = append(executed.Items, file)
		}
		if capture.reason != "" {
			executed.State, executed.Reason = testevidence.Incomplete, capture.reason
		}
		e, _ := json.Marshal(executed)
		r, _ := json.Marshal(regions)
		l, _ := json.Marshal(loaded)
		out = append(out, NativeEvidence{Test: p.id(id), Executed: e, Regions: r, Loaded: l})
	}
	return out, nil
}

func (p *dotnetPlugin) regions(loaded testevidence.Component[testevidence.Blob]) testevidence.Component[testevidence.RegionsFile] {
	out := testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present, Items: []testevidence.RegionsFile{}}
	if loaded.State != testevidence.Present {
		out.State, out.Reason = testevidence.Incomplete, "dotnet-regions-loaded-incomplete"
	}
	for _, blob := range loaded.Items {
		if !strings.HasSuffix(blob.Path.Lexical, ".cs") {
			continue
		}
		source, err := p.source(blob.Path.Lexical)
		if err != nil {
			out.State, out.Reason = testevidence.Incomplete, err.Error()
			continue
		}
		bodies, ok := csharpBodies(source.Data)
		if !ok {
			out.State, out.Reason = testevidence.Incomplete, "dotnet-csharp-regions-unsupported"
			continue
		}
		file := testevidence.RegionsFile{Blob: blob, Bodies: []testevidence.Body{}}
		for _, b := range bodies {
			file.Bodies = append(file.Bodies, testevidence.Body{ID: fmt.Sprintf("%s:%d", b.Name, b.Start), Range: testevidence.Range{Start: int32(b.Start), StartColumn: 1, End: int32(b.End), EndColumn: 1}})
		}
		out.Items = append(out.Items, file)
	}
	return out
}

// Restrict the existing shared decoder to the known Cobertura hierarchy. A
// future/unknown structure cannot disappear through XML field projection.
func dotnetCobertura(data []byte) error {
	if len(data) == 0 || len(data) > nativelinecov.MaxReportBytes {
		return fmt.Errorf("dotnet-coverage-size")
	}
	children := map[string]string{"": "coverage", "coverage": "sources packages", "sources": "source", "source": "", "packages": "package", "package": "classes", "classes": "class", "class": "methods lines", "methods": "method", "method": "lines", "lines": "line", "line": "conditions", "conditions": "condition", "condition": ""}
	attributes := map[string]string{
		"coverage":  "line-rate branch-rate lines-covered lines-valid branches-covered branches-valid complexity version timestamp",
		"package":   "name line-rate branch-rate complexity",
		"class":     "name filename line-rate branch-rate complexity",
		"method":    "name signature line-rate branch-rate complexity",
		"line":      "number hits branch condition-coverage",
		"condition": "number type coverage",
	}
	var stack []string
	roots, nodes := 0, 0
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("dotnet-coverage-xml-invalid:%w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			nodes++
			if t.Name.Space != "" || nodes > 100000 || len(stack) > 16 {
				return fmt.Errorf("dotnet-coverage-xml-bounds")
			}
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			} else {
				roots++
			}
			if !strings.Contains(" "+children[parent]+" ", " "+t.Name.Local+" ") {
				return fmt.Errorf("dotnet-coverage-xml-unsupported")
			}
			for _, a := range t.Attr {
				if a.Name.Space != "" || !strings.Contains(" "+attributes[t.Name.Local]+" ", " "+a.Name.Local+" ") {
					return fmt.Errorf("dotnet-coverage-xml-attribute-unsupported")
				}
			}
			stack = append(stack, t.Name.Local)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(bytes.TrimSpace(t)) > 0 && (len(stack) == 0 || stack[len(stack)-1] != "source") {
				return fmt.Errorf("dotnet-coverage-xml-text")
			}
		}
	}
	if roots != 1 || len(stack) != 0 {
		return fmt.Errorf("dotnet-coverage-xml-truncated")
	}
	return nil
}
