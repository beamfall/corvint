// Package compactionkernel makes compaction survival a checkable property. A
// kernel is the smallest pinned statement of what governs a session: the tree
// revision, the governing authority files with their blob hashes, and an
// optional bounded list of requirement ids with their pinned spec blob hashes.
// It is rendered into a fenced envelope a host hook injects, and verified again
// out of whatever text survived compaction.
//
// Claude Code's PreCompact hook should call `corvint kernel` and carry the
// rendered block into the compacted transcript; the following
// SessionStart(compact) hook should pipe that transcript into
// `corvint kernel verify`. A host that emits no compaction signal degrades to
// the session-start injection alone; nothing here depends on a host-specific
// compaction API.
package compactionkernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	// Profile names this kernel's wire shape.
	Profile = "corvint-compaction-kernel/0"
	// MaxKernelBytes bounds the canonical JSON document, envelope excluded. The
	// kernel must stay small enough that no compaction budget can justify
	// dropping it (CKN-V0-002).
	MaxKernelBytes = 600
	// maxRequirements bounds the requested requirement list before any sizing,
	// so an absurd request is refused rather than silently truncated.
	maxRequirements = 16
	// requirementsIndexPath is the pinned map from requirement id to spec file.
	requirementsIndexPath = "docs/specs/REQUIREMENTS.tsv"

	beginMarker = "BEGIN CORVINT KERNEL"
	endMarker   = "END CORVINT KERNEL"
	fence       = "```"
)

// Entry is one row of the harness governance array, in the shape the
// user-prompt packet already produces.
type Entry struct {
	Authority string
	BlobHash  string
	Line      int
	Path      string
	Relation  string
}

// Authority is one governing file pinned to the blob the index read.
type Authority struct {
	Authority string
	BlobHash  string
	Path      string
}

// Requirement binds a requirement id to the pinned blob of the spec that
// states it.
type Requirement struct {
	BlobHash string
	ID       string
	Line     int
	Path     string
}

// Kernel is the compaction-survival statement itself.
type Kernel struct {
	Authorities  []Authority
	Digest       string
	Omitted      int
	Requirements []Requirement
	Revision     string
}

// Mismatch names one specific way a recovered kernel no longer matches the
// repository. It never carries source content.
type Mismatch struct {
	Actual   string `json:"actual"`
	Expected string `json:"expected"`
	Path     string `json:"path"`
	Reason   string `json:"reason"`
}

// Verdict is the result of checking recovered text against the current index.
type Verdict struct {
	Digest     string     `json:"digest"`
	Mismatches []Mismatch `json:"mismatches"`
	Revision   string     `json:"revision"`
	State      string     `json:"state"`
}

// Build compiles the kernel for one index. governance is the harness
// governance array; requirementIDs is optional.
func Build(index *contextindex.Index, governance []Entry, requirementIDs []string) (Kernel, error) {
	if index == nil || !objectID(index.Revision) {
		return Kernel{}, &gokernel.Error{Code: "invalid-kernel-index", Message: "compaction kernel requires an immutable index"}
	}
	authorities, err := governingAuthorities(index, governance)
	if err != nil {
		return Kernel{}, err
	}
	requirements, err := resolveRequirements(index, requirementIDs)
	if err != nil {
		return Kernel{}, err
	}
	kernel := Kernel{Authorities: authorities, Requirements: requirements, Revision: index.Revision}
	return fitted(kernel)
}

// InjectionBlock is the orchestrator's entry point: the exact block the
// session-start and user-prompt adapter paths inject, with no requirement list.
func InjectionBlock(index *contextindex.Index, governance []Entry) (string, error) {
	kernel, err := Build(index, governance, nil)
	if err != nil {
		return "", err
	}
	return Render(kernel), nil
}

// GovernanceFromPacket converts the `governance` member of a harness packet
// into kernel entries, so no caller re-implements the authority selection the
// prompt compiler already made.
func GovernanceFromPacket(packet map[string]any) []Entry {
	rows, _ := packet["governance"].([]any)
	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		value, ok := row.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, Entry{
			Authority: text(value["authority"]),
			BlobHash:  text(value["blob_hash"]),
			Line:      number(value["line"]),
			Path:      text(value["path"]),
			Relation:  text(value["relation"]),
		})
	}
	return entries
}

// Render produces the exact envelope a hook injects: a fenced block whose body
// is the marker, the canonical JSON on one line, and the closing marker.
func Render(kernel Kernel) string {
	encoded, err := gokernel.CanonicalJSON(document(kernel))
	if err != nil {
		// document is built from strings and ints only; canonical encoding of
		// that shape cannot fail, so an error here is not a reachable state.
		return ""
	}
	return fence + "\n" + beginMarker + "\n" + string(encoded) + "\n" + endMarker + "\n" + fence + "\n"
}

// Verify finds the envelope in arbitrary text -- a post-compaction summary, a
// transcript, a pasted note -- recomputes the digest, and checks every pinned
// blob hash against the current index.
func Verify(text string, index *contextindex.Index) (Verdict, error) {
	if index == nil {
		return Verdict{}, &gokernel.Error{Code: "invalid-kernel-index", Message: "compaction kernel verification requires an index"}
	}
	body, found, agreeing := envelopeBody(text)
	if !found {
		return Verdict{State: "missing", Mismatches: []Mismatch{}}, nil
	}
	if !agreeing {
		return corrupt("text carries kernel envelopes that disagree"), nil
	}
	recovered, err := decodeDocument(body)
	if err != nil {
		return corrupt(err.Error()), nil
	}
	// The digest covers decoded members, so re-spaced or duplicate-member
	// bodies would still match it; the body must be the canonical bytes.
	canonical, err := gokernel.CanonicalJSON(recovered)
	if err != nil || string(canonical) != body {
		return corrupt("kernel body is not canonical JSON"), nil
	}
	claimed, ok := recovered["digest"].(string)
	if !ok {
		return corrupt("kernel carries no digest"), nil
	}
	delete(recovered, "digest")
	recomputed, err := digestOf(recovered)
	if err != nil {
		return corrupt("kernel is not canonically encodable"), nil
	}
	if recomputed != claimed {
		return corrupt("digest does not cover the kernel body"), nil
	}
	if !wellFormed(recovered) {
		return corrupt("kernel pins no governing authority"), nil
	}
	mismatches := pinMismatches(recovered, index)
	verdict := Verdict{
		Digest:     claimed,
		Mismatches: mismatches,
		Revision:   stringMember(recovered, "revision"),
		State:      "intact",
	}
	if len(mismatches) != 0 {
		verdict.State = "stale"
	}
	return verdict, nil
}

func governingAuthorities(index *contextindex.Index, governance []Entry) ([]Authority, error) {
	seen := make(map[string]struct{}, len(governance))
	authorities := make([]Authority, 0, len(governance))
	for _, entry := range governance {
		if entry.Relation != "governing" || entry.Path == "" {
			continue
		}
		if _, duplicate := seen[entry.Path]; duplicate {
			continue
		}
		source, tracked := index.Sources[entry.Path]
		if !tracked || source.BlobHash == "" {
			return nil, &gokernel.Error{Code: "unsupported-kernel-authority", Message: "governing authority is not pinned in this index"}
		}
		seen[entry.Path] = struct{}{}
		authorities = append(authorities, Authority{Authority: entry.Authority, BlobHash: source.BlobHash, Path: entry.Path})
	}
	if len(authorities) == 0 {
		return nil, &gokernel.Error{Code: "unsupported-kernel-authority", Message: "compaction kernel requires at least one governing authority"}
	}
	sort.Slice(authorities, func(i, j int) bool { return authorities[i].Path < authorities[j].Path })
	return authorities, nil
}

// resolveRequirements reads the requirement index and the spec blob hashes out
// of the pinned sources, never the worktree, so a kernel built from an older
// revision cites that revision's specs (CKN-V0-003).
func resolveRequirements(index *contextindex.Index, requirementIDs []string) ([]Requirement, error) {
	if len(requirementIDs) == 0 {
		return []Requirement{}, nil
	}
	if len(requirementIDs) > maxRequirements {
		return nil, &gokernel.Error{Code: "unsupported-kernel-requirement", Message: "requirement list exceeds the bounded contract"}
	}
	rows, err := requirementRows(index)
	if err != nil {
		return nil, err
	}
	requirements := make([]Requirement, 0, len(requirementIDs))
	seen := make(map[string]struct{}, len(requirementIDs))
	for _, id := range requirementIDs {
		if _, repeated := seen[id]; repeated {
			continue
		}
		seen[id] = struct{}{}
		row, known := rows[id]
		if !known {
			return nil, &gokernel.Error{Code: "unsupported-kernel-requirement", Message: "requirement id is absent from the pinned requirement index"}
		}
		source, tracked := index.Sources[row.Path]
		if !tracked || source.BlobHash == "" {
			return nil, &gokernel.Error{Code: "unsupported-kernel-requirement", Message: "requirement spec is not pinned in this index"}
		}
		row.BlobHash = source.BlobHash
		requirements = append(requirements, row)
	}
	sort.Slice(requirements, func(i, j int) bool { return requirements[i].ID < requirements[j].ID })
	return requirements, nil
}

func requirementRows(index *contextindex.Index) (map[string]Requirement, error) {
	source, tracked := index.Sources[requirementsIndexPath]
	if !tracked {
		return nil, &gokernel.Error{Code: "unsupported-kernel-requirement", Message: "the pinned requirement index is absent"}
	}
	body, valid, loaded := source.Text()
	if !valid || !loaded {
		return nil, &gokernel.Error{Code: "unsupported-kernel-requirement", Message: "the pinned requirement index is unreadable"}
	}
	rows := make(map[string]Requirement)
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 || fields[0] == "id" {
			continue
		}
		number, err := strconv.Atoi(fields[2])
		if err != nil || number < 1 {
			continue
		}
		rows[fields[0]] = Requirement{ID: fields[0], Line: number, Path: fields[1]}
	}
	return rows, nil
}

// fitted drops requirement rows from the tail until the canonical document fits
// the byte bound, then refuses if the authorities alone still exceed it.
func fitted(kernel Kernel) (Kernel, error) {
	for {
		kernel.Digest = ""
		digest, err := digestOf(payload(kernel))
		if err != nil {
			return Kernel{}, err
		}
		kernel.Digest = digest
		encoded, err := gokernel.CanonicalJSON(document(kernel))
		if err != nil {
			return Kernel{}, err
		}
		if len(encoded) <= MaxKernelBytes {
			return kernel, nil
		}
		if len(kernel.Requirements) == 0 {
			return Kernel{}, &gokernel.Error{Code: "unsupported-kernel-budget", Message: "governing authorities alone exceed the compaction kernel bound"}
		}
		kernel.Requirements = kernel.Requirements[:len(kernel.Requirements)-1]
		kernel.Omitted++
	}
}

func payload(kernel Kernel) map[string]any {
	authorities := make([]any, 0, len(kernel.Authorities))
	for _, authority := range kernel.Authorities {
		authorities = append(authorities, map[string]any{
			"authority": authority.Authority, "blob_hash": authority.BlobHash, "path": authority.Path,
		})
	}
	requirements := make([]any, 0, len(kernel.Requirements))
	for _, requirement := range kernel.Requirements {
		requirements = append(requirements, map[string]any{
			"blob_hash": requirement.BlobHash, "id": requirement.ID,
			"line": requirement.Line, "path": requirement.Path,
		})
	}
	return map[string]any{
		"authorities": authorities, "omitted": kernel.Omitted, "profile": Profile,
		"requirements": requirements, "revision": kernel.Revision,
	}
}

func document(kernel Kernel) map[string]any {
	value := payload(kernel)
	value["digest"] = kernel.Digest
	return value
}

func digestOf(value map[string]any) (string, error) {
	encoded, err := gokernel.CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// envelopeBody extracts the one JSON line between the markers. Surrounding
// prose, fences and indentation are ignored: the markers are the contract.
// Every envelope in the text must carry the same body; the third result is
// false when two disagree.
func envelopeBody(text string) (string, bool, bool) {
	lines := strings.Split(text, "\n")
	body, found := "", false
	for number, line := range lines {
		if strings.TrimSpace(line) != beginMarker {
			continue
		}
		if number+2 >= len(lines) || strings.TrimSpace(lines[number+2]) != endMarker {
			return "", true, true // present but unterminated: corrupt, never missing.
		}
		next := strings.TrimSpace(lines[number+1])
		if !found {
			body, found = next, true
			continue
		}
		if next != body {
			return "", true, false // a repeated block is one kernel; differing ones are none.
		}
	}
	return body, found, true
}

// decodeDocument keeps numbers as json.Number so re-encoding the recovered
// members reproduces the exact bytes the digest covered.
func decodeDocument(body string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, &gokernel.Error{Code: "unsupported-kernel-envelope", Message: "kernel envelope is not one canonical JSON object"}
	}
	if decoder.More() {
		return nil, &gokernel.Error{Code: "unsupported-kernel-envelope", Message: "kernel envelope carries trailing content"}
	}
	return value, nil
}

func pinMismatches(recovered map[string]any, index *contextindex.Index) []Mismatch {
	mismatches := make([]Mismatch, 0)
	mismatches = append(mismatches, sectionMismatches(recovered, "authorities", "authority-blob-changed", "authority-unavailable", index)...)
	mismatches = append(mismatches, sectionMismatches(recovered, "requirements", "requirement-blob-changed", "requirement-unavailable", index)...)
	return mismatches
}

func sectionMismatches(recovered map[string]any, member, reason, unavailable string, index *contextindex.Index) []Mismatch {
	rows, _ := recovered[member].([]any)
	mismatches := make([]Mismatch, 0, len(rows))
	for _, row := range rows {
		value, ok := row.(map[string]any)
		if !ok {
			continue
		}
		path, pinned := text(value["path"]), text(value["blob_hash"])
		source, tracked := index.Sources[path]
		if !tracked {
			mismatches = append(mismatches, Mismatch{Actual: "", Expected: pinned, Path: path, Reason: unavailable})
			continue
		}
		if source.BlobHash != pinned {
			mismatches = append(mismatches, Mismatch{Actual: source.BlobHash, Expected: pinned, Path: path, Reason: reason})
		}
	}
	return mismatches
}

// wellFormed rejects a self-digested body that Build could never emit: a
// foreign profile, a revision that is not an object id, no governing
// authority, or a pin row without a path and object id. Such a body pins
// nothing, so it must not verify as intact.
func wellFormed(recovered map[string]any) bool {
	if text(recovered["profile"]) != Profile {
		return false
	}
	if !objectID(text(recovered["revision"])) {
		return false
	}
	if !integerAtLeast(recovered["omitted"], 0) {
		return false
	}
	authorities, _ := recovered["authorities"].([]any)
	if len(authorities) == 0 {
		return false
	}
	for _, row := range authorities {
		if !pinRow(row) {
			return false
		}
	}
	requirements, _ := recovered["requirements"].([]any)
	for _, row := range requirements {
		if !pinRow(row) {
			return false
		}
		if line, _ := row.(map[string]any); !integerAtLeast(line["line"], 1) {
			return false
		}
	}
	return true
}

// integerAtLeast reports whether a decoded member is the decimal integer Build
// would have encoded, no smaller than minimum: never a fraction, exponent,
// negative zero or string.
func integerAtLeast(value any, minimum int64) bool {
	literal, ok := value.(json.Number)
	if !ok {
		return false
	}
	parsed, err := strconv.ParseInt(string(literal), 10, 64)
	return err == nil && parsed >= minimum && strconv.FormatInt(parsed, 10) == string(literal)
}

func pinRow(row any) bool {
	value, ok := row.(map[string]any)
	if !ok || text(value["path"]) == "" {
		return false
	}
	return objectID(text(value["blob_hash"]))
}

func corrupt(reason string) Verdict {
	return Verdict{State: "corrupt", Mismatches: []Mismatch{{Reason: reason}}}
}

func stringMember(value map[string]any, member string) string { return text(value[member]) }

func text(value any) string {
	result, _ := value.(string)
	return result
}

func number(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0
		}
		return int(parsed)
	}
	return 0
}

func objectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}
