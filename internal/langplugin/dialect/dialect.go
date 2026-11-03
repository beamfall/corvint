// SPDX-License-Identifier: Apache-2.0

// Package dialect is the hook registry through which a language plugin hands
// its dialect algorithm bodies to core (docs/specs/language-plugins-v0.md,
// LGP-V0-023 to LGP-V0-027). It imports only the standard library, so a plugin
// registers here without importing internal/contextindex, and core looks the
// hooks up by the dialect name internal/langplugin/decl declares, never by a
// language name of its own.
package dialect

import (
	"fmt"
	"regexp"
	"sort"
	"sync"
)

// Probe is one lexical probe an impact profile runs on each masked code line:
// a line Pattern matches reports Code as an impact unknown.
type Probe struct {
	Code    string
	Pattern *regexp.Regexp
}

// Reflection matches the reflective-dispatch tokens every impact profile
// reports as observed-reflective-dispatch.
var Reflection = regexp.MustCompile(`\b(?:send|public_send|__send__|define_method|define_singleton_method|method_missing|class_eval|module_eval|eval|constantize|const_get)\b`)

// Tree is the read-only view of the committed tree an impact profile's
// verification command reads. Core implements it over the index.
type Tree interface {
	// Tracks reports whether name is a tracked path.
	Tracks(name string) bool
	// TracksUnder reports whether any tracked path starts with prefix.
	TracksUnder(prefix string) bool
	// RootText returns a tracked file's bounded committed bytes, or false
	// when it is untracked, oversized or unreadable.
	RootText(name string) (string, bool)
}

// Impact is the bounded syntax profile of one impact dialect (LGP-V0-025).
type Impact struct {
	// CodeLines masks comments and literals, preserving line and column
	// positions; masked bytes are never facts.
	CodeLines func(text string) []string
	// Probes run, in order, on each masked line.
	Probes []Probe
	// Loads matches a load or import call whose argument StaticLoad judges.
	Loads *regexp.Regexp
	// StaticLoad reports whether the raw text after a Loads match is one
	// complete quoted argument.
	StaticLoad func(tail string) bool
	// Verification names the repository-wide command that runs the
	// dialect's tests, or "" when the tree does not prove one. Optional.
	Verification func(Tree) string
}

// Line is one normalized source line; Line is the 1-based physical line the
// text starts on.
type Line struct {
	Line int
	Text string
}

// Declaration is one symbol a region dialect's extractor found.
type Declaration struct {
	Kind, Name string
	Line       int
}

// Stop names why an extraction ended before the end of the source.
type Stop string

const (
	// LineCap means the line cap was reached.
	LineCap Stop = "line-cap"
	// SymbolCap means the symbol cap was reached.
	SymbolCap Stop = "symbol-cap"
)

// Extraction is a region dialect's symbol extraction of one source. With no
// Stop and neither unterminated flag set, Declarations is the source's whole
// declaration set.
type Extraction struct {
	Declarations        []Declaration
	Stop                Stop
	UnterminatedComment bool
	UnterminatedString  bool
}

// Region is a symbol-region normalizer family (LGP-V0-025).
type Region struct {
	// Normalize returns the comment- and whitespace-insensitive code lines,
	// or false when the walk could not finish.
	Normalize func(text string) ([]Line, bool)
	// Symbols extracts declarations within the line and symbol caps.
	// Optional.
	Symbols func(text string, maxLines, maxSymbols int) Extraction
}

var (
	mu      sync.RWMutex
	impacts = map[string]Impact{}
	regions = map[string]Region{}
)

// RegisterImpact registers the impact profile of dialect name. It panics on a
// duplicate or incomplete registration, which is a link-time programming error.
func RegisterImpact(name string, impact Impact) {
	if impact.CodeLines == nil || impact.Loads == nil || impact.StaticLoad == nil {
		panic(fmt.Sprintf("dialect: impact %q lacks CodeLines, Loads or StaticLoad", name))
	}
	register(impacts, name, impact)
}

// RegisterRegion registers the region normalizer of dialect name. It panics on
// a duplicate or incomplete registration.
func RegisterRegion(name string, region Region) {
	if region.Normalize == nil {
		panic(fmt.Sprintf("dialect: region %q lacks Normalize", name))
	}
	register(regions, name, region)
}

func register[T any](table map[string]T, name string, hook T) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := table[name]; dup || name == "" {
		panic(fmt.Sprintf("dialect: duplicate or empty registration %q", name))
	}
	table[name] = hook
}

// LookupImpact returns the impact profile registered for dialect name.
func LookupImpact(name string) (Impact, bool) {
	mu.RLock()
	defer mu.RUnlock()
	impact, ok := impacts[name]
	return impact, ok
}

// LookupRegion returns the region normalizer registered for dialect name.
func LookupRegion(name string) (Region, bool) {
	mu.RLock()
	defer mu.RUnlock()
	region, ok := regions[name]
	return region, ok
}

// ImpactNames returns every registered impact dialect, sorted.
func ImpactNames() []string { return names(impacts) }

// RegionNames returns every registered region dialect, sorted.
func RegionNames() []string { return names(regions) }

func names[T any](table map[string]T) []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(table))
	for name := range table {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
