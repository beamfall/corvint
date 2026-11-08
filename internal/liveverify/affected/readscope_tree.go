// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"fmt"
	"io/fs"
	"maps"
	"path"
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

const ReadScopeTreesPath = ".corvint/test-read-scope-trees.json"
const ReadScopeTreesProfile = "corvint-test-read-scope-trees/0"
const MaxReadScopeTreesBytes = 1 << 16

// ReadScopeTreeValue preserves legacy reasons and optional narrowing metadata.
// The tree path remains the map key, never a member of the object value.
type ReadScopeTreeValue struct {
	Reason string
	Rule   *ReadScopeTree
}

func (value *ReadScopeTreeValue) UnmarshalJSON(body []byte) error {
	*value = ReadScopeTreeValue{}
	if len(body) > 0 && body[0] == '"' {
		return json.Unmarshal(body, &value.Reason)
	}
	var object struct {
		Reason           string   `json:"reason"`
		ContentPredicate string   `json:"contentPredicate,omitempty"`
		Samples          []string `json:"samples,omitempty"`
		ListingOnly      bool     `json:"listingOnly,omitempty"`
	}
	if err := json.Unmarshal(body, &object, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	value.Reason = object.Reason
	value.Rule = &ReadScopeTree{ContentPredicate: object.ContentPredicate, Samples: object.Samples, ListingOnly: object.ListingOnly}
	return nil
}

// ParseReadScopeTrees validates the optional metadata independently of the
// frozen string-only read-scope declaration. A planner discards all metadata
// on any error; checkers report the invalid package/tree to its author.
func ParseReadScopeTrees(body []byte) (map[string]map[string]ReadScopeTreeValue, error) {
	var record struct {
		Profile string                                   `json:"profile"`
		Trees   map[string]map[string]ReadScopeTreeValue `json:"trees"`
	}
	if len(body) > MaxReadScopeTreesBytes {
		return nil, fmt.Errorf("%s: metadata exceeds byte limit", ReadScopeTreesPath)
	}
	if err := json.Unmarshal(body, &record, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", ReadScopeTreesPath, err)
	}
	if record.Profile != ReadScopeTreesProfile || record.Trees == nil {
		return nil, fmt.Errorf("%s: want profile %s and trees", ReadScopeTreesPath, ReadScopeTreesProfile)
	}
	for _, directory := range slices.Sorted(maps.Keys(record.Trees)) {
		entries := record.Trees[directory]
		if directory == "." || !ValidRelativePath(directory) || path.Clean(directory) != directory || len(entries) == 0 {
			return nil, fmt.Errorf("%s: invalid package %q", ReadScopeTreesPath, directory)
		}
		for _, tree := range slices.Sorted(maps.Keys(entries)) {
			value := entries[tree]
			name, rest, slash := strings.Cut(tree, "/")
			if !slash || rest != "" || !ValidReadScopeEntry(tree) || name == "" || strings.TrimSpace(value.Reason) == "" {
				return nil, fmt.Errorf("%s: package %s entry %s needs a whole top-level tree and non-empty reason", ReadScopeTreesPath, directory, tree)
			}
			if value.Rule != nil {
				value.Rule.Tree = tree
				if err := value.Rule.Validate(); err != nil {
					return nil, fmt.Errorf("%s: package %s entry %s: %w (AFP-V0-3302)", ReadScopeTreesPath, directory, tree, err)
				}
			}
		}
	}
	return record.Trees, nil
}

func readScopeTrees(source *Source) (map[string]map[string]ReadScopeTreeValue, string) {
	body, err := source.ReadBounded(ReadScopeTreesPath, MaxReadScopeTreesBytes)
	if err != nil {
		return nil, ""
	}
	sum := sha256.Sum256(body)
	trees, err := ParseReadScopeTrees(body)
	if err != nil {
		trees = nil // malformed metadata never weakens the plain whole-tree scope
	}
	return trees, hex.EncodeToString(sum[:])
}

func (graph *Graph) applyReadScopeTrees(trees map[string]map[string]ReadScopeTreeValue) {
	for id, unit := range graph.units {
		files := append(append([]string(nil), unit.Sources...), unit.Tests...)
		if !strings.HasPrefix(id, "go:") || !unit.ReadScoped || len(files) == 0 {
			continue
		}
		for _, entry := range unit.ReadScope {
			if value := trees[relativeDirectory(files[0])][entry]; value.Rule != nil {
				unit.ReadScopeTrees = append(unit.ReadScopeTrees, *value.Rule)
			}
		}
		graph.units[id] = unit
	}
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
	if name == ReadScopesPath || name == ReadScopeTreesPath || directory == "." || name == directory || strings.HasPrefix(name, directory+"/") {
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
	if name == ReadScopeTreesPath {
		return &ReadScopeCause{Declaration: ReadScopeTreesPath, Entry: ReadScopeTreesPath}
	}
	cause := readScopeCause(id, directory, nil, name)
	if name == ReadScopesPath || name == ReadScopeTreesPath || name == ShellReadScopesPath {
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
