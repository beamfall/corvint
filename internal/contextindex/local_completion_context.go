package contextindex

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
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
	// dropped counts, by kind, the anchors past localPromptMaxAnchors that
	// the partial packet does not resolve (LCP-V0-035).
	dropped      map[string]any
	droppedTotal int
	// tops, directories and bases are the tracked tree's shape, built once on first
	// use by treeShape (LCP-V0-039).
	tops, directories, bases map[string]struct{}
	// overflowed marks the path anchor being resolved as over the candidate
	// bound and already counted ambiguous (LCP-V0-039).
	overflowed bool
}

func (compiler *localPromptCompiler) addGovernance() {
	row, ok := compiler.context.governingRow()
	if ok && row.downgrade == "" {
		admitted := len(compiler.rows)
		compiler.addEvidence("governance", row.path, row.line, "governing", "project-instructions")
		if row.link != "" && len(compiler.rows) > admitted {
			// TCP-V0-274: both paths and both pins, the target's as blob_hash.
			compiler.rows[admitted].value["link_path"] = row.link
			compiler.rows[admitted].value["link_blob_hash"] = compiler.index.Sources[row.link].BlobHash
		}
		return
	}
	// LCP-V0-955: the selector names why governance is not cited; a symlinked governing file keeps
	// the TCP-V0-274 reason below.
	if ok && row.link == "" {
		selector := localSelector("governance", row.path)
		selector["reason"], selector["warnings"] = DowngradedAuthority, downgradeCodes(row)
		compiler.unavailable = append(compiler.unavailable, selector)
		return
	}
	for _, candidate := range compiler.context.candidates[governingRelation] {
		selector := localSelector("governance", candidate)
		if reason := compiler.context.linkRefusals[candidate]; reason != "" {
			selector["reason"] = reason
		} else if ok && row.link != "" {
			selector["reason"] = row.downgrade // TCP-V0-274
		} else {
			selector["reason"] = "unread"
		}
		compiler.unavailable = append(compiler.unavailable, selector)
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
// Prose that cannot name anything in this repository (URLs, numbers, paths
// outside the root, unbacked identifiers) is not an anchor (LCP-V0-039).
func (compiler *localPromptCompiler) addAnchors() {
	ids := compiler.context.requirementIDs()
	mentions, remaining := compiler.mentions(localPromptURL.ReplaceAllString(compiler.task, " "))
	paths, consumed := compiler.explicitPaths(remaining)
	consumed = append(append(consumed, ids...), paths...)
	// Longest first, so removing `cmd/x` never leaves `/y.go` of `cmd/x/y.go`.
	slices.SortFunc(consumed, func(a, b string) int { return cmp.Or(cmp.Compare(len(b), len(a)), strings.Compare(a, b)) })
	for _, token := range consumed {
		remaining = strings.ReplaceAll(remaining, token, " ")
	}
	standalone := strings.Trim(remaining, " \t\r\n`?!")
	identifiers := compiler.anchorIdentifiers(localPromptIdentifiers(remaining), standalone)
	ids, mentions, paths, identifiers = compiler.keepAnchors(ids, mentions, paths, identifiers)
	if len(ids) != 0 {
		definitions := compiler.definitions(ids)
		for _, id := range ids {
			if !compiler.ready() {
				return
			}
			compiler.resolveDefinitions(definitions[id])
		}
	}
	for _, mention := range mentions {
		if !compiler.ready() {
			return
		}
		compiler.resolveMention(mention)
	}
	for _, token := range paths {
		if !compiler.ready() {
			return
		}
		compiler.resolvePath(token)
	}
	for _, identifier := range identifiers {
		if !compiler.ready() {
			return
		}
		compiler.resolveIdentifier(identifier.name)
	}
}

// anchorIdentifiers keeps the identifiers that are explicit anchors, before the
// anchor bound, so prose never spends it (LCP-V0-039). A backticked name
// (weight 3) is explicit whether or not it exists. An unbackticked qualified
// name (weight 2) or code-shaped word (weight 1) is an anchor only when it is
// source-backed: a declaration carries its name, or for a qualified name its
// terminal part. The whole prompt naming one declaration stays an anchor.
func (compiler *localPromptCompiler) anchorIdentifiers(identifiers []taskIdentifier, standalone string) []taskIdentifier {
	wanted := map[string]bool{}
	for _, identifier := range identifiers {
		if identifier.weight < 3 {
			wanted[identifier.name] = false
			wanted[identifier.name[strings.LastIndex(identifier.name, ".")+1:]] = false
		}
	}
	for _, symbol := range compiler.index.Symbols {
		if seen, ok := wanted[symbol.Name]; ok && !seen {
			wanted[symbol.Name] = true
		}
	}
	kept := []taskIdentifier{}
	for _, identifier := range identifiers {
		backed := wanted[identifier.name]
		switch {
		case identifier.weight >= 3:
		case identifier.weight == 2 && !localPromptAbbreviation(identifier.name) && (backed || wanted[identifier.name[strings.LastIndex(identifier.name, ".")+1:]]):
		case identifier.weight == 1 && backed && (definitionEligible(identifier) || standalone == identifier.name):
		default:
			continue
		}
		kept = append(kept, identifier)
	}
	return kept
}

// keepAnchors keeps at most localPromptMaxAnchors anchors, in the fixed order
// requirement IDs, mentions, paths, identifiers, each already sorted, and counts
// what it drops by kind. The dropped anchors are raw task terms, so the packet
// names them by kind and count only (LCP-V0-011, LCP-V0-035).
func (compiler *localPromptCompiler) keepAnchors(ids []string, mentions []taskMention, paths []string, identifiers []taskIdentifier) ([]string, []taskMention, []string, []taskIdentifier) {
	room := localPromptMaxAnchors
	keep := func(kind string, total int) int {
		kept := min(total, room)
		room -= kept
		if dropped := total - kept; dropped > 0 {
			if compiler.dropped == nil {
				compiler.dropped = map[string]any{}
			}
			compiler.dropped[kind] = dropped
			compiler.droppedTotal += dropped
		}
		return kept
	}
	ids = ids[:keep("requirement_ids", len(ids))]
	mentions = mentions[:keep("mentions", len(mentions))]
	paths = paths[:keep("paths", len(paths))]
	identifiers = identifiers[:keep("identifiers", len(identifiers))]
	return ids, mentions, paths, identifiers
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
	backticked := map[string]bool{}
	for _, match := range contextBacktick.FindAllStringSubmatch(task, -1) {
		backticked[match[1]] = true
	}
	// Preserve qualified names as one anchor. Never turn the namespace into a
	// second unknown identifier or silently assert the namespace is verified.
	// Only a backticked one is explicit; an unbackticked one such as `e.g` or
	// `arxiv.org` is an anchor only when source backs it (LCP-V0-039).
	for _, qualified := range localPromptQualified.FindAllString(task, -1) {
		weights[qualified] = 2
		if backticked[qualified] {
			weights[qualified] = 3
		}
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

// explicitPaths returns the distinct path selectors in the task and every raw
// token it consumed, foreign path prose included, so neither becomes an
// identifier (LCP-V0-039).
func (compiler *localPromptCompiler) explicitPaths(task string) (paths, consumed []string) {
	for _, field := range strings.FieldsFunc(task, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("`\"'(),:;?!<>", r)
	}) {
		token := strings.TrimRight(strings.Trim(field, "*[]{}"), ".")
		if token == "" {
			continue
		}
		// A qualified declaration name is an identifier, not an unknown file;
		// a tracked file's base name such as `main.go` stays a file.
		if path.Ext(token) != "" && !strings.Contains(token, "/") && !trackedPath(compiler.index, token) && !compiler.trackedBase(token) && compiler.hasSymbol(strings.TrimPrefix(path.Ext(token), ".")) {
			continue
		}
		// A glob such as `pkg/*.go` or `*_test.sh` names no single file.
		selector, ok := "", false
		if !strings.Contains(token, "*") && (!strings.HasPrefix(field, "*") || strings.HasPrefix(field, "**")) {
			selector, ok = compiler.selector(token)
		}
		if ok {
			paths = append(paths, selector)
		}
		if ok || strings.Contains(token, "/") {
			consumed = append(consumed, token)
		}
	}
	sort.Strings(paths)
	return slices.Compact(paths), consumed
}

// localPromptExtension is a file extension as written in prose: one lower- or
// upper-case word that starts with a letter, so `1.64`, `rc.2` and
// `packet.ParsePacket` are not file names.
var localPromptExtension = regexp.MustCompile(`^\.(?:[a-z][a-z0-9]*|[A-Z][A-Z0-9]*)$`)

// localPromptURL is a URL; it names a resource outside this tree, never an
// anchor, and is removed before any selector is read (LCP-V0-039).
var localPromptURL = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://\S+`)

// selector normalizes a path-shaped token to a repository-relative selector
// and reports whether it can name a file in this tree (LCP-V0-039). A tracked
// path, a bare file name with a word extension, or a relative path under a
// tracked top-level entry is a selector, and an absolute path under the index
// root is rebased first. A tracked directory, a path outside the root, a path
// with an empty, `.` or `..` segment, and slash prose such as `origin/main`
// name no file and are not anchors, so they are neither resolved nor missing.
func (compiler *localPromptCompiler) selector(token string) (string, bool) {
	token = strings.TrimPrefix(token, "./")
	if trackedPath(compiler.index, token) {
		return token, true
	}
	if strings.HasPrefix(token, "/") {
		relative, ok := compiler.rebase(token)
		if !ok {
			return "", false
		}
		token = relative
	} else if relative, ok := compiler.rebase("/" + token); ok {
		// A derived over-bound query drops the leading `/` (AHI-016).
		token = relative
	}
	if trackedPath(compiler.index, token) {
		return token, true
	}
	if !strings.Contains(token, "/") {
		return token, localPromptFileName(token)
	}
	token = strings.TrimRight(token, "/")
	segments := strings.Split(token, "/")
	if slices.ContainsFunc(segments, func(segment string) bool { return segment == "" || segment == "." || segment == ".." }) {
		return "", false
	}
	if trackedPath(compiler.index, token) {
		return token, true
	}
	tops, directories := compiler.treeShape()
	if _, ok := directories[token]; ok || len(segments) < 2 {
		return "", false
	}
	_, ok := tops[segments[0]]
	return token, ok
}

// localPromptFileName reports whether a bare token is written as a file name:
// a non-empty stem and a word extension, and not a dotted abbreviation such
// as `e.g` or `a.k.a` whose every part is one letter.
func localPromptFileName(token string) bool {
	extension := path.Ext(token)
	if strings.TrimSuffix(token, extension) == "" || !localPromptExtension.MatchString(extension) {
		return false
	}
	return !localPromptAbbreviation(token)
}

func localPromptAbbreviation(token string) bool {
	for _, part := range strings.Split(token, ".") {
		if len(part) > 1 {
			return false
		}
	}
	return true
}

// rebase turns an absolute path under the index root into a repository path.
// macOS reaches /tmp and /var through /private, so both spellings match.
func (compiler *localPromptCompiler) rebase(token string) (string, bool) {
	root := path.Clean(filepath.ToSlash(compiler.index.Root))
	if compiler.index.Root == "" || root == "/" {
		return "", false
	}
	roots := []string{root, "/private" + root}
	if trimmed, ok := strings.CutPrefix(root, "/private/"); ok {
		roots = append(roots, "/"+trimmed)
	}
	for _, candidate := range roots {
		if relative, ok := strings.CutPrefix(token, candidate+"/"); ok && relative != "" {
			return relative, true
		}
	}
	return "", false
}

func (compiler *localPromptCompiler) trackedBase(token string) bool {
	compiler.treeShape()
	_, ok := compiler.bases[token]
	return ok
}

// treeShape returns, once per compile, the tracked top-level entries and every
// tracked directory, and records every tracked base name.
func (compiler *localPromptCompiler) treeShape() (map[string]struct{}, map[string]struct{}) {
	if compiler.directories != nil {
		return compiler.tops, compiler.directories
	}
	compiler.tops, compiler.directories, compiler.bases = map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for candidate := range compiler.context.trackedPaths() {
		segments := strings.Split(candidate, "/")
		compiler.tops[segments[0]] = struct{}{}
		compiler.bases[segments[len(segments)-1]] = struct{}{}
		for end := 1; end < len(segments); end++ {
			compiler.directories[strings.Join(segments[:end], "/")] = struct{}{}
		}
	}
	return compiler.tops, compiler.directories
}

// A task mention is one whitespace-delimited field naming a line, a line range,
// a declaration in a path, or a commit (LCP-V0-013). The path part must be
// path-shaped, so `host:8080` or `issue#12` stays ordinary text.
type taskMention struct {
	path, symbol, commit string
	start, end           int
}

var localPromptLineMention = regexp.MustCompile(`^([^\s:#]+):([0-9]{1,9})(?:-([0-9]{1,9})|:[0-9]{1,9})?$`)
var localPromptSymbolMention = regexp.MustCompile(`^([^\s:#]+)#([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)$`)
var localPromptCommitMention = regexp.MustCompile(`^(?:[0-9a-f]{7,40}|[0-9a-f]{64})$`)

// A mention never ends in punctuation or a symbol other than `_`, which can end
// an identifier; so `:4-`, `:4…` and `:4**` all end at the line number.
func trailingMentionRune(r rune) bool {
	return r != '_' && (unicode.IsPunct(r) || unicode.IsSymbol(r))
}

// mentions returns the distinct mentions and the task without their fields, so
// removing a mention never cuts into another word.
func (compiler *localPromptCompiler) mentions(task string) ([]taskMention, string) {
	found := []taskMention{}
	rest := []string{}
	for _, field := range strings.Fields(task) {
		token := strings.TrimRightFunc(strings.TrimLeft(field, "`\"'*([{<"), trailingMentionRune)
		mention, ok := compiler.mention(strings.TrimPrefix(token, "./"))
		if !ok {
			rest = append(rest, field)
			continue
		}
		found = append(found, mention)
	}
	slices.SortFunc(found, compareTaskMentions)
	return slices.Compact(found), strings.Join(rest, " ")
}

func compareTaskMentions(a, b taskMention) int {
	return cmp.Or(strings.Compare(a.commit, b.commit), strings.Compare(a.path, b.path),
		strings.Compare(a.symbol, b.symbol), cmp.Compare(a.start, b.start), cmp.Compare(a.end, b.end))
}

func (compiler *localPromptCompiler) mention(token string) (taskMention, bool) {
	if match := localPromptLineMention.FindStringSubmatch(token); match != nil {
		if selector, ok := compiler.selector(match[1]); ok {
			start, _ := strconv.Atoi(match[2])
			end := start
			if match[3] != "" {
				end, _ = strconv.Atoi(match[3])
			}
			return taskMention{path: selector, start: start, end: end}, true
		}
	}
	if match := localPromptSymbolMention.FindStringSubmatch(token); match != nil {
		if selector, ok := compiler.selector(match[1]); ok {
			return taskMention{path: selector, symbol: match[2]}, true
		}
	}
	// A commit needs a digit and a letter so hex-only words stay ordinary text.
	if localPromptCommitMention.MatchString(token) && strings.ContainsAny(token, "0123456789") && strings.ContainsAny(token, "abcdef") {
		return taskMention{commit: token}, true
	}
	return taskMention{}, false
}

func (compiler *localPromptCompiler) resolveMention(mention taskMention) {
	switch {
	case mention.commit != "":
		compiler.resolveCommit(mention.commit)
	case mention.symbol != "":
		compiler.resolvePathSymbol(mention.path, mention.symbol)
	default:
		compiler.resolveLines(mention.path, mention.start, mention.end)
	}
}

// Only the bound commit is known without reading history, and it adds no path
// row; any other commit is evidence this profile cannot read, never not-found.
func (compiler *localPromptCompiler) resolveCommit(commit string) {
	compiler.anchors++
	if !strings.HasPrefix(compiler.index.CommitRevision, commit) {
		compiler.unread++
	}
}

// A line or range must lie inside the bound source. An unreadable source is
// left to addEvidence, which reports it unavailable rather than out of range.
func (compiler *localPromptCompiler) resolveLines(token string, start, end int) {
	candidates := compiler.pathCandidates(token)
	if !compiler.ready() || compiler.overflowed {
		return
	}
	// A range that no content can satisfy is not-found even on a dirty path.
	if start < 1 || end < start {
		compiler.countAnchor(0)
		return
	}
	matches := []string{}
	for _, candidate := range candidates {
		text, loaded := sourceTextBounded(compiler.index.Sources[candidate])
		if !loaded || end <= sourceLineCount(text) {
			matches = append(matches, candidate)
		}
	}
	if compiler.crowded(len(matches)) {
		return
	}
	compiler.countAnchor(len(matches) + compiler.staleCandidates(candidates, matches))
	for _, candidate := range matches {
		compiler.addEvidence("task_evidence", candidate, start, "explicit-line", "task-text")
	}
}

func sourceLineCount(text string) int {
	lines := strings.Count(text, "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		lines++
	}
	return lines
}

// A declaration is matched by exact name within the named path only. A dotted
// name that is not itself a symbol falls back to its terminal part, which stays
// qualification-unverified exactly as a bare qualified identifier does.
func (compiler *localPromptCompiler) resolvePathSymbol(token, name string) {
	candidates := compiler.pathCandidates(token)
	if !compiler.ready() || compiler.overflowed {
		return
	}
	if strings.Contains(name, ".") && !compiler.pathHasSymbol(candidates, name) {
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
		if symbol.Name == name && slices.Contains(candidates, symbol.Path) {
			matches = append(matches, symbol)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		return matches[i].Line < matches[j].Line
	})
	matched := []string{}
	for _, symbol := range matches {
		matched = append(matched, symbol.Path)
	}
	if compiler.crowded(len(matches)) {
		return
	}
	compiler.countAnchor(len(matches) + compiler.staleCandidates(candidates, matched))
	for _, symbol := range matches {
		compiler.addEvidence("task_evidence", symbol.Path, symbol.Line, "explicit-identifier", "syntax")
	}
}

// A dirty candidate with no match in its bound blob may hold the anchor in the
// worktree, so it counts as changed rather than not-found and adds no row.
func (compiler *localPromptCompiler) staleCandidates(candidates, matched []string) int {
	stale := 0
	for _, candidate := range candidates {
		if slices.Contains(compiler.index.DirtyPaths, candidate) && !slices.Contains(matched, candidate) {
			stale++
		}
	}
	compiler.dirty += stale
	return stale
}

func (compiler *localPromptCompiler) pathHasSymbol(candidates []string, name string) bool {
	return slices.ContainsFunc(compiler.index.Symbols, func(symbol Symbol) bool {
		return symbol.Name == name && slices.Contains(candidates, symbol.Path)
	})
}

func (compiler *localPromptCompiler) resolveDefinitions(owners []specDefinition) {
	compiler.countAnchor(len(owners))
	for _, owner := range owners {
		compiler.addEvidence("task_evidence", owner.path, owner.line, "requirement-definition", "repository-spec")
	}
}

func (compiler *localPromptCompiler) resolvePath(token string) {
	matches := compiler.pathCandidates(token)
	if !compiler.ready() || compiler.overflowed || compiler.crowded(len(matches)) {
		return
	}
	compiler.countAnchor(len(matches))
	for _, candidate := range matches {
		compiler.addEvidence("task_evidence", candidate, 1, "explicit-path", "task-text")
	}
}

// pathCandidates resolves a tracked path exactly, or a bare file name by its
// base name across tracked paths; more than one candidate is ambiguous. A base
// name shared by more than localPromptMaxCandidates tracked files, such as
// `main.go`, is ambiguous with no candidate rows rather than a refusal of the
// whole prompt (LCP-V0-039); the tracked set is already in memory, so counting
// it reads nothing new.
func (compiler *localPromptCompiler) pathCandidates(token string) []string {
	compiler.overflowed = false
	matches := []string{}
	if trackedPath(compiler.index, token) {
		return append(matches, token)
	}
	if strings.Contains(token, "/") {
		return matches
	}
	for candidate := range compiler.context.trackedPaths() {
		if !compiler.ready() {
			return nil
		}
		if path.Base(candidate) == token {
			if len(matches) >= localPromptMaxCandidates {
				compiler.overflowAnchor()
				return nil
			}
			matches = append(matches, candidate)
		}
	}
	sort.Strings(matches)
	return matches
}

// crowded reports, and counts as ambiguous with no rows, an ambiguous anchor
// whose candidates no longer fit the packet's candidate bound beside the rows
// earlier anchors added, so one common name such as `main.go` never refuses
// the whole prompt (LCP-V0-039). A single candidate still meets the bound in
// addEvidence.
func (compiler *localPromptCompiler) crowded(matches int) bool {
	if matches < 2 || len(compiler.rows)+len(compiler.unavailable)+matches <= localPromptMaxCandidates {
		return false
	}
	compiler.overflowAnchor()
	return true
}

// overflowAnchor counts an anchor whose candidates exceed the bound as
// ambiguous; its resolver then counts nothing more for it.
func (compiler *localPromptCompiler) overflowAnchor() {
	compiler.anchors++
	compiler.ambiguous++
	compiler.overflowed = true
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
	if compiler.crowded(len(matches)) {
		return
	}
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
	case compiler.droppedTotal > 0:
		reason = "anchor-bound-exceeded"
	}
	state := "resolved"
	if reason != "none" {
		state = "unresolved"
	}
	resolution := map[string]any{
		"state": state, "reason": reason, "anchors": compiler.anchors,
		"missing_anchors": compiler.missing, "ambiguous_anchors": compiler.ambiguous,
		"unavailable_evidence": compiler.unread, "unverified_qualifications": compiler.qualified,
	}
	// Present only when anchors were dropped, so every packet within the
	// bound keeps its bytes (LCP-V0-035).
	if compiler.droppedTotal > 0 {
		resolution["dropped_anchors"], resolution["dropped_anchor_kinds"] = compiler.droppedTotal, maps.Clone(compiler.dropped)
	}
	return resolution
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
	if compiler.index.SourceDirty() {
		freshness = "mixed-worktree"
	}
	packet := map[string]any{
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
	if taskState := compiler.index.taskStateMember(); taskState != nil {
		packet["task_state"] = taskState
	}
	return packet
}
