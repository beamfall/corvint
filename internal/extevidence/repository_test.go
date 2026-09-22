package extevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// e2eRepository is a second, independent repository: two commits on the main
// branch plus an unmerged orphan commit.
type e2eRepository struct {
	root, first, head, orphan, tree string
}

func gitIn(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func writeFile(t *testing.T, root, relative, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func initRepository(t *testing.T, root string) {
	t.Helper()
	gitIn(t, root, "init", "-q")
	gitIn(t, root, "config", "user.email", "corvint@example.test")
	gitIn(t, root, "config", "user.name", "Corvint Test")
}

func newE2E(t *testing.T) e2eRepository {
	t.Helper()
	root := t.TempDir()
	initRepository(t, root)
	writeFile(t, root, "tests/account.spec.ts", "test('account')\n")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-qm", "first")
	first := gitIn(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "tests/journey.spec.ts", "test('journey')\n")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-qm", "second")
	head := gitIn(t, root, "rev-parse", "HEAD")
	tree := gitIn(t, root, "rev-parse", "HEAD^{tree}")
	branch := gitIn(t, root, "rev-parse", "--abbrev-ref", "HEAD")
	gitIn(t, root, "checkout", "-q", "--orphan", "island")
	gitIn(t, root, "rm", "-rqf", ".")
	writeFile(t, root, "island.txt", "island\n")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-qm", "island")
	orphan := gitIn(t, root, "rev-parse", "HEAD")
	gitIn(t, root, "checkout", "-q", "-f", branch)
	return e2eRepository{root: root, first: first, head: head, orphan: orphan, tree: tree}
}

type pair struct {
	app repository
	e2e e2eRepository
}

func newPair(t *testing.T) pair {
	t.Helper()
	return pair{app: newRepository(t), e2e: newE2E(t)}
}

func (p pair) values() map[string]string {
	return map[string]string{
		"APP_ORIGIN": p.app.first, "APP_REVISION": p.app.head,
		"E2E_ORIGIN": p.e2e.first, "E2E_REVISION": p.e2e.head, "E2E_TREE": p.e2e.tree,
	}
}

// conformance reads one V1 fixture and fills its {{NAME}} placeholders.
func conformance(t *testing.T, name string, values map[string]string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "conformance-v1", name))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		data = bytes.ReplaceAll(data, []byte("{{"+key+"}}"), []byte(value))
	}
	return data
}

func with(values map[string]string, key, value string) map[string]string {
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = v
	}
	out[key] = value
	return out
}

func section1(t *testing.T, p pair, data []byte, checkouts []Checkout, changed ...string) map[string]any {
	t.Helper()
	source := writeRecord(t, t.TempDir(), "provider.json", data)
	out, err := contextindex.CanonicalJSON(Section(context.Background(), p.app.index(), []string{source}, checkouts, changed, 20))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func providerRow(section map[string]any) map[string]any {
	return section["providers"].([]any)[0].(map[string]any)
}

func repositoryRows(section map[string]any) map[string]map[string]any {
	rows := map[string]map[string]any{}
	for _, row := range providerRow(section)["repositories"].([]any) {
		rows[row.(map[string]any)["id"].(string)] = row.(map[string]any)
	}
	return rows
}

// itemsByPath indexes a list's items by their path, or by entity when pathless.
func itemsByPath(section map[string]any, list string) map[string]map[string]any {
	items := map[string]map[string]any{}
	for _, raw := range section[list].([]any) {
		entry := raw.(map[string]any)
		key, _ := entry["path"].(string)
		if key == "" {
			key = entry["entity"].(string)
		}
		items[key] = entry
	}
	return items
}

func unknownStates(section map[string]any) map[string]string {
	states := map[string]string{}
	for _, raw := range section["unknowns"].([]any) {
		entry := raw.(map[string]any)
		from := entry["relation"].(map[string]any)["from"].(map[string]any)
		key, _ := from["path"].(string)
		states[from["repository"].(string)+":"+key] = entry["state"].(string)
		if entry["reason"] == "" {
			panic("unknown without reason")
		}
	}
	return states
}

func TestProviderRecordV1SchemaStrict(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	valid := conformance(t, "two-repository.json", p.values())
	if _, err := Decode1(valid); err != nil {
		t.Fatalf("fixture must decode: %v", err)
	}
	repositories := func(r map[string]any) []any { return r["repositories"].([]any) }
	cases := map[string]func(record map[string]any){
		"no repositories":         func(r map[string]any) { r["repositories"] = []any{} },
		"duplicate repository id": func(r map[string]any) { r["repositories"] = append(repositories(r), repositories(r)[0]) },
		"short origin":            func(r map[string]any) { repositories(r)[0].(map[string]any)["origin"] = "abc" },
		"unknown role":            func(r map[string]any) { repositories(r)[0].(map[string]any)["role"] = "authority" },
		"remote with scheme":      func(r map[string]any) { repositories(r)[0].(map[string]any)["remote"] = "https://example.test/a" },
		"remote with .git":        func(r map[string]any) { repositories(r)[0].(map[string]any)["remote"] = "example.test/a.git" },
		"remote with query":       func(r map[string]any) { repositories(r)[0].(map[string]any)["remote"] = "example.test/a?token=x" },
		"mixed endpoint shape": func(r map[string]any) {
			relations(r)[0].(map[string]any)["from"].(map[string]any)["entity"] = "cap-account"
		},
		"string endpoint":       func(r map[string]any) { relations(r)[0].(map[string]any)["from"] = "path:pkg/main.go" },
		"unknown endpoint key":  func(r map[string]any) { relations(r)[0].(map[string]any)["from"].(map[string]any)["branch"] = "main" },
		"record-level blob key": func(r map[string]any) { relations(r)[0].(map[string]any)["blob"] = p.app.mainBlob },
		"too many repositories": func(r map[string]any) {
			for i := len(repositories(r)); i <= MaxRepositories; i++ {
				r["repositories"] = append(repositories(r), map[string]any{"id": "extra" + string(rune('a'+i)), "revision": "r"})
			}
		},
	}
	for name, edit := range cases {
		if _, err := Decode1(mutate(t, valid, edit)); err == nil {
			t.Errorf("%s: record must be invalid", name)
		}
	}
}

func TestTwoRepositoryProviderComposes(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	section := section1(t, p, conformance(t, "two-repository.json", p.values()), []Checkout{{ID: "e2e", Source: p.e2e.root}}, "pkg/main.go")
	row := providerRow(section)
	if row["schema"] != Schema1 || row["root_repository"] != "application" || row["state"] != StateLoaded {
		t.Fatalf("provider row = %v", row)
	}
	repositories := repositoryRows(section)
	want := map[string][3]string{
		"application": {IdentityResolved, BindingRoot, FreshnessEqual},
		"e2e":         {IdentityResolved, BindingCheckout, FreshnessEqual},
		"handbook":    {IdentityUnresolved, BindingUnresolved, FreshnessIdentityUnresolved},
	}
	for id, states := range want {
		got := repositories[id]
		if got["identity"] != states[0] || got["binding"] != states[1] || got["freshness"] != states[2] {
			t.Errorf("%s = %v, want %v", id, got, states)
		}
	}
	if _, has := repositories["e2e"]["remote"]; has {
		t.Error("a local-only repository must bind without any remote")
	}
	results := itemsByPath(section, "results")
	result := results["pkg/main.go"]
	if result["repository"] != "application" || result["relation_state"] != RelationFresh || result["crosses_repositories"] != false {
		t.Fatalf("result = %v", result)
	}
	if !strings.Contains(result["reason"].(string), "changed path application:pkg/main.go") {
		t.Fatalf("reason must qualify the path with its repository: %q", result["reason"])
	}
	verification := itemsByPath(section, "verification")
	cross := verification["tests/account.spec.ts"]
	if cross["repository"] != "e2e" || cross["verification"] != VerificationVerified || cross["relation_state"] != RelationFresh || cross["crosses_repositories"] != true {
		t.Fatalf("cross-repository verification = %v", cross)
	}
	endpoint := cross["endpoints"].([]any)[0].(map[string]any)
	for key, value := range map[string]any{"repository": "e2e", "origin": p.e2e.first, "revision": p.e2e.head, "tree": p.e2e.tree, "captured_revision": p.e2e.head, "checkout": p.e2e.root, "binding": BindingCheckout, "freshness": FreshnessEqual} {
		if endpoint[key] != value {
			t.Errorf("endpoint[%s] = %v, want %v", key, endpoint[key], value)
		}
	}
	if journey := verification["tests/journey.spec.ts"]; journey["relation"].(map[string]any)["type"] != "asserts" || journey["entity"] != "mockdocs:surface-account" {
		t.Fatalf("a downstream entity's cross-repository assertion must be listed: %v", journey)
	}
	if removed := verification["tests/removed.spec.ts"]; removed["verification"] != VerificationMissing || removed["relation_state"] != RelationStale {
		t.Fatalf("a missing reference on one side must mark the relation stale: %v", removed)
	}
	if handbook := verification["account.md"]; handbook["relation_state"] != RelationUnresolved || handbook["verification"] != VerificationNotVerified {
		t.Fatalf("an unresolved identity must stay visible as unresolved: %v", handbook)
	}
	unknowns := unknownStates(section)
	if unknowns["contracts:account.proto"] != unknownUnresolved {
		t.Errorf("an undeclared repository endpoint must be unresolved: %v", unknowns)
	}
	if unknowns["application:pkg/main.go"] != unknownUnsupported {
		t.Errorf("a path-to-path relation must be an explicit unsupported unknown: %v", unknowns)
	}
	if rows := section["checkouts"].([]any); len(rows) != 1 || rows[0].(map[string]any)["bound"] != float64(1) {
		t.Fatalf("checkout rows = %v", section["checkouts"])
	}
}

func TestPerRepositoryFreshness(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	e2e := []Checkout{{ID: "e2e", Source: p.e2e.root}}
	cases := map[string]struct {
		values map[string]string
		state  string
	}{
		"equal":           {p.values(), FreshnessEqual},
		"ancestor":        {with(with(p.values(), "E2E_REVISION", p.e2e.first), "E2E_TREE", gitIn(t, p.e2e.root, "rev-parse", p.e2e.first+"^{tree}")), FreshnessRepositoryAhead},
		"unrelated":       {with(with(p.values(), "E2E_REVISION", p.e2e.orphan), "E2E_TREE", gitIn(t, p.e2e.root, "rev-parse", p.e2e.orphan+"^{tree}")), FreshnessUnrelatedHistory},
		"unavailable":     {with(p.values(), "E2E_REVISION", strings.Repeat("f", 40)), FreshnessRevisionUnavailable},
		"tree mismatch":   {with(p.values(), "E2E_TREE", strings.Repeat("e", 40)), FreshnessTreeMismatch},
		"foreign history": {with(p.values(), "E2E_REVISION", p.app.head), FreshnessRevisionUnavailable},
	}
	for name, test := range cases {
		section := section1(t, p, conformance(t, "two-repository.json", test.values), e2e, "pkg/main.go")
		repositories := repositoryRows(section)
		if got := repositories["e2e"]["freshness"]; got != test.state {
			t.Errorf("%s: e2e freshness = %v, want %s", name, got, test.state)
		}
		// Freshness is per repository: the application side never moves.
		if got := repositories["application"]["freshness"]; got != FreshnessEqual {
			t.Errorf("%s: application freshness = %v, want equal", name, got)
		}
		cross := itemsByPath(section, "verification")["tests/account.spec.ts"]
		wantRelation := map[string]string{FreshnessEqual: RelationFresh, FreshnessRevisionUnavailable: RelationNotVerified}[test.state]
		if wantRelation == "" {
			wantRelation = RelationStale
		}
		if cross["relation_state"] != wantRelation {
			t.Errorf("%s: cross-repository relation state = %v, want %s", name, cross["relation_state"], wantRelation)
		}
		if result := itemsByPath(section, "results")["pkg/main.go"]; result["relation_state"] != RelationFresh {
			t.Errorf("%s: same-repository result must stay fresh, got %v", name, result["relation_state"])
		}
	}
	// The primary repository is evaluated on its own terms too.
	behind := section1(t, p, conformance(t, "two-repository.json", with(p.values(), "APP_REVISION", p.app.first)), e2e, "pkg/main.go")
	if got := repositoryRows(behind)["application"]["freshness"]; got != FreshnessRepositoryAhead {
		t.Errorf("application behind: freshness = %v", got)
	}
	ahead := section1(t, p, conformance(t, "two-repository.json", p.values()), e2e, "pkg/main.go")
	if got := repositoryRows(ahead)["application"]["captured_revision"]; got != p.app.head {
		t.Errorf("captured revision = %v", got)
	}
}

func TestCheckoutBinding(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	record := conformance(t, "two-repository.json", p.values())
	cases := map[string]struct {
		checkouts []Checkout
		binding   string
		relation  string
	}{
		"no checkout":         {nil, BindingUnbound, RelationNotVerified},
		"missing directory":   {[]Checkout{{ID: "e2e", Source: filepath.Join(p.e2e.root, "absent")}}, BindingUnavailable, RelationNotVerified},
		"subdirectory":        {[]Checkout{{ID: "e2e", Source: filepath.Join(p.e2e.root, "tests")}}, BindingUnavailable, RelationNotVerified},
		"different identity":  {[]Checkout{{ID: "e2e", Source: p.app.root}}, BindingMismatch, RelationUnresolved},
		"bound by identity":   {[]Checkout{{ID: "e2e", Source: p.e2e.root}}, BindingCheckout, RelationFresh},
		"unused checkout id":  {[]Checkout{{ID: "unused", Source: p.e2e.root}}, BindingUnbound, RelationNotVerified},
		"root-relative match": {[]Checkout{{ID: "e2e", Source: relativeTo(t, p.app.root, p.e2e.root)}}, BindingCheckout, RelationFresh},
	}
	for name, test := range cases {
		section := section1(t, p, record, test.checkouts, "pkg/main.go")
		if got := repositoryRows(section)["e2e"]["binding"]; got != test.binding {
			t.Errorf("%s: binding = %v, want %s", name, got, test.binding)
		}
		cross := itemsByPath(section, "verification")["tests/account.spec.ts"]
		if cross["relation_state"] != test.relation {
			t.Errorf("%s: relation state = %v, want %s", name, cross["relation_state"], test.relation)
		}
		rows, _ := section["checkouts"].([]any)
		for _, row := range rows {
			if reason := row.(map[string]any)["reason"].(string); strings.Contains(reason, p.e2e.root) || strings.Contains(reason, p.app.root) {
				t.Errorf("%s: a checkout reason must not carry a resolved path: %q", name, reason)
			}
		}
	}
}

func relativeTo(t *testing.T, base, target string) string {
	t.Helper()
	relative, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatal(err)
	}
	return relative
}

func TestRepositoryIdentityConformance(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	ambiguous := section1(t, p, conformance(t, "ambiguous.json", p.values()), nil, "pkg/main.go")
	for id, row := range repositoryRows(ambiguous) {
		if row["identity"] != IdentityAmbiguous || row["freshness"] != FreshnessIdentityAmbiguous {
			t.Errorf("shared origin: %s = %v", id, row)
		}
	}
	if providerRow(ambiguous)["root_repository"] != "" || len(ambiguous["results"].([]any)) != 0 {
		t.Fatalf("an ambiguous identity must bind nothing and join nothing: %v", ambiguous)
	}
	missing := section1(t, p, conformance(t, "missing-identity.json", p.values()), nil, "pkg/main.go")
	if row := repositoryRows(missing)["application"]; row["identity"] != IdentityUnresolved || row["freshness"] != FreshnessIdentityUnresolved {
		t.Fatalf("missing origin: %v", row)
	}
	if providerRow(missing)["state"] != StateLoaded || !strings.Contains(providerRow(missing)["reason"].(string), "per declared repository") {
		t.Fatalf("an unresolved identity is a loaded record with an explicit state, not an error: %v", providerRow(missing))
	}
	abstention := section1(t, p, conformance(t, "abstention.json", p.values()), nil, "pkg/main.go")
	if len(abstention["results"].([]any)) != 0 || len(abstention["unknowns"].([]any)) != 3 {
		t.Fatalf("abstention must list every refused relation and no result: %v", abstention)
	}
	for _, raw := range abstention["unknowns"].([]any) {
		if reason, _ := raw.(map[string]any)["reason"].(string); reason == "" {
			t.Fatalf("an unknown must carry a reason: %v", raw)
		}
	}
	invalid := section1(t, p, conformance(t, "invalid-remote.json", p.values()), nil, "pkg/main.go")
	row := providerRow(invalid)
	if row["state"] != StateInvalid || strings.Contains(row["reason"].(string), "token") {
		t.Fatalf("a credential-bearing remote must be refused without repeating it: %v", row)
	}
}

func TestRootBindingAmbiguousWithTwoRootCommits(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initRepository(t, root)
	writeFile(t, root, "a.txt", "a\n")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-qm", "a")
	first := gitIn(t, root, "rev-parse", "HEAD")
	branch := gitIn(t, root, "rev-parse", "--abbrev-ref", "HEAD")
	gitIn(t, root, "checkout", "-q", "--orphan", "other")
	gitIn(t, root, "rm", "-rqf", ".")
	writeFile(t, root, "b.txt", "b\n")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-qm", "b")
	second := gitIn(t, root, "rev-parse", "HEAD")
	gitIn(t, root, "checkout", "-q", "-f", branch)
	gitIn(t, root, "merge", "-q", "--allow-unrelated-histories", "-m", "join", "other")
	head := gitIn(t, root, "rev-parse", "HEAD")
	index := &contextindex.Index{Root: root, CommitRevision: head, Tracked: map[string]struct{}{"a.txt": {}, "b.txt": {}}}
	record := Record1{Repositories: []Repository1{{ID: "left", Origin: first, Revision: head}, {ID: "right", Origin: second, Revision: head}}}
	states, primary := repositoryStates(context.Background(), record, resolveBindings(context.Background(), indexRoot(index), nil))
	if primary != "" || states["left"].binding != BindingAmbiguous || states["right"].binding != BindingAmbiguous {
		t.Fatalf("two declared roots of one checkout must not choose one: primary=%q left=%s right=%s", primary, states["left"].binding, states["right"].binding)
	}
}

func TestProviderV1DeterministicPrivateAndBounded(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	relative := relativeTo(t, p.app.root, p.e2e.root)
	source := writeRecord(t, t.TempDir(), "provider.json", conformance(t, "two-repository.json", p.values()))
	run := func(limit int) []byte {
		out, err := contextindex.CanonicalJSON(Section(context.Background(), p.app.index(), []string{source}, []Checkout{{ID: "e2e", Source: relative}}, []string{"pkg/main.go"}, limit))
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	first, second := run(20), run(20)
	if !bytes.Equal(first, second) {
		t.Fatalf("section must be deterministic:\n%s\n%s", first, second)
	}
	canonical, err := filepath.EvalSymlinks(p.e2e.root)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, []byte(canonical)) {
		t.Fatal("the section must echo the checkout as given, never the resolved directory")
	}
	for _, forbidden := range []string{"test('account')", "island"} {
		if bytes.Contains(first, []byte(forbidden)) {
			t.Fatalf("the section must carry no source bodies: found %q", forbidden)
		}
	}
	var bounded map[string]any
	if err := json.Unmarshal(run(1), &bounded); err != nil {
		t.Fatal(err)
	}
	omitted := bounded["omitted"].(map[string]any)
	if len(bounded["verification"].([]any)) != 1 || omitted["verification"].(float64) < 3 {
		t.Fatalf("V1 items must obey the EEP-V0-012 bounds: %v", omitted)
	}
}

func TestV0SectionCarriesNoV1Members(t *testing.T) {
	t.Parallel()
	repo := newRepository(t)
	source := writeRecord(t, t.TempDir(), "mock.json", fixture(t, repo.head))
	section := Section(context.Background(), repo.index(), []string{source}, nil, []string{"pkg/main.go"}, 10)
	if _, present := section["checkouts"]; present {
		t.Fatal("a V0 run without checkouts must not gain a checkouts member")
	}
	row := section["providers"].([]any)[0].(map[string]any)
	for _, key := range []string{"schema", "repositories", "root_repository"} {
		if _, present := row[key]; present {
			t.Errorf("V0 provider row gained %q", key)
		}
	}
	for _, list := range []string{"results", "downstream", "verification"} {
		for _, raw := range section[list].([]any) {
			for _, key := range []string{"endpoints", "relation_state", "crosses_repositories", "repository"} {
				if _, present := raw.(map[string]any)[key]; present {
					t.Errorf("V0 %s item gained %q", list, key)
				}
			}
		}
	}
}

func TestParseCheckout(t *testing.T) {
	t.Parallel()
	if got, err := ParseCheckout("e2e=../e2e"); err != nil || got != (Checkout{ID: "e2e", Source: "../e2e"}) {
		t.Fatalf("ParseCheckout = %+v, %v", got, err)
	}
	for _, bad := range []string{"e2e", "=dir", "e2e=", "bad id=dir", "a:b=dir"} {
		if _, err := ParseCheckout(bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}
