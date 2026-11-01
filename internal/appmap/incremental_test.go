package appmap

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// incrementalEdit changes the fixture worktree; n makes each application distinct.
type incrementalEdit struct {
	name  string
	apply func(t *testing.T, root string, n int)
}

func appendLine(t *testing.T, root, rel, line string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

// rewriteFlow replaces the first match of pattern in a flow intent of the fixture with repl.
func rewriteFlow(t *testing.T, root, rel, pattern, repl string) {
	t.Helper()
	p := filepath.Join(root, rel)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(pattern)
	loc := re.FindIndex(data)
	if loc == nil {
		t.Fatalf("%s: no %s", rel, pattern)
	}
	out := append(append(append([]byte{}, data[:loc[0]]...), repl...), data[loc[1]:]...)
	if err = os.WriteFile(p, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// incrementalEdits mixes changes no build input reads (an asset or lockfile content edit) with
// changes the build reads or that alter the tracked path set, so both outcomes are exercised.
var incrementalEdits = []incrementalEdit{
	{"asset content", func(t *testing.T, root string, n int) { appendLine(t, root, "assets/logo.png", fmt.Sprint("px", n)) }},
	{"lockfile content", func(t *testing.T, root string, n int) { appendLine(t, root, "yarn.lock", fmt.Sprint("dep@", n)) }},
	{"admitted non-UI source", func(t *testing.T, root string, n int) { appendLine(t, root, "server/handler.go", fmt.Sprint("// ", n)) }},
	{"markdown", func(t *testing.T, root string, n int) { appendLine(t, root, "docs/notes.md", fmt.Sprint("note ", n)) }},
	{"asset under the test root", func(t *testing.T, root string, n int) {
		appendLine(t, root, "e2e/fixtures/pic.png", fmt.Sprint("px", n))
	}},
	{"router", func(t *testing.T, root string, n int) {
		appendLine(t, root, "app/routes.js", fmt.Sprint("// edit ", n))
	}},
	{"router screen", func(t *testing.T, root string, n int) {
		// Alternate the home URL, so the home page object's navigation and binding come and go.
		url := "home"
		if n%2 == 1 {
			url = "home2"
		}
		rewriteFlow(t, root, "app/routes.js", `url: 'home2?'`, fmt.Sprintf("url: '%s'", url))
	}},
	{"router state", func(t *testing.T, root string, n int) {
		// Alternate the declared page object's state, so its binding comes and goes.
		state := "teesheets"
		if n%2 == 1 {
			state = "teesheetz"
		}
		rewriteFlow(t, root, "app/routes.js", `'app\.clubs\.teeshee(ts|tz)'`, fmt.Sprintf("'app.clubs.%s'", state))
	}},
	{"app module", func(t *testing.T, root string, n int) {
		appendLine(t, root, "app/lib/format.js", fmt.Sprint("// edit ", n))
	}},
	{"flow outcome", func(t *testing.T, root string, n int) {
		rewriteFlow(t, root, "flows/view-members.json", `"behavior": "[^"]*"`, fmt.Sprintf(`"behavior": "members are listed %d"`, n))
	}},
	{"flow state", func(t *testing.T, root string, n int) {
		// Alternate between the declared state and one no screen has, so a step unknown comes and goes.
		state := "/clubs/{clubId}/members"
		if n%2 == 0 {
			state = fmt.Sprintf("/nowhere/%d", n)
		}
		rewriteFlow(t, root, "flows/view-members.json", `"state": "[^"]*"`, fmt.Sprintf(`"state": %q`, state))
	}},
	{"spec", func(t *testing.T, root string, n int) {
		appendLine(t, root, "e2e/specs/login.spec.ts", fmt.Sprint("// edit ", n))
	}},
	{"added asset", func(t *testing.T, root string, n int) { appendLine(t, root, fmt.Sprintf("assets/new-%d.bin", n), "x") }},
	{"removed asset", func(t *testing.T, root string, n int) {
		matches, _ := filepath.Glob(filepath.Join(root, "assets", "new-*.bin"))
		if len(matches) == 0 {
			appendLine(t, root, "assets/logo.png", fmt.Sprint("px", n))
			return
		}
		if err := os.Remove(matches[0]); err != nil {
			t.Fatal(err)
		}
	}},
	{"asset mode", func(t *testing.T, root string, n int) {
		p := filepath.Join(root, "assets/logo.png")
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.Chmod(p, info.Mode()^0o111); err != nil {
			t.Fatal(err)
		}
	}},
}

// incrementalHistory commits a seeded random history over the fixture and returns its commits.
func incrementalHistory(t *testing.T, seed int64, length int) (string, []string) {
	t.Helper()
	root, _ := fixtureRepo(t)
	gitTest(t, root, "config", "core.fileMode", "true")
	for _, rel := range []string{"assets/logo.png", "yarn.lock", "e2e/fixtures/pic.png", "server/handler.go", "docs/notes.md"} {
		appendLine(t, root, rel, "seed")
	}
	// A first-party module outside the test root that a spec imports (AMAP-V0-245).
	appendLine(t, root, "app/lib/format.js", "export const format = (x) => String(x);")
	appendLine(t, root, "e2e/specs/format.spec.ts", "import { test } from '@playwright/test';\n"+
		"import { format } from '../../app/lib/format';\n"+
		"test('formats', async ({ page }) => {\n  await page.goto('/#!/home');\n  format(1);\n});")
	commits := []string{commitAll(t, root, "assets")}
	rng := rand.New(rand.NewSource(seed))
	for i := 0; i < length; i++ {
		names := []string{}
		for k := 0; k < 1+rng.Intn(2); k++ {
			e := incrementalEdits[rng.Intn(len(incrementalEdits))]
			e.apply(t, root, i*10+k)
			names = append(names, e.name)
		}
		gitTest(t, root, "add", "-A")
		gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "--allow-empty", "-m", strings.Join(names, ", "))
		commits = append(commits, gitTest(t, root, "rev-parse", "HEAD"))
	}
	return root, commits
}

// AMAP-V0-211: over seeded fixture histories, a map built from the map of any other commit has
// the digest a full build gives, whether it was carried forward or rebuilt; both happen.
func TestAMAPV0211IncrementalDigestEqualsFullBuild(t *testing.T) {
	ctx := context.Background()
	reused, rederived, rebuilt := 0, 0, 0
	// Seed 24 commits flow-only changes on adjacent commits (AMAP-V0-240).
	for _, seed := range []int64{1, 7, 1908, 24} {
		root, commits := incrementalHistory(t, seed, 7)
		full := make([]*Map, len(commits))
		for i, c := range commits {
			full[i] = build(t, root, c)
		}
		for i := range commits {
			for j := range commits {
				if i == j {
					continue
				}
				m, inc, err := BuildFrom(ctx, root, "appmap.json", commits[j], Roots{}, full[i])
				if err != nil {
					t.Fatalf("seed %d %d->%d: %v", seed, i, j, err)
				}
				if m.Digest != full[j].Digest || m.Revision != commits[j] {
					t.Fatalf("seed %d %d->%d (%+v): digest %s, full build %s", seed, i, j, inc, m.Digest, full[j].Digest)
				}
				switch {
				case inc.Reused:
					reused++
				case len(inc.Rederived) > 0:
					rederived++
				default:
					rebuilt++
				}
			}
		}
	}
	// AMAP-V0-240: a span whose only read change is to flow intents re-derives the flows alone.
	if reused == 0 || rederived == 0 || rebuilt == 0 {
		t.Fatalf("history exercised reused=%d rederived=%d rebuilt=%d", reused, rederived, rebuilt)
	}
}

// AMAP-V0-211: which change carries the map forward and which rebuilds it, named by reason.
func TestAMAPV0211ReuseDecision(t *testing.T) {
	ctx := context.Background()
	root, commits := incrementalHistory(t, 0, 0)
	base := commits[0]
	prev := build(t, root, base)
	step := func(name string) (*Map, Incremental) {
		t.Helper()
		for _, e := range incrementalEdits {
			if e.name == name {
				e.apply(t, root, 99)
			}
		}
		head := commitAll(t, root, name)
		m, inc, err := BuildFrom(ctx, root, "appmap.json", head, Roots{}, prev)
		if err != nil {
			t.Fatal(err)
		}
		if want := build(t, root, head); m.Digest != want.Digest {
			t.Fatalf("%s: digest differs from a full build", name)
		}
		gitTest(t, root, "reset", "-q", "--hard", base)
		return m, inc
	}
	// AMAP-V0-245: a source edit outside the test files re-derives the routers, test joins and
	// flows and keeps each test file's own entry.
	sources := "routers,test-joins,flows"
	for name, want := range map[string]string{
		"asset content": "", "lockfile content": "", "admitted non-UI source": sources, "markdown": sources,
		"asset under the test root": sources, "router": sources, "router screen": sources, "router state": sources,
		"app module": sources, "spec": RebuildChangedInput, "added asset": RebuildChangedInput, "asset mode": RebuildChangedInput,
	} {
		_, inc := step(name)
		reason, rederived := want, ""
		if want == sources {
			reason, rederived = "", sources
		}
		if inc.Reason != reason || strings.Join(inc.Rederived, ",") != rederived || inc.Reused != (want == "") || inc.Changed != 1 || inc.From != base {
			t.Errorf("%s: %+v, want %q", name, inc, want)
		}
	}
	gitTest(t, root, "commit", "-q", "--allow-empty", "-m", "empty")
	empty := gitTest(t, root, "rev-parse", "HEAD")
	if m, inc, err := BuildFrom(ctx, root, "appmap.json", empty, Roots{}, prev); err != nil || !inc.Reused || inc.Changed != 0 || m.Revision != empty {
		t.Fatalf("empty change: %+v %v", inc, err)
	}
	// A map that is not of this manifest, or of a revision the root lacks, rebuilds in full.
	other := *prev
	other.Manifest.Path = "other.json"
	if _, inc, _ := BuildFrom(ctx, root, "appmap.json", empty, Roots{}, &other); inc.Reason != RebuildManifestPath {
		t.Fatalf("manifest path: %+v", inc)
	}
	other = *prev
	other.Revision = strings.Repeat("ab", 20)
	if m, inc, err := BuildFrom(ctx, root, "appmap.json", empty, Roots{}, &other); err != nil || inc.Reason != RebuildFromUnreadable || m.Digest != build(t, root, empty).Digest {
		t.Fatalf("unknown revision: %+v %v", inc, err)
	}
	other = *prev
	other.Roots = []RootPin{{Repo: "tests", Revision: base}}
	if _, inc, _ := BuildFrom(ctx, root, "appmap.json", empty, Roots{}, &other); inc.Reason != RebuildAliasedRoots {
		t.Fatalf("aliased roots: %+v", inc)
	}
	if _, _, err := BuildFrom(ctx, root, "appmap.json", "-bad", Roots{}, prev); codeOf(err) != "appmap-invalid-revision" {
		t.Fatalf("bad revision: %v", err)
	}
}

// AMAP-V0-215: a map without this compiler's builder version is rebuilt in full with a stated
// reason; the builder member is outside the digest, so a map written before it parses unchanged.
func TestAMAPV0215BuilderVersionForcesRebuild(t *testing.T) {
	ctx := context.Background()
	root, commits := incrementalHistory(t, 0, 0)
	prev := build(t, root, commits[0])
	if prev.Builder != BuilderVersion {
		t.Fatalf("builder %q", prev.Builder)
	}
	appendLine(t, root, "assets/logo.png", "px")
	head := commitAll(t, root, "asset")
	want := build(t, root, head)
	for _, builder := range []string{"", "appmap-builder/0", "appmap-builder/1"} {
		old := *prev
		old.Builder = builder
		raw, err := Encode(&old)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseMap(raw)
		if err != nil || parsed.Digest != prev.Digest {
			t.Fatalf("builder %q: map does not parse with its digest: %v", builder, err)
		}
		m, inc, err := BuildFrom(ctx, root, "appmap.json", head, Roots{}, parsed)
		if err != nil || inc.Reused || inc.Reason != RebuildBuilderVersion || m.Digest != want.Digest || m.Builder != BuilderVersion {
			t.Fatalf("builder %q: %+v %v", builder, inc, err)
		}
	}
	if _, inc, err := BuildFrom(ctx, root, "appmap.json", head, Roots{}, prev); err != nil || !inc.Reused {
		t.Fatalf("same builder: %+v %v", inc, err)
	}
}

// AMAP-V0-216: a two-root map is carried when neither root changed a build input, with each
// aliased pin moved to the root's new commit; a read change, a dirty root, a missing root or a
// different stated revision rebuilds in full, which gives or refuses what a full build does.
func TestAMAPV0216AliasedRootsIncremental(t *testing.T) {
	ctx := context.Background()
	app, _, e2e, _ := twoRoots(t, false)
	appendLine(t, e2e, "assets/logo.png", "seed")
	appendLine(t, app, "assets/logo.png", "seed")
	appRev, e2eBase := commitAll(t, app, "asset"), commitAll(t, e2e, "asset")
	roots := Roots{Repos: map[string]string{"e2e": e2e}}
	prev, err := BuildRoots(ctx, app, "appmap.json", appRev, roots)
	if err != nil || len(prev.Roots) != 1 {
		t.Fatalf("two-root map: %v", err)
	}
	full := func(rev string) *Map {
		t.Helper()
		m, err := BuildRoots(ctx, app, "appmap.json", rev, roots)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	appendLine(t, e2e, "assets/logo.png", "px")
	e2eHead := commitAll(t, e2e, "e2e asset")
	appendLine(t, app, "assets/logo.png", "px")
	appHead := commitAll(t, app, "app asset")
	m, inc, err := BuildFrom(ctx, app, "appmap.json", appHead, roots, prev)
	if err != nil || !inc.Reused || inc.Changed != 2 || m.Roots[0].Revision != e2eHead || m.Digest != full(appHead).Digest {
		t.Fatalf("unread edits in both roots: %+v %v", inc, err)
	}
	if prev.Roots[0].Revision != e2eBase {
		t.Fatal("carrying changed the previous map's pins")
	}
	appendLine(t, e2e, "e2e/pages/home.page.ts", "// edit")
	commitAll(t, e2e, "page object")
	if m, inc, err = BuildFrom(ctx, app, "appmap.json", appHead, roots, prev); err != nil || inc.Reason != RebuildChangedInput || m.Digest != full(appHead).Digest {
		t.Fatalf("read edit in the aliased root: %+v %v", inc, err)
	}
	gitTest(t, e2e, "reset", "-q", "--hard", e2eHead)
	appendLine(t, e2e, "e2e/pages/home.page.ts", "// uncommitted")
	if _, inc, err = BuildFrom(ctx, app, "appmap.json", appHead, roots, prev); codeOf(err) != "appmap-root-dirty" || inc.Reason != RebuildRootDirty {
		t.Fatalf("dirty aliased root: %+v %v", inc, err)
	}
	gitTest(t, e2e, "reset", "-q", "--hard", e2eHead)
	if _, inc, err = BuildFrom(ctx, app, "appmap.json", appHead, Roots{}, prev); codeOf(err) != "appmap-root-unavailable" || inc.Reason != RebuildAliasedRoots {
		t.Fatalf("missing root: %+v %v", inc, err)
	}
	stated := Roots{Repos: roots.Repos, Revisions: map[string]string{"e2e": e2eBase}}
	if m, inc, err = BuildFrom(ctx, app, "appmap.json", appHead, stated, prev); err != nil || inc.Reason != RebuildAliasedRoots || !m.Roots[0].Stated {
		t.Fatalf("stated revision against a HEAD pin: %+v %v", inc, err)
	}
	if m2, inc, err := BuildFrom(ctx, app, "appmap.json", appHead, stated, m); err != nil || !inc.Reused || m2.Digest != m.Digest {
		t.Fatalf("stated pin carried: %+v %v", inc, err)
	}
}

// AMAP-V0-215: the builder identity binds the binary's VCS revision, so a map an older Corvint
// revision wrote is rebuilt even when BuilderVersion was not bumped, and a binary built from a
// modified tree never carries a map forward.
func TestAMAPV0215BuilderBindsBinaryRevision(t *testing.T) {
	info := func(kv ...string) *debug.BuildInfo {
		b := new(debug.BuildInfo)
		for i := 0; i+1 < len(kv); i += 2 {
			b.Settings = append(b.Settings, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
		}
		return b
	}
	for _, tc := range []struct {
		b    *debug.BuildInfo
		ok   bool
		want string
	}{
		{nil, false, BuilderVersion},
		{info(), true, BuilderVersion},
		{info("vcs.revision", "0123456789abcdef", "vcs.modified", "false"), true, BuilderVersion + "+0123456789ab"},
		{info("vcs.revision", "0123456789abcdef", "vcs.modified", "true"), true, BuilderVersion + "+0123456789ab-dirty"},
	} {
		if got := builderOf(tc.b, tc.ok); got != tc.want {
			t.Fatalf("builderOf: %q, want %q", got, tc.want)
		}
	}
	clean, dirty := BuilderVersion+"+0123456789ab", BuilderVersion+"+0123456789ab-dirty"
	if !builderReuses(clean, clean) || builderReuses(BuilderVersion+"+ba9876543210", clean) || builderReuses(BuilderVersion, clean) || builderReuses(dirty, dirty) {
		t.Fatal("builderReuses")
	}
	root, commits := incrementalHistory(t, 0, 0)
	old := *build(t, root, commits[0])
	old.Builder = builderIdentity + "+older"
	m, inc, err := BuildFrom(context.Background(), root, "appmap.json", commits[0], Roots{}, &old)
	if err != nil || inc.Reused || inc.Reason != RebuildBuilderVersion || m.Builder != builderIdentity {
		t.Fatalf("other revision's map: %+v %v", inc, err)
	}
}

// AMAP-V0-240: content edits confined to flow intents re-derive only the flows and give the full
// build's map; a flow edit with any other read change, an added intent, or an intent another map
// entry names rebuilds in full with changed-input.
func TestAMAPV0240FlowsOnlyRederive(t *testing.T) {
	ctx := context.Background()
	root, commits := incrementalHistory(t, 0, 0)
	base := commits[0]
	prev := build(t, root, base)
	step := func(name string, edit func()) Incremental {
		t.Helper()
		edit()
		head := commitAll(t, root, name)
		m, inc, err := BuildFrom(ctx, root, "appmap.json", head, Roots{}, prev)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := build(t, root, head); m.Digest != want.Digest || m.Revision != head {
			t.Fatalf("%s (%+v): digest %s, full build %s", name, inc, m.Digest, want.Digest)
		}
		gitTest(t, root, "reset", "-q", "--hard", base)
		return inc
	}
	apply := func(names ...string) func() {
		return func() {
			for _, e := range incrementalEdits {
				for _, n := range names {
					if e.name == n {
						e.apply(t, root, 4)
					}
				}
			}
		}
	}
	for _, tc := range []struct {
		name      string
		edit      func()
		rederived bool
	}{
		{"flow state", apply("flow state"), true},
		{"flow outcome and an unread asset", apply("flow outcome", "asset content"), true},
		{"flow and spec", apply("flow state", "spec"), false},
		{"added flow", func() {
			data, err := os.ReadFile(filepath.Join(root, "flows/view-members.json"))
			if err != nil {
				t.Fatal(err)
			}
			extra := regexp.MustCompile(`"flow_id": "[^"]*"`).ReplaceAll(data, []byte(`"flow_id": "extra"`))
			if err = os.WriteFile(filepath.Join(root, "flows/extra.json"), extra, 0o644); err != nil {
				t.Fatal(err)
			}
		}, false},
	} {
		inc := step(tc.name, tc.edit)
		if got := len(inc.Rederived) == 1 && inc.Rederived[0] == "flows" && inc.Reason == "" && !inc.Reused; got != tc.rederived {
			t.Errorf("%s: %+v, rederived want %v", tc.name, inc, tc.rederived)
		}
		if !tc.rederived && inc.Reason != RebuildChangedInput {
			t.Errorf("%s: reason %q", tc.name, inc.Reason)
		}
	}
	m, _, err := BuildFrom(ctx, root, "appmap.json", base, Roots{}, prev)
	if err != nil || m.Digest != prev.Digest {
		t.Fatalf("same revision: %v", err)
	}
	// The re-derived map carries the edited intent, and a later re-derivation reverts it.
	apply("flow outcome")()
	edited := commitAll(t, root, "outcome")
	mid, inc, err := BuildFrom(ctx, root, "appmap.json", edited, Roots{}, prev)
	if err != nil || len(inc.Rederived) != 1 || behaviorOf(mid) != "members are listed 4" || behaviorOf(prev) == behaviorOf(mid) {
		t.Fatalf("edited outcome: %+v %v %q", inc, err, behaviorOf(mid))
	}
	rewriteFlow(t, root, "flows/view-members.json", `"behavior": "[^"]*"`, `"behavior": "members are listed"`)
	reverted := commitAll(t, root, "reverted outcome")
	if back, inc, err := BuildFrom(ctx, root, "appmap.json", reverted, Roots{}, mid); err != nil || len(inc.Rederived) != 1 || behaviorOf(back) != behaviorOf(prev) || back.Digest != build(t, root, reverted).Digest {
		t.Fatalf("reverted outcome: %+v %v", inc, err)
	}
	// An intent another entry names (here a spec importing it) is a dependency the map keeps.
	gitTest(t, root, "reset", "-q", "--hard", base)
	appendLine(t, root, "e2e/specs/login.spec.ts", `import members from "../../flows/view-members.json";`)
	importing := commitAll(t, root, "spec imports a flow")
	named := build(t, root, importing)
	apply("flow outcome")()
	edited = commitAll(t, root, "flow edit")
	if m, inc, err := BuildFrom(ctx, root, "appmap.json", edited, Roots{}, named); err != nil || inc.Reason != RebuildChangedInput || m.Digest != build(t, root, edited).Digest {
		t.Fatalf("intent named by a spec: %+v %v", inc, err)
	}
}

// behaviorOf returns the first outcome behavior of the view-members flow.
func behaviorOf(m *Map) string {
	for _, f := range m.Flows {
		if f.FlowID == "view-members" && len(f.Outcomes) > 0 {
			return f.Outcomes[0].Behavior
		}
	}
	return ""
}

// AMAP-V0-240: a flows-directory file the flows stage never loads is not a flow edit. Here the
// tests root's tsconfig extends flows/origins.json, which LoadIntentsAt skips, so a change to its
// compilerOptions.paths changes the full build's import resolution and must rebuild in full.
func TestAMAPV0240OnlyLoadedIntentsAreFlowEdits(t *testing.T) {
	ctx := context.Background()
	root, _ := fixtureRepo(t)
	write := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	origins := func(target string) string {
		return fmt.Sprintf(`{"compilerOptions": {"baseUrl": "../e2e", "paths": {"@pages/*": [%q]}}}`+"\n", target)
	}
	write("e2e/tsconfig.json", `{"extends": "../flows/origins.json"}`+"\n")
	write("flows/origins.json", origins("pages/*"))
	base := commitAll(t, root, "aliased pages")
	prev := build(t, root, base)
	write("flows/origins.json", origins("nowhere/*"))
	head := commitAll(t, root, "alias moved")
	want := build(t, root, head)
	if want.Digest == prev.Digest {
		t.Fatal("fixture: the origins.json paths edit does not change the full build")
	}
	m, inc, err := BuildFrom(ctx, root, "appmap.json", head, Roots{}, prev)
	if err != nil {
		t.Fatal(err)
	}
	if m.Digest != want.Digest || inc.Reason != RebuildChangedInput || len(inc.Rederived) != 0 {
		t.Fatalf("origins.json edit: %+v, digest %s, full build %s", inc, m.Digest, want.Digest)
	}
}

// AMAP-V0-245: a source edit that may change the context index beyond its own content, or that a
// config may read, rebuilds in full and says why; either way the map is the full build's.
func TestAMAPV0245SourceEditFailsClosed(t *testing.T) {
	ctx := context.Background()
	root, commits := incrementalHistory(t, 0, 0)
	base := commits[0]
	prev := build(t, root, base)
	manifest, err := os.ReadFile(filepath.Join(root, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, rel, content, want string }{
		{"generated module", "app/lib/format.js", "// Code generated by gen. DO NOT EDIT.\nexport const format = (x) => String(x);\n", RebuildIndexAdmission},
		{"over the size bound", "docs/notes.md", strings.Repeat("x", contextindex.SourceSizeBound+1), RebuildIndexAdmission},
		{"JSON-shaped source", "server/handler.go", "{\"compilerOptions\": {}}\n", RebuildConfigShaped},
		{"test source", "e2e/specs/format.spec.ts", "import { test } from '@playwright/test';\n", RebuildChangedInput},
		{"package manifest", "package.json", "{\"name\": \"fixture-admin\"}\n", RebuildChangedInput},
		{"manifest", "appmap.json", strings.Replace(string(manifest), `"hash_prefix": "#!",`, `"hash_prefix": "#!", "base_path": "/x",`, 1), RebuildChangedInput},
	} {
		writeFile(t, root, c.rel, c.content)
		head := commitAll(t, root, c.name)
		m, inc, err := BuildFrom(ctx, root, "appmap.json", head, Roots{}, prev)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if inc.Reason != c.want || inc.Reused || len(inc.Rederived) != 0 {
			t.Errorf("%s: %+v, want reason %q", c.name, inc, c.want)
		}
		if m.Digest != build(t, root, head).Digest {
			t.Errorf("%s: digest differs from a full build", c.name)
		}
		gitTest(t, root, "reset", "-q", "--hard", base)
	}
}

// AMAP-V0-245: a router edit re-derives the routers and joins when the tests are read from an
// aliased root, and for a dialect whose routers do not read the context index.
func TestAMAPV0245SourceEditRederives(t *testing.T) {
	ctx := context.Background()
	app, appRev, e2e, _ := twoRoots(t, false)
	roots := Roots{Repos: map[string]string{"e2e": e2e}}
	prev, err := BuildRoots(ctx, app, "appmap.json", appRev, roots)
	if err != nil {
		t.Fatal(err)
	}
	rewriteFlow(t, app, "app/routes.js", `url: 'home'`, "url: 'home2'")
	head := commitAll(t, app, "router")
	m, inc, err := BuildFrom(ctx, app, "appmap.json", head, roots, prev)
	if err != nil || inc.Reason != "" || strings.Join(inc.Rederived, ",") != "routers,test-joins,flows" {
		t.Fatalf("aliased tests: %+v %v", inc, err)
	}
	if full, err := BuildRoots(ctx, app, "appmap.json", head, roots); err != nil || m.Digest != full.Digest || m.Digest == prev.Digest {
		t.Fatalf("aliased tests: digest %s, full build %s (%v), previous %s", m.Digest, full.Digest, err, prev.Digest)
	}
	// The kept page object's binding follows the re-derived screens.
	if screenOf(prev, "e2e/pages/home.page.ts") == "" || screenOf(m, "e2e/pages/home.page.ts") != "" {
		t.Fatalf("home page object binding: %q -> %q", screenOf(prev, "e2e/pages/home.page.ts"), screenOf(m, "e2e/pages/home.page.ts"))
	}

	root, rev := mountRepo(t, strings.Replace(mountManifest, "MOUNT", "/area/*", 1), mountApp)
	prev = build(t, root, rev)
	for i, route := range []string{"things/:id", "items/:id"} {
		rewriteFlow(t, root, "src/area.tsx", `(things|items)/:id`, route)
		head = commitAll(t, root, fmt.Sprint("route ", i))
		m, inc, err = BuildFrom(ctx, root, "appmap.json", head, Roots{}, prev)
		if err != nil || inc.Reason != "" || strings.Join(inc.Rederived, ",") != "routers,test-joins" {
			t.Fatalf("react routes %d: %+v %v", i, inc, err)
		}
		if m.Digest != build(t, root, head).Digest {
			t.Fatalf("react routes %d: digest differs from a full build", i)
		}
		prev = m
	}
}

func screenOf(m *Map, p string) string {
	for _, tf := range m.Files {
		if tf.Path == p {
			return tf.Screen
		}
	}
	return ""
}
