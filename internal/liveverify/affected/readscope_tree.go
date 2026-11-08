// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// ReadScopeTree narrows selection, never filesystem access (AFP-V0-3300).
// A missing or invalid predicate remains a whole-tree read until repaired.
type ReadScopeTree struct {
	Tree             string   `json:"tree"`
	ContentPredicate string   `json:"contentPredicate,omitempty"`
	Samples          []string `json:"samples,omitempty"`
	ListingOnly      bool     `json:"listingOnly,omitempty"`
}

// ReadScopeEntry admits the legacy string and the additive tree object.
type ReadScopeEntry struct {
	Path string
	Rule *ReadScopeTree
}

func (entry *ReadScopeEntry) UnmarshalJSON(body []byte) error {
	*entry = ReadScopeEntry{}
	if len(body) > 0 && body[0] == '"' {
		return json.Unmarshal(body, &entry.Path)
	}
	var rule ReadScopeTree
	if err := json.Unmarshal(body, &rule, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	if !strings.HasSuffix(rule.Tree, "/") || !ValidReadScopeEntry(rule.Tree) {
		return fmt.Errorf("tree scope must name a canonical subtree: %q", rule.Tree)
	}
	entry.Path, entry.Rule = rule.Tree, &rule
	return nil
}

// ReadScopeIdentities preserves all selection metadata when comparing the
// BASE and HEAD declarations. Legacy entries retain their string identity.
func ReadScopeIdentities(scope []string, rules []ReadScopeTree) []string {
	identities := slices.Clone(scope)
	for i, entry := range scope {
		for _, rule := range rules {
			if rule.Tree == entry {
				body, _ := json.Marshal(rule)
				identities[i] = string(body)
			}
		}
	}
	return identities
}

// Validate checks the positive examples supplied from the guard's own
// matches, including the empty-match refusal (AFP-V0-3302). These examples
// are reviewed authority, not a proof that a regex covers arbitrary code.
func (rule ReadScopeTree) Validate() error {
	if rule.ListingOnly {
		if rule.ContentPredicate != "" || len(rule.Samples) != 0 {
			return fmt.Errorf("listingOnly cannot carry a content predicate or samples")
		}
		return nil
	}
	if rule.ContentPredicate == "" {
		return fmt.Errorf("contentPredicate is missing")
	}
	predicate, err := regexp.Compile(rule.ContentPredicate)
	if err != nil {
		return fmt.Errorf("contentPredicate does not compile: %w", err)
	}
	if predicate.MatchString("") {
		return fmt.Errorf("contentPredicate matches the empty string")
	}
	if len(rule.Samples) == 0 {
		return fmt.Errorf("contentPredicate needs positive sample lines from the guard")
	}
	for _, sample := range rule.Samples {
		if !predicate.MatchString(sample) {
			return fmt.Errorf("contentPredicate does not cover sample %q", sample)
		}
	}
	return nil
}

type readScopeInputs struct {
	content map[string]DriverRevision
	listing map[string]bool // present only when both direct-child listings are known
}

// WithReadScopeSources observes each required file or listing once. No
// revision data enters the reusable graph cache (AFP-V0-3303).
func (graph *Graph) WithReadScopeSources(base, head *Source, dirty []string) *Graph {
	copied := *graph
	inputs := &readScopeInputs{content: map[string]DriverRevision{}, listing: map[string]bool{}}
	copied.readScopeInputs = inputs
	if graph.sparse || base == nil || head == nil {
		return &copied
	}
	listed, read := map[string]bool{}, map[string]bool{}
	dirty = NormalizePaths(dirty)
	for _, id := range graph.order {
		unit := graph.units[id]
		for _, rule := range unit.ReadScopeTrees {
			for _, name := range dirty {
				if !scopeEntryHolds(rule.Tree, name) {
					continue
				}
				if rule.ListingOnly {
					if !listed[rule.Tree] {
						listed[rule.Tree] = true
						directory := strings.TrimSuffix(rule.Tree, "/")
						before, errBefore := base.ReadDir(directory)
						after, errAfter := head.ReadDir(directory)
						if errBefore == nil && errAfter == nil {
							inputs.listing[rule.Tree] = !sameReadScopeListing(before, after)
						}
					}
				} else if !read[name] {
					read[name] = true
					before, errBefore := base.ReadBounded(name, MaxSourceBytes)
					after, errAfter := head.ReadBounded(name, MaxSourceBytes)
					if errBefore == nil && errAfter == nil {
						inputs.content[name] = DriverRevision{Base: before, Head: after, BaseExists: true, HeadExists: true}
					}
				}
			}
		}
	}
	return &copied
}

func sameReadScopeListing(before, after []fs.DirEntry) bool {
	if len(before) != len(after) {
		return false
	}
	for i, entry := range before {
		other := after[i]
		// Unknown entry kinds are never evidence for exclusion.
		if entry.Type() != 0 && entry.Type() != fs.ModeDir || other.Type() != 0 && other.Type() != fs.ModeDir || entry.Name() != other.Name() || entry.Type() != other.Type() {
			return false
		}
	}
	return true
}

func scopeEntryHolds(entry, name string) bool {
	target := strings.TrimSuffix(entry, "/")
	return name == target || strings.HasPrefix(target, name+"/") || target != entry && strings.HasPrefix(name, entry)
}

func (graph *Graph) inReadScope(directory string, unit Unit, name string) bool {
	if name == ReadScopesPath || directory == "." || name == directory || strings.HasPrefix(name, directory+"/") {
		return true
	}
	for _, entry := range unit.ReadScope {
		if graph.scopeEntryBears(unit, entry, name) {
			return true
		}
	}
	return false
}

func (graph *Graph) scopeEntryBears(unit Unit, entry, name string) bool {
	if !scopeEntryHolds(entry, name) {
		return false
	}
	for _, rule := range unit.ReadScopeTrees {
		if rule.Tree == entry {
			return graph.treeScopeBears(rule, name)
		}
	}
	return true
}

func (graph *Graph) treeReadScopeCause(id, directory string, unit Unit, name string) *ReadScopeCause {
	cause := readScopeCause(id, directory, nil, name)
	if name == ReadScopesPath || name == ShellReadScopesPath {
		return cause
	}
	for _, entry := range unit.ReadScope {
		if graph.scopeEntryBears(unit, entry, name) {
			cause.Entry = entry
			break
		}
	}
	return cause
}

func (graph *Graph) treeScopeBears(rule ReadScopeTree, name string) bool {
	// A record for the tree itself or an ancestor has no ordinary child
	// change kind. In particular, never exclude a directory/symlink swap.
	if name == strings.TrimSuffix(rule.Tree, "/") || strings.HasPrefix(rule.Tree, name+"/") {
		return true
	}
	if rule.Validate() != nil || graph.readScopeInputs == nil {
		return true
	}
	if rule.ListingOnly {
		changed, known := graph.readScopeInputs.listing[rule.Tree]
		return !known || changed
	}
	revision, known := graph.readScopeInputs.content[name]
	if !known || !revision.BaseExists || !revision.HeadExists {
		return true
	}
	predicate, err := regexp.Compile(rule.ContentPredicate)
	if err != nil {
		return true
	}
	for _, body := range [][]byte{revision.Base, revision.Head} {
		if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 || predicate.Match(body) {
			return true
		}
	}
	return false
}
