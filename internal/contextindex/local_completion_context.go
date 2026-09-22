package contextindex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PinnedIntentPointer is a caller-owned enrollment declaration, not authority.
// An empty BlobHash means the path was unavailable when enrollment was pinned.
type PinnedIntentPointer struct {
	Path     string `json:"path"`
	Revision string `json:"revision"`
	BlobHash string `json:"blob_hash"`
}

const localPromptMaxAnchors = 32
const localPromptMaxCandidates = 128

// DogfoodPromptContext implements only LCP-V0-010/011. It projects immutable
// selectors and fixed diagnostics; it never emits task fragments, reads history,
// or writes state. The caller owns index acquisition and its stability bracket.
// budgetBytes includes the canonical packet's terminating LF, not its enclosing
// event. Empty task is permitted for startup and remains unresolved; the event
// adapter must reject an empty user-prompt before calling this compiler.
// An envelope that cannot retain mandatory omissions is refused.
func DogfoodPromptContext(ctx context.Context, index *Index, task string, scope []PinnedIntentPointer, limit, budgetBytes int) (map[string]any, error) {
	if err := validateLocalPrompt(index, task, scope, limit, budgetBytes); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	compiler := &localPromptCompiler{ctx: ctx, index: index, task: strings.TrimSpace(task)}
	// Reuse TaskContext's authority/definition resolver with an empty subject;
	// never run its lexical slots or startHistory for this conservative profile.
	compiler.context = newTaskContextCompiler(index, compiler.task, "")
	compiler.addGovernance()
	compiler.addScope(scope)
	compiler.addAnchors()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if compiler.err != nil {
		return nil, compiler.err
	}
	return compiler.packet(limit, budgetBytes)
}

func validateLocalPrompt(index *Index, task string, scope []PinnedIntentPointer, limit, budget int) error {
	if index == nil || !localObjectID(index.Revision) {
		return &Error{Code: "invalid-dogfood-context", Message: "prompt context requires an immutable index"}
	}
	if !utf8.ValidString(task) || len(task) > 16_384 || utf8.RuneCountInString(task) > 2_000 {
		return &Error{Code: "invalid-dogfood-context", Message: "prompt exceeds the bounded task contract"}
	}
	if limit < 1 || limit > 50 || budget < 1 || budget > 8_000 || len(scope) > 16 {
		return &Error{Code: "invalid-dogfood-context", Message: "prompt limits or scope exceed the bounded contract"}
	}
	seen := map[string]bool{}
	for _, pointer := range scope {
		cleaned, err := cleanImpactPath(pointer.Path)
		if err != nil || cleaned != pointer.Path || seen[pointer.Path] || !localObjectID(pointer.Revision) || pointer.BlobHash != "" && !localObjectID(pointer.BlobHash) {
			return &Error{Code: "invalid-dogfood-context", Message: "invalid or duplicate pinned scope selector"}
		}
		seen[pointer.Path] = true
	}
	return nil
}

func localObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

type localPromptRow struct {
	section string
	value   map[string]any
}

type localPromptCompiler struct {
	ctx                                        context.Context
	err                                        error
	qualified                                  int
	index                                      *Index
	context                                    *taskContextCompiler
	task                                       string
	rows                                       []localPromptRow
	unavailable                                []any
	anchors, missing, ambiguous, unread, dirty int
	scopeStale, scopeUnavailable               int
	requirementsCapped                         bool
}

func (compiler *localPromptCompiler) addGovernance() {
	row, ok := compiler.context.governingRow()
	if ok {
		compiler.addEvidence("governance", row.path, row.line, "governing", "project-instructions")
		return
	}
	for _, candidate := range compiler.context.candidates[governingRelation] {
		compiler.unavailable = append(compiler.unavailable, localSelector("governance", candidate))
		break // TaskContext never substitutes a lower-precedence unread authority.
	}
}

func (compiler *localPromptCompiler) addScope(scope []PinnedIntentPointer) {
	ordered := append([]PinnedIntentPointer{}, scope...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	for _, pointer := range ordered {
		state := "current"
		source, exists := compiler.index.Sources[pointer.Path]
		switch {
		case pointer.BlobHash == "" || !exists:
			state = "unavailable"
			compiler.scopeUnavailable++
		case source.BlobHash != pointer.BlobHash || slices.Contains(compiler.index.DirtyPaths, pointer.Path):
			state = "stale"
			compiler.scopeStale++
		}
		compiler.rows = append(compiler.rows, localPromptRow{"declared_scope", map[string]any{
			"path": pointer.Path, "revision": pointer.Revision, "blob_hash": pointer.BlobHash,
			"state": state, "relation": "enrollment-declaration",
		}})
	}
}

// addAnchors recognizes only explicit selectors. It never treats the enrolled
// scope or the governing row as support for the words of the current task.
func (compiler *localPromptCompiler) addAnchors() {
	remaining := compiler.task
	ids := compiler.context.requirementIDs()
	paths := compiler.explicitPaths()
	for _, token := range append(append([]string{}, ids...), paths...) {
		remaining = strings.ReplaceAll(remaining, token, " ")
	}
	identifiers := localPromptIdentifiers(remaining)
	if len(ids)+len(paths)+len(identifiers) > localPromptMaxAnchors {
		compiler.failBounds()
		return
	}
	if len(ids) != 0 {
		definitions := compiler.definitions(ids)
		for _, id := range ids {
			if !compiler.ready() {
				return
			}
			compiler.resolveDefinitions(definitions[id])
		}
	}
	for _, token := range paths {
		if !compiler.ready() {
			return
		}
		compiler.resolvePath(token)
	}
	standalone := strings.Trim(remaining, " \t\r\n`?!")
	for _, identifier := range identifiers {
		if !compiler.ready() {
			return
		}
		exactStandalone := standalone == identifier.name && compiler.hasSymbol(identifier.name)
		if identifier.weight >= 3 || definitionEligible(identifier) || exactStandalone {
			compiler.resolveIdentifier(identifier.name)
		}
	}
}

// Reuse the clause grammar one bounded source at a time, allowing cancellation
// between source reads and retaining only definitions the caller named.
func (compiler *localPromptCompiler) definitions(ids []string) map[string][]specDefinition {
	found := map[string][]specDefinition{}
	for _, candidate := range compiler.context.specPaths() {
		if !compiler.ready() {
			break
		}
		definitions, capped := compiler.context.specDefinitions([]string{candidate})
		compiler.requirementsCapped = compiler.requirementsCapped || capped
		for _, id := range ids {
			found[id] = append(found[id], definitions[id]...)
			if len(found[id]) > localPromptMaxCandidates {
				compiler.failBounds()
				return found
			}
		}
	}
	return found
}

var localPromptIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var localPromptQualified = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+`)

func localPromptIdentifiers(task string) []taskIdentifier {
	weights := map[string]int{}
	// Preserve qualified names as one anchor. Never turn the namespace into a
	// second unknown identifier or silently assert the namespace is verified.
	for _, qualified := range localPromptQualified.FindAllString(task, -1) {
		weights[qualified] = 3
	}
	plain := localPromptQualified.ReplaceAllString(task, " ")
	for _, identifier := range taskIdentifiers(plain) {
		if definitionEligible(identifier) {
			weights[identifier.name] = identifier.weight
		}
	}
	for _, match := range contextBacktick.FindAllStringSubmatch(plain, -1) {
		weights[match[1]] = 3
	}
	standalone := strings.Trim(plain, " \t\r\n`?!")
	if localPromptIdentifier.MatchString(standalone) && weights[standalone] == 0 {
		weights[standalone] = 1
	}
	result := []taskIdentifier{}
	for name, weight := range weights {
		result = append(result, taskIdentifier{name: name, weight: weight})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result
}

func (compiler *localPromptCompiler) ready() bool {
	if compiler.err == nil {
		compiler.err = compiler.ctx.Err()
	}
	return compiler.err == nil
}

func (compiler *localPromptCompiler) failBounds() {
	compiler.err = &Error{Code: "unsupported-dogfood-context-bounds", Message: "explicit anchor context exceeds its bounded candidate profile"}
}

func (compiler *localPromptCompiler) explicitPaths() []string {
	found := []string{}
	for _, field := range strings.FieldsFunc(compiler.task, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("`\"'(),:;?!", r)
	}) {
		token := strings.TrimRight(strings.TrimPrefix(field, "./"), ".")
		if token == "" {
			continue
		}
		// A qualified declaration name is an identifier, not an unknown file.
		if path.Ext(token) != "" && !strings.Contains(token, "/") && !trackedPath(compiler.index, token) && compiler.hasSymbol(strings.TrimPrefix(path.Ext(token), ".")) {
			continue
		}
		if strings.Contains(token, "/") || path.Ext(token) != "" || trackedPath(compiler.index, token) {
			found = append(found, token)
		}
	}
	sort.Strings(found)
	return slices.Compact(found)
}

func (compiler *localPromptCompiler) resolveDefinitions(owners []specDefinition) {
	compiler.countAnchor(len(owners))
	for _, owner := range owners {
		compiler.addEvidence("task_evidence", owner.path, owner.line, "requirement-definition", "repository-spec")
	}
}

func (compiler *localPromptCompiler) resolvePath(token string) {
	matches := []string{}
	if trackedPath(compiler.index, token) {
		matches = append(matches, token)
	} else if !strings.Contains(token, "/") {
		for candidate := range compiler.context.trackedPaths() {
			if !compiler.ready() {
				return
			}
			if len(matches) >= localPromptMaxCandidates {
				compiler.failBounds()
				return
			}
			if path.Base(candidate) == token {
				matches = append(matches, candidate)
			}
		}
	}
	sort.Strings(matches)
	compiler.countAnchor(len(matches))
	for _, candidate := range matches {
		compiler.addEvidence("task_evidence", candidate, 1, "explicit-path", "task-text")
	}
}

func (compiler *localPromptCompiler) hasSymbol(name string) bool {
	for _, symbol := range compiler.index.Symbols {
		if !compiler.ready() {
			return false
		}
		if symbol.Name == name {
			return true
		}
	}
	return false
}

func (compiler *localPromptCompiler) resolveIdentifier(name string) {
	if strings.Contains(name, ".") && !compiler.hasSymbol(name) {
		name = name[strings.LastIndex(name, ".")+1:]
		compiler.qualified++
	}
	matches := []Symbol{}
	for _, symbol := range compiler.index.Symbols {
		if !compiler.ready() {
			return
		}
		if len(matches) >= localPromptMaxCandidates {
			compiler.failBounds()
			return
		}
		if symbol.Name == name {
			matches = append(matches, symbol)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		return matches[i].Line < matches[j].Line
	})
	compiler.countAnchor(len(matches))
	for _, symbol := range matches {
		compiler.addEvidence("task_evidence", symbol.Path, symbol.Line, "explicit-identifier", "syntax")
	}
}

func (compiler *localPromptCompiler) countAnchor(matches int) {
	compiler.anchors++
	if matches == 0 {
		compiler.missing++
	}
	if matches > 1 {
		compiler.ambiguous++
	}
}

func (compiler *localPromptCompiler) addEvidence(section, candidate string, line int, relation, authority string) {
	if !compiler.ready() {
		return
	}
	if len(compiler.rows)+len(compiler.unavailable) >= localPromptMaxCandidates {
		compiler.failBounds()
		return
	}
	source := compiler.index.Sources[candidate]
	if !compiler.context.readable(candidate) || !localObjectID(source.BlobHash) || source.Mode == "120000" {
		compiler.unavailable = append(compiler.unavailable, localSelector(section, candidate))
		if section == "task_evidence" {
			compiler.unread++
		}
		return
	}
	for _, row := range compiler.rows {
		if row.section == section && row.value["path"] == candidate && row.value["line"] == line {
			return
		}
	}
	if section == "task_evidence" && slices.Contains(compiler.index.DirtyPaths, candidate) {
		compiler.dirty++
	}
	compiler.rows = append(compiler.rows, localPromptRow{section, map[string]any{
		"path": candidate, "blob_hash": source.BlobHash, "line": line,
		"relation": relation, "authority": authority,
	}})
}

func (compiler *localPromptCompiler) resolution() map[string]any {
	reason := "none"
	switch {
	case compiler.anchors == 0:
		reason = "explicit-task-anchor-required"
	case compiler.unread > 0 || compiler.requirementsCapped:
		reason = "anchor-evidence-unavailable"
	case compiler.missing > 0:
		reason = "anchor-not-found"
	case compiler.ambiguous > 0:
		reason = "ambiguous-anchor"
	case compiler.dirty > 0:
		reason = "anchor-worktree-changed"
	case compiler.qualified > 0:
		reason = "qualification-unverified"
	}
	state := "resolved"
	if reason != "none" {
		state = "unresolved"
	}
	return map[string]any{
		"state": state, "reason": reason, "anchors": compiler.anchors,
		"missing_anchors": compiler.missing, "ambiguous_anchors": compiler.ambiguous,
		"unavailable_evidence": compiler.unread, "unverified_qualifications": compiler.qualified,
	}
}

func localSelector(section, candidate string) map[string]any {
	return map[string]any{"section": section, "path": candidate}
}

func (compiler *localPromptCompiler) packet(limit, budget int) (map[string]any, error) {
	for included := min(limit, len(compiler.rows)); included >= 0; included-- {
		if !compiler.ready() {
			return nil, compiler.err
		}
		packet := compiler.selectedPacket(included, budget)
		coverage := packet["coverage"].(map[string]any)
		for range 8 {
			encoded, err := CanonicalJSON(packet)
			if err != nil {
				return nil, err
			}
			size := len(encoded) + 1
			if coverage["packet_bytes"] == size {
				if size <= budget {
					return packet, nil
				}
				break
			}
			coverage["packet_bytes"] = size
		}
	}
	return nil, &Error{Code: "unsupported-dogfood-context-budget", Message: "prompt budget cannot retain mandatory identity, resolution and critical omissions"}
}

func (compiler *localPromptCompiler) selectedPacket(included, budget int) map[string]any {
	sections := map[string][]any{"governance": {}, "declared_scope": {}, "task_evidence": {}}
	missing := []any{}
	for number, row := range compiler.rows {
		if number < included {
			sections[row.section] = append(sections[row.section], row.value)
			continue
		}
		missing = append(missing, localSelector(row.section, row.value["path"].(string)))
	}
	resolution := compiler.resolution()
	for _, row := range compiler.rows[included:] {
		if row.section == "task_evidence" && resolution["reason"] == "none" {
			resolution["state"], resolution["reason"] = "unresolved", "task-evidence-omitted"
		}
	}
	digest := sha256.Sum256([]byte(compiler.task))
	freshness := "clean"
	if len(compiler.index.DirtyPaths) != 0 {
		freshness = "mixed-worktree"
	}
	return map[string]any{
		"profile": "corvint-dogfood-prompt/0", "revision": compiler.index.Revision,
		"status_sha256": compiler.index.StatusSHA256, "freshness": freshness,
		"task_sha256": hex.EncodeToString(digest[:]), "resolution": resolution,
		"governance": sections["governance"], "declared_scope": sections["declared_scope"], "task_evidence": sections["task_evidence"],
		"coverage": map[string]any{
			"candidates": len(compiler.rows), "included_results": included, "omitted_results": len(compiler.rows) - included,
			"critical_missing": missing, "unavailable_selectors": append([]any{}, compiler.unavailable...),
			"stale_scope": compiler.scopeStale, "unavailable_scope": compiler.scopeUnavailable,
			"excluded_sources": len(compiler.index.Exclusions), "unparsed_sources": len(compiler.index.Unparsed),
			"requirements_capped": compiler.requirementsCapped, "dirty_paths": len(compiler.index.DirtyPaths),
			"budget_bytes": budget, "packet_bytes": 0, "within_budget": true,
		},
	}
}
