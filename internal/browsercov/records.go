// SPDX-License-Identifier: Apache-2.0

package browsercov

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/linecov"
)

// Run is one test's observations from a passing run.
type Run struct {
	Package, Name string
	Elapsed       time.Duration
	Obs           []Observation
}

// Records turns runs into linecov records. linecov.Build unions blocks by
// exact span, so every record of one file must cut it into the same blocks:
// a file's blocks split at every point where any run's execution state
// changes, then at line ends and surrounding whitespace (LPCV-V0-973). A file
// any run reached only through a source map is cut into whole lines.
func Records(module string, runs []Run) []linecov.Record {
	type fileRuns struct {
		text   string
		mapped bool
		byRun  map[int][]Observation
	}
	files := map[string]*fileRuns{}
	for i, r := range runs {
		for _, o := range r.Obs {
			f := files[o.Path]
			if f == nil {
				f = &fileRuns{text: o.Text, byRun: map[int][]Observation{}}
				files[o.Path] = f
			}
			f.mapped = f.mapped || o.Units == nil
			f.byRun[i] = append(f.byRun[i], o)
		}
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	profiles := make([]strings.Builder, len(runs))
	for i := range profiles {
		profiles[i].WriteString("mode: set\n")
	}
	for _, p := range paths {
		f := files[p]
		var blocks []block
		var hit func(run int, b block) bool
		if f.mapped {
			lines := map[int]map[int]bool{}
			all := map[int]bool{}
			for run, obs := range f.byRun {
				lines[run] = lineStates(f.text, obs)
				for l := range lines[run] {
					all[l] = true
				}
			}
			blocks = lineBlocks(f.text, all)
			hit = func(run int, b block) bool { return lines[run][int(b.line)-1] }
		} else {
			units := map[int][]int8{}
			for run, obs := range f.byRun {
				units[run] = mergeUnits(obs)
			}
			blocks = unitBlocks(f.text, units)
			hit = func(run int, b block) bool { return units[run][b.unit] == 1 }
		}
		for run := range f.byRun {
			for _, b := range blocks {
				n := 0
				if hit(run, b) {
					n = 1
				}
				fmt.Fprintf(&profiles[run], "%s/%s:%d.%d,%d.%d 1 %d\n", module, p, b.line, b.col, b.line, b.endCol, n)
			}
		}
	}
	out := make([]linecov.Record, len(runs))
	for i, r := range runs {
		out[i] = linecov.Record{Package: r.Package, Name: r.Name, Elapsed: r.Elapsed, Profile: []byte(profiles[i].String())}
	}
	return out
}

// block is one line-local span: 1-based line, 1-based byte columns with an
// exclusive end, and (direct files) the UTF-16 unit it starts at.
type block struct {
	line, col, endCol int32
	unit              int
}

func mergeUnits(obs []Observation) []int8 {
	var u []int8
	for _, o := range obs {
		if u == nil {
			u = append([]int8(nil), o.Units...)
			continue
		}
		for i := range min(len(u), len(o.Units)) {
			u[i] = max(u[i], o.Units[i])
		}
	}
	return u
}

// lineStates reduces observations to 0-based line -> executed; a direct
// observation's line counts when any non-blank unit on it lies in a range.
func lineStates(text string, obs []Observation) map[int]bool {
	out := map[int]bool{}
	for _, o := range obs {
		if o.Units == nil {
			for l, ran := range o.Lines {
				out[l] = out[l] || ran
			}
			continue
		}
		line, u := 0, 0
		for _, c := range text {
			if c == '\n' {
				line++
			} else if c != ' ' && c != '\t' && c != '\r' && u < len(o.Units) && o.Units[u] >= 0 {
				out[line] = out[line] || o.Units[u] == 1
			}
			u += utf16.RuneLen(c)
		}
	}
	return out
}

// lineBlocks returns one block per listed line holding non-blank text.
func lineBlocks(text string, lines map[int]bool) []block {
	var out []block
	for i, l := range strings.Split(text, "\n") {
		if !lines[i] {
			continue
		}
		start := len(l) - len(strings.TrimLeft(l, " \t"))
		end := len(strings.TrimRight(l, " \t\r"))
		if end > start {
			out = append(out, block{line: int32(i + 1), col: int32(start + 1), endCol: int32(end + 1)})
		}
	}
	return out
}

// unitBlocks cuts a direct file at every state change of any run, at line
// ends and at blanks bounding a run of text, keeping spans some run's ranges
// reach.
func unitBlocks(text string, runs map[int][]int8) []block {
	n := len(utf16.Encode([]rune(text)))
	cut := make([]bool, n+1)
	for _, u := range runs {
		for i := 1; i < min(n, len(u)); i++ {
			if u[i] != u[i-1] {
				cut[i] = true
			}
		}
	}
	reached := func(i int) bool {
		for _, u := range runs {
			if i < len(u) && u[i] >= 0 {
				return true
			}
		}
		return false
	}
	var out []block
	line, col, unit := int32(1), 0, 0 // col: byte offset within the line
	var cur *block
	lastText := 0 // byte column just past the current block's last non-blank
	flush := func() {
		if cur != nil {
			cur.endCol = int32(lastText + 1)
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, c := range text {
		w := utf8.RuneLen(c)
		if c == '\n' {
			flush()
			line, col = line+1, 0
			unit += utf16.RuneLen(c)
			continue
		}
		if cut[unit] {
			flush()
		}
		if c != ' ' && c != '\t' && c != '\r' {
			if cur == nil && reached(unit) {
				cur = &block{line: line, col: int32(col + 1), unit: unit}
			}
			if cur != nil {
				lastText = col + w
			}
		}
		col += w
		unit += utf16.RuneLen(c)
	}
	flush()
	return out
}
