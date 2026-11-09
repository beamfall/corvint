// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"fmt"
	"go/parser"
	"go/token"
	"slices"
	"strings"
)

// RecordEntryScopesPath holds audited reads of added record entries. Unlike a
// general read scope, this declaration never narrows modifications or deletions.
const RecordEntryScopesPath = ".corvint/record-entry-read-scopes.json"

const RecordEntryScopesProfile = "corvint-record-entry-read-scopes/0"

// RecordEntryScope binds a reviewed read scope to all of a unit's source and
// test bytes. Reads contains exact record filenames or either record directory
// for a consumer whose result depends on the set of entries. An empty list
// declares fixture-only/name-only uses, not a repository read (AFP-V0-3420).
type RecordEntryScope struct {
	SourceSHA256 string `json:"sourceSha256"`
	// Inputs names data that supplies filenames to the reader (for example a
	// ledger and its receipts). Those bytes participate in the binding too.
	Inputs []string `json:"inputs,omitempty"`
	Reads  []string `json:"reads"`
	Reason string   `json:"reason"`
}

// ReadRecordEntryScopes reads the bounded, closed declaration. Callers must
// retain readers on any error, and validate SourceSHA256 before narrowing.
func ReadRecordEntryScopes(root string) (map[string]RecordEntryScope, error) {
	body, err := DiskSource(root).ReadBounded(RecordEntryScopesPath, min(1<<20, MaxSourceBytes))
	if err != nil {
		return nil, err
	}
	var declared struct {
		Profile string                      `json:"profile"`
		Units   map[string]RecordEntryScope `json:"units"`
	}
	if err := json.Unmarshal(body, &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if declared.Profile != RecordEntryScopesProfile || declared.Units == nil || len(declared.Units) > 256 {
		return nil, fmt.Errorf("invalid record entry scope profile or units")
	}
	for id, scope := range declared.Units {
		digest, err := hex.DecodeString(scope.SourceSHA256)
		if (!strings.HasPrefix(id, "go:") && !strings.HasPrefix(id, "shell:")) || err != nil || len(digest) != sha256.Size || scope.Reads == nil || len(scope.Reads) > 256 || strings.TrimSpace(scope.Reason) == "" || len(scope.Reason) > 2048 {
			return nil, fmt.Errorf("invalid record entry scope for %q", id)
		}
		if len(scope.Inputs) > 4096 {
			return nil, fmt.Errorf("too many record entry inputs for %q", id)
		}
		for i, input := range scope.Inputs {
			if !ValidRelativePath(input) || (i > 0 && scope.Inputs[i-1] >= input) {
				return nil, fmt.Errorf("invalid record entry input %q for %q", input, id)
			}
		}
		for i, entry := range scope.Reads {
			_, record := recordDirectory(entry)
			if !ValidRelativePath(strings.TrimSuffix(entry, "/")) || (!record && entry != SealedChangesDirectory && entry != BuildLogDirectory) || (i > 0 && scope.Reads[i-1] >= entry) {
				return nil, fmt.Errorf("invalid record entry read %q for %q", entry, id)
			}
		}
	}
	return declared.Units, nil
}

// RecordEntrySourceDigest binds filenames as well as bytes, so a new test in
// an existing unit invalidates its scope. Unreadable, oversized or unparseable
// Go input cannot certify a narrowing (AFP-V0-3421). This is a source binding,
// not inferred data flow: the declaration's reason must cite the actual reads.
func RecordEntrySourceDigest(root string, unit Unit, inputs ...string) (string, error) {
	files := NormalizePaths(append(append(slices.Clone(unit.Sources), unit.Tests...), inputs...))
	if len(files) == 0 || len(files) > 4096 || unit.PathTokensBounded {
		return "", fmt.Errorf("record reader source inventory is unknown")
	}
	hash := sha256.New()
	hash.Write([]byte("corvint-record-entry-sources/0\n"))
	reader := DiskSource(root)
	remaining := 32 << 20
	for _, name := range files {
		body, err := reader.ReadBounded(name, min(MaxSourceBytes, remaining))
		if err != nil {
			return "", err
		}
		remaining -= len(body)
		if remaining <= 0 {
			return "", fmt.Errorf("record reader sources exceed byte bound")
		}
		if strings.HasSuffix(name, ".go") {
			if _, err := parser.ParseFile(token.NewFileSet(), name, body, 0); err != nil {
				return "", err
			}
		}
		fmt.Fprintf(hash, "%d:%s:%d:", len(name), name, len(body))
		hash.Write(body)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// AddedRecordEntries observes Git A status against an immutable base and the
// current index/worktree. Untracked paths, M, D and rename sources never enter
// this set. Failure returns no proof, preserving the existing reader rules.
func AddedRecordEntries(ctx context.Context, git, root, base string, dirty []string) []string {
	if len(base) != 40 || strings.ContainsAny(base, "^~:/ ") || slices.Contains(dirty, RecordEntryScopesPath) {
		return nil
	}
	if _, err := hex.DecodeString(base); err != nil {
		return nil
	}
	hasRecord := slices.ContainsFunc(dirty, func(p string) bool { _, ok := recordDirectory(p); return ok })
	if !hasRecord {
		return nil
	}
	deadline, cancel := context.WithTimeout(ctx, StatusDeadline)
	defer cancel()
	raw, err := boundedGitRunner(git)(deadline, root, MaxStatusBytes,
		"diff", "--name-status", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", base, "--", SealedChangesDirectory, BuildLogDirectory)
	if err != nil || len(raw) >= MaxStatusBytes {
		return nil
	}
	return addedRecordEntries(raw, dirty)
}

func addedRecordEntries(raw []byte, dirty []string) []string {
	if len(raw) == 0 {
		return nil
	}
	fields := strings.Split(string(raw), "\x00")
	if fields[len(fields)-1] != "" || len(fields)%2 != 1 {
		return nil
	}
	var added []string
	for i := 0; i < len(fields)-1; i += 2 {
		status, name := fields[i], fields[i+1]
		if !slices.Contains([]string{"A", "M", "D", "T", "U"}, status) || !ValidRelativePath(name) {
			return nil
		}
		if _, record := recordDirectory(name); record && status == "A" && slices.Contains(dirty, name) {
			added = append(added, name)
		}
	}
	return added
}

// WithRecordEntryBase adds per-change evidence, never cached graph state.
// Unknown scopes and stale source bindings retain today's selection. A path
// probe or materialized snapshot must not call this worktree-status method.
func (graph *Graph) WithRecordEntryBase(ctx context.Context, git, root, base string, dirty []string) *Graph {
	// Each comparison starts from no exclusion evidence, so a reused graph
	// never keeps an earlier comparison's additions or scopes.
	copy := *graph
	copy.addedRecords, copy.recordEntryScopes = nil, nil
	added := AddedRecordEntries(ctx, git, root, base, dirty)
	if len(added) == 0 {
		return &copy
	}
	scopes, err := ReadRecordEntryScopes(root)
	if err != nil {
		return &copy
	}
	copy.addedRecords = added
	copy.recordEntryScopes = make(map[string][]string)
	for id, scope := range scopes {
		unit, ok := graph.units[id]
		if !ok {
			continue
		}
		digest, err := RecordEntrySourceDigest(root, unit, scope.Inputs...)
		if err == nil && digest == scope.SourceSHA256 {
			copy.recordEntryScopes[id] = slices.Clone(scope.Reads)
		}
	}
	return &copy
}

// RecordEntryAllows reports whether reader rules may select id for dirtyPath.
// It only excludes a proven addition outside a current audited scope. Other
// reasons (imports, ownership, unbounded reads) are unaffected (AFP-V0-3422).
func (graph *Graph) RecordEntryAllows(id, dirtyPath string) bool {
	if !slices.Contains(graph.addedRecords, dirtyPath) {
		return true
	}
	reads, known := graph.recordEntryScopes[id]
	if !known {
		return true
	}
	return slices.ContainsFunc(reads, func(entry string) bool {
		return entry == dirtyPath || strings.HasSuffix(entry, "/") && strings.HasPrefix(dirtyPath, entry)
	})
}
