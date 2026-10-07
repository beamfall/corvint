package appmap

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitAll(t *testing.T, root, message string) string {
	t.Helper()
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", message)
	return gitTest(t, root, "rev-parse", "HEAD")
}

// fixtureRepo commits testdata/fixture into a fresh repository and returns its root and revision.
func fixtureRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	err := filepath.WalkDir("testdata/fixture", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel("testdata/fixture", p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(root, rel)
		if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "init", "-q")
	return root, commitAll(t, root, "fixture")
}

func build(t *testing.T, root, rev string) *Map {
	t.Helper()
	m, err := Build(context.Background(), root, "appmap.json", rev)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func codeOf(err error) string {
	var coded *gokernel.Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("projection is not JSON: %v\n%s", err, raw)
	}
	return out
}

func screenByID(t *testing.T, m *Map, state string) *Screen {
	t.Helper()
	s := m.screen(screenID(m.App, state))
	if s == nil {
		t.Fatalf("no screen %s", state)
	}
	return s
}

func hasUnknown(m *Map, kind, ref, reason string) bool {
	for _, u := range m.Unknowns {
		if u.Kind == kind && u.Ref == ref && u.Reason == reason {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, root, name, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// AMAP-V0-002 AMAP-V0-003 AMAP-V0-006: nested, relative and absolute states resolve to templates
// with inherited permissions and flags; non-literal, orphaned and colliding states read UNKNOWN.
func TestAMAPV0002HierarchyResolution(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	cases := map[string]string{
		"app":                 "/",
		"app.home":            "/home",
		"app.clubs":           "/clubs/{clubId}",
		"app.clubs.teesheets": "/clubs/{clubId}/teesheets",
		"reports":             "/clubs/{clubId}/reports/{reportId}",
		"login":               "/login",
	}
	for state, template := range cases {
		s := screenByID(t, m, state)
		if s.Status != StatusResolved || s.Template != template {
			t.Errorf("%s: %s %q, want resolved %q", state, s.Status, s.Template, template)
		}
	}
	tee := screenByID(t, m, "app.clubs.teesheets")
	if strings.Join(tee.Query, ",") != "date" || strings.Join(tee.Flags, ",") != "new_teesheet" || tee.Parent != "screen:admin:app.clubs" {
		t.Errorf("teesheets query/flags/parent: %+v", tee)
	}
	settings := screenByID(t, m, "app.clubs.settings")
	if strings.Join(settings.Permissions, ",") != "club.view" || settings.PermissionsFrom != "screen:admin:app.clubs" {
		t.Errorf("settings did not inherit permissions: %+v", settings)
	}
	if strings.Join(screenByID(t, m, "reports").Params, ",") != "clubId,reportId" {
		t.Errorf("reports params: %v", screenByID(t, m, "reports").Params)
	}
	for state, reason := range map[string]string{"app.dynamic": "non-literal-url", "orphan.child": "missing-parent"} {
		s := screenByID(t, m, state)
		if s.Status != StatusUnknown || s.Reason != reason || s.Template != "" {
			t.Errorf("%s: %s %s %q, want UNKNOWN %s", state, s.Status, s.Reason, s.Template, reason)
		}
	}
	if !hasUnknown(m, "template", "/clubs/{}/members", "ambiguous-template") {
		t.Error("colliding /members states not reported")
	}
	for q, want := range map[string]string{"/clubs/*/members": "ambiguous-template", "nothing": "no-matching-screen"} {
		doc := screenDoc(t, m, q, Options{})
		cands, _ := doc["candidates"].([]any)
		if doc["status"] != StatusUnknown || doc["reason"] != want || (want == "ambiguous-template") != (len(cands) == 2) {
			t.Errorf("screen %q: %v %v %v", q, doc["status"], doc["reason"], cands)
		}
	}
	x := newScreenIndex(m.HashPrefix, m.Screens)
	for url, want := range map[string]string{
		"/#!/clubs/7/teesheets?date=1": "screen:admin:app.clubs.teesheets",
		"https://host/#!/home":         "screen:admin:app.home",
		"/#!/clubs/7/members":          "",
		"/#!/nowhere":                  "",
	} {
		if got, _ := x.byURL(url); got != want {
			t.Errorf("byURL(%s) = %q, want %q", url, got, want)
		}
	}
}

// AMAP-V0-004: flow navigation steps land on screens with selectors and reuse points; a step whose
// state matches two screens is UNKNOWN, never assigned to either.
func TestAMAPV0004FlowSteps(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	fl := m.flow("book-tee-time")
	want := []string{"screen:admin:app.home", "screen:admin:app.clubs.teesheets", "screen:admin:app.clubs.teesheets"}
	for i, st := range fl.Steps {
		if st.Screen != want[i] || st.Status != StatusResolved || len(st.Reuse) == 0 {
			t.Errorf("step %s: %+v", st.ID, st)
		}
	}
	if fl.Steps[2].Selector.Strength != "medium" || fl.Steps[2].Reuse[0] != "method:e2e/pages/teesheet.page.ts#book" {
		t.Errorf("book step: %+v", fl.Steps[2])
	}
	members := m.flow("view-members").Steps[0]
	if members.Status != StatusUnknown || members.Reason != "ambiguous-template" || members.Screen != "" {
		t.Errorf("ambiguous step resolved: %+v", members)
	}
	pre := screenByID(t, m, "app.home").Preconditions
	gap := strings.Replace(strings.Replace(readText(t, root, "flows/book-tee-time.json"), `"book-tee-time"`, `"gap"`, 1),
		`"/clubs/{clubId}/teesheets", "locator": {"test_id": "slot"}`, `"/clubs/{clubId}/members", "locator": {"test_id": "slot"}`, 1)
	writeFile(t, root, "flows/gap.json", gap)
	gapped := build(t, root, commitAll(t, root, "flow with an unplaced middle step"))
	if st := gapped.flow("gap").Steps; st[1].Status != StatusUnknown || st[2].Status != StatusResolved {
		t.Fatalf("gap flow steps: %+v", st)
	}
	for _, e := range gapped.Edges {
		if strings.HasPrefix(e.Source, "step:gap/") {
			t.Errorf("edge inferred across an unplaced step: %+v", e)
		}
	}
	if len(pre) != 1 || pre[0].Text != "club has open slots" {
		t.Errorf("home preconditions: %+v", pre)
	}
}

// AMAP-V0-005 AMAP-V0-008: a spec that reaches a screen only by clicking through page objects is
// attributed through the import graph; alias and missing imports leave the join UNKNOWN.
func TestAMAPV0005ImportGraphJoin(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	click := m.file("e2e/specs/teesheet-click.spec.ts")
	if len(click.Gotos) != 0 || click.Join != StatusResolved {
		t.Fatalf("click spec: gotos %v join %s", click.Gotos, click.Join)
	}
	attributed := func(state, file string) *Attribution {
		for _, a := range screenByID(t, m, state).Specs {
			if a.File == file {
				return &a
			}
		}
		return nil
	}
	a := attributed("app.clubs.teesheets", "e2e/specs/teesheet-click.spec.ts")
	if a == nil || a.Basis != "import" || strings.Join(a.Via, ">") != "e2e/pages/teesheet.page.ts" {
		t.Fatalf("click spec not attributed through imports: %+v", a)
	}
	w := attributed("app.clubs.teesheets", "e2e/specs/booking-workflow.spec.ts")
	if w == nil || strings.Join(w.Via, ">") != "e2e/workflows/booking.ts>e2e/pages/teesheet.page.ts" {
		t.Fatalf("workflow chain: %+v", w)
	}
	if g := attributed("login", "e2e/specs/login.spec.ts"); g == nil || g.Basis != "goto" {
		t.Fatalf("goto attribution: %+v", g)
	}
	if g := attributed("app.clubs.settings", "e2e/specs/login.spec.ts"); g == nil {
		t.Fatal("whole-segment template goto not attributed")
	}
	for _, p := range []string{"e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts"} {
		if f := m.file(p); f.Join != StatusUnknown || !hasUnknown(m, "import", fileID(p), "unresolved-import") {
			t.Errorf("%s: join %s", p, f.Join)
		}
		if attributed("app.clubs.teesheets", p) != nil {
			t.Errorf("%s attributed despite an unresolved import", p)
		}
	}
	if m.testJoin() != StatusUnknown {
		t.Fatal("screen join complete while specs have unresolved imports")
	}
	sequence := false
	for _, e := range m.Edges {
		if e.Basis == "test-sequence" && e.Source == click.ID && e.From == "screen:admin:app.home" && e.To == "screen:admin:app.clubs.teesheets" &&
			e.Anchor.Start == 6 && e.Anchor.End == 8 {
			sequence = true
		}
	}
	if !sequence {
		t.Fatalf("no test-sequence edge from the click spec: %+v", m.Edges)
	}
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	clean := build(t, root, commitAll(t, root, "drop unresolved specs"))
	if clean.testJoin() != StatusResolved {
		t.Fatal("join stays UNKNOWN with every import resolved")
	}
}

// AMAP-V0-007: selector kinds map to strength classes; a non-literal argument is unknown.
func TestAMAPV0007SelectorStrength(t *testing.T) {
	f := readFacts(`page.getByTestId('a'); page.getByRole('button', { name: 'Go' }); page.getByText('hi');
page.locator('[data-testid="b"]'); page.locator('.c'); page.locator(dynamic); page.getByLabel('Email');`)
	got := []string{}
	for _, s := range f.selectors {
		got = append(got, s.Kind+":"+s.Value+":"+s.Name+":"+s.Strength)
	}
	want := "test-id:a::strong role:button:Go:medium text:hi::weak test-id:b::strong css:.c::weak css:::unknown label:Email::medium"
	if strings.Join(got, " ") != want {
		t.Fatalf("selectors:\n got %s\nwant %s", strings.Join(got, " "), want)
	}
	if newSelector("test-id", "a", "", strengthStrong, 1).ID != newSelector("test-id", "a", "", strengthStrong, 9).ID {
		t.Fatal("selector ID depends on its line")
	}
}

// AMAP-V0-009: the artifact is byte-identical across builds, digest-sealed and refused when edited.
func TestAMAPV0009DeterministicArtifact(t *testing.T) {
	root, rev := fixtureRepo(t)
	first, err := Encode(build(t, root, rev))
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Encode(build(t, root, rev))
	if string(first) != string(second) {
		t.Fatal("two builds differ")
	}
	file := filepath.Join(t.TempDir(), "map.json")
	if err = os.WriteFile(file, first, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadMap(file); err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(first), `"teesheet.view"`, `"teesheet.edit"`, 1)
	if err = os.WriteFile(file, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadMap(file); codeOf(err) != "appmap-invalid-map" {
		t.Fatalf("edited map accepted: %v", err)
	}
}

func screenDoc(t *testing.T, m *Map, q string, o Options) map[string]any {
	t.Helper()
	raw, err := ProjectScreen(context.Background(), m, q, o)
	if err != nil {
		t.Fatal(err)
	}
	return decode(t, raw)
}

func freshnessOf(doc map[string]any, section, id string) string {
	items, _ := doc[section].([]any)
	for _, it := range items {
		v := it.(map[string]any)
		if v["id"] == id {
			return v["anchor"].(map[string]any)["freshness"].(string)
		}
	}
	return ""
}

// AMAP-V0-010: an element whose anchored lines changed after the map revision reads STALE; an
// untouched element in the same changed file stays FRESH; an unreadable revision is UNKNOWN.
func TestAMAPV0010StaleAnchors(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	page := "e2e/pages/teesheet.page.ts"
	data, _ := os.ReadFile(filepath.Join(root, page))
	writeFile(t, root, page, strings.Replace(string(data), "getByTestId('slot')", "getByTestId('slot-cell')", 1))
	commitAll(t, root, "rename slot test id")
	doc := screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Full: true})
	if doc["evaluated_revision"] == m.Revision {
		t.Fatal("projection evaluated the map revision, not HEAD")
	}
	if got := freshnessOf(doc, "page_object_methods", "method:"+page+"#selectSlot"); got != Stale {
		t.Errorf("changed method reads %q", got)
	}
	if got := freshnessOf(doc, "page_object_methods", "method:"+page+"#book"); got != Fresh {
		t.Errorf("unchanged method in a changed file reads %q", got)
	}
	if got := doc["screen"].(map[string]any)["anchor"].(map[string]any)["freshness"]; got != Fresh {
		t.Errorf("router anchor reads %v", got)
	}
	gitTest(t, root, "rm", "-q", page)
	commitAll(t, root, "delete page object")
	doc = screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Full: true})
	if got := freshnessOf(doc, "page_object_methods", "method:"+page+"#book"); got != Stale {
		t.Errorf("deleted file reads %q", got)
	}
	doc = screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Revision: "no-such-rev", Full: true})
	if got := freshnessOf(doc, "page_object_methods", "method:"+page+"#book"); got != FreshUnknown {
		t.Errorf("unresolvable revision reads %q", got)
	}
	if doc := screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Revision: rev, Full: true}); freshnessOf(doc, "page_object_methods", "method:"+page+"#selectSlot") != Fresh {
		t.Error("map revision itself is not FRESH")
	}
}

// bigMap adds n specs attributed to one screen and n steps on it, so an uncapped projection would
// be hundreds of kilobytes.
func bigMap(t *testing.T, root, rev string, n int) *Map {
	t.Helper()
	m := build(t, root, rev)
	s := m.screen("screen:admin:app.clubs.teesheets")
	for i := 0; i < n; i++ {
		p := "e2e/specs/generated-" + strings.Repeat("x", 40) + "-" + strconv.Itoa(i) + ".spec.ts"
		s.Specs = append(s.Specs, Attribution{File: p, Basis: "import", Via: []string{"e2e/pages/teesheet.page.ts"}})
		s.Steps = append(s.Steps, "step:book-tee-time/book")
	}
	return m
}

// AMAP-V0-011: every projection stays within its budget, reports omitted counts, and --full
// raises the ceiling without removing it; a budget too small for the head is refused.
func TestAMAPV0011ProjectionBudgets(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := bigMap(t, root, rev, 2000)
	ctx := context.Background()
	o := Options{Root: root}
	calls := map[string]func(Options) ([]byte, error){
		"screen":   func(o Options) ([]byte, error) { return ProjectScreen(ctx, m, "app.clubs.teesheets", o) },
		"flow":     func(o Options) ([]byte, error) { return ProjectFlow(ctx, m, "book-tee-time", o) },
		"find":     func(o Options) ([]byte, error) { return ProjectFind(ctx, m, "e2e", o) },
		"scaffold": func(o Options) ([]byte, error) { return ProjectScaffold(ctx, m, "book-tee-time", o) },
	}
	defaults := map[string]int{"screen": DefaultScreenBudget, "flow": DefaultFlowBudget, "find": DefaultFindBudget, "scaffold": DefaultScaffoldBudget}
	for name, call := range calls {
		raw, err := call(o)
		if err != nil || len(raw) > defaults[name] {
			t.Fatalf("%s: %d bytes over %d: %v", name, len(raw), defaults[name], err)
		}
		if _, ok := decode(t, raw)["omitted"].(map[string]any); !ok {
			t.Fatalf("%s: no omitted counts", name)
		}
		full, err := call(Options{Root: root, Full: true})
		if err != nil || len(full) > FullBudget || decode(t, full)["full"] != true {
			t.Fatalf("%s --full: %d bytes: %v", name, len(full), err)
		}
		if _, err = call(Options{Root: root, Budget: 300}); codeOf(err) != "appmap-budget-too-small" {
			t.Errorf("%s: tiny budget: %v", name, err)
		}
		for _, bad := range []Options{{Budget: MaxBudget + 1}, {Budget: 1}, {Budget: 4096, Full: true}} {
			if _, err = call(bad); codeOf(err) != "appmap-invalid-query" {
				t.Errorf("%s: budget %+v: %v", name, bad, err)
			}
		}
	}
	doc := screenDoc(t, m, "app.clubs.teesheets", o)
	omitted := doc["omitted"].(map[string]any)
	if omitted["specs"].(float64) < 1900 || omitted["steps"].(float64) < 1900 {
		t.Fatalf("omitted counts: %v", omitted)
	}
	specs := len(doc["specs"].([]any))
	if specs == 0 || float64(specs)+omitted["specs"].(float64) != float64(len(m.screen("screen:admin:app.clubs.teesheets").Specs)) {
		t.Fatalf("kept %d specs plus omitted %v does not add up", specs, omitted["specs"])
	}
	// --full lifts the default cap but keeps the 1 MiB ceiling: a map whose screen fits there
	// omits nothing.
	full := screenDoc(t, bigMap(t, root, rev, 200), "app.clubs.teesheets", Options{Root: root, Full: true})
	for k, v := range full["omitted"].(map[string]any) {
		if v.(float64) != 0 {
			t.Errorf("--full omitted %v %s", v, k)
		}
	}
}

// AMAP-V0-012: find matches case-insensitively over element IDs and labels and refuses an
// unbounded query.
func TestAMAPV0012Find(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	ctx := context.Background()
	for _, q := range []string{"", "a", strings.Repeat("a", 129)} {
		if _, err := ProjectFind(ctx, m, q, Options{}); codeOf(err) != "appmap-invalid-query" {
			t.Errorf("query %q: %v", q, err)
		}
	}
	raw, err := ProjectFind(ctx, m, "SELECTSLOT", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"method:e2e/pages/teesheet.page.ts#selectSlot"`) {
		t.Fatalf("find missed the method: %s", raw)
	}
}

// AMAP-V0-013: the scaffold borrows the closest asserting spec's imports, which resolve against
// the committed test tree, and calls existing page-object methods for steps they implement.
func TestAMAPV0013Scaffold(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	raw, err := ProjectScaffold(context.Background(), m, "book-tee-time", Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	doc := decode(t, raw)
	closest := doc["closest"].(map[string]any)
	if closest["file"] != "e2e/specs/teesheet-click.spec.ts" || closest["status"] != StatusResolved {
		t.Fatalf("closest: %v", closest)
	}
	proposed := doc["proposed_path"].(string)
	if proposed != "e2e/specs/book-tee-time.spec.ts" {
		t.Fatalf("proposed path %s", proposed)
	}
	relative := 0
	for _, it := range doc["imports"].([]any) {
		stmt := it.(string)
		if strings.HasPrefix(stmt, "// UNRESOLVED") {
			t.Errorf("unresolved import: %s", stmt)
		}
		spec := stmt[strings.LastIndexAny(stmt[:len(stmt)-2], `'"`)+1 : len(stmt)-2]
		if strings.HasPrefix(spec, ".") {
			relative++
			target := filepath.Join(root, filepath.Dir(proposed), spec+".ts")
			if _, err := os.Stat(target); err != nil {
				t.Errorf("import %s does not resolve from %s", spec, proposed)
			}
		}
	}
	if relative != 2 {
		t.Fatalf("relative imports: %v", doc["imports"])
	}
	lines := []string{}
	for _, l := range doc["lines"].([]any) {
		lines = append(lines, l.(string))
	}
	body := strings.Join(lines, "\n")
	for _, want := range []string{"await homePage.goToTeeSheets();", "await teeSheetPage.selectSlot();", "await teeSheetPage.book();", "expect outcome booked"} {
		if !strings.Contains(body, want) {
			t.Errorf("scaffold lacks %q:\n%s", want, body)
		}
	}
	raw, err = ProjectScaffold(context.Background(), m, "view-members", Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if c := decode(t, raw)["closest"].(map[string]any); c["status"] != StatusUnknown || c["reason"] != "no-asserting-spec" {
		t.Fatalf("flow with no asserting spec: %v", c)
	}
}

type fakeOverlay struct {
	facts []Fact
	err   error
	asked []string
}

func (f *fakeOverlay) Facts(_ context.Context, ids []string) ([]Fact, error) {
	f.asked = ids
	return f.facts, f.err
}

// AMAP-V0-014: overlay facts are keyed by element ID, printed as learned, filtered to the
// projection's elements, and never change the map's own nodes.
func TestAMAPV0014OverlaySeam(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	method := "method:e2e/pages/teesheet.page.ts#selectSlot"
	ov := &fakeOverlay{facts: []Fact{
		{ElementID: method, Source: "runs", Kind: "run-verified", Text: "clicked in run 3", Authority: "accepted"},
		{ElementID: "screen:admin:login", Source: "runs", Kind: "note", Text: "not on this screen"},
		{ElementID: method, Source: "runs", Kind: "note", Text: "token ghp_" + strings.Repeat("A", 36)},
	}}
	plain := screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Full: true})
	with := screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Full: true, Overlays: []Overlay{ov}})
	learned := with["learned"].([]any)
	if len(learned) != 1 {
		t.Fatalf("learned: %v", learned)
	}
	fact := learned[0].(map[string]any)
	if fact["element_id"] != method || fact["authority"] != AuthorityLearned {
		t.Fatalf("fact: %v", fact)
	}
	for _, key := range []string{"screen", "specs", "steps", "page_object_methods", "edges_in"} {
		a, _ := json.Marshal(plain[key])
		b, _ := json.Marshal(with[key])
		if string(a) != string(b) {
			t.Errorf("overlay changed %s", key)
		}
	}
	asked := strings.Join(ov.asked, " ")
	if !strings.Contains(asked, method) || !strings.Contains(asked, "screen:admin:app.clubs.teesheets") {
		t.Errorf("overlay asked for %s", asked)
	}
	ov.err = errors.New("store unavailable")
	failed := screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Full: true, Overlays: []Overlay{ov}})
	if !strings.Contains(fmtJSON(failed["unknowns"]), "overlay-unavailable") {
		t.Fatal("overlay failure not reported")
	}
}

func fmtJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// AMAP-V0-001 AMAP-V0-015: an invalid manifest or revision is refused with a coded error, and
// building and projecting never change the repository.
func TestAMAPV0015ReadOnlyAndRefusals(t *testing.T) {
	root, rev := fixtureRepo(t)
	before := gitTest(t, root, "status", "--porcelain", "--ignored")
	m := build(t, root, rev)
	ctx := context.Background()
	o := Options{Root: root}
	_, _ = ProjectScreen(ctx, m, "app.home", o)
	_, _ = ProjectFlow(ctx, m, "book-tee-time", o)
	_, _ = ProjectScaffold(ctx, m, "book-tee-time", o)
	if after := gitTest(t, root, "status", "--porcelain", "--ignored"); after != before {
		t.Fatalf("worktree changed:\n%s", after)
	}
	if _, err := Build(ctx, root, "appmap.json", "no-such-rev"); codeOf(err) != "appmap-invalid-revision" {
		t.Errorf("bad revision: %v", err)
	}
	if _, err := Build(ctx, root, "../appmap.json", rev); codeOf(err) != "appmap-invalid-manifest" {
		t.Errorf("escaping manifest: %v", err)
	}
	for name, manifest := range map[string]string{
		"unknown member": `{"schema":"application-map-manifest/0","app":"admin","routers":[{"path":"app/routes.js","dialect":"ui-router-states/0"}],"tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]},"extra":1}`,
		"bad dialect":    `{"schema":"application-map-manifest/0","app":"admin","routers":[{"path":"app/routes.js","dialect":"react"}],"tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]}}`,
		"outside root":   `{"schema":"application-map-manifest/0","app":"admin","routers":[{"path":"app/routes.js","dialect":"ui-router-states/0"}],"tests":{"root":"e2e","specs":["specs"],"page_objects":["e2e/pages"]}}`,
		"missing router": `{"schema":"application-map-manifest/0","app":"admin","routers":[{"path":"app/none.js","dialect":"ui-router-states/0"}],"tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]}}`,
	} {
		writeFile(t, root, "bad.json", manifest)
		bad := commitAll(t, root, name)
		if _, err := Build(ctx, root, "bad.json", bad); codeOf(err) != "appmap-invalid-manifest" {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := ProjectScreen(ctx, m, "app.home", o); err != nil {
		t.Fatal(err)
	}
}

// AMAP-V0-011: every budget either fits the projection or is refused as too small; no budget makes
// render overrun its own cap.
func TestAMAPV0011EveryBudgetFits(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := bigMap(t, root, rev, 40)
	ctx := context.Background()
	for budget := MinBudget; budget <= 8192; budget += 7 {
		o := Options{Budget: budget} // no Root: anchors read UNKNOWN without running git
		for name, call := range map[string]func() ([]byte, error){
			"screen":   func() ([]byte, error) { return ProjectScreen(ctx, m, "app.clubs.teesheets", o) },
			"home":     func() ([]byte, error) { return ProjectScreen(ctx, m, "app.home", o) },
			"flow":     func() ([]byte, error) { return ProjectFlow(ctx, m, "book-tee-time", o) },
			"find":     func() ([]byte, error) { return ProjectFind(ctx, m, "teesheet", o) },
			"scaffold": func() ([]byte, error) { return ProjectScaffold(ctx, m, "book-tee-time", o) },
		} {
			raw, err := call()
			if err != nil && codeOf(err) != "appmap-budget-too-small" {
				t.Fatalf("%s at %d: %v", name, budget, err)
			}
			if len(raw) > budget {
				t.Fatalf("%s at %d: %d bytes", name, budget, len(raw))
			}
		}
	}
}

func readText(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
