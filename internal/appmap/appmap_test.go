package appmap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Review regressions (Codex round 1). Each case fails without its fix.

// AMAP-V0-002: a URL shorter than a multi-placeholder template is no match, never a panic.
func TestAMAPV0002WildcardExhaustedSubject(t *testing.T) {
	for _, c := range []struct {
		pattern, s string
		want       bool
	}{{"foo{}-{}", "foo", false}, {"foo{}-{}", "foo-", false}, {"foo{}-{}", "fooa-b", true}, {"{}", "", false}} {
		if got := wildcardMatch(c.pattern, c.s); got != c.want {
			t.Errorf("wildcardMatch(%q, %q) = %v", c.pattern, c.s, got)
		}
	}
}

// AMAP-V0-005 AMAP-V0-007: a literal that only starts an argument expression is not a literal, and
// a role name is read at any position of the options object.
func TestAMAPV0007PartialLiterals(t *testing.T) {
	f := readFacts(`page.getByTestId('save-' + id); page.goto('/home' + suffix); page.goto('/home', { waitUntil: 'load' });
page.getByRole('button', { exact: true, name: 'Save' }); page.getByRole('link', { name: 'A' + b });
page.getByRole('tab', { nested: { name: 'Inner' } });`)
	got := []string{}
	for _, s := range f.selectors {
		got = append(got, s.Kind+":"+s.Value+":"+s.Name+":"+s.Strength)
	}
	want := "test-id:::unknown role:button:Save:medium role:::unknown role:tab::medium"
	if strings.Join(got, " ") != want {
		t.Fatalf("selectors:\n got %s\nwant %s", strings.Join(got, " "), want)
	}
	if len(f.gotos) != 2 || f.gotos[0].url != "" || f.gotos[0].reason != "non-literal-url" || f.gotos[1].url != "/home" {
		t.Fatalf("gotos: %+v", f.gotos)
	}
}

// AMAP-V0-013: an unreadable selector has no value to compare, so it is never offered as reuse.
func TestAMAPV0013UnknownSelectorNotReused(t *testing.T) {
	unknown := newSelector("test-id", "", "", strengthUnknown, 4)
	b := &builder{order: []string{"e2e/pages/a.ts"}, files: map[string]*TestFile{"e2e/pages/a.ts": {
		Selectors: []Selector{unknown, newSelector("test-id", "", "", strengthUnknown, 9)},
		Methods:   []Method{{ID: "method:e2e/pages/a.ts#go", Selectors: []Selector{unknown}}},
	}}}
	if got := b.reuseIndex(); len(got) != 0 {
		t.Fatalf("unknown selector indexed as reuse: %v", got)
	}
}

// AMAP-V0-005: a spec the index could not read has no TestFile, so the join cannot be complete.
func TestAMAPV0005UnreadSpecKeepsJoinUnknown(t *testing.T) {
	root, _ := fixtureRepo(t)
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	if err := os.WriteFile(filepath.Join(root, "e2e/specs/binary.spec.ts"), []byte("import x from '\xff\xfe';\x00\x01"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := build(t, root, commitAll(t, root, "unreadable spec"))
	file := false
	for _, u := range m.Unknowns {
		file = file || (u.Kind == "file" && u.Path == "e2e/specs/binary.spec.ts")
	}
	if !file {
		t.Fatalf("unreadable spec not reported: %+v", m.Unknowns)
	}
	if m.testJoin() != StatusUnknown {
		t.Fatal("join RESOLVED with an unread spec")
	}
}

// AMAP-V0-001: a declared test or flow directory that is absent, or a file, at the revision is
// refused rather than silently contributing no files.
func TestAMAPV0001DeclaredDirectoriesExist(t *testing.T) {
	root, _ := fixtureRepo(t)
	ctx := context.Background()
	for name, tests := range map[string]string{
		"missing specs":    `{"root":"e2e","specs":["e2e/nope"],"page_objects":["e2e/pages"]}`,
		"file as scenario": `{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"],"scenarios":["e2e/specs/login.spec.ts"]}`,
		"missing root":     `{"root":"tests","specs":["tests/specs"],"page_objects":["tests/pages"]}`,
	} {
		writeFile(t, root, "bad.json", `{"schema":"application-map-manifest/0","app":"admin","routers":[{"path":"app/routes.js","dialect":"ui-router-states/0"}],"tests":`+tests+`}`)
		if _, err := Build(ctx, root, "bad.json", commitAll(t, root, name)); codeOf(err) != "appmap-invalid-manifest" {
			t.Errorf("%s: %v", name, err)
		}
	}
	writeFile(t, root, "bad.json", `{"schema":"application-map-manifest/0","app":"admin","routers":[{"path":"app/routes.js","dialect":"ui-router-states/0"}],"flows":"none","tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]}}`)
	if _, err := Build(ctx, root, "bad.json", commitAll(t, root, "missing flows")); codeOf(err) != "appmap-invalid-manifest" {
		t.Errorf("missing flows: %v", err)
	}
}

// AMAP-V0-010: at the map's own revision anchors are still checked against Git, so a map whose
// anchor claims content the revision does not hold reads STALE, not FRESH.
func TestAMAPV0010SameRevisionVerified(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	page := "e2e/pages/teesheet.page.ts"
	f := m.file(page)
	for i := range f.Methods {
		if f.Methods[i].Name == "selectSlot" {
			f.Methods[i].Anchor.Blob = strings.Repeat("0", 40)
			f.Methods[i].Anchor.SpanSHA256 = strings.Repeat("0", 64)
		}
	}
	doc := screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Revision: rev, Full: true})
	if got := freshnessOf(doc, "page_object_methods", "method:"+page+"#selectSlot"); got != Stale {
		t.Errorf("forged anchor at the map revision reads %q", got)
	}
	if got := freshnessOf(doc, "page_object_methods", "method:"+page+"#book"); got != Fresh {
		t.Errorf("honest anchor at the map revision reads %q", got)
	}
}

// AMAP-V0-013: a page object the closest spec imports under an alias is imported again under its
// own name, so the generated constructor is bound.
func TestAMAPV0013AliasedImportRebound(t *testing.T) {
	root, _ := fixtureRepo(t)
	spec := "e2e/specs/teesheet-click.spec.ts"
	data, _ := os.ReadFile(filepath.Join(root, spec))
	text := strings.Replace(string(data), "import { HomePage } from", "import { HomePage as Home } from", 1)
	writeFile(t, root, spec, strings.Replace(text, "new HomePage(page)", "new Home(page)", 1))
	m := build(t, root, commitAll(t, root, "aliased import"))
	raw, err := ProjectScaffold(context.Background(), m, "book-tee-time", Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	doc := decode(t, raw)
	if doc["closest"].(map[string]any)["file"] != spec {
		t.Fatalf("closest: %v", doc["closest"])
	}
	imports, body := fmtJSON(doc["imports"]), fmtJSON(doc["lines"])
	if !strings.Contains(body, "new HomePage(page)") || !strings.Contains(imports, `import { HomePage } from \"../pages/home.page\";`) {
		t.Fatalf("HomePage constructed but not bound:\n%s\n%s", imports, body)
	}
}

// AMAP-V0-012: find matches method, file and selector IDs, not only their labels.
func TestAMAPV0012FindByID(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	sel := newSelector("test-id", "slot", "", strengthStrong, 0).ID
	for _, c := range []struct{ query, want string }{
		{"page.ts#selectslot", `"method:e2e/pages/teesheet.page.ts#selectSlot"`},
		{sel[len("selector:"):], `"` + sel + `"`},
		{"file:e2e/specs/login", `"file:e2e/specs/login.spec.ts"`},
	} {
		raw, err := ProjectFind(context.Background(), m, c.query, Options{Full: true})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), c.want) {
			t.Errorf("find %q missed %s: %s", c.query, c.want, raw)
		}
	}
}

// AMAP-V0-014: a learned fact is printed only while its element is; trimming the element drops
// the fact and counts it as omitted.
func TestAMAPV0014FactsFollowTrimmedElements(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	probe := &fakeOverlay{}
	screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Full: true, Overlays: []Overlay{probe}})
	ov := &fakeOverlay{}
	for _, id := range probe.asked {
		ov.facts = append(ov.facts, Fact{ElementID: id, Source: "runs", Kind: "note", Text: strings.Repeat("n", 40)})
	}
	trimmed := false
	for budget := 1024; budget <= 6144; budget += 256 {
		raw, err := ProjectScreen(context.Background(), m, "app.clubs.teesheets", Options{Root: root, Budget: budget, Overlays: []Overlay{ov}})
		if err != nil {
			continue
		}
		doc := decode(t, raw)
		learned := doc["learned"].([]any)
		delete(doc, "learned")
		rest := fmtJSON(doc)
		for _, it := range learned {
			id := it.(map[string]any)["element_id"].(string)
			if !strings.Contains(rest, `"`+id+`"`) {
				t.Fatalf("budget %d: fact about %s printed without its element", budget, id)
			}
		}
		omitted := int(doc["omitted"].(map[string]any)["learned"].(float64))
		if len(learned)+omitted != len(ov.facts) {
			t.Fatalf("budget %d: %d learned + %d omitted != %d facts", budget, len(learned), omitted, len(ov.facts))
		}
		trimmed = trimmed || omitted > 0
	}
	if !trimmed {
		t.Fatal("no budget trimmed any fact; the case is not exercised")
	}
}

// AMAP-V0-007: escapes decode to the program's value, an escape whose value is not exact (legacy
// octal, a lone surrogate) is not a literal, and a role name a spread, computed, shorthand or
// repeated key could override is unknown rather than read as absent or first-seen.
func TestAMAPV0007EscapesAndSpreads(t *testing.T) {
	f := readFacts(`page.getByTestId('save\u002dbtn'); page.getByTestId("a\x41\u{42}\tz"); page.getByTestId(` + "`tab\\x41`" + `);
page.getByTestId('x\101'); page.getByTestId('\uD800'); page.getByTestId('\uD83D\uDE00'); page.getByTestId('q\'s');
page.getByRole('button', { name: 'Book', ...options }); page.getByRole('button', { ...o, name: 'Book' });
page.getByRole('button', { [k]: 'x', name: 'B' }); page.getByRole('button', { name }); page.getByRole('button', { name: 'B', name: 'C' });
page.getByRole('button', { 'name': 'Pay\u0021' }); page.goto('/h\u006fme');`)
	got := []string{}
	for _, s := range f.selectors {
		got = append(got, s.Kind+":"+s.Value+":"+s.Name+":"+s.Strength)
	}
	want := []string{"test-id:save-btn::strong", "test-id:aAB\tz::strong", "test-id:tabA::strong", "test-id:::unknown", "test-id:::unknown",
		"test-id:\U0001F600::strong", "test-id:q's::strong", "role:::unknown", "role:::unknown", "role:::unknown", "role:::unknown", "role:::unknown",
		"role:button:Pay!:medium"}
	for i := range want {
		w, _ := strconv.Unquote(`"` + want[i] + `"`)
		want[i] = w
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("selectors:\n got %q\nwant %q", got, want)
	}
	if len(f.gotos) != 1 || f.gotos[0].url != "/home" {
		t.Fatalf("gotos: %+v", f.gotos)
	}
}

// AMAP-V0-005: a spec reaching a screen only through a page object bound to no screen is
// attributed to none, so the join stays UNKNOWN.
func TestAMAPV0005UnboundPageObjectKeepsJoinUnknown(t *testing.T) {
	root, _ := fixtureRepo(t)
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	writeFile(t, root, "e2e/pages/orphan.page.ts", "export class OrphanPage {\n  async open() {\n    await this.page.getByTestId('orphan').click();\n  }\n}\n")
	writeFile(t, root, "e2e/specs/orphan.spec.ts", "import { test } from '@playwright/test';\nimport { OrphanPage } from '../pages/orphan.page';\n\ntest('orphan', async ({ page }) => {\n  await new OrphanPage(page).open();\n});\n")
	m := build(t, root, commitAll(t, root, "unbound page object"))
	if !hasUnknown(m, "page-object", fileID("e2e/pages/orphan.page.ts"), "page-object-unbound") {
		t.Fatalf("unbound page object not reported: %+v", m.Unknowns)
	}
	if m.testJoin() != StatusUnknown {
		t.Fatal("join RESOLVED with a spec reaching an unbound page object")
	}
}

// AMAP-V0-010: a screen's template, permissions and flags derive from its ancestors, so an
// ancestor's changed state makes the screen's lineage STALE in map_screen and map_flow even though
// its own anchor is unchanged.
func TestAMAPV0010AncestorLineageStale(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	routes := "app/routes.js"
	data, _ := os.ReadFile(filepath.Join(root, routes))
	writeFile(t, root, routes, strings.Replace(string(data), "url: 'clubs/:clubId'", "url: 'club/:clubId'", 1))
	commitAll(t, root, "rename the clubs segment")
	sv := screenDoc(t, m, "app.clubs.teesheets", Options{Root: root, Full: true})["screen"].(map[string]any)
	if sv["anchor"].(map[string]any)["freshness"] != Fresh || sv["lineage_freshness"] != Stale {
		t.Fatalf("screen: own %v lineage %v", sv["anchor"], sv["lineage_freshness"])
	}
	raw, err := ProjectFlow(context.Background(), m, "book-tee-time", Options{Root: root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range decode(t, raw)["screens"].([]any) {
		v := it.(map[string]any)
		if v["id"] == "screen:admin:app.clubs.teesheets" && v["lineage_freshness"] != Stale {
			t.Fatalf("flow screen lineage %v", v["lineage_freshness"])
		}
		if v["id"] == "screen:admin:app.home" && v["lineage_freshness"] != Fresh {
			t.Fatalf("unrelated lineage %v", v["lineage_freshness"])
		}
	}
}

// AMAP-V0-011: map_flow reports the unknowns of the screens it prints, such as unreadable flags.
func TestAMAPV0011FlowReportsScreenUnknowns(t *testing.T) {
	root, _ := fixtureRepo(t)
	routes := "app/routes.js"
	data, _ := os.ReadFile(filepath.Join(root, routes))
	writeFile(t, root, routes, strings.Replace(string(data), "flags: ['new_teesheet']", "flags: FLAGS", 1))
	m := build(t, root, commitAll(t, root, "non-literal flags"))
	raw, err := ProjectFlow(context.Background(), m, "book-tee-time", Options{Root: root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range decode(t, raw)["unknowns"].([]any) {
		u := it.(map[string]any)
		found = found || (u["kind"] == "screen-flags" && u["ref"] == "screen:admin:app.clubs.teesheets")
	}
	if !found {
		t.Fatalf("flow omits the screen's flags unknown: %s", raw)
	}
}

// AMAP-V0-013: a step with reuse but no selector, as only a hand-edited map can hold, scaffolds a
// TODO instead of crashing.
func TestAMAPV0013ReuseWithoutSelector(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	fl := m.flow("book-tee-time")
	reused := false
	for i := range fl.Steps {
		reused = reused || len(fl.Steps[i].Reuse) > 0
		fl.Steps[i].Selector = nil
	}
	if !reused {
		t.Fatal("fixture flow has no reuse to forge")
	}
	raw, err := ProjectScaffold(context.Background(), m, "book-tee-time", Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if body := string(raw); strings.Contains(body, "// reuse ") || !strings.Contains(body, "no locator declared") {
		t.Fatalf("scaffold: %s", body)
	}
}

// AMAP-V0-013: two reused page objects that export the same class name cannot both be imported;
// the second is reported as a binding collision and its steps fall back to TODO lines.
func TestAMAPV0013GeneratedBindingCollision(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "e2e/pages/a/slot.page.ts", "export class SlotPage {\n  async pick() {\n    await this.page.getByTestId('slot').click();\n  }\n}\n")
	writeFile(t, root, "e2e/pages/b/slot.page.ts", "export class SlotPage {\n  async reserve() {\n    await this.page.getByRole('button', { name: 'Book' }).click();\n  }\n}\n")
	m := build(t, root, commitAll(t, root, "same class twice"))
	raw, err := ProjectScaffold(context.Background(), m, "book-tee-time", Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	doc := decode(t, raw)
	imports, lines := []string{}, []string{}
	for _, it := range doc["imports"].([]any) {
		imports = append(imports, it.(string))
	}
	for _, it := range doc["lines"].([]any) {
		lines = append(lines, it.(string))
	}
	in, body := strings.Join(imports, "\n"), strings.Join(lines, "\n")
	if strings.Count(in, "\nimport { SlotPage }") != 1 || !strings.Contains(in, "// UNRESOLVED import { SlotPage } collides") ||
		strings.Count(body, "const slotPage") != 1 || !strings.Contains(body, "await slotPage.pick();") || strings.Contains(body, "reserve") {
		t.Fatalf("imports:\n%s\nlines:\n%s", in, body)
	}
	found := false
	for _, it := range doc["unknowns"].([]any) {
		u := it.(map[string]any)
		found = found || (u["reason"] == "binding-collision" && u["path"] == "e2e/pages/b/slot.page.ts")
	}
	if !found {
		t.Fatalf("no collision unknown: %v", doc["unknowns"])
	}
}

// AMAP-V0-007: role options passed by reference, a regex name and a string continued across lines
// are handled without guessing: the first two are unknown, and the continuation keeps later line
// numbers exact.
func TestAMAPV0007RegexReferencedOptionsAndContinuations(t *testing.T) {
	f := readFacts("page.getByRole('button', options);\npage.getByRole('button', { name: /Book/ });\npage.getByTestId('a\\\nb');\npage.getByTestId('c');")
	got := []string{}
	for _, s := range f.selectors {
		got = append(got, fmt.Sprintf("%s:%s:%s:%s@%d", s.Kind, s.Value, s.Name, s.Strength, s.Line))
	}
	want := "role:::unknown@1|role:::unknown@2|test-id:ab::strong@3|test-id:c::strong@5"
	if strings.Join(got, "|") != want {
		t.Fatalf("selectors:\n got %s\nwant %s", strings.Join(got, "|"), want)
	}
}

// AMAP-V0-002: a repeated object key takes the last value in JavaScript, so the map never reports
// the superseded one; a repeated router data key reads non-literal.
func TestAMAPV0002RepeatedKeys(t *testing.T) {
	toks, _ := lexJS(`{ a: '1', a: '2' }`)
	v, _ := parseValue(toks, 0)
	if a, ok := v.get("a"); !ok || a.str != "2" {
		t.Fatalf("get a = %+v", a)
	}
	root, _ := fixtureRepo(t)
	routes := "app/routes.js"
	data, _ := os.ReadFile(filepath.Join(root, routes))
	writeFile(t, root, routes, strings.Replace(string(data), "data: { permissions: ['teesheet.view'],", "data: { permissions: ['public'], permissions: ['teesheet.view'],", 1))
	m := build(t, root, commitAll(t, root, "repeated permissions"))
	s := screenByID(t, m, "app.clubs.teesheets")
	if !hasUnknown(m, "screen-permissions", s.ID, "non-literal-value") || strings.Contains(strings.Join(s.Permissions, ","), "public") {
		t.Fatalf("permissions %v unknowns %+v", s.Permissions, m.Unknowns)
	}
}

// AMAP-V0-005: a page object with any navigation target the map cannot place is not bound to its
// one placeable target, and keeps the join UNKNOWN.
func TestAMAPV0005UnresolvedTargetBlocksBinding(t *testing.T) {
	root, _ := fixtureRepo(t)
	page := "e2e/pages/home.page.ts"
	data, _ := os.ReadFile(filepath.Join(root, page))
	writeFile(t, root, page, strings.Replace(string(data), "  async goToTeeSheets() {", "  async away() {\n    await this.page.goto(target);\n  }\n\n  async goToTeeSheets() {", 1))
	m := build(t, root, commitAll(t, root, "dynamic goto"))
	if f := m.file(page); f.Screen != "" || !hasUnknown(m, "page-object", f.ID, "page-object-unresolved-target") {
		t.Fatalf("home page bound to %q despite an unplaceable target", f.Screen)
	}
	if m.testJoin() != StatusUnknown {
		t.Fatal("join RESOLVED")
	}
}

// AMAP-V0-005: a resolved first-party import outside tests.root is not read, so the importing
// file's join is UNKNOWN rather than a silently shortened chain.
func TestAMAPV0005ImportOutsideTests(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "shared/flows.ts", "export function go() {}\n")
	spec := "e2e/specs/shared.spec.ts"
	writeFile(t, root, spec, "import { test } from '@playwright/test';\nimport { go } from '../../shared/flows';\n\ntest('shared', async () => {\n  go();\n});\n")
	m := build(t, root, commitAll(t, root, "import outside tests"))
	if f := m.file(spec); f.Join != StatusUnknown || !hasUnknown(m, "import", f.ID, "import-outside-tests") {
		t.Fatalf("join %s unknowns %+v", f.Join, m.Unknowns)
	}
}

// AMAP-V0-010: an import attribution rests on every file of its chain, so a changed intermediate
// workflow makes the attribution's chain_freshness STALE while the spec's own anchor is FRESH.
func TestAMAPV0010ChainFreshness(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	wf := "e2e/workflows/booking.ts"
	data, _ := os.ReadFile(filepath.Join(root, wf))
	writeFile(t, root, wf, strings.Replace(string(data), "await sheet.book();", "await sheet.book();\n  await sheet.book();", 1))
	commitAll(t, root, "change the workflow")
	doc := screenDoc(t, m, "app.home", Options{Root: root, Full: true})
	found := false
	for _, it := range doc["specs"].([]any) {
		v := it.(map[string]any)
		if v["file"] == "e2e/specs/booking-workflow.spec.ts" {
			found = true
			if v["anchor"].(map[string]any)["freshness"] != Fresh || v["chain_freshness"] != Stale {
				t.Fatalf("spec %v chain %v", v["anchor"], v["chain_freshness"])
			}
		}
		if v["file"] == "e2e/specs/teesheet-click.spec.ts" && v["chain_freshness"] != Fresh {
			t.Fatalf("unchanged chain reads %v", v["chain_freshness"])
		}
	}
	if !found {
		t.Fatalf("workflow spec not attributed: %v", doc["specs"])
	}
}

// AMAP-V0-013: a reused method whose anchor is STALE at the evaluated revision is listed with its
// freshness and reported, but never called.
func TestAMAPV0013StaleReuseNotCalled(t *testing.T) {
	root, rev := fixtureRepo(t)
	m := build(t, root, rev)
	page := "e2e/pages/teesheet.page.ts"
	data, _ := os.ReadFile(filepath.Join(root, page))
	writeFile(t, root, page, strings.Replace(string(data), "{ name: 'Book' }).click();", "{ name: 'Book' }).dblclick();", 1))
	commitAll(t, root, "change book")
	raw, err := ProjectScaffold(context.Background(), m, "book-tee-time", Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, "await teeSheetPage.book();") || !strings.Contains(body, "await teeSheetPage.selectSlot();") ||
		!strings.Contains(body, "is STALE at the evaluated revision") || !strings.Contains(body, `"reason":"stale-reuse"`) {
		t.Fatalf("scaffold: %s", body)
	}
	for _, it := range decode(t, raw)["reuse"].([]any) {
		v := it.(map[string]any)
		if want := map[bool]string{true: Stale, false: Fresh}[v["method"] == "method:"+page+"#book"]; v["freshness"] != want {
			t.Fatalf("reuse %v", v)
		}
	}
}
