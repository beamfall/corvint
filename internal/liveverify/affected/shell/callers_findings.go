package shell

import (
	"bytes"
	"fmt"
	"path"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// ForeignCallerFinding is one file on which the foreign-caller declaration
// and the tree disagree (TSP-V0-920). Kind is "undeclared" for a candidate
// file that names the script but is listed as neither a caller nor a mention,
// and "stale" for a listed file that no longer names it. Line is the first
// line holding the script's file name; it is 0 for a stale file and for a
// shell file that only builds a name the script may be (Built).
type ForeignCallerFinding struct {
	Script string
	File   string
	Line   int
	Built  bool
	Kind   string
}

// String renders the finding as the file:line an operator fixes.
func (f ForeignCallerFinding) String() string {
	at := f.File
	if f.Line > 0 {
		at = fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	switch {
	case f.Kind == "stale":
		return fmt.Sprintf("%s: listed for %s in %s but no longer names it", at, f.Script, affected.ShellForeignCallersPath)
	case f.Built:
		return fmt.Sprintf("%s: builds a name %s may be but is not a declared caller or mention in %s", at, f.Script, affected.ShellForeignCallersPath)
	default:
		return fmt.Sprintf("%s: names %s but is not a declared caller or mention in %s (a comment or message is a mention)", at, f.Script, affected.ShellForeignCallersPath)
	}
}

// ForeignCallerFindings applies the shell plugin's own completeness rule
// (foreignCallersComplete) and returns each disagreement by file and line,
// sorted by script, file and kind, so a refused declaration names its
// offending files instead of only FrontierForeignCallersInvalid. It returns
// nil with no declaration and the read error for an invalid one. The rule is
// the plugin's: only callerCandidate files count, by the script's file name or
// by a name a shell file builds (TSP-V0-960).
func ForeignCallerFindings(root *affected.Source) ([]ForeignCallerFinding, error) {
	declared, err := readForeignCallers(root)
	if declared == nil || err != nil {
		return nil, err
	}
	built, err := builtNames(root)
	if err != nil {
		return nil, err
	}
	candidates, err := root.Files(callerCandidate)
	if err != nil {
		return nil, err
	}
	var findings []ForeignCallerFinding
	naming := make(map[string]map[string]bool, len(declared))
	for _, candidate := range candidates {
		body, err := root.Read(candidate)
		if err != nil {
			return nil, err
		}
		for script, entry := range declared {
			if candidate == script {
				continue
			}
			base := []byte(path.Base(script))
			line, isBuilt := 0, false
			if at := bytes.Index(body, base); at >= 0 {
				line = bytes.Count(body[:at], []byte("\n")) + 1
			} else if built[candidate].names(path.Base(script)) {
				isBuilt = true
			} else {
				continue
			}
			if naming[script] == nil {
				naming[script] = map[string]bool{}
			}
			naming[script][candidate] = true
			if !slices.Contains(entry.Callers, candidate) && !slices.Contains(entry.Mentions, candidate) {
				findings = append(findings, ForeignCallerFinding{Script: script, File: candidate, Line: line, Built: isBuilt, Kind: "undeclared"})
			}
		}
	}
	for script, entry := range declared {
		for _, file := range append(slices.Clone(entry.Callers), entry.Mentions...) {
			if !naming[script][file] {
				findings = append(findings, ForeignCallerFinding{Script: script, File: file, Kind: "stale"})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Script != b.Script {
			return a.Script < b.Script
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Kind < b.Kind
	})
	return findings, nil
}

// builtNames is the scan UnitsSource makes of every shell file for the
// script names it builds and runs, so ForeignCallerFindings counts the same
// built callers the plugin does.
func builtNames(root *affected.Source) (map[string]computedNames, error) {
	files, err := root.Files(func(name string) bool { return hasAnySuffix(name, Suffixes) })
	if err != nil {
		return nil, err
	}
	hidden, err := hiddenFiles(root)
	if err != nil {
		return nil, err
	}
	files = append(files, hidden...)
	known := make(map[string]bool, len(files))
	for _, relative := range files {
		known[relative] = true
	}
	built := map[string]computedNames{}
	for _, relative := range files {
		body, err := root.Read(relative)
		if err != nil || !utf8.Valid(body) {
			continue
		}
		if _, names, _ := observe(relative, string(body), known); !names.empty() {
			built[relative] = names
		}
	}
	return built, nil
}
