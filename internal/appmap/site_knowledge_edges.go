package appmap

import (
	"context"
	"html"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Stage 4 static navigation edges (SKH-V0-040..043, SKH-V0-110..115). Links in templates and
// components, form actions and programmatic navigation are read lexically from committed source
// and joined to the AMAP-V0 screen graph with provenance `static`. They are listed beside the map's
// own test-observed and flow-declared edges and never merged into them; PROVEN edges stay in the
// ANL-V0 ledger (`flows appmap edges`). Nothing is captured here (SKH-V0-042).

// EdgesSchema is the profile of `flows appmap links`.
const EdgesSchema = "site-knowledge-edges/0"

// Edge provenance classes (SKH-V0-040, SKH-V0-110).
const (
	ProvenanceStatic       = "static"
	ProvenanceTestObserved = "test-observed"
	ProvenanceFlowDeclared = "flow-declared"
)

// Static link kinds (SKH-V0-111).
const (
	LinkHref         = "a-href"
	LinkUISref       = "ui-sref"
	LinkRouterLink   = "router-link"
	LinkTo           = "link-to"
	LinkFormAction   = "form-action"
	LinkRailsLink    = "link_to"
	LinkRailsButton  = "button_to"
	LinkRailsForm    = "form_with"
	LinkRedirect     = "redirect_to"
	LinkStateGo      = "state-go"
	LinkNavigate     = "navigate"
	maxStaticEdges   = 8192
	staticEdgesLimit = "static and lexical: links are read from committed templates and components at the map revision; " +
		"a static edge says a link exists in source, not that it is reachable or was taken; PROVEN edges come only " +
		"from the ANL-V0 ledger (flows appmap edges); dynamic targets are unknown, never resolved by guessing"
)

// skTemplateSuffix are the template and component files read; skUnsupportedTemplate are named
// as unknowns, never read (SKH-V0-114).
var (
	skTemplateSuffix      = map[string]bool{".html": true, ".htm": true, ".erb": true, ".vue": true, ".rb": true}
	skUnsupportedTemplate = map[string]bool{".haml": true, ".slim": true, ".pug": true, ".jade": true, ".hbs": true,
		".handlebars": true, ".mustache": true, ".twig": true, ".njk": true, ".liquid": true, ".svelte": true}
)

// SKEdge is one edge of the screen graph with its provenance (SKH-V0-040). A static edge's From is
// the screen its source file renders, when that is known (SKH-V0-112); its To is the screen the
// literal target resolves to (SKH-V0-113). Either side that is not known is UNKNOWN with a reason.
type SKEdge struct {
	ID         string `json:"id"`
	Provenance string `json:"provenance"`
	Kind       string `json:"kind"`
	From       string `json:"from,omitempty"`
	FromStatus string `json:"from_status"`
	FromReason string `json:"from_reason,omitempty"`
	To         string `json:"to,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	Target     string `json:"target,omitempty"`
	// Observed is true on a static edge whose resolved screen pair the map also holds as a
	// test-observed edge; the two stay separate records.
	Observed  bool   `json:"observed,omitempty"`
	Source    string `json:"source,omitempty"`
	Authority string `json:"authority"`
	Anchor    Anchor `json:"anchor"`
}

// skLinkSite is one link found in source before resolution.
type skLinkSite struct {
	kind    string
	target  string // literal URL, state or Rails helper name; empty when dynamic
	space   string // url, state or helper
	dynamic bool
	routed  bool // a router-relative target, written without the router base path (AMAP-V0-191)
	line    int
}

var (
	skAttr = regexp.MustCompile(`(?:^|\s)(\[?(?:href|to|ui-sref|routerLink|action)\]?|:to|v-bind:to|:href)\s*=\s*(?:"([^"]*)"|'([^']*)'|\{\s*(?:"([^"]*)"|'([^']*)'|` + "`([^`$]*)`" + `)\s*\}|\{([^}]*)\})`)
	skTag  = regexp.MustCompile(`<\s*([A-Za-z][\w.-]*)[^<>]*$`)
	// skRubyLink finds link_to, button_to, redirect_to, form_with and form_tag calls.
	skRubyLink = regexp.MustCompile(`(?:^|[^.\w])(link_to|button_to|redirect_to|form_with|form_tag)\b\s*\(?`)
	skHelper   = regexp.MustCompile(`^([a-z_]\w*)_(?:path|url)\b`)
	// skNavCall finds programmatic navigation with its receiver.
	skNavCall = regexp.MustCompile(`(?:\$state\.(go|transitionTo)|(?:\brouter|\bhistory)\.(push|replace|navigateByUrl|navigate)|\blocation\.assign|(?:^|[^.\w])(navigate))\s*\(`)
	// skInterp marks a template value that interpolates.
	skInterp = regexp.MustCompile(`\{\{|\$\{|<%|#\{`)
)

// skLinkTags are the elements whose `to` attribute is a router link.
var skLinkTags = wordSet("Link NavLink router-link RouterLink")

// skScheme is a URL scheme with or without an authority (javascript:, blob:, file:).
var skScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// skExternal reports a target that leaves the application or stays on the page.
// The target is first normalised as a URL parser would: leading and trailing C0
// controls and spaces are stripped and every tab and newline removed, so
// " javascript:" and "java\tscript:" carry their scheme (V1-3199). A backslash
// counts as a slash, so "/\\host" is protocol-relative. A template attribute is also
// checked after HTML character references decode, as a browser reads "&#x6A;avascript:".
func skExternal(v string) bool {
	v = strings.TrimFunc(v, func(r rune) bool { return r <= ' ' })
	v = strings.Map(func(r rune) rune {
		switch r {
		case '\t', '\n', '\r':
			return -1
		}
		return r
	}, v)
	v = strings.ReplaceAll(v, `\`, "/")
	return skScheme.MatchString(v) || strings.HasPrefix(v, "//") || strings.HasPrefix(v, "#")
}

// templateLinks reads the attribute links of one template or component text.
func templateLinks(text string) []skLinkSite {
	out := []skLinkSite{}
	for n, line := range strings.Split(text, "\n") {
		for _, mm := range skAttr.FindAllStringSubmatchIndex(line, -1) {
			attr := strings.Trim(line[mm[2]:mm[3]], "[]")
			tag := ""
			if t := skTag.FindStringSubmatch(line[:mm[0]]); t != nil {
				tag = t[1]
			}
			// Groups 2..6 hold a quoted literal; group 7 a JSX expression.
			value, literalValue := "", false
			for g := 4; g <= 12; g += 2 {
				if mm[g] >= 0 {
					value, literalValue = line[mm[g]:mm[g+1]], true
				}
			}
			if mm[14] >= 0 {
				value = line[mm[14]:mm[15]]
			}
			bound := mm[14] >= 0 || strings.HasPrefix(attr, ":") || strings.HasPrefix(attr, "v-bind:") || strings.HasPrefix(line[mm[2]:mm[3]], "[")
			if tag == "" {
				continue // not an element attribute: a script assignment, or a tag opened on an earlier line
			}
			site := skLinkSite{line: n + 1}
			switch strings.TrimPrefix(strings.TrimPrefix(attr, "v-bind"), ":") {
			case "href":
				if strings.EqualFold(tag, "link") || strings.EqualFold(tag, "base") {
					continue
				}
				site.kind, site.space = LinkHref, "url"
			case "to":
				if !skLinkTags[tag] {
					continue
				}
				site.kind, site.space, site.routed = LinkTo, "url", true
			case "ui-sref":
				site.kind, site.space = LinkUISref, "state"
				if i := strings.Index(value, "("); i >= 0 {
					value = value[:i]
				}
			case "routerLink":
				site.kind, site.space, site.routed = LinkRouterLink, "url", true
				if bound {
					// [routerLink]="'/x'" or "['/x', 'y']" with literal parts only.
					value, literalValue, bound = skAngularLink(value)
				}
			case "action":
				if !strings.EqualFold(tag, "form") {
					continue
				}
				site.kind, site.space = LinkFormAction, "url"
			default:
				continue
			}
			value = strings.TrimSpace(value)
			switch {
			case bound || !literalValue || skInterp.MatchString(value):
				site.dynamic = true
			case value == "" || skExternal(value) || skExternal(html.UnescapeString(value)):
				continue
			default:
				site.target = value
			}
			out = append(out, site)
		}
	}
	return out
}

// skAngularLink reads a bound routerLink of string literals: '/a' or ['/a', 'b'].
func skAngularLink(v string) (string, bool, bool) {
	v = strings.TrimSpace(v)
	parts := []string{v}
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		parts = strings.Split(v[1:len(v)-1], ",")
	}
	segs := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) < 2 || (p[0] != '\'' && p[0] != '"') || p[len(p)-1] != p[0] || strings.ContainsAny(p[1:len(p)-1], `'"\`) {
			return "", false, true
		}
		if seg := strings.Trim(p[1:len(p)-1], "/"); seg != "" {
			segs = append(segs, seg)
		}
	}
	out := "/" + strings.Join(segs, "/")
	if !strings.HasPrefix(strings.Trim(parts[0], `'" `), "/") {
		return "", false, true // a relative routerLink depends on the current route
	}
	return out, true, false
}

// rubyArgs splits a Ruby call's top-level arguments, quote and bracket aware, up to the end of
// the call, an ERB `%>` or a `do` block, which it reports.
func rubyArgs(s string) ([]string, bool) {
	out, depth, quote, start, end, closed := []string{}, 0, byte(0), 0, len(s), false
scan:
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '%' && i+1 < len(s) && s[i+1] == '>', c == '#':
			end = i // an ERB close, or an unquoted `#` that starts a Ruby comment
			break scan
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			if depth == 0 {
				end, closed = i, c == ')'
				break scan
			}
			depth--
		case c == ',' && depth == 0:
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	rest := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s[start:end]), "-"))
	block := skOpensBlock(rest)
	if closed {
		// `link_to(target, ...) do`: the block opens after the closing parenthesis.
		after, _, _ := strings.Cut(s[end+1:], "%>")
		after = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(after), "-"))
		block = block || after == "do" || strings.HasPrefix(after, "do |") || strings.HasPrefix(after, "do|")
	}
	if block {
		rest = strings.TrimSpace(strings.TrimSuffix(rest, "do"))
		if i := strings.LastIndex(rest, " do |"); i >= 0 {
			rest = strings.TrimSpace(rest[:i])
		}
	}
	if rest != "" {
		out = append(out, rest)
	}
	return out, block
}

// skOpensBlock reports a Ruby line that ends by opening a `do` block.
func skOpensBlock(line string) bool {
	line = strings.TrimSpace(line)
	if i := strings.LastIndex(line, " do |"); i >= 0 && strings.HasSuffix(line, "|") {
		return true
	}
	return strings.HasSuffix(line, " do") || line == "do"
}

// rubyTarget reads one Rails link target: a literal path, a `*_path`/`*_url` helper, or dynamic.
func rubyTarget(arg string) (string, string, bool) {
	arg = strings.TrimSpace(arg)
	if len(arg) >= 2 && (arg[0] == '"' || arg[0] == '\'') && arg[len(arg)-1] == arg[0] && !strings.Contains(arg, "#{") {
		return arg[1 : len(arg)-1], "url", false
	}
	if m := skHelper.FindStringSubmatch(arg); m != nil {
		// The helper call must be the whole argument: `order_path(o) + '/edit'` is dynamic (SKH-V0-141).
		rest := strings.TrimSpace(arg[len(m[0]):])
		if rest == "" || strings.HasPrefix(rest, "(") && rubyCloseParen(rest) == len(rest)-1 {
			return m[1], "helper", false
		}
	}
	return "", "", true
}

// rubyCloseParen returns the index of the parenthesis that closes s[0], skipping quoted strings,
// or -1 when it is unbalanced.
func rubyCloseParen(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'':
			for i++; i < len(s) && s[i] != c; i++ {
				if s[i] == '\\' {
					i++
				}
			}
			if i >= len(s) {
				return -1
			}
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

// rubyLinks reads link_to, button_to, redirect_to, form_with url: and form_tag in Ruby or ERB.
// Calls are found in code (masked Ruby lines, so comments and strings hold no call); arguments are
// read from raw, the same lines unmasked, so literal targets keep their text.
func rubyLinks(code, raw []string) []skLinkSite {
	out := []skLinkSite{}
	for n, line := range code {
		for _, mm := range skRubyLink.FindAllStringSubmatchIndex(line, -1) {
			call := line[mm[2]:mm[3]]
			text := line
			if n < len(raw) && len(raw[n]) == len(line) {
				text = raw[n]
			}
			// Arguments start after the call name: in masked code the blanked literal reads as spaces.
			rest := strings.TrimLeft(text[mm[3]:], " \t")
			rest = strings.TrimPrefix(rest, "(")
			args, block := rubyArgs(rest)
			pick := ""
			switch call {
			case "link_to", "button_to":
				// The block form takes the target first; otherwise it follows the link body.
				switch {
				case block && len(args) >= 1:
					pick = args[0]
				case len(args) >= 2:
					pick = args[1]
				}
			case "redirect_to", "form_tag":
				if len(args) >= 1 {
					pick = args[0]
				}
			case "form_with":
				for _, a := range args {
					if v, ok := strings.CutPrefix(a, "url:"); ok {
						pick = v
					}
				}
				if pick == "" {
					continue // a model form: the target is the model's route, not read
				}
			}
			kind := map[string]string{"link_to": LinkRailsLink, "button_to": LinkRailsButton, "redirect_to": LinkRedirect,
				"form_with": LinkRailsForm, "form_tag": LinkRailsForm}[call]
			site := skLinkSite{kind: kind, line: n + 1}
			target, space, dynamic := rubyTarget(pick)
			switch {
			case dynamic || pick == "":
				site.dynamic = true
			case space == "url" && skExternal(target):
				continue
			default:
				site.target, site.space = target, space
			}
			out = append(out, site)
		}
	}
	return out
}

// maskComments blanks every open..close comment span, keeping newlines so that line numbers
// still match the committed bytes; an unclosed span runs to the end of the text.
func maskComments(text, open, close string) string {
	b := []byte(text)
	for i := 0; ; {
		start := strings.Index(text[i:], open)
		if start < 0 {
			return string(b)
		}
		start += i
		end := len(text)
		if e := strings.Index(text[start+len(open):], close); e >= 0 {
			end = start + len(open) + e + len(close)
		}
		for j := start; j < end; j++ {
			if b[j] != '\n' {
				b[j] = ' '
			}
		}
		i = end
	}
}

// jsCodeAt reports whether byte pos of one source line is code: not inside a string literal or
// a comment opened earlier on the line.
func jsCodeAt(line string, pos int) bool {
	var quote byte
	for i := 0; i < pos && i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
		case c == '/' && i+1 < len(line) && line[i+1] == '/':
			return false
		case c == '/' && i+1 < len(line) && line[i+1] == '*':
			e := strings.Index(line[i+2:], "*/")
			if e < 0 || i+2+e+2 > pos {
				return false
			}
			i += 2 + e + 1
		}
	}
	return quote == 0
}

// scriptLinks reads programmatic navigation from JavaScript and TypeScript tokens' source lines.
func scriptLinks(text string) []skLinkSite {
	out := []skLinkSite{}
	for n, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		for _, mm := range skNavCall.FindAllStringSubmatchIndex(line, -1) {
			if !jsCodeAt(line, mm[1]-1) {
				continue // inside a string literal or after a comment opener
			}
			if strings.HasSuffix(strings.TrimSpace(line[:mm[0]+1]), "function") {
				continue // a declaration, not a call
			}
			// A router or history call and a bare navigate() take router-relative paths;
			// location.assign takes a browser URL.
			site := skLinkSite{kind: LinkNavigate, space: "url", routed: mm[4] >= 0 || mm[6] >= 0, line: n + 1}
			if mm[2] >= 0 {
				site.kind, site.space = LinkStateGo, "state"
			}
			method := ""
			if mm[4] >= 0 {
				method = line[mm[4]:mm[5]]
			}
			arg := strings.TrimSpace(line[mm[1]:])
			if method == "navigate" && strings.HasPrefix(arg, "[") {
				end := strings.Index(arg, "]")
				if end < 0 {
					site.dynamic = true
				} else if v, ok, bound := skAngularLink(arg[:end+1]); ok && !bound {
					site.target = v
				} else {
					site.dynamic = true
				}
				out = append(out, site)
				continue
			}
			v, ok := skStringArg(arg)
			switch {
			case !ok:
				site.dynamic = true
			case site.space == "url" && skExternal(v):
				continue
			default:
				site.target = v
			}
			out = append(out, site)
		}
	}
	return out
}

// skStringArg reads a leading string literal argument that ends at `,` or `)`.
func skStringArg(arg string) (string, bool) {
	if arg == "" {
		return "", false
	}
	q := arg[0]
	if q != '\'' && q != '"' && q != '`' {
		return "", false
	}
	end := strings.IndexByte(arg[1:], q)
	if end < 0 {
		return "", false
	}
	v := arg[1 : end+1]
	rest := strings.TrimSpace(arg[end+2:])
	if strings.ContainsAny(v, `\`) || (q == '`' && strings.Contains(v, "${")) || !(strings.HasPrefix(rest, ",") || strings.HasPrefix(rest, ")")) {
		return "", false
	}
	return v, true
}

// skViewScreens binds each Rails view template and controller action to the screen of its route
// (SKH-V0-112): `app/views/CTRL/ACTION.*` and the `def ACTION` of `app/controllers/CTRL_controller.rb`.
type skViewScreens struct {
	views   map[string]string // "ctrl#action" -> screen ID
	statesO map[string]string // state name -> screen ID
	roots   []string
}

func (v *skViewScreens) actionOf(p string) (ctrl, action string, ok bool) {
	for _, root := range v.roots {
		rel := p
		if root != "." {
			if !strings.HasPrefix(p, root+"/") {
				continue
			}
			rel = strings.TrimPrefix(p, root+"/")
		}
		if rest, ok := strings.CutPrefix(rel, "app/views/"); ok {
			dir, file := path.Split(rest)
			name, _, _ := strings.Cut(file, ".")
			if dir == "" || strings.HasPrefix(name, "_") {
				return "", "", false
			}
			return strings.TrimSuffix(dir, "/"), name, true
		}
	}
	return "", "", false
}

// readViewScreens compiles the map's Rails routes once to bind views and actions to screens.
func readViewScreens(r repo, m *Map, repos map[string]string) (*skViewScreens, error) {
	v := &skViewScreens{views: map[string]string{}, statesO: map[string]string{}}
	for _, s := range m.Screens {
		if s.Status == StatusResolved {
			v.statesO[s.State] = s.ID
		}
	}
	man, err := readManifestAt(r, m, repos)
	if err != nil {
		return nil, err
	}
	compiled, err := compileRailsRoutes(r, m.Revision, man)
	if err != nil {
		return nil, err
	}
	for root := range compiled.roots {
		v.roots = append(v.roots, root)
	}
	sort.Strings(v.roots)
	for _, rt := range compiled.routes {
		if id := v.statesO[rt.name]; id != "" && rt.verb == "GET" {
			k := rt.ctrl + "#" + rt.action
			if prior, ok := v.views[k]; ok && prior != id {
				v.views[k] = "" // two screens render the action: not bound
				continue
			}
			v.views[k] = id
		}
	}
	return v, nil
}

// Links reports the screen graph's edges by provenance: the static edges of the declared sources
// and the map's own test-observed and flow-declared edges (SKH-V0-040..043, SKH-V0-110..115).
func Links(ctx context.Context, m *Map, sources []string, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	static, edges, unsupported, files, skipped, err := buildLinks(ctx, m, sources, o)
	if err != nil {
		return nil, err
	}
	kinds, reasons, resolved, fromBound, alsoObserved := map[string]int{}, map[string]int{}, 0, 0, 0
	for _, e := range static {
		kinds[e.Kind]++
		if e.Status == StatusResolved {
			resolved++
		} else {
			reasons[e.Reason]++
		}
		if e.FromStatus == StatusResolved {
			fromBound++
		}
		if e.Observed {
			alsoObserved++
		}
	}
	provenance := map[string]int{ProvenanceStatic: len(static)}
	for _, e := range edges {
		provenance[e.Provenance]++
	}
	items := make([]any, 0, len(static)+len(edges))
	for _, e := range append(static, edges...) {
		items = append(items, e)
	}
	unknowns := make([]any, len(unsupported))
	for i, u := range unsupported {
		unknowns[i] = u
	}
	head := []field{{"schema", EdgesSchema}, {"app", m.App}, {"map_revision", m.Revision}, {"map_digest", m.Digest},
		{"budget", budget}, {"full", o.Full}, {"authority", skAuthority}, {"limits", staticEdgesLimit}, {"sources", sources},
		{"files_read", files}, {"files_skipped", skipped}, {"provenance", provenance}, {"static_kinds", kinds},
		{"static_resolved", resolved}, {"static_from_bound", fromBound}, {"static_unknown_reasons", reasons},
		{"static_also_observed", alsoObserved}, {"ledger", "flows appmap edges"}, {"unknowns_total", len(unsupported)}}
	return render(head, []section{{"edges", items}, {"unknowns", unknowns}}, budget)
}

// buildLinks reads the declared sources' static links and the map's test-observed and
// flow-declared edges (SKH-V0-110..116); Links prints them and How admits them (SKH-V0-145).
func buildLinks(ctx context.Context, m *Map, sources []string, o Options) (static, edges []SKEdge, unsupported []SKUnknown, files, skipped int, err error) {
	unsupported = []SKUnknown{}
	in, _, _, skipped, err := readSKSources(ctx, m, sources, o, func(p string) bool {
		ext := strings.ToLower(path.Ext(p))
		if skUnsupportedTemplate[ext] {
			unsupported = append(unsupported, SKUnknown{Path: p, Form: ext, Construct: "template-language", Knowledge: "navigation-edges", Reason: "unsupported-template-language"})
			return false
		}
		return (webSuffix[ext] || skTemplateSuffix[ext]) && !strings.HasSuffix(p, ".d.ts")
	})
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}
	r := repo{ctx: ctx, root: o.Root}
	views, err := readViewScreens(r, m, o.Repos)
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}
	idx := newScreenIndex(m.HashPrefix, m.BasePath, m.Screens)
	routed := newScreenIndex(m.HashPrefix, "", m.Screens)
	observed := map[string]bool{}
	edges = []SKEdge{}
	for _, e := range m.Edges {
		prov := ProvenanceTestObserved
		if e.Basis == "flow-step" {
			prov = ProvenanceFlowDeclared
		}
		if prov == ProvenanceTestObserved {
			observed[e.From+"\x00"+e.To] = true
		}
		edges = append(edges, SKEdge{ID: "edge:" + skDigest(prov, e.From, e.To, e.Source, e.Anchor.Path, strconv.Itoa(e.Anchor.Start)),
			Provenance: prov, Kind: e.Basis, From: e.From, FromStatus: StatusResolved, To: e.To, Status: StatusResolved,
			Source: e.Source, Authority: skAuthority, Anchor: e.Anchor})
	}
	static = []SKEdge{}
	for _, src := range in {
		p, text := src.entry.path, string(src.data)
		ext := strings.ToLower(path.Ext(p))
		if ext == ".html" || ext == ".htm" || ext == ".erb" || ext == ".vue" {
			text = maskComments(text, "<!--", "-->")
		}
		if ext == ".erb" {
			text = maskComments(text, "<%#", "%>")
		}
		sites := []skLinkSite{}
		var defs []*rubyDef
		switch {
		case ext == ".rb":
			rb := parseRubyStructure(p, src.data)
			defs = rb.defs
			sites = rubyLinks(rb.code, rb.raw)
		case ext == ".erb":
			lines := strings.Split(text, "\n")
			sites = append(templateLinks(text), rubyLinks(lines, lines)...)
		default:
			sites = templateLinks(text)
			if webSuffix[ext] || ext == ".vue" || ext == ".html" || ext == ".htm" {
				sites = append(sites, scriptLinks(text)...)
			}
		}
		for _, site := range sites {
			e := SKEdge{Provenance: ProvenanceStatic, Kind: site.kind, Target: site.target, Authority: skAuthority,
				Anchor: spanOf(src.entry, src.data, site.line, site.line)}
			e.From, e.FromStatus, e.FromReason = views.fromScreen(p, site.line, defs)
			switch {
			case secretscreen.MatchString(site.target):
				// A secret-shaped literal is never printed (SKH-V0-006).
				e.Target, e.Status, e.Reason = "", StatusUnknown, "screened-target"
			case site.dynamic:
				e.Status, e.Reason = StatusUnknown, "dynamic-target"
			case site.space == "state" || site.space == "helper":
				if strings.HasPrefix(site.target, ".") || strings.HasPrefix(site.target, "^") {
					e.Status, e.Reason = StatusUnknown, "relative-state"
				} else if id := views.statesO[site.target]; id != "" {
					e.To, e.Status = id, StatusResolved
				} else {
					e.Status, e.Reason = StatusUnknown, "no-matching-screen"
				}
			case !strings.HasPrefix(site.target, "/"):
				e.Status, e.Reason = StatusUnknown, "relative-target"
			default:
				ix := idx
				if site.routed {
					ix = routed
				}
				if id, why := ix.byURL(site.target); id != "" {
					e.To, e.Status = id, StatusResolved
				} else {
					e.Status, e.Reason = StatusUnknown, why
				}
			}
			e.Observed = e.FromStatus == StatusResolved && e.Status == StatusResolved && observed[e.From+"\x00"+e.To]
			e.ID = "edge:" + skDigest(e.Provenance, e.Kind, p, strconv.Itoa(site.line), e.Target, strconv.Itoa(len(static)))
			static = append(static, e)
			if len(static) > maxStaticEdges {
				return nil, nil, nil, 0, 0, bound("declared sources hold more than " + strconv.Itoa(maxStaticEdges) + " static links")
			}
		}
	}
	return static, edges, unsupported, len(in), skipped, nil
}

// fromScreen names the screen a source file renders: a Rails view by its path, a controller line
// by its enclosing action; any other file is not bound (SKH-V0-112).
func (v *skViewScreens) fromScreen(p string, line int, defs []*rubyDef) (string, string, string) {
	key := ""
	if ctrl, action, ok := v.actionOf(p); ok {
		key = ctrl + "#" + action
	} else if strings.HasSuffix(p, "_controller.rb") {
		for _, root := range v.roots {
			rel := p
			if root != "." {
				if !strings.HasPrefix(p, root+"/") {
					continue
				}
				rel = strings.TrimPrefix(p, root+"/")
			}
			ctrl, ok := strings.CutPrefix(strings.TrimSuffix(rel, "_controller.rb"), "app/controllers/")
			if !ok {
				continue
			}
			for _, d := range defs {
				if d.instance && d.line <= line && line <= d.end {
					key = ctrl + "#" + d.name
				}
			}
		}
		if key == "" {
			return "", StatusUnknown, "no-enclosing-action"
		}
	}
	if key == "" {
		return "", StatusUnknown, "source-not-bound"
	}
	id, ok := v.views[key]
	switch {
	case !ok:
		return "", StatusUnknown, "no-screen-for-action"
	case id == "":
		return "", StatusUnknown, "ambiguous-action-screen"
	}
	return id, StatusResolved, ""
}
