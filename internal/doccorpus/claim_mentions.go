package doccorpus

import (
	"fmt"
	"sort"
	"strings"
)

// Claim mention check (DCX-V1-340..346; references DCX-V1-347). One deterministic, documented extractor finds
// code-like tokens in claim text; the build resolves each against the claim's cited spans and
// then against the corpus inventory. It makes no semantic judgment and never raises trust: it only
// marks claims whose named code cannot be found.

const (
	// MaxClaimMentions bounds the tokens checked per claim; later tokens are counted as omitted.
	MaxClaimMentions = 64
	// MaxMentionBytes bounds one token; a longer candidate is not a mention.
	MaxMentionBytes = 256

	MentionResolvedInSpan    = "resolved-in-span"
	MentionResolvedElsewhere = "resolved-elsewhere"
	MentionUnresolved        = "unresolved"

	MentionsConsistent          = "consistent"
	MentionsPartiallyUnresolved = "partially-unresolved"
	MentionsUnresolved          = "unresolved"
	// MentionsNone is the implicit state of a checked-provider claim with no code-like token; it
	// has no stored check.
	MentionsNone = "no-mentions"
)

// CodeToken is one code-like mention in claim text (DCX-V1-340). Kind is one of code, route,
// scope, member, path or identifier.
type CodeToken struct {
	Token string `json:"token"`
	Kind  string `json:"kind"`
}

// ClaimMention is one token's resolution. Target names the subject ID, the indexed symbol
// (`path:line`) or the tracked path it resolved to elsewhere; TargetKind is subject, symbol or path.
type ClaimMention struct {
	Token      string `json:"token"`
	Kind       string `json:"kind"`
	Resolution string `json:"resolution"`
	Target     string `json:"target,omitempty"`
	TargetKind string `json:"target_kind,omitempty"`
}

// ClaimMentionCheck is the stored mention check of one claim with at least one token.
type ClaimMentionCheck struct {
	ClaimID    string         `json:"claim_id"`
	Subject    string         `json:"subject"`
	State      string         `json:"state"`
	InSpan     int            `json:"resolved_in_span"`
	Elsewhere  int            `json:"resolved_elsewhere"`
	Unresolved int            `json:"unresolved"`
	Omitted    int            `json:"omitted"`
	Mentions   []ClaimMention `json:"mentions"`
}

var httpMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "HEAD": true, "OPTIONS": true}

func identByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// isIdent reports an ASCII identifier: [A-Za-z_][A-Za-z0-9_]*.
func isIdent(s string) bool {
	if s == "" || s[0] >= '0' && s[0] <= '9' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !identByte(s[i]) {
			return false
		}
	}
	return true
}

func hasLetter(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' {
			return true
		}
	}
	return false
}

func lowerByte(b byte) bool { return b >= 'a' && b <= 'z' }
func upperByte(b byte) bool { return b >= 'A' && b <= 'Z' }

// casedIdent reports snake_case (an underscore and a letter), camelCase (at least two leading
// lowercase letters, then an uppercase letter) or PascalCase with an internal lower-to-upper
// transition after its first letter.
func casedIdent(s string) bool {
	if !isIdent(s) || !hasLetter(s) {
		return false
	}
	if strings.Contains(strings.Trim(s, "_"), "_") {
		return true
	}
	if len(s) > 2 && lowerByte(s[0]) && lowerByte(s[1]) {
		for i := 2; i < len(s); i++ {
			if upperByte(s[i]) {
				return true
			}
		}
	}
	if upperByte(s[0]) {
		for i := 2; i < len(s); i++ {
			if upperByte(s[i]) && lowerByte(s[i-1]) {
				return true
			}
		}
	}
	return false
}

// stripCall removes one trailing "()" call marker.
func stripCall(s string) string { return strings.TrimSuffix(s, "()") }

// trimChunk strips leading quotes and opening brackets, then, from the right, quotes, sentence
// punctuation and any closing bracket whose opener does not occur in the chunk. Right trimming
// removes no opener, so opener presence is computed once and the trim is linear in the chunk.
func trimChunk(s string) string {
	s = strings.TrimLeft(s, "\"'([{<")
	unopened := map[byte]bool{}
	for _, pair := range []string{"()", "[]", "{}", "<>"} {
		unopened[pair[1]] = strings.IndexByte(s, pair[0]) < 0
	}
	end := len(s)
	for end > 0 && (strings.IndexByte("\"'.,;:!?", s[end-1]) >= 0 || unopened[s[end-1]]) {
		end--
	}
	return s[:end]
}

// pathChunk reports a slash path whose last segment has a dot extension of 1..8 letters or
// digits; URLs, "//" and characters outside [A-Za-z0-9_./@-] are refused.
func pathChunk(s string) bool {
	if !strings.Contains(s, "/") || strings.Contains(s, "//") || strings.Contains(s, "://") {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !identByte(s[i]) && !strings.ContainsRune("./@-", rune(s[i])) {
			return false
		}
	}
	return extension(s[strings.LastIndex(s, "/")+1:], 8)
}

func extension(base string, max int) bool {
	dot := strings.LastIndex(base, ".")
	if dot <= 0 || dot == len(base)-1 || len(base)-dot-1 > max {
		return false
	}
	for i := dot + 1; i < len(base); i++ {
		if !(lowerByte(base[i]) || upperByte(base[i]) || base[i] >= '0' && base[i] <= '9') {
			return false
		}
	}
	return true
}

// fileExtensions are the extensions that make a backticked dotted name a file name rather than a
// member access (DCX-V1-340).
var fileExtensions = func() map[string]bool {
	m := map[string]bool{}
	for _, e := range strings.Fields("bash c cc cfg conf cpp cs css csv go gradle h hpp htm html ini java js json jsx kt lock md mjs mod php proto py rb rs scss sh sql sum svg swift toml ts tsx txt vue xml yaml yml") {
		m[e] = true
	}
	return m
}()

// fileName reports a backticked, uncalled dotted name of identifier parts whose last part is a
// listed file extension, such as `main.go` or `app.config.json`; `os.stat` is not one.
func fileName(s string, backtick bool) bool {
	i := strings.LastIndex(s, ".")
	return backtick && i > 0 && fileExtensions[s[i+1:]] && allIdent(strings.Split(s, "."), 1)
}

func allIdent(parts []string, minLen int) bool {
	for _, p := range parts {
		if !isIdent(p) || len(p) < minLen {
			return false
		}
	}
	return len(parts) > 1
}

// classifyToken returns the kind of one candidate, or "" when it is not code-like. Backticked
// content that matches no other rule is kind code.
func classifyToken(s string, backtick bool) string {
	if s == "" || len(s) > MaxMentionBytes {
		return ""
	}
	if method, rest, ok := strings.Cut(s, " "); ok && httpMethods[method] && strings.HasPrefix(rest, "/") && !strings.ContainsAny(rest, " \t") {
		return "route"
	}
	bare := stripCall(s)
	switch {
	case strings.Contains(bare, "::") && allIdent(strings.Split(bare, "::"), 1):
		return "scope"
	case strings.Count(bare, "#") == 1:
		left, right, _ := strings.Cut(bare, "#")
		right = strings.TrimRight(right, "?!")
		if isIdent(right) && left != "" && allIdent(strings.FieldsFunc(left, func(r rune) bool { return r == '.' || r == ':' }), 1) || isIdent(right) && isIdent(left) {
			return "member"
		}
	case strings.Contains(bare, ".") && allIdent(strings.Split(bare, "."), 2) && !fileName(s, backtick) && (backtick || s != bare || anyCased(strings.Split(bare, ".")) || mixedParts(strings.Split(bare, "."))):
		return "member"
	case pathChunk(s), fileName(s, backtick):
		return "path"
	case casedIdent(bare) || s != bare && isIdent(bare) && hasLetter(bare):
		return "identifier"
	}
	if backtick && strings.TrimSpace(s) != "" {
		return "code"
	}
	return ""
}

func anyCased(parts []string) bool {
	for _, p := range parts {
		if casedIdent(p) {
			return true
		}
	}
	return false
}

// mixedParts admits a dotted prose chunk such as os.Exit, where a later part starts uppercase.
func mixedParts(parts []string) bool {
	for _, p := range parts[1:] {
		if upperByte(p[0]) {
			return true
		}
	}
	return false
}

// ExtractCodeTokens returns the distinct code-like tokens of text in order of first appearance
// (DCX-V1-340). The rules are ASCII and deterministic:
//
//  1. A single-backtick span on one line is one candidate (its trimmed content).
//  2. Outside backticks, text splits on whitespace; each chunk is trimmed of leading quotes and
//     opening brackets, trailing quotes and sentence punctuation, and a trailing closing
//     bracket whose opener does not occur in the chunk.
//  3. An uppercase HTTP method chunk followed by a chunk starting with "/" is one route token.
//  4. A candidate is then route (METHOD /path), scope (A::B), member (A#b, or dotted a.b parts
//     that are identifiers of two or more bytes and either backticked, called, cased or with an
//     uppercase later part, unless it is a file name), path (a slash path whose last segment has
//     an extension, or a file name: a backticked dotted name whose last part is in
//     fileExtensions), identifier (snake_case, camelCase, PascalCase with an internal
//     transition, or name()), else code when backticked.
//
// At most MaxClaimMentions tokens are returned; omitted counts the rest.
func ExtractCodeTokens(text string) (tokens []CodeToken, omitted int) {
	type chunk struct {
		s        string
		backtick bool
	}
	chunks := []chunk{}
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, chunk{cur.String(), false})
			cur.Reset()
		}
	}
	for i := 0; i < len(text); i++ {
		b := text[i]
		if b == '`' {
			end := strings.IndexAny(text[i+1:], "`\n")
			if end > 0 && text[i+1+end] == '`' {
				flush()
				chunks = append(chunks, chunk{strings.TrimSpace(text[i+1 : i+1+end]), true})
				i += end + 1
				continue
			}
			flush()
			continue
		}
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			flush()
			continue
		}
		cur.WriteByte(b)
	}
	flush()
	seen := map[string]bool{}
	add := func(token, kind string) {
		if kind == "" || seen[token] {
			return
		}
		seen[token] = true
		if len(tokens) >= MaxClaimMentions {
			omitted++
			return
		}
		tokens = append(tokens, CodeToken{token, kind})
	}
	for i := 0; i < len(chunks); i++ {
		c := chunks[i]
		if c.backtick {
			add(c.s, classifyToken(c.s, true))
			continue
		}
		s := trimChunk(c.s)
		if httpMethods[s] && i+1 < len(chunks) && !chunks[i+1].backtick {
			if next := trimChunk(chunks[i+1].s); strings.HasPrefix(next, "/") {
				add(s+" "+next, classifyToken(s+" "+next, false))
				i++
				continue
			}
		}
		add(s, classifyToken(s, false))
	}
	return tokens, omitted
}

// containsWord reports a literal occurrence of needle whose identifier-character ends are not
// extended by identifier characters in haystack.
func containsWord(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	for from := 0; ; {
		i := strings.Index(haystack[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(needle)
		before := i == 0 || !identByte(needle[0]) || !identByte(haystack[i-1])
		after := end == len(haystack) || !identByte(needle[len(needle)-1]) || !identByte(haystack[end])
		if before && after {
			return true
		}
		from = i + 1
	}
}

// tokenComponents are the identifier parts a compound token is checked by.
func tokenComponents(t CodeToken) []string {
	bare := stripCall(t.Token)
	switch t.Kind {
	case "route":
		_, p, _ := strings.Cut(bare, " ")
		return []string{p}
	case "scope", "member":
		parts := strings.FieldsFunc(bare, func(r rune) bool { return r == ':' || r == '#' || r == '.' })
		for i := range parts {
			parts[i] = strings.TrimRight(parts[i], "?!")
		}
		return parts
	}
	return nil
}

func mentionInSpan(t CodeToken, spans string, anchorPaths []string) bool {
	bare := stripCall(t.Token)
	if containsWord(spans, t.Token) || containsWord(spans, bare) {
		return true
	}
	if t.Kind == "route" {
		method, p, _ := strings.Cut(bare, " ")
		return containsWord(spans, p) && (containsWord(spans, method) || containsWord(strings.ToLower(spans), strings.ToLower(method)))
	}
	if t.Kind == "path" {
		for _, p := range anchorPaths {
			if p == bare || strings.HasSuffix(p, "/"+bare) {
				return true
			}
		}
		return false
	}
	parts := tokenComponents(t)
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if !containsWord(spans, p) {
			return false
		}
	}
	return true
}

// mentionIndex holds the corpus inventory a token can resolve to elsewhere.
type mentionIndex struct {
	subjects map[string]string // subject ID or name -> smallest subject ID
	symbols  map[string]string // symbol name or last component -> smallest path:line
	paths    map[string]string // tracked path or path suffix -> smallest tracked path
}

func setMin(m map[string]string, key, value string) {
	if key == "" {
		return
	}
	if old, ok := m[key]; !ok || value < old {
		m[key] = value
	}
}

func (c *compiler) mentionIndex() *mentionIndex {
	x := &mentionIndex{subjects: map[string]string{}, symbols: map[string]string{}, paths: map[string]string{}}
	for _, s := range c.artifact.Subjects {
		setMin(x.subjects, s.ID, s.ID)
		setMin(x.subjects, s.Name, s.ID)
		for _, an := range s.Evidence.Anchors {
			if an.Symbol != "" {
				setMin(x.symbols, an.Symbol, fmt.Sprintf("%s:%d", an.Path, an.Start))
			}
		}
	}
	for _, index := range c.indexes {
		for _, s := range index.Symbols {
			target := fmt.Sprintf("%s:%d", s.Path, s.Line)
			setMin(x.symbols, s.Name, target)
			if i := strings.LastIndexAny(s.Name, ".:#"); i >= 0 {
				setMin(x.symbols, s.Name[i+1:], target)
			}
		}
		for p := range index.Tracked {
			for q := p; ; {
				setMin(x.paths, q, p)
				i := strings.Index(q, "/")
				if i < 0 {
					break
				}
				q = q[i+1:]
			}
		}
	}
	return x
}

func (x *mentionIndex) resolve(t CodeToken) (target, kind string) {
	bare := stripCall(t.Token)
	keys := []string{t.Token, bare}
	if parts := tokenComponents(t); len(parts) > 1 && t.Kind != "route" {
		keys = append(keys, parts[len(parts)-1])
	}
	for _, k := range keys {
		if id, ok := x.subjects[k]; ok {
			return id, "subject"
		}
	}
	for _, k := range keys {
		if s, ok := x.symbols[k]; ok {
			return s, "symbol"
		}
	}
	if t.Kind == "path" {
		if p, ok := x.paths[strings.TrimPrefix(bare, "./")]; ok {
			return p, "path"
		}
	}
	return "", ""
}

// checkClaimMentions resolves every non-native claim's code-like tokens (DCX-V1-341, DCX-V1-342).
func (c *compiler) checkClaimMentions() error {
	native := map[string]bool{}
	for _, p := range c.manifest.Providers {
		native[p.ID] = p.Kind == "native"
	}
	var index *mentionIndex
	out := []ClaimMentionCheck{}
	for _, claim := range c.artifact.Claims {
		if !mentionChecked(c.artifact, native, claim) {
			continue
		}
		tokens, omitted := ExtractCodeTokens(claim.Text)
		if len(tokens) == 0 {
			continue
		}
		if index == nil {
			index = c.mentionIndex()
		}
		var spans strings.Builder
		paths := []string{}
		for _, an := range claim.Evidence.Anchors {
			paths = append(paths, an.Path)
			source, ok := c.sources[inputKey(an.Revision, an.Path)]
			if !ok {
				continue
			}
			if data, err := span(source.Data, an.Start, an.End); err == nil {
				spans.Write(data)
				spans.WriteByte('\n')
			}
		}
		text := spans.String()
		check := ClaimMentionCheck{ClaimID: claim.ID, Subject: claim.Subject, Omitted: omitted, Mentions: []ClaimMention{}}
		for _, t := range tokens {
			m := ClaimMention{Token: t.Token, Kind: t.Kind, Resolution: MentionUnresolved}
			if mentionInSpan(t, text, paths) {
				m.Resolution = MentionResolvedInSpan
				check.InSpan++
			} else if target, kind := index.resolve(t); target != "" {
				m.Resolution, m.Target, m.TargetKind = MentionResolvedElsewhere, target, kind
				check.Elsewhere++
			} else {
				check.Unresolved++
			}
			check.Mentions = append(check.Mentions, m)
		}
		switch {
		case check.Unresolved == 0:
			check.State = MentionsConsistent
		case check.Unresolved == len(check.Mentions):
			check.State = MentionsUnresolved
		default:
			check.State = MentionsPartiallyUnresolved
		}
		out = append(out, check)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ClaimID < out[j].ClaimID })
	if len(out) > 0 {
		c.artifact.ClaimMentions = out
	}
	return nil
}

// claimMentionCheck returns the stored check of one claim.
func claimMentionCheck(a *Artifact, id string) (ClaimMentionCheck, bool) {
	i := sort.Search(len(a.ClaimMentions), func(i int) bool { return a.ClaimMentions[i].ClaimID >= id })
	if i < len(a.ClaimMentions) && a.ClaimMentions[i].ClaimID == id {
		return a.ClaimMentions[i], true
	}
	return ClaimMentionCheck{}, false
}

// attachClaimMentions serves each visible claim's mention check with the claim (DCX-V1-343).
func attachClaimMentions(a *Artifact, r *Receipt) {
	r.ClaimMentions = nil
	if len(a.ClaimMentions) == 0 {
		return
	}
	for _, value := range r.Results {
		claim, ok := value.(Claim)
		if !ok {
			continue
		}
		if check, ok := claimMentionCheck(a, claim.ID); ok {
			if r.ClaimMentions == nil {
				r.ClaimMentions = map[string]ClaimMentionCheck{}
			}
			r.ClaimMentions[claim.ID] = check
		}
	}
}

// claimMentionGaps reports every claim with an unresolved mention (DCX-V1-344).
func claimMentionGaps(a *Artifact) []Gap {
	out := []Gap{}
	for _, check := range a.ClaimMentions {
		if check.State == MentionsConsistent {
			continue
		}
		names := []string{}
		for _, m := range check.Mentions {
			if m.Resolution == MentionUnresolved && len(names) < 8 {
				names = append(names, m.Token)
			}
		}
		reason := fmt.Sprintf("%d of %d code-like mentions resolve neither in a cited span nor to a corpus subject, indexed symbol or tracked path: %s", check.Unresolved, len(check.Mentions), strings.Join(names, ", "))
		if check.Unresolved > len(names) {
			reason += ", ..."
		}
		out = append(out, Gap{check.ClaimID, "claim-mentions-" + check.State, reason})
	}
	return out
}

// checkedClaims counts the claims the mention check examines.
func checkedClaims(a *Artifact) int {
	native := map[string]bool{}
	for _, p := range a.Manifest.Providers {
		native[p.ID] = p.Kind == "native"
	}
	n := 0
	for _, claim := range a.Claims {
		if mentionChecked(a, native, claim) {
			n++
		}
	}
	return n
}

// mentionChecked reports whether the mention check examines claim: every non-native claim except
// a supplied test-run receipt, which changes no other coverage metric (DCP-V1-211).
func mentionChecked(a *Artifact, native map[string]bool, claim Claim) bool {
	return !native[claim.Provider] && a.Details[claim.ID].TestRun == nil
}

const claimMentionCoverageLimitation = "mechanical token resolution only; a resolved mention is not a correct claim and no trust is raised"

// claimMentionCoverage counts non-native claims by mention state, present only when a check is
// stored (DCX-V1-345).
func claimMentionCoverage(a *Artifact) []any {
	if len(a.ClaimMentions) == 0 {
		return nil
	}
	total := checkedClaims(a)
	counts := map[string]int{}
	for _, check := range a.ClaimMentions {
		counts[check.State]++
	}
	counts[MentionsNone] = total - len(a.ClaimMentions)
	rows := []any{}
	for _, state := range []string{MentionsConsistent, MentionsNone, MentionsPartiallyUnresolved, MentionsUnresolved} {
		rows = append(rows, map[string]any{"metric": "claim_mentions:" + state, "value": counts[state], "denominator": total, "defined": total > 0, "definition": "non-native claims whose code-like mentions are " + state + " / all non-native claims", "revision": a.Manifest.Repository.Revision, "limitations": []string{claimMentionCoverageLimitation}})
	}
	return rows
}
