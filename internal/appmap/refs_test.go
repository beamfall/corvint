package appmap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// refsFixture commits testdata/refs (a react-router app with Playwright specs) and builds its map.
func refsFixture(t *testing.T) (string, *Map) {
	t.Helper()
	root, rev := fixtureRepoFrom(t, "testdata/refs")
	return root, build(t, root, rev)
}

type refsDoc struct {
	Schema           string         `json:"schema"`
	Check            string         `json:"check"`
	Limits           string         `json:"limits"`
	SourceFiles      int            `json:"source_files"`
	SourceSkipped    int            `json:"source_files_skipped"`
	TestFilesRead    int            `json:"test_files_read"`
	TestFilesUnread  int            `json:"test_files_unread"`
	References       int            `json:"references"`
	Checked          int            `json:"checked"`
	UncheckedTotal   int            `json:"unchecked_total"`
	MissesUnlabelled int            `json:"misses_unlabelled"`
	MissesLabelled   int            `json:"misses_labelled"`
	Misses           []refMiss      `json:"misses"`
	Unchecked        []refUnchecked `json:"unchecked"`
	Omitted          map[string]int `json:"omitted"`
}

func checkRefs(t *testing.T, root string, m *Map, o Options, sources ...string) (refsDoc, *RefsReport) {
	t.Helper()
	o.Root = root
	r, err := CheckRefs(context.Background(), m, sources, o)
	if err != nil {
		t.Fatal(err)
	}
	var doc refsDoc
	if err := json.Unmarshal(r.Document, &doc); err != nil {
		t.Fatalf("%v\n%s", err, r.Document)
	}
	return doc, r
}

func missKeys(d refsDoc) []string {
	out := []string{}
	for _, m := range d.Misses {
		out = append(out, strings.Join([]string{m.Kind, m.Value, m.Match, m.Class}, "|"))
	}
	return out
}

// AMAP-V0-090: refs reads the map revision only, writes nothing and refuses invalid sources and
// revisions with owned codes.
func TestAMAPV0090RefsReadOnlyContract(t *testing.T) {
	root, m := refsFixture(t)
	writeFile(t, root, "src/uncommitted.ts", "export const invoices = 'cancel-order secondary';\n")
	doc, _ := checkRefs(t, root, m, Options{Full: true}, "src")
	if doc.Schema != RefsSchema || doc.Check != "name-level" || doc.Limits != RefsLimits || !strings.HasPrefix(doc.Limits, "name-level only") {
		t.Fatalf("%+v", doc)
	}
	if doc.SourceFiles != 3 || doc.MissesUnlabelled != 3 { // the uncommitted file is never read
		t.Fatalf("%+v", doc)
	}
	if status := gitTest(t, root, "status", "--porcelain", "--ignored"); status != "?? src/uncommitted.ts" {
		t.Fatalf("mutated repo: %q", status)
	}
	for _, sources := range [][]string{nil, {"../src"}, {"/src"}, {"src", "src"}, {"nope"}, {"src/api.ts"},
		strings.Split(strings.Repeat("src,", maxRefSources+1), ",")[:maxRefSources+1]} {
		if _, err := CheckRefs(context.Background(), m, sources, Options{Root: root}); codeOf(err) != "appmap-invalid-query" {
			t.Errorf("%v: %v", sources, err)
		}
	}
	bad := *m
	bad.Revision = strings.Repeat("0", 40)
	if _, err := CheckRefs(context.Background(), &bad, []string{"src"}, Options{Root: root}); codeOf(err) != "appmap-invalid-revision" {
		t.Fatal(err)
	}
	if _, err := CheckRefs(context.Background(), m, []string{"src"}, Options{Root: root, Budget: 1}); codeOf(err) != "appmap-invalid-query" {
		t.Fatal(err)
	}
	if _, err := CheckRefs(context.Background(), m, []string{"src"}, Options{Root: root, Budget: MinBudget}); codeOf(err) != "appmap-budget-too-small" {
		t.Fatal(err)
	}
}

// AMAP-V0-091: route, selector and mocked-endpoint references are read from dotted calls.
func TestAMAPV0091ScanRefs(t *testing.T) {
	src := "await page.goto('/a');\nawait expect(page).toHaveURL('/b');\npage.waitForURL('/c')\n" +
		"page.route('/api/items', h)\npage.getByTestId('x')\npage.locator('#y')\npage.getByText('Hi')\n" +
		"goto('/not-dotted'); page.click('#z'); page.goto\n"
	got := []string{}
	for _, r := range scanRefs(src) {
		got = append(got, r.kind+" "+r.v.value+r.v.reason+" "+string(rune('0'+r.line)))
	}
	want := []string{"route /a 1", "route /b 2", "route /c 3", "endpoint api 4", "endpoint items 4",
		"selector x 5", "selector y 6", "selector text-level-selector 7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}

// AMAP-V0-092: whole literals are exact; templates and concatenations are prefixes; a regular
// expression's literal start is a labelled prefix; other values are unchecked with a reason.
func TestAMAPV0092ValueModes(t *testing.T) {
	for _, tc := range []struct{ src, value, mode, class, reason string }{
		{"page.goto('/orders')", "/orders", matchExact, "", ""},
		{"page.goto(`/orders/${id}/edit`)", "/orders/", matchPrefix, "", ""},
		{"page.goto(`${base}/orders/${id}`)", "/orders/", matchPrefix, "", ""},
		{"page.goto('/orders/' + id)", "/orders/", matchPrefix, "", ""},
		{"page.goto(`${url}`)", "", "", "", "dynamic-value"},
		{"page.goto(url)", "", "", "", "non-literal-value"},
		{"page.goto('/a' ? b : c)", "", "", "", "non-literal-value"},
		{`page.toHaveURL(/^\/orders\/\d+$/)`, "/orders/", matchPrefix, classRegex, ""},
		{`page.toHaveURL(/.*settings/)`, "", "", "", "regex-without-literal"},
		{"page.waitForURL('/files/**')", "/files/", matchPrefix, "", ""},
		{`page.toHaveURL(/^\/missing\*/)`, "/missing*", matchPrefix, classRegex, ""}, // an escaped `*` is not a glob
		{"page.waitForURL('**/files')", "", "", "", "glob-without-literal"},
		{"page.goto('/" + strings.Repeat("a", maxRefValue) + "')", "", "", "", "value-too-long"},
	} {
		refs := scanRefs(tc.src)
		if len(refs) != 1 {
			t.Fatalf("%s: %+v", tc.src, refs)
		}
		if v := refs[0].v; v.value != tc.value || v.mode != tc.mode || v.class != tc.class || v.reason != tc.reason {
			t.Errorf("%s: %+v", tc.src, v)
		}
	}
	for src, want := range map[string]string{
		`/\/a\/b/`: "/a/b", `/^\/ab?c/`: "/a", `/\/ab+c/`: "/ab", `/\/a\d/`: "/a", `/\/a{2}/`: "/", `/(x)/`: "", `/\/a.b/i`: "/a",
	} {
		if got := regexLiteral(src); got != want {
			t.Errorf("regexLiteral(%s) = %q, want %q", src, got, want)
		}
	}
}

// AMAP-V0-093: routes match resolved screen templates; prefixes match any template they can reach.
func TestAMAPV0093RouteCheck(t *testing.T) {
	_, m := refsFixture(t)
	screens := []Screen{}
	for _, s := range m.Screens {
		if !strings.HasSuffix(s.ID, ":fallback") {
			screens = append(screens, s)
		}
	}
	if len(screens) != len(m.Screens)-1 {
		t.Fatalf("fixture screens %+v", m.Screens)
	}
	x := newScreenIndex(m.HashPrefix, m.BasePath, screens)
	for _, tc := range []struct {
		value, mode string
		miss        bool
		reason      string
	}{
		{"/home", matchExact, false, ""},
		{"https://shop.example.test/home?tab=1#top", matchExact, false, ""},
		{"/orders/42", matchExact, false, ""},
		{"/files/a/b", matchExact, false, ""},
		{"/reports", matchExact, true, ""},
		{"/home/extra", matchExact, true, ""},
		{"/ord", matchPrefix, false, ""},
		{"/orders/", matchPrefix, false, ""},
		{"/files/x/y", matchPrefix, false, ""},
		{"/", matchPrefix, false, ""},
		{"https://shop.example.test", matchPrefix, false, ""},
		{"/zz", matchPrefix, true, ""},
		{"/home/x", matchPrefix, true, ""},
		{"orders", matchExact, false, "relative-url"},
	} {
		miss, class, reason := routeCheck(x, refValue{value: tc.value, mode: tc.mode})
		if miss != tc.miss || class != "" || reason != tc.reason {
			t.Errorf("%s %s: miss=%v class=%q reason=%q", tc.mode, tc.value, miss, class, reason)
		}
	}
}

// AMAP-V0-094: selector and endpoint names are checked by name occurrence in the source only.
func TestAMAPV0094NameLevelCheck(t *testing.T) {
	for sel, want := range map[string][]refValue{
		`#list .row-item`:                 {{value: "list", mode: matchExact}, {value: "row-item", mode: matchExact}},
		`button:has-text("Go .x")`:        {},
		`[data-region="orders"] > li`:     {{value: "data-region", mode: matchExact}, {value: "orders", mode: matchExact}},
		`[data-id^=row-]`:                 {{value: "data-id", mode: matchExact}, {value: "row-", mode: matchPrefix}},
		`[id='main']`:                     {{value: "main", mode: matchExact}},
		`[ id = 'main' ]`:                 {{value: "main", mode: matchExact}},
		`[data-state = "deleted"]`:        {{value: "data-state", mode: matchExact}, {value: "deleted", mode: matchExact}},
		`[data-id ^= row-]`:               {{value: "data-id", mode: matchExact}, {value: "row-", mode: matchPrefix}},
		`[data-status="404"]`:             {{value: "data-status", mode: matchExact}, {value: "404", mode: matchExact}},
		`#7a`:                             {},
		`div.1x`:                          {},
		`[aria-label="Close"]`:            {},
		`#a:nth-child(2) .b::after`:       {{value: "a", mode: matchExact}, {value: "b", mode: matchExact}},
		`[data-testid="save"], .cta-link`: {{value: "data-testid", mode: matchExact}, {value: "save", mode: matchExact}, {value: "cta-link", mode: matchExact}},
	} {
		if got := cssNames(sel, false); !reflect.DeepEqual(got, want) {
			t.Errorf("cssNames(%s) = %+v", sel, got)
		}
	}
	if got := cssNames(`[data-testid="row-`, true); !reflect.DeepEqual(got, []refValue{{value: "data-testid", mode: matchExact}, {value: "row-", mode: matchPrefix}}) {
		t.Errorf("prefix css: %+v", got)
	}
	if got := selectorNames("locator", refValue{value: `[data-testid=save]`, mode: matchExact}); !reflect.DeepEqual(got, []refValue{{value: "save", mode: matchExact}}) {
		t.Errorf("test-id attribute: %+v", got)
	}
	for in, want := range map[string]string{
		"**/api/v2/orders*":                    "api= v2= orders^",
		"https://x.test:8080/api/v1/items?q=1": "api= v1= items=",
		"x.test/api/42/users/{id}":             "api= users=",
		"/api/items/[0-9]":                     "api= items=",
		"**/*":                                 "no-checkable-name",
		"localhost:3000/graphql#frag":          "graphql=",
		"/static/app.js":                       "static= app.js=",
	} {
		parts := []string{}
		for _, v := range endpointSegments(refValue{value: in, mode: matchExact}) {
			if v.reason != "" {
				parts = append(parts, v.reason)
			} else if v.mode == matchPrefix {
				parts = append(parts, v.value+"^")
			} else {
				parts = append(parts, v.value+"=")
			}
		}
		if got := strings.Join(parts, " "); got != want {
			t.Errorf("endpointSegments(%s) = %q, want %q", in, got, want)
		}
	}
	idx := newNameIndex([][]byte{[]byte(`<div data-testid="save-order" className="btn">`), []byte("fetch('/static/app.js')")})
	for _, tc := range []struct {
		name, mode string
		want       bool
	}{
		{"save-order", matchExact, true}, {"save", matchExact, false}, {"save", matchPrefix, true},
		{"order", matchPrefix, false}, {"btn", matchExact, true}, {"app.js", matchExact, true}, {"app.ts", matchExact, false},
	} {
		if got := idx.has(tc.name, tc.mode); got != tc.want {
			t.Errorf("has(%s, %s) = %v", tc.name, tc.mode, got)
		}
	}
}

// AMAP-V0-095: catch-all routes, regular-expression patterns and application-templated names are
// labelled misses; unlabelled misses sort first.
func TestAMAPV0095LabelledClasses(t *testing.T) {
	root, m := refsFixture(t)
	doc, r := checkRefs(t, root, m, Options{Full: true}, "src")
	want := []string{
		"endpoint|invoices|exact|", "selector|cancel-order|exact|", "selector|secondary|exact|",
		"endpoint|refunds|prefix|regular-expression", "route|/reports|exact|catch-all-route", "selector|order-row-7|exact|app-templated-name",
	}
	if got := missKeys(doc); !reflect.DeepEqual(got, want) {
		t.Fatalf("misses %q", got)
	}
	if doc.MissesUnlabelled != 3 || doc.MissesLabelled != 3 || r.Unlabelled != 3 || doc.References != 30 || doc.Checked != 26 || doc.UncheckedTotal != 4 {
		t.Fatalf("%+v", doc)
	}
	if loc := doc.Misses[1].Tests; !reflect.DeepEqual(loc, []string{"e2e/pages/orders.page.ts:11"}) {
		t.Fatalf("location %q", loc)
	}
	reasons := []string{}
	for _, u := range doc.Unchecked {
		reasons = append(reasons, u.Kind+"|"+u.Reason)
	}
	if !reflect.DeepEqual(reasons, []string{"route|glob-without-literal", "selector|non-literal-value", "selector|text-level-selector", "selector|xpath-selector"}) {
		t.Fatalf("unchecked %q", reasons)
	}
}

// AMAP-V0-096: the report fits its budget with omitted counts and always keeps the name-level
// statement; occurrences beyond three keep only the count.
func TestAMAPV0096BoundedOutput(t *testing.T) {
	root, m := refsFixture(t)
	doc, r := checkRefs(t, root, m, Options{Budget: 1100}, "src")
	if len(r.Document) > 1100 || doc.Limits != RefsLimits || doc.Omitted["misses"] == 0 || len(doc.Misses)+doc.Omitted["misses"] != 6 {
		t.Fatalf("%d bytes: %s", len(r.Document), r.Document)
	}
	spec := strings.Repeat("await page.getByTestId('ghost').click();\n", 5)
	writeFile(t, root, "e2e/specs/ghost.spec.ts", "test('g', async ({ page }) => {\n"+spec+"});\n")
	rev := commitAll(t, root, "ghost")
	doc, _ = checkRefs(t, root, build(t, root, rev), Options{Full: true}, "src")
	for _, mm := range doc.Misses {
		if mm.Value == "ghost" && (mm.Occurrences != 5 || len(mm.Tests) != refLocationsMax) {
			t.Fatalf("%+v", mm)
		}
	}
}

// AMAP-V0-097: oversized and binary source files are skipped and counted; map test files that are
// aliased or no longer match the map's blob are counted unread.
func TestAMAPV0097CorpusBounds(t *testing.T) {
	root, _ := refsFixture(t)
	writeFile(t, root, "src/huge.ts", "// invoices\n"+strings.Repeat("x", maxTestFileBytes))
	if err := os.WriteFile(filepath.Join(root, "src", "logo.bin"), []byte("secondary\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := build(t, root, commitAll(t, root, "big"))
	doc, _ := checkRefs(t, root, m, Options{Full: true}, "src")
	if doc.SourceFiles != 3 || doc.SourceSkipped != 2 || doc.MissesUnlabelled != 3 || doc.TestFilesRead != 2 {
		t.Fatalf("%+v", doc)
	}
	alias := *m
	alias.Files = append([]TestFile{}, m.Files...)
	alias.Files[1].Anchor.Blob = strings.Repeat("1", 40)
	doc, _ = checkRefs(t, root, &alias, Options{Full: true}, "src")
	if doc.TestFilesRead != 1 || doc.TestFilesUnread != 1 {
		t.Fatalf("%+v", doc)
	}
	// AMAP-V0-350: an aliased map test file no --repo binds refuses rather than counting unread.
	alias.Files[0].Anchor.Repo = "other"
	if _, err := CheckRefs(context.Background(), &alias, []string{"src"}, Options{Root: root, Full: true}); codeOf(err) != "appmap-root-unavailable" ||
		!strings.Contains(err.Error(), "root other holds 1 map test files") || !strings.Contains(err.Error(), "--repo other=DIR") {
		t.Fatalf("unbound alias: %v", err)
	}
}

// AMAP-V0-098: the report's unlabelled-miss count decides the exit; labelled misses alone pass.
func TestAMAPV0098UnlabelledCount(t *testing.T) {
	root, _ := refsFixture(t)
	writeFile(t, root, "src/extra.ts", "export const names = ['invoices', 'cancel-order', 'secondary'];\n")
	m := build(t, root, commitAll(t, root, "extra"))
	doc, r := checkRefs(t, root, m, Options{}, "src")
	if r.Unlabelled != 0 || doc.MissesUnlabelled != 0 || doc.MissesLabelled != 3 {
		t.Fatalf("%+v", doc)
	}
}

// AMAP-V0-099: secret-shaped values are never printed or checked.
func TestAMAPV0099SecretShapedValues(t *testing.T) {
	value := "/login?password=" + strings.Repeat("Zx9", 8)
	if !secretscreen.MatchString(value) {
		t.Fatalf("fixture value is not secret-shaped")
	}
	refs := scanRefs("page.goto('" + value + "'); page.getByTestId('" + value + "')")
	if len(refs) != 2 || refs[0].v.reason != "secret-shaped" || refs[1].v.reason != "secret-shaped" || refs[0].v.value != "" {
		t.Fatalf("%+v", refs)
	}
}

// AMAP-V0-097: a map test file that is not read (its blob differs from the anchor) still stays out
// of the corpus, so its names cannot satisfy another test's reference.
func TestAMAPV0097UnreadTestNotCorpus(t *testing.T) {
	root, _ := refsFixture(t)
	writeFile(t, root, "e2e/specs/ghost.spec.ts", "test('g', async ({ page }) => { await page.getByTestId('ghost-only').click(); });\n")
	writeFile(t, root, "e2e/pages/ghost.page.ts", "export const ghost = 'ghost-only';\n")
	m := build(t, root, commitAll(t, root, "ghost"))
	mismatch := *m
	mismatch.Files = append([]TestFile{}, m.Files...)
	found := false
	for i, f := range mismatch.Files {
		if f.Path == "e2e/pages/ghost.page.ts" {
			mismatch.Files[i].Anchor.Blob, found = strings.Repeat("1", 40), true
		}
	}
	if !found {
		t.Fatalf("fixture files %+v", m.Files)
	}
	doc, _ := checkRefs(t, root, &mismatch, Options{Full: true}, "src", "e2e")
	if doc.TestFilesUnread != 1 || doc.SourceFiles != 3 || !reflect.DeepEqual(missKeys(doc)[:4], []string{
		"endpoint|invoices|exact|", "selector|cancel-order|exact|", "selector|ghost-only|exact|", "selector|secondary|exact|"}) {
		t.Fatalf("%+v", doc)
	}
}

// AMAP-V0-097: the 128 MiB corpus bound counts every source path, so identical blobs under many
// paths cannot pass it by sharing one object.
func TestAMAPV0097DuplicateBlobsBound(t *testing.T) {
	root, _ := refsFixture(t)
	writeFile(t, root, "src/big.ts", strings.Repeat("x", maxTestFileBytes))
	commitAll(t, root, "big")
	oid := gitTest(t, root, "rev-parse", "HEAD:src/big.ts")
	var index strings.Builder
	for i := 0; i < maxTestBytes/maxTestFileBytes; i++ {
		fmt.Fprintf(&index, "100644 %s\tsrc/dup/%03d.ts\n", oid, i)
	}
	c := exec.Command("git", "update-index", "--index-info")
	c.Dir, c.Stdin = root, strings.NewReader(index.String())
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "dup")
	m := build(t, root, gitTest(t, root, "rev-parse", "HEAD"))
	if _, err := CheckRefs(context.Background(), m, []string{"src"}, Options{Root: root, Full: true}); codeOf(err) != "appmap-bound-exceeded" {
		t.Fatalf("admitted %d paths of one blob: %v", maxTestBytes/maxTestFileBytes+1, err)
	}
}
