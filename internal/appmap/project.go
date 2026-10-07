package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Projection byte budgets (AMAP-V0-011).
const (
	DefaultScreenBudget   = 4096
	DefaultFlowBudget     = 6144
	DefaultFindBudget     = 2048
	DefaultScaffoldBudget = 6144
	MinBudget             = 256
	MaxBudget             = 65536
	FullBudget            = 1 << 20
)

// Options are the shared projection inputs.
type Options struct {
	// Root is the repository the freshness check reads; Revision is the evaluated revision
	// (default HEAD) that every returned anchor is compared against (AMAP-V0-010).
	Root, Revision string
	// Budget overrides the projection's default byte budget; Full raises it to FullBudget.
	Budget int
	Full   bool
	// Overlays supply learned facts (AMAP-V0-014). The CLI passes none.
	Overlays []Overlay
}

func budgetError(format string, args ...any) error {
	return &gokernel.Error{Code: "appmap-budget-too-small", Message: fmt.Sprintf(format, args...)}
}

func (o Options) budget(def int) (int, error) {
	switch {
	case o.Full && o.Budget != 0:
		return 0, &gokernel.Error{Code: "appmap-invalid-query", Message: "--budget and --full are exclusive"}
	case o.Full:
		return FullBudget, nil
	case o.Budget == 0:
		return def, nil
	case o.Budget < MinBudget || o.Budget > MaxBudget:
		return 0, &gokernel.Error{Code: "appmap-invalid-query", Message: fmt.Sprintf("--budget must be %d to %d bytes", MinBudget, MaxBudget)}
	}
	return o.Budget, nil
}

// LoadMap reads an application-map/0 file: a regular file, bounded, closed JSON, digest checked.
func LoadMap(filename string) (*Map, error) {
	bad := func(msg string) error { return &gokernel.Error{Code: "appmap-invalid-map", Message: msg} }
	before, err := os.Lstat(filename)
	if err != nil || !before.Mode().IsRegular() {
		return nil, bad("--map must name a regular file")
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, bad("cannot open --map")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, bad("--map changed while being read")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxMapBytes+1))
	if err != nil || len(raw) > maxMapBytes {
		return nil, bad("--map is unreadable or exceeds the map bound")
	}
	var m Map
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&m) != nil || d.Decode(new(any)) != io.EOF {
		return nil, bad("--map is not application-map/0 JSON")
	}
	if m.Schema != MapSchema {
		return nil, bad("--map schema must be " + MapSchema)
	}
	if sum, err := mapDigest(&m); err != nil || sum != m.Digest {
		return nil, bad("--map digest does not match its content")
	}
	return &m, nil
}

// freshness compares returned anchors with the evaluated revision (AMAP-V0-010).
type freshness struct {
	evaluated string
	same      bool
	failed    bool
	oids      map[string]string
	data      map[string][]byte
}

func newFreshness(ctx context.Context, m *Map, o Options, anchors []Anchor) *freshness {
	f := &freshness{oids: map[string]string{}, data: map[string][]byte{}}
	revision := o.Revision
	if revision == "" {
		revision = "HEAD"
	}
	r := repo{ctx: ctx, root: o.Root}
	rev, err := r.resolve(revision)
	if o.Root == "" || err != nil {
		f.failed, f.evaluated = true, FreshUnknown
		return f
	}
	f.evaluated = rev
	if rev == m.Revision {
		f.same = true
		return f
	}
	paths := []string{}
	seen := map[string]bool{}
	for _, a := range anchors {
		if a.Path != "" && !seen[a.Path] {
			seen[a.Path] = true
			paths = append(paths, a.Path)
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return f
	}
	entries, err := r.tree(rev, paths, 4*len(paths)+64)
	if err != nil {
		f.failed = true
		return f
	}
	changed := []blobEntry{}
	pinned := map[string]string{}
	for _, a := range anchors {
		pinned[a.Path] = a.Blob
	}
	for _, e := range entries {
		if !seen[e.path] {
			continue
		}
		f.oids[e.path] = e.oid
		if e.oid != pinned[e.path] && e.size <= maxRouterBytes {
			changed = append(changed, e)
		}
	}
	blobs, err := r.blobs(changed, maxRouterBytes, 64*maxRouterBytes)
	if err != nil {
		f.failed = true
		return f
	}
	for _, e := range changed {
		f.data[e.path] = blobs[e.oid]
	}
	return f
}

// of is FRESH when the anchored lines are byte-identical at the evaluated revision, STALE when the
// path is gone or the lines differ, and UNKNOWN when Git could not answer.
func (f *freshness) of(a Anchor) string {
	switch {
	case f.same:
		return Fresh
	case f.failed:
		return FreshUnknown
	}
	oid, ok := f.oids[a.Path]
	if !ok {
		return Stale
	}
	if oid == a.Blob {
		return Fresh
	}
	data, ok := f.data[a.Path]
	if !ok {
		return FreshUnknown
	}
	if spanOf(blobEntry{path: a.Path, oid: oid}, data, a.Start, a.End).SpanSHA256 == a.SpanSHA256 {
		return Fresh
	}
	return Stale
}

// field and section are the parts of a projection document; render fits them to the budget.
type field struct {
	key   string
	value any
}

type section struct {
	key   string
	items []any
}

// render writes head fields, then each section's leading items, then `omitted` counts, keeping
// the whole document within budget bytes (AMAP-V0-011). Sections are filled evenly first and then
// in order; a head that cannot fit refuses rather than truncating.
func render(head []field, sections []section, budget int) ([]byte, error) {
	headRaw := make([][]byte, len(head))
	for i, f := range head {
		raw, err := json.Marshal(f.value)
		if err != nil {
			return nil, err
		}
		key, _ := json.Marshal(f.key)
		headRaw[i] = append(append(key, ':'), raw...)
	}
	items := make([][][]byte, len(sections))
	longest := 0
	for i, s := range sections {
		for _, it := range s.items {
			raw, err := json.Marshal(it)
			if err != nil {
				return nil, err
			}
			items[i] = append(items[i], raw)
		}
		longest = max(longest, len(s.items))
	}
	keep := make([]int, len(sections))
	size := func(keep []int) int {
		n := 2 // {}
		for _, h := range headRaw {
			n += len(h) + 1
		}
		omitted := 2 + len(`"omitted":`)
		for i, s := range sections {
			n += len(s.key) + 6 // "key":[], with its trailing comma
			for k := 0; k < keep[i]; k++ {
				n += len(items[i][k]) + 1
			}
			if keep[i] > 0 {
				n--
			}
			omitted += len(s.key) + 3 + len(strconv.Itoa(len(items[i])-keep[i])) + 1
		}
		if len(sections) > 0 {
			omitted--
		}
		return n + omitted
	}
	set := func(k int) {
		for i := range keep {
			keep[i] = min(k, len(items[i]))
		}
	}
	set(0)
	if size(keep) > budget {
		return nil, budgetError("the projection head needs more than %d bytes; raise --budget or use --full", budget)
	}
	lo, hi := 0, longest
	for lo < hi {
		mid := (lo + hi + 1) / 2
		set(mid)
		if size(keep) <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	set(lo)
	for i := range keep {
		l, h := keep[i], len(items[i])
		for l < h {
			mid := (l + h + 1) / 2
			keep[i] = mid
			if size(keep) <= budget {
				l = mid
			} else {
				h = mid - 1
			}
		}
		keep[i] = l
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for _, h := range headRaw {
		buf.Write(h)
		buf.WriteByte(',')
	}
	for i, s := range sections {
		key, _ := json.Marshal(s.key)
		buf.Write(key)
		buf.WriteString(":[")
		for k := 0; k < keep[i]; k++ {
			if k > 0 {
				buf.WriteByte(',')
			}
			buf.Write(items[i][k])
		}
		buf.WriteString("],")
	}
	buf.WriteString(`"omitted":{`)
	for i, s := range sections {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, _ := json.Marshal(s.key)
		buf.Write(key)
		buf.WriteByte(':')
		buf.WriteString(strconv.Itoa(len(items[i]) - keep[i]))
	}
	buf.WriteString("}}")
	if buf.Len() > budget {
		return nil, errors.New("internal: projection exceeded its budget")
	}
	return buf.Bytes(), nil
}

// projection gathers what one call returns, the anchors it cites and the elements overlays may
// annotate.
type projection struct {
	ctx      context.Context
	m        *Map
	o        Options
	anchors  []Anchor
	elements map[string]bool
	fresh    *freshness
}

func newProjection(ctx context.Context, m *Map, o Options) *projection {
	return &projection{ctx: ctx, m: m, o: o, elements: map[string]bool{}}
}

func (p *projection) cite(a Anchor) { p.anchors = append(p.anchors, a) }
func (p *projection) element(ids ...string) {
	for _, id := range ids {
		if id != "" {
			p.elements[id] = true
		}
	}
}

// check runs the freshness comparison once over every cited anchor.
func (p *projection) check() { p.fresh = newFreshness(p.ctx, p.m, p.o, p.anchors) }

func (p *projection) state(a Anchor) string { return p.fresh.of(a) }

// envelope is the head every projection starts with.
func (p *projection) envelope(schema string, budget int) []field {
	return []field{{"schema", schema}, {"app", p.m.App}, {"map_revision", p.m.Revision}, {"map_digest", p.m.Digest},
		{"evaluated_revision", p.fresh.evaluated}, {"budget", budget}, {"full", p.o.Full}}
}

// Overlay bounds: facts longer than maxFactBytes are dropped; past maxFacts the rest are dropped
// and reported.
const (
	maxFactBytes = 1024
	maxFacts     = 4096
)

type learnedItem struct {
	Fact
	Freshness string `json:"freshness,omitempty"`
}

// learned asks every overlay once for facts about the projection's elements (AMAP-V0-014).
func (p *projection) learned(stale map[string]bool) ([]any, []any) {
	items, unknowns := []any{}, []any{}
	if len(p.o.Overlays) == 0 || len(p.elements) == 0 {
		return items, unknowns
	}
	ids := sortedKeys(p.elements)
	facts := []learnedItem{}
	for _, ov := range p.o.Overlays {
		got, err := ov.Facts(p.ctx, ids)
		if err != nil {
			unknowns = append(unknowns, Unknown{Kind: "overlay", Ref: "learned", Reason: "overlay-unavailable"})
			continue
		}
		for _, f := range got {
			if !p.elements[f.ElementID] || len(f.Text) > maxFactBytes || len(f.Source)+len(f.Kind)+len(f.Revision) > maxFactBytes ||
				secretscreen.MatchString(f.Text+"\n"+f.Source+"\n"+f.Kind+"\n"+f.Revision) {
				continue
			}
			if len(facts) == maxFacts {
				unknowns = append(unknowns, Unknown{Kind: "overlay", Ref: "learned", Reason: "overlay-bound-exceeded"})
				break
			}
			f.Authority = AuthorityLearned
			item := learnedItem{Fact: f}
			if stale[f.ElementID] {
				item.Freshness = Stale
			}
			facts = append(facts, item)
		}
	}
	sort.SliceStable(facts, func(i, j int) bool {
		x, y := facts[i], facts[j]
		if x.ElementID != y.ElementID {
			return x.ElementID < y.ElementID
		}
		if x.Source != y.Source {
			return x.Source < y.Source
		}
		if x.Kind != y.Kind {
			return x.Kind < y.Kind
		}
		return x.Text < y.Text
	})
	for _, f := range facts {
		items = append(items, f)
	}
	return items, unknowns
}

func (m *Map) screen(id string) *Screen {
	i := sort.Search(len(m.Screens), func(i int) bool { return m.Screens[i].ID >= id })
	if i < len(m.Screens) && m.Screens[i].ID == id {
		return &m.Screens[i]
	}
	return nil
}

func (m *Map) file(p string) *TestFile {
	i := sort.Search(len(m.Files), func(i int) bool { return m.Files[i].Path >= p })
	if i < len(m.Files) && m.Files[i].Path == p {
		return &m.Files[i]
	}
	return nil
}

func (m *Map) flow(q string) *Flow {
	for i := range m.Flows {
		if m.Flows[i].ID == q || m.Flows[i].FlowID == q {
			return &m.Flows[i]
		}
	}
	return nil
}

// url prints a screen's template the way a browser loads it, with the hash-routing prefix.
func (m *Map) url(s *Screen) string {
	if s == nil || s.Template == "" {
		return ""
	}
	if m.HashPrefix != "" {
		return "/" + m.HashPrefix + s.Template
	}
	return s.Template
}

// findScreen resolves a screen query: an element ID, a state name, a route template (`*` or
// `{name}` for parameters) or a concrete URL. None, or more than one candidate, is UNKNOWN with
// the candidates that collided (AMAP-V0-003); the map never prefers one.
func (m *Map) findScreen(q string) (string, string, []string) {
	if strings.HasPrefix(q, "screen:") {
		if m.screen(q) != nil {
			return q, "", nil
		}
		return "", "no-matching-screen", nil
	}
	if s := m.screen(screenID(m.App, q)); s != nil {
		return s.ID, "", nil
	}
	x := newScreenIndex(m.HashPrefix, m.Screens)
	var ids []string
	var reason string
	switch {
	case strings.Contains(q, "{") || strings.Contains(q, "*") || strings.Contains(q, "/:"):
		if m.HashPrefix != "" {
			q = strings.TrimPrefix(q, "/"+m.HashPrefix)
		}
		segs := strings.Split(q, "/")
		for i, s := range segs {
			if s == "*" {
				segs[i] = "{p}"
			}
		}
		ids, reason = x.templateMatches(strings.Join(segs, "/"))
	case strings.HasPrefix(q, "/") || strings.Contains(q, "://"):
		ids, reason = x.urlMatches(q)
	default:
		return "", "no-matching-screen", nil
	}
	id, reason := pick(ids, reason)
	if id != "" {
		return id, "", nil
	}
	return "", reason, ids
}

type selectorView struct {
	Kind     string `json:"kind"`
	Value    string `json:"value"`
	Name     string `json:"name,omitempty"`
	Strength string `json:"strength"`
}

func viewSelector(s *Selector) *selectorView {
	if s == nil {
		return nil
	}
	return &selectorView{Kind: s.Kind, Value: s.Value, Name: s.Name, Strength: s.Strength}
}

type anchorView struct {
	Ref       string `json:"ref"`
	Blob      string `json:"blob"`
	Freshness string `json:"freshness"`
}

func anchorRef(a Anchor) string {
	if a.Start > 0 {
		return fmt.Sprintf("%s:%d-%d", a.Path, a.Start, a.End)
	}
	return a.Path
}

func (p *projection) anchorView(a Anchor) anchorView {
	return anchorView{Ref: anchorRef(a), Blob: a.Blob, Freshness: p.state(a)}
}

type stepView struct {
	ID         string        `json:"id"`
	Action     string        `json:"action"`
	Screen     string        `json:"screen,omitempty"`
	Template   string        `json:"template,omitempty"`
	Status     string        `json:"status"`
	Reason     string        `json:"reason,omitempty"`
	Selector   *selectorView `json:"selector,omitempty"`
	Reuse      []string      `json:"reuse"`
	ReuseTotal int           `json:"reuse_total"`
	Freshness  string        `json:"freshness"`
}

const reuseShown = 3

func (p *projection) stepView(fl *Flow, st Step) stepView {
	v := stepView{ID: st.ID, Action: st.Action, Screen: st.Screen, Status: st.Status, Reason: st.Reason, Selector: viewSelector(st.Selector),
		Reuse: st.Reuse[:min(len(st.Reuse), reuseShown)], ReuseTotal: len(st.Reuse), Freshness: p.state(fl.Anchor)}
	if s := p.m.screen(st.Screen); s != nil {
		v.Template = s.Template
	}
	return v
}

type edgeView struct {
	From   string     `json:"from"`
	To     string     `json:"to"`
	Basis  string     `json:"basis"`
	Source string     `json:"source"`
	Anchor anchorView `json:"anchor"`
}

type specView struct {
	File       string     `json:"file"`
	Basis      string     `json:"basis"`
	Via        []string   `json:"via"`
	Assertions int        `json:"assertions"`
	Join       string     `json:"join"`
	Anchor     anchorView `json:"anchor"`
}

func (p *projection) specView(a Attribution) specView {
	v := specView{File: a.File, Basis: a.Basis, Via: a.Via, Join: StatusUnknown}
	if f := p.m.file(a.File); f != nil {
		v.Assertions, v.Join, v.Anchor = f.Assertions, f.Join, p.anchorView(f.Anchor)
	}
	return v
}

// unknownsFor lists the map's unknowns that name any of refs.
func (m *Map) unknownsFor(refs map[string]bool) []any {
	out := []any{}
	for _, u := range m.Unknowns {
		if refs[u.Ref] {
			out = append(out, u)
		}
	}
	return out
}

// unresolvedSpecs lists the specs whose import join is UNKNOWN: any of them may reach any screen.
func (m *Map) unresolvedSpecs() []any {
	out := []any{}
	for _, f := range m.Files {
		if f.Role == roleSpec && f.Join != StatusResolved {
			out = append(out, f.Path)
		}
	}
	return out
}

func strings2any(in []string) []any {
	out := make([]any, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}
