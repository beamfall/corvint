// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"maps"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

// recordEntrySuffix returns the suffix every entry of the record directory
// record carries (recordDirectory): `.cem.json` for sealed CEMs, `.md` for
// build-log entries.
func recordEntrySuffix(record string) (string, bool) {
	switch record {
	case SealedChangesDirectory:
		return ".cem.json", true
	case BuildLogDirectory:
		return ".md", true
	}
	return "", false
}

// RecordReaders returns, in graph order, the Go and shell units with tests and no
// declared read scope that read new entries of the record directory record
// (SealedChangesDirectory or BuildLogDirectory), and the walkers of it, for
// the AFP-V0-2745 ratchet (tools/unbounded-readers).
//
// A reader is selected by the reader rules, by a path literal, joined
// literals (AFP-V0-2746) or a glob, for an entry of record that no literal
// names by its whole file name: the entry namespace, every name with at
// least one byte before the suffix, is matched against each literal and glob
// of the unit (AFP-V0-2747) rather than against one synthetic name, so a
// literal fragment completed at the entry position (`docs/build-log/2026-`)
// or a glob whose last component admits an entry (`2026-*.md`) makes a
// reader. A unit that locates the root at run time and names an ancestor of
// record is one too: the unbounded-reader rule selects it on every change.
//
// A walker names a proper ancestor of record other than the root
// (`../docs`), so a walk or a name computed at run time could reach every
// new entry, while the reader rules do not select it for every one
// (AFP-V0-2747): no literal joins the rest of the way, and it does not
// locate the root. A narrower reader match, a fragment, an entry name or a
// glob constrained in the name (`../docs/*/2026-*.md`), selects it for some
// entries only and leaves it a walker (AFP-V0-2748, coversRecord). It
// returns nil, nil for any other directory.
func (graph *Graph) RecordReaders(record string) (readers, walkers []string) {
	suffix, ok := recordEntrySuffix(record)
	if !ok {
		return nil, nil
	}
	tokens := graph.pathTokens()
	for i, id := range graph.order {
		unit := graph.units[id]
		goUnit := strings.HasPrefix(id, "go:")
		if (!goUnit && !strings.HasPrefix(id, "shell:")) || len(unit.Tests) == 0 || unit.ReadScoped {
			continue
		}
		// Every probe is tried: a read through one probe must not hide
		// the ancestor another names.
		reads, walks := false, false
		for _, probe := range recordProbes(record, suffix, unit) {
			// Go units match at any depth except CEM evidence, as the planner does.
			if namesAny(tokens[i], strings.Split(probe, "/"), goUnit && !resolvedEvidence(probe)) {
				reads = reads || graph.resolves(id, probe)
				walks = walks || goUnit && graph.namesRecordAncestor(id, probe, record)
			}
			if !reads {
				_, _, reads = graph.globRead(id, []string{probe})
			}
		}
		_, locates := graph.causes[id]
		switch {
		case walks && !locates && !graph.coversRecord(id, record, suffix):
			walkers = append(walkers, id)
		case reads || walks:
			readers = append(readers, id)
		}
	}
	return readers, walkers
}

// coversRecord reports whether the reader rules select Go unit id for every
// entry of record, whatever its name (AFP-V0-2748). One of its tokens must
// resolve, as resolves resolves it, to record itself or to the root or a
// proper ancestor of record from which its literals join the rest of the way
// (AFP-V0-2746). Alternatively one of its globs must cover record
// (globCoversRecord). A fragment, an entry name or a glob constrained in the
// name selects the unit for some entries only, so it covers nothing. The
// token must name two entries that share no leading or trailing rune before
// the suffix, so a run matching the name alone does not count.
func (graph *Graph) coversRecord(id, record, suffix string) bool {
	unit := graph.units[id]
	for _, glob := range unit.PathGlobs {
		if globCoversRecord(glob, record, suffix) {
			return true
		}
	}
	files := append(append([]string(nil), unit.Sources...), unit.Tests...)
	if len(files) == 0 {
		return false
	}
	directory := relativeDirectory(files[0])
	_, locates := graph.causes[id]
	anchored := rootAnchored(unit.PathTokens) || locates
	first := strings.Split(record+strings.Repeat("f", 40)+suffix, "/")
	second := strings.Split(record+"0"+suffix, "/")
	whole := func(base, value string) bool {
		below := record
		if resolved := path.Join(base, value); resolved != "." {
			var ok bool
			if below, ok = strings.CutPrefix(record, resolved+"/"); !ok {
				return false
			}
		}
		return below == "" || joinsComponents(unit.PathTokens, strings.Split(strings.TrimSuffix(below, "/"), "/"))
	}
	for _, value := range unit.PathTokens {
		if !namesPath(value, first) || !namesPath(value, second) {
			continue
		}
		if rest, rooted := strings.CutPrefix(value, "/"); rooted {
			if whole("", rest) {
				return true
			}
			continue
		}
		if whole(directory, value) || (anchored && whole("", value)) {
			return true
		}
	}
	return false
}

// globCoversRecord reports whether glob selects every entry of record
// whatever its name (AFP-V0-2748), as globMatches matches it: it matches
// record or one of its ancestors, or its leading components match record and
// its last component admits every name of the entry namespace (globCovers).
func globCoversRecord(glob, record, suffix string) bool {
	directory := strings.Split(strings.TrimSuffix(record, "/"), "/")
	if globMatches(glob, directory) {
		return true
	}
	components := strings.Split(strings.TrimPrefix(glob, "/"), "/")
	head, last := components[:len(components)-1], components[len(components)-1]
	offset := len(directory) - len(head)
	if offset < 0 || (strings.HasPrefix(glob, "/") && offset != 0) {
		return false
	}
	for index, component := range head {
		if matched, err := path.Match(component, directory[offset+index]); err != nil || !matched {
			return false
		}
	}
	return globCovers(last, suffix)
}

// globCovers reports whether the glob component pattern matches every name
// of the entry namespace of suffix: every name with at least one byte before
// suffix and no `/` (AFP-V0-2748). It searches the product of the set of
// items the name read so far can have reached and globEntry's suffix and
// length state for a name of the namespace that the pattern does not match,
// over globAlphabet. It reports true when no such name exists. A malformed
// or empty pattern covers nothing.
func globCovers(pattern, suffix string) bool {
	items, _ := globItems(pattern)
	if len(items) == 0 {
		return false
	}
	runes := globAlphabet(pattern, suffix, items)
	// reached is a set of item positions, len(items) meaning the whole
	// pattern; a `*` may match nothing, so it also reaches the next one.
	closure := func(reached []byte) string {
		for position, item := range items {
			if reached[position] == 1 && item.star {
				reached[position+1] = 1
			}
		}
		return string(reached)
	}
	type state struct {
		reached         string
		matched, length int
	}
	full := len(suffix) + 1
	start := make([]byte, len(items)+1)
	start[0] = 1
	seen := map[state]bool{}
	queue := []state{{closure(start), 0, 0}}
	seen[queue[0]] = true
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.matched == len(suffix) && current.length == full && current.reached[len(items)] == 0 {
			return false
		}
		for _, char := range runes {
			next := make([]byte, len(items)+1)
			for position, item := range items {
				if current.reached[position] == 0 || !item.admits(char) {
					continue
				}
				if item.star {
					next[position] = 1
				} else {
					next[position+1] = 1
				}
			}
			following := state{closure(next), suffixPrefix(suffix, current.matched, char), min(current.length+1, full)}
			if !seen[following] {
				seen[following] = true
				queue = append(queue, following)
			}
		}
	}
	return true
}

// recordProbes returns the entries of record through which unit could read a
// new entry (AFP-V0-2747): one whose name no literal states, which every
// reader of the directory itself selects; for each literal whose last
// component follows the record directory's own last component, the entry
// completing that component as a fragment (`2026-` gives `2026-x.md`) unless
// it already ends with the entry suffix and so names one entry; and for each
// glob, an entry its last component admits (globEntry).
func recordProbes(record, suffix string, unit Unit) []string {
	probes := []string{record + strings.Repeat("f", 40) + suffix}
	last := path.Base(record)
	for _, value := range unit.PathTokens {
		components := strings.Split(value, "/")
		if n := len(components); n < 2 || components[n-2] != last {
			continue
		}
		fragment := components[len(components)-1]
		if fragment != "" && fragment != "." && fragment != ".." && !strings.HasSuffix(fragment, suffix) {
			probes = append(probes, record+fragment+"x"+suffix)
		}
	}
	for _, glob := range unit.PathGlobs {
		if name, ok := globEntry(path.Base(glob), suffix); ok {
			probes = append(probes, record+name)
		}
	}
	return probes
}

// namesRecordAncestor reports whether a token of Go unit id, resolved from
// its directory, or from the root when the unit is anchored or locates the
// root (resolves), names probe through a proper ancestor of its record
// directory other than the root (AFP-V0-2745).
func (graph *Graph) namesRecordAncestor(id, probe, record string) bool {
	unit := graph.units[id]
	files := append(append([]string(nil), unit.Sources...), unit.Tests...)
	if len(files) == 0 {
		return false
	}
	directory := relativeDirectory(files[0])
	_, locates := graph.causes[id]
	anchored := rootAnchored(unit.PathTokens) || locates
	parts := strings.Split(probe, "/")
	ancestor := func(base, value string) bool {
		resolved := path.Join(base, value)
		return resolved != "." && resolved != "" && strings.HasPrefix(record, resolved+"/") && resolved+"/" != record && resolvesWithin(base, value, probe)
	}
	for _, value := range unit.PathTokens {
		if !namesPath(value, parts) {
			continue
		}
		if rest, rooted := strings.CutPrefix(value, "/"); rooted {
			if ancestor("", rest) {
				return true
			}
			continue
		}
		if ancestor(directory, value) || (anchored && ancestor("", value)) {
			return true
		}
	}
	return false
}

// globItem is one element of a glob component in path.Match syntax: a `*`,
// a `?`, a character class kept whole for path.Match to decide, or one
// literal rune.
type globItem struct {
	star, any bool
	class     string
	char      rune
}

func (item globItem) admits(char rune) bool {
	switch {
	case item.star || item.any:
		return char != '/'
	case item.class != "":
		matched, err := path.Match(item.class, string(char))
		return err == nil && matched
	}
	return char == item.char
}

// globEntry returns a name that the glob component pattern matches and that
// ends with suffix after at least one byte, when pattern holds a wildcard (a
// `*`, a `?` or a class): the glob then reads entries no literal names, such
// as a new entry of a record directory whose names end in suffix. A pattern
// with no wildcard names one file and returns false, as does a malformed one
// or one that admits no such name (`*.json` for `.md`). It searches the
// product of pattern's items with the longest prefix of suffix the name read
// so far ends with, over the runes of suffix and pattern and the printable
// ASCII, which cover every class of rune the search tells apart
// (AFP-V0-2747).
func globEntry(pattern, suffix string) (string, bool) {
	items, wild := globItems(pattern)
	if !wild {
		return "", false
	}
	runes := globAlphabet(pattern, suffix, items)
	// A state is the next item, how much of suffix the name ends with, and
	// its length in runes up to one more than suffix.
	type state struct{ item, matched, length int }
	type step struct {
		from  state
		char  rune
		reads bool // false for a `*` that matches nothing more
	}
	full := len(suffix) + 1
	start := state{}
	steps := map[state]step{start: {}}
	queue := []state{start}
	push := func(next, from state, char rune, reads bool) {
		if _, seen := steps[next]; !seen {
			steps[next] = step{from, char, reads}
			queue = append(queue, next)
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.item == len(items) {
			if current.matched != len(suffix) || current.length != full {
				continue
			}
			var name []rune
			for at := current; at != start; at = steps[at].from {
				if steps[at].reads {
					name = append(name, steps[at].char)
				}
			}
			for left, right := 0, len(name)-1; left < right; left, right = left+1, right-1 {
				name[left], name[right] = name[right], name[left]
			}
			return string(name), true
		}
		item := items[current.item]
		if item.star {
			push(state{current.item + 1, current.matched, current.length}, current, 0, false)
		}
		for _, char := range runes {
			if !item.admits(char) {
				continue
			}
			next := state{current.item + 1, suffixPrefix(suffix, current.matched, char), min(current.length+1, full)}
			if item.star {
				next.item = current.item
			}
			push(next, current, char, true)
		}
	}
	return "", false
}

// globAlphabet returns the runes globEntry and globCovers search: the
// printable ASCII, the runes of suffix and pattern, two non-ASCII letters,
// and each bound of a class of items with the runes just outside it, so every
// run of runes that the items and suffix tell apart has a member; never `/`.
// Letters and digits come first, so a witness reads as a file name.
func globAlphabet(pattern, suffix string, items []globItem) []rune {
	alphabet := map[rune]bool{}
	for char := rune(' '); char <= '~'; char++ {
		alphabet[char] = true
	}
	for _, char := range suffix + pattern + "é世" {
		alphabet[char] = true
	}
	for _, item := range items {
		for _, char := range classBounds(item.class) {
			switch {
			case 0xD800 <= char && char <= 0xDFFF:
				// No rune is a surrogate: the runes beside the block
				// stand for it.
				alphabet[0xD7FF], alphabet[0xE000] = true, true
			case utf8.ValidRune(char):
				alphabet[char] = true
			}
		}
	}
	delete(alphabet, '/')
	return slices.SortedFunc(maps.Keys(alphabet), func(a, b rune) int {
		if alnum(a) != alnum(b) {
			if alnum(a) {
				return -1
			}
			return 1
		}
		return int(a - b)
	})
}

// classBounds returns, for each range or rune of the character class class
// (`[a-z]`, `[^.]`, as globItems keeps it), its bounds and the runes just
// outside them; it returns nil for "", which is no class.
func classBounds(class string) []rune {
	body := strings.TrimPrefix(strings.TrimPrefix(class, "["), "^")
	body = strings.TrimSuffix(body, "]")
	read := func() rune {
		if body[0] == '\\' && len(body) > 1 {
			body = body[1:]
		}
		char, size := utf8.DecodeRuneInString(body)
		body = body[size:]
		return char
	}
	var bounds []rune
	for body != "" {
		low := read()
		high := low
		if len(body) > 1 && body[0] == '-' {
			body = body[1:]
			high = read()
		}
		bounds = append(bounds, low-1, low, high, high+1)
	}
	return bounds
}

// globItems splits a glob component into its items and reports whether one
// is a wildcard; a malformed pattern reports no wildcard.
func globItems(pattern string) ([]globItem, bool) {
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, false
	}
	var items []globItem
	wild := false
	for offset := 0; offset < len(pattern); {
		switch pattern[offset] {
		case '*':
			items, wild, offset = append(items, globItem{star: true}), true, offset+1
			continue
		case '?':
			items, wild, offset = append(items, globItem{any: true}), true, offset+1
			continue
		case '[':
			end := offset + 1
			if end < len(pattern) && pattern[end] == '^' {
				end++
			}
			for first := true; end < len(pattern) && (pattern[end] != ']' || first); first = false {
				if pattern[end] == '\\' {
					end++
				}
				end++
			}
			if end >= len(pattern) {
				return nil, false
			}
			items, wild, offset = append(items, globItem{class: pattern[offset : end+1]}), true, end+1
			continue
		case '\\':
			offset++
		}
		char, size := utf8.DecodeRuneInString(pattern[offset:])
		items, offset = append(items, globItem{char: char}), offset+size
	}
	return items, wild
}

// suffixPrefix returns the length of the longest prefix of suffix that the
// name ends with once char follows a name ending with suffix[:matched].
func suffixPrefix(suffix string, matched int, char rune) int {
	text := suffix[:matched] + string(char)
	for length := min(len(suffix), len(text)); length > 0; length-- {
		if strings.HasSuffix(text, suffix[:length]) {
			return length
		}
	}
	return 0
}

func alnum(char rune) bool {
	return 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z' || '0' <= char && char <= '9'
}
