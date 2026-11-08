// SPDX-License-Identifier: Apache-2.0
package browsercov

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/linecov/v8"
	"github.com/Beamfall/corvint/internal/testevidence"
)

// BrowserNative uses the existing content/source-map attribution adapter. The
// caller supplies immutable sources and complete loader accounting independently.
func BrowserNative(repo *Repo, c jslinecov.CoverageCapture, scripts []Script) testevidence.Native {
	c.Lines = v8.Lines{}
	if repo == nil || len(scripts) > 4096 {
		c.Complete = false
		c.Reason = "browser-coverage-unavailable"
		return jslinecov.NativeCoverage(c)
	}
	ranges := 0
	byPath := map[string][]Observation{}
	for _, script := range scripts {
		if len(script.Source) > jslinecov.MaxRunnerArtifact || len(script.Functions) > 65536 {
			c.Complete = false
			c.Reason = "browser-coverage-cap"
			break
		}
		units := len(utf16.Encode([]rune(script.Source)))
		for _, fn := range script.Functions {
			for _, r := range fn.Ranges {
				ranges++
				if ranges > 65536 || r.Count < 0 || r.StartOffset < 0 || r.EndOffset < r.StartOffset || r.EndOffset > units {
					c.Complete = false
					c.Reason = "browser-range-invalid"
					return jslinecov.NativeCoverage(c)
				}
			}
		}
		observations, why := repo.Attribute(script)
		if why != "" {
			c.Complete = false
			c.Reason = "browser-source-map-incomplete"
			break
		}
		for _, o := range observations {
			source, ok := c.Sources[o.Path]
			if !ok || source.OID != repo.OIDs[o.Path] || !bytes.Equal(source.Text, []byte(o.Text)) {
				c.Complete = false
				c.Reason = "browser-source-unbound"
				continue
			}
			c.Loaded = append(c.Loaded, o.Path)
			byPath[o.Path] = append(byPath[o.Path], o)
		}
	}
	for p, observations := range byPath {
		text := string(c.Sources[p].Text)
		lines := certifiedLineStates(text, observations)
		n := strings.Count(text, "\n")
		if text != "" && !strings.HasSuffix(text, "\n") {
			n++
		}
		// Unknown coverage cannot prove that an enclosing body never ran.
		// Widen the capture rather than silently losing positive evidence
		// from another observation while retaining exclusions elsewhere.
		if len(lines) != n {
			c.Complete = false
			c.Reason = "browser-coverage-unavailable"
		}
		c.Lines[p] = map[int]bool{}
		for l, ran := range lines {
			c.Lines.Add(p, l+1, ran)
		}
	}
	return jslinecov.NativeCoverage(c)
}

// RSP-V0-005: whole-line exclusion requires every UTF-16 unit, including
// whitespace and line endings, to have a known state in every observation.
// Only direct, exact-length unit arrays containing 0 or 1 are understood.
// Mapped line summaries, mixed representations and future states cannot prove
// whole-line coverage. The legacy display-oriented lineStates is insufficient.
func certifiedLineStates(text string, observations []Observation) map[int]bool {
	units := utf16.Encode([]rune(text))
	out := map[int]bool{}
	if len(observations) == 0 {
		return out
	}
	for _, o := range observations {
		if o.Units == nil || o.Lines != nil || len(o.Units) != len(units) || o.Text != text {
			return out
		}
	}
	line, known, ran := 0, true, false
	for i, u := range units {
		for _, o := range observations {
			switch o.Units[i] {
			case 0:
			case 1:
				ran = true
			default:
				known = false
			}
		}
		if u == '\n' || i == len(units)-1 {
			if known {
				out[line] = ran
			}
			line, known, ran = line+1, true, false
		}
	}
	return out
}

// ServerAttribution contains core-observed facts, never a plugin claim that its
// own process was traced. FreshSerial means a fresh traced server process for
// precisely one test; Workers=1 alone does not establish it.
type ServerAttribution struct {
	Tests                                                        []string
	Test                                                         string
	InsideTree, CoverageBuilt                                    bool
	ResetDumpQualified, ResetObserved, DumpObserved, FreshSerial bool
}

// JoinServer preserves the T1 record shape. Without qualified reset/dump or
// proven fresh-server isolation it unions server coverage into every sharing
// test. The core must supply all sharers, including browser/project retries.
func JoinServer(records []testevidence.Record, server testevidence.Native, a ServerAttribution) ([]testevidence.Record, error) {
	return JoinServerDumps(records, []ServerDump{{Coverage: server, Attribution: a}})
}

// ServerDump is one core-attributed observation in a server collection.
type ServerDump struct {
	Coverage    testevidence.Native
	Attribution ServerAttribution
}

// JoinServerDumps joins all available dumps against the original browser records.
// Every dump must describe the same server's complete sharer set. Progress stays
// local to this call: neither matching file names nor a previously complete
// record proves that a test received its own isolated dump. Missing dumps leave
// executed incomplete; replay the collection with additional dumps to complete it.
func JoinServerDumps(records []testevidence.Record, dumps []ServerDump) ([]testevidence.Record, error) {
	if len(dumps) == 0 || len(dumps) > 4096 || len(records) > 4096 || len(dumps[0].Attribution.Tests) == 0 || len(dumps[0].Attribution.Tests) > 4096 {
		return nil, fmt.Errorf("server-attribution-unavailable")
	}
	sharers := map[string]bool{}
	for _, id := range dumps[0].Attribution.Tests {
		sharers[id] = true
	}
	joined := map[string]bool{}
	out := records
	for _, dump := range dumps {
		if len(dump.Attribution.Tests) != len(sharers) {
			return nil, fmt.Errorf("server-attribution-invalid")
		}
		for _, id := range dump.Attribution.Tests {
			if !sharers[id] {
				return nil, fmt.Errorf("server-attribution-invalid")
			}
		}
		var err error
		out, err = joinServerDump(out, dump.Coverage, dump.Attribution, joined)
		if err != nil {
			return nil, err
		}
	}
	for i := range out {
		if sharers[out[i].Test] && !joined[out[i].Test] && out[i].Executed.State == testevidence.Present {
			out[i].Executed.State = testevidence.Incomplete
			out[i].Executed.Reason = "browser-server-dump-pending"
		}
	}
	return out, nil
}

func joinServerDump(records []testevidence.Record, server testevidence.Native, a ServerAttribution, joined map[string]bool) ([]testevidence.Record, error) {
	if len(a.Tests) == 0 || len(a.Tests) > 4096 || len(records) > 4096 {
		return nil, fmt.Errorf("server-attribution-unavailable")
	}
	wanted := map[string]bool{}
	for _, id := range a.Tests {
		if id == "" || wanted[id] {
			return nil, fmt.Errorf("server-attribution-invalid")
		}
		wanted[id] = true
	}
	if a.Test != "" && !wanted[a.Test] {
		return nil, fmt.Errorf("server-attribution-invalid")
	}
	isolated := a.InsideTree && a.Test != "" && (a.FreshSerial && len(a.Tests) == 1 || a.ResetDumpQualified && a.ResetObserved && a.DumpObserved)
	seen := map[string]bool{}
	out := make([]testevidence.Record, 0, len(records))
	for _, r := range records {
		if seen[r.Test] {
			return nil, fmt.Errorf("server-record-duplicate")
		}
		seen[r.Test] = true
		if !wanted[r.Test] {
			out = append(out, r)
			continue
		}
		if !a.InsideTree {
			r.External.Items = append(append([]testevidence.Contact{}, r.External.Items...), testevidence.Contact{Kind: "loopback"})
			// Preserve unknown trace state; external is still visible even when absent.
			if r.External.State == testevidence.Absent {
				r.External.State = testevidence.Incomplete
				r.External.Reason = "external-server"
			}
			r.Executed.State, r.Executed.Reason = testevidence.Incomplete, "external-server"
			out = append(out, r)
			continue
		}
		if !a.CoverageBuilt {
			r.Executed.State, r.Executed.Reason = testevidence.Incomplete, "server-coverage-absent"
			out = append(out, r)
			continue
		}
		if isolated && r.Test != a.Test {
			out = append(out, r)
			continue
		}
		n, err := joinNative(testevidence.Native{Executed: r.Executed, Regions: r.Regions, Loaded: r.Loaded}, server)
		if err != nil {
			return nil, err
		}
		r.Executed, r.Regions, r.Loaded = n.Executed, n.Regions, n.Loaded
		joined[r.Test] = true
		out = append(out, r)
	}
	for id := range wanted {
		if !seen[id] {
			return nil, fmt.Errorf("server-record-missing")
		}
	}
	return out, nil
}

// Joining components of one collection is not Union of independent collections:
// it must not fabricate producers or increase CollectionCount.
func joinNative(a, b testevidence.Native) (testevidence.Native, error) {
	for _, n := range []testevidence.Native{a, b} {
		check := testevidence.New("check", "check", "unused")
		check.Executed, check.Regions, check.Loaded = n.Executed, n.Regions, n.Loaded
		if err := check.ValidateComponents(); err != nil {
			return testevidence.Native{}, err
		}
	}
	out := testevidence.Native{Executed: joinComponent(a.Executed, b.Executed), Regions: joinComponent(a.Regions, b.Regions), Loaded: joinComponent(a.Loaded, b.Loaded)}
	seen := map[testevidence.Path]testevidence.Blob{}
	blobs := append([]testevidence.Blob{}, out.Loaded.Items...)
	for _, f := range out.Executed.Items {
		blobs = append(blobs, f.Blob)
	}
	for _, f := range out.Regions.Items {
		blobs = append(blobs, f.Blob)
	}
	for _, b := range blobs {
		if old, ok := seen[b.Path]; ok && old != b {
			return out, fmt.Errorf("server-blob-drift")
		}
		seen[b.Path] = b
	}
	r := testevidence.New("check", "check", "unused")
	r.Executed, r.Regions, r.Loaded = out.Executed, out.Regions, out.Loaded
	if err := r.ValidateComponents(); err != nil {
		return out, err
	}
	return out, nil
}
func joinComponent[T any](a, b testevidence.Component[T]) testevidence.Component[T] {
	out := testevidence.Component[T]{State: testevidence.Present, Items: append(append([]T{}, a.Items...), b.Items...)}
	if a.State != testevidence.Present || b.State != testevidence.Present {
		out.State = testevidence.Incomplete
		out.Reason = "server-component-incomplete"
	}
	return out
}

// RunnerCoveragePreparation exposes the existing coverage hook as scratch
// assets. Core creates the fresh directory and writes these files before it
// launches Playwright inside the tracer; this function performs no I/O.
func RunnerCoveragePreparation(work, nodeOptions string) (map[string][]byte, map[string]string, error) {
	if !filepath.IsAbs(work) || strings.ContainsAny(work+nodeOptions, "\x00\r\n") {
		return nil, nil, fmt.Errorf("coverage-work-unavailable")
	}
	hook := filepath.Join(work, "browser-hook.cjs")
	files := map[string][]byte{"browser-hook.cjs": append([]byte{}, hookSource...)}
	env := map[string]string{"NODE_OPTIONS": strings.TrimSpace(nodeOptions + " --require " + quoteNodeOption(hook)), "NODE_V8_COVERAGE": filepath.Join(work, "node-coverage"), "CORVINT_BROWSERCOV_DIR": work, "CORVINT_BROWSERCOV_HOOK": hook}
	return files, env, nil
}
