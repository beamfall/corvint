package doccorpus

import (
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

var extractFiles = map[string]string{
	"app/config/routes.rb":         "Rails.application.routes.draw do\n  get '/orders', to: 'list_orders'\n  post '/orders', to: 'create_order'\n  resources :invoices\n  get '/missing', to: 'nope'\nend\n",
	"app/handlers.rb":              "def list_orders\nend\ndef create_order\n  OrderMailerJob.perform_later(1)\n  publish('order.created')\nend\ndef create_order\nend\n",
	"app/services/billing.rb":      "class Billing\n  def charge\n    GhostJob.perform_later\n  end\n  subscribe('order.created') do |event|\n  end\n  publish('order.created'); publish('order.created')\nend\n",
	"app/jobs/order_mailer_job.rb": "class OrderMailerJob < ApplicationJob\nend\n",
	"app/config/events.yml":        "- order.created\n- order.shipped\n",
	"app/db/schema.rb":             "create_table \"orders\" do |t|\n  t.string \"status\"\n  t.integer \"total\"\nend\n  t.string \"stray\"\n",
	"app/web/router.ts":            "export const routes = [\n  { path: '/orders', component: OrdersPage },\n  { path: '/admin', component: AdminPage },\n  { path: dynamicPath },\n];\n",
	"app/web/pages/orders.ts":      "export function OrdersPage() { return 1; }\n",
	"app/tool/routes.txt":          "GET /api/health\nPOST /api/orders\n",
}

func extractProfile(toolDigest string) map[string]any {
	ex := func(fields ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i < len(fields); i += 2 {
			m[fields[i].(string)] = fields[i+1]
		}
		return m
	}
	return map[string]any{"schema": ExtractorProfileSchema, "provider": "app", "version": "1", "extractors": []any{
		ex("id", "routes", "subject", "endpoint", "files", []string{"app/config/routes.rb"}, "pattern", `^\s*(?P<method>get|post|put|patch|delete)\s+'(?P<path>[^']+)'(?:,\s*to:\s*'(?P<handler>\w+)')?`, "candidate", `^\s*(get|post|put|patch|delete|resources)\b`),
		ex("id", "handlers", "subject", "symbol", "files", []string{"app/*.rb"}, "pattern", `^def (?P<name>\w+)`),
		ex("id", "components", "subject", "symbol", "files", []string{"app/web/**/*.ts"}, "pattern", `^export function (?P<name>\w+)`),
		ex("id", "jobs", "subject", "job", "role", "declares", "files", []string{"app/jobs/*.rb"}, "pattern", `^class (?P<name>\w+Job)\b`),
		ex("id", "enqueue", "subject", "job", "role", "produces", "files", []string{"app/**/*.rb"}, "pattern", `\b(?P<name>\w+Job)\.perform_later`),
		ex("id", "events", "subject", "event", "role", "declares", "files", []string{"app/config/events.yml"}, "pattern", `^- (?P<name>[\w.]+)$`),
		ex("id", "publish", "subject", "event", "role", "produces", "files", []string{"app/**/*.rb"}, "pattern", `\bpublish\('(?P<name>[\w.]+)'\)`),
		ex("id", "subscribe", "subject", "event", "role", "consumes", "files", []string{"app/**/*.rb"}, "pattern", `\bsubscribe\('(?P<name>[\w.]+)'\)`),
		ex("id", "schema", "subject", "data_entity", "files", []string{"app/db/schema.rb"}, "pattern", `^create_table "(?P<entity>\w+)"`, "field_pattern", `^\s+t\.\w+ "(?P<field>\w+)"`, "end_pattern", `^end$`),
		ex("id", "router", "subject", "ui_surface", "files", []string{"app/web/router.ts"}, "pattern", `path: '(?P<path>[^']+)', component: (?P<handler>\w+)`, "candidate", `path:`),
		ex("id", "tool-routes", "subject", "endpoint", "files", []string{"app/tool/routes.txt"}, "pattern", `^(?P<method>[A-Z]+) (?P<path>/\S*)$`, "tool_output", map[string]string{"tool": "rails routes", "sha256": toolDigest}),
		ex("id", "missing", "subject", "symbol", "files", []string{"app/absent.rb"}, "pattern", `^def (?P<name>\w+)`),
		ex("id", "nomatch", "subject", "symbol", "files", []string{"app/none/**/*.go"}, "pattern", `^func (?P<name>\w+)`),
	}}
}

// extractFixture commits the source files and a profile; extras adds an unreadable binary and a
// symlink under a matched glob.
func extractFixture(t *testing.T, extras bool, edit func(map[string]any)) (string, string) {
	t.Helper()
	return extractFixtureFiles(t, extras, nil, edit)
}

// extractFixtureFiles is extractFixture with additional committed source files.
func extractFixtureFiles(t *testing.T, extras bool, more map[string]string, edit func(map[string]any)) (string, string) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	write := func(p string, data []byte) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for p, data := range extractFiles {
		write(p, []byte(data))
	}
	for p, data := range more {
		write(p, []byte(data))
	}
	if extras {
		write("app/web/logo.ts", []byte("export function Logo\x00() {}\n"))
		if err := os.Symlink("pages/orders.ts", filepath.Join(root, "app/web/link.ts")); err != nil {
			t.Fatal(err)
		}
	}
	profile := extractProfile(Digest([]byte(extractFiles["app/tool/routes.txt"])))
	if edit != nil {
		edit(profile)
	}
	data, err := json.Marshal(profile, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	write("corvint/extract.json", data)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "source")
	return root, git(t, root, "rev-parse", "HEAD")
}

func extractInventory(t *testing.T, root, rev string) (ExtractorInventory, []byte) {
	t.Helper()
	data, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: "corvint/extract.json"})
	if err != nil {
		t.Fatal(err)
	}
	var inv ExtractorInventory
	if err := json.Unmarshal(data, &inv, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	return inv, data
}

func extractSubject(inv ExtractorInventory, id string) (Subject, bool) {
	for _, s := range inv.Provider.Subjects {
		if s.ID == id {
			return s, true
		}
	}
	return Subject{}, false
}

func extractRelation(inv ExtractorInventory, from, kind, to string) bool {
	for _, r := range inv.Provider.Relations {
		if r.From == from && r.Type == kind && r.To == to {
			return true
		}
	}
	return false
}

func extractGap(inv ExtractorInventory, extractor, kind, path string, line int) bool {
	for _, g := range inv.Gaps {
		if g.Extractor == extractor && g.Kind == kind && g.Path == path && g.Line == line {
			return true
		}
	}
	return false
}

func extractEdit(index int, key string, value any) func(map[string]any) {
	return func(p map[string]any) {
		ex := p["extractors"].([]any)[index].(map[string]any)
		if value == nil {
			delete(ex, key)
			return
		}
		ex[key] = value
	}
}

func TestDCXV1390ExtractorProfileIsClosed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{"schema", func(p map[string]any) { p["schema"] = "corvint-extractor-profile/0" }, "schema must be"},
		{"unknown member", func(p map[string]any) { p["taxonomy"] = "rails" }, "invalid closed JSON input"},
		{"provider", func(p map[string]any) { p["provider"] = "App" }, "lowercase provider id"},
		{"subject", extractEdit(1, "subject", "controller"), "subject must be"},
		{"role required", extractEdit(3, "role", nil), "need a role"},
		{"role forbidden", extractEdit(1, "role", "declares"), "need a role"},
		{"required group", extractEdit(0, "pattern", `^\s*get '(?P<path>[^']+)'`), `required named group "method"`},
		{"unadmitted group", extractEdit(1, "pattern", `^def (?P<name>\w+)(?P<extra>.*)`), `unadmitted or repeated group "extra"`},
		{"invalid regexp", extractEdit(1, "pattern", `^def (?P<name>\w+`), "not a valid RE2 pattern"},
		{"file pattern", extractEdit(1, "files", []string{"../app/*.rb"}), "invalid file pattern"},
		{"partial double star", extractEdit(1, "files", []string{"app/**x/*.rb"}), "invalid file pattern"},
		{"tool output glob", extractEdit(10, "files", []string{"app/tool/*.txt"}), "tool_output needs one literal file"},
		{"field option", extractEdit(1, "field_pattern", `(?P<field>\w+)`), "data_entity options"},
		{"capture groups", extractEdit(1, "pattern", `^def (?P<name>\w+)`+strings.Repeat(`(x)?`, maxExtractorGroups)), "capture groups"},
		{"duplicate id", extractEdit(1, "id", "routes"), "unique lowercase identifiers"},
		{"too many", func(p map[string]any) {
			list := p["extractors"].([]any)
			for len(list) <= MaxExtractors {
				list = append(list, list[1])
			}
			p["extractors"] = list
		}, "extractors"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root, rev := extractFixture(t, false, c.edit)
			_, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: "corvint/extract.json"})
			tkRefused(t, err, c.want)
		})
	}
	t.Run("profile must be a pinned blob", func(t *testing.T) {
		t.Parallel()
		root, rev := extractFixture(t, false, nil)
		_, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: "corvint/absent.json"})
		tkRefused(t, err, "not a regular Git blob")
		_, err = BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: "HEAD", Profile: "corvint/extract.json"})
		tkRefused(t, err, "full commit revision")
	})
}

func TestDCXV1391ExtractorEmitsGenericSubjectsWithExactAnchors(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, false, nil)
	inv, _ := extractInventory(t, root, rev)
	if inv.Schema != ExtractorInventorySchema || inv.Status != "generated" || inv.Revision != rev || inv.Profile.Path != "corvint/extract.json" || inv.Profile.Blob != git(t, root, "rev-parse", rev+":corvint/extract.json") {
		t.Fatalf("inventory identity: %+v", inv)
	}
	want := map[string]string{
		"app:endpoint:GET /orders":                  "endpoint",
		"app:endpoint:POST /orders":                 "endpoint",
		"app:symbol:list_orders":                    "symbol",
		"app:symbol:OrdersPage":                     "symbol",
		"app:job:OrderMailerJob":                    "job",
		"app:event:order.created":                   "event",
		"app:event:order.shipped":                   "event",
		"app:data_entity:orders":                    "data_entity",
		"app:data_entity:orders.status":             "data_entity",
		"app:ui_surface:/orders":                    "ui_surface",
		"app:interaction:enqueue:app/handlers.rb:4": "interaction",
	}
	for id, kind := range want {
		s, ok := extractSubject(inv, id)
		if !ok || s.Kind != kind || s.Provider != "app" || s.Evidence.Trust != "generated" || len(s.Evidence.Anchors) == 0 {
			t.Fatalf("subject %s: %+v %v", id, s, ok)
		}
	}
	s, _ := extractSubject(inv, "app:endpoint:GET /orders")
	a := s.Evidence.Anchors[0]
	raw := []byte(extractFiles["app/config/routes.rb"])
	excerpt, _ := span(raw, 2, 2)
	if a.Path != "app/config/routes.rb" || a.Start != 2 || a.End != 2 || a.SHA256 != Digest(raw) || a.SpanSHA256 != Digest(excerpt) || a.Kind != "source" || a.Authority != "external-provider" || a.Blob != git(t, root, "rev-parse", rev+":app/config/routes.rb") || s.Evidence.Derivation != "source-derived" {
		t.Fatalf("endpoint anchor is not exact: %+v", s.Evidence)
	}
	if s, _ := extractSubject(inv, "app:symbol:create_order"); len(s.Evidence.Anchors) != 2 {
		t.Fatalf("one subject per kind and name keeps every site: %+v", s)
	}
	for _, kind := range []string{"capability", "use_case", "controller", "route"} {
		for _, s := range inv.Provider.Subjects {
			if s.Kind == kind {
				t.Fatalf("profile introduced a non-generic subject kind %s", kind)
			}
		}
	}
}

func TestDCXV1392ExtractorDerivesImplementsAndCallsRelations(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, false, nil)
	inv, _ := extractInventory(t, root, rev)
	for _, r := range [][3]string{
		{"app:symbol:list_orders", "implements", "app:endpoint:GET /orders"},
		{"app:symbol:OrdersPage", "implements", "app:ui_surface:/orders"},
		{"app:interaction:enqueue:app/handlers.rb:4", "calls", "app:job:OrderMailerJob"},
		{"app:interaction:publish:app/handlers.rb:5", "calls", "app:event:order.created"},
		{"app:interaction:subscribe:app/services/billing.rb:5", "depends_on", "app:event:order.created"},
		{"app:data_entity:orders.status", "related_to", "app:data_entity:orders"},
		{"app:data_entity:orders.total", "related_to", "app:data_entity:orders"},
	} {
		if !extractRelation(inv, r[0], r[1], r[2]) {
			t.Fatalf("missing relation %v in %+v", r, inv.Provider.Relations)
		}
	}
	for _, g := range []struct {
		extractor, kind, path string
		line                  int
	}{
		{"routes", "ambiguous-handler", "app/config/routes.rb", 3},
		{"routes", "unresolved-handler", "app/config/routes.rb", 5},
		{"router", "unresolved-handler", "app/web/router.ts", 3},
		{"enqueue", "undeclared-reference", "app/services/billing.rb", 3},
	} {
		if !extractGap(inv, g.extractor, g.kind, g.path, g.line) {
			t.Fatalf("missing gap %+v in %+v", g, inv.Gaps)
		}
	}
	for _, r := range inv.Provider.Relations {
		if r.From == "app:symbol:create_order" || strings.Contains(r.To, "/missing") && r.Type == "implements" {
			t.Fatalf("an ambiguous or unresolved handler was guessed: %+v", r)
		}
	}
	if _, ok := extractSubject(inv, "app:interaction:enqueue:app/services/billing.rb:3"); ok {
		t.Fatal("an undeclared job reference became a subject")
	}
}

func TestDCXV1393ExtractorProviderRecordBuildsUnchanged(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, false, nil)
	inv, _ := extractInventory(t, root, rev)
	m, err := Inventory(context.Background(), root, rev, "app", "2026-10-23T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	record, err := Encode(inv.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "evidence"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/extract.json"), record, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/extract.json")
	git(t, root, "commit", "-qm", "extractor record")
	recordRevision := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, Provider{ID: "app", Kind: "records", Version: "1", Revision: recordRevision, Record: "evidence/extract.json"})
	m.Scopes = append(m.Scopes, Scope{"evidence/extract.json", recordRevision})
	m.Inputs = append(m.Inputs, Input{Path: "evidence/extract.json", Revision: recordRevision, Blob: git(t, root, "rev-parse", recordRevision+":evidence/extract.json"), SHA256: Digest(record), Provider: "app", Purpose: "provider"})
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, s := range a.Subjects {
		if s.Provider == "app" {
			kinds[s.Kind] = true
			if s.Evidence.Trust != "generated" {
				t.Fatalf("extracted subject is not generated: %+v", s)
			}
		}
	}
	for _, kind := range []string{"endpoint", "ui_surface", "data_entity", "job", "event", "symbol", "interaction"} {
		if !kinds[kind] {
			t.Fatalf("built corpus lacks extracted %s subjects: %v", kind, kinds)
		}
	}
}

func TestDCXV1394ExtractorReadsOnlyPinnedToolOutput(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, false, nil)
	inv, _ := extractInventory(t, root, rev)
	s, ok := extractSubject(inv, "app:endpoint:GET /api/health")
	if !ok || s.Evidence.Derivation != "imported" || s.Evidence.Anchors[0].Kind != "imported" || s.Evidence.Anchors[0].Path != "app/tool/routes.txt" {
		t.Fatalf("pinned tool output not imported: %+v", s)
	}
	root, rev = extractFixture(t, false, func(p map[string]any) {
		p["extractors"].([]any)[10].(map[string]any)["tool_output"] = map[string]string{"tool": "rails routes", "sha256": strings.Repeat("0", 64)}
	})
	inv, _ = extractInventory(t, root, rev)
	if _, ok := extractSubject(inv, "app:endpoint:GET /api/health"); ok || !extractGap(inv, "tool-routes", "tool-output-digest-mismatch", "app/tool/routes.txt", 0) {
		t.Fatalf("unpinned tool output was read: %+v", inv.Gaps)
	}
}

func TestDCXV1395ExtractorReportsUnreadableFilesAndShapesAsGaps(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, true, nil)
	inv, _ := extractInventory(t, root, rev)
	for _, g := range []struct {
		extractor, kind, path string
		line                  int
	}{
		{"routes", "unsupported-shape", "app/config/routes.rb", 4},
		{"router", "unsupported-shape", "app/web/router.ts", 4},
		{"publish", "multiple-matches", "app/services/billing.rb", 7},
		{"schema", "field-outside-entity", "app/db/schema.rb", 5},
		{"missing", "missing-file", "app/absent.rb", 0},
		{"nomatch", "no-files-matched", "", 0},
		{"components", "not-utf8", "app/web/logo.ts", 0},
		{"components", "not-regular-blob", "app/web/link.ts", 0},
	} {
		if !extractGap(inv, g.extractor, g.kind, g.path, g.line) {
			t.Fatalf("missing gap %+v in %+v", g, inv.Gaps)
		}
	}
	for _, g := range inv.Gaps {
		if g.Line > 0 && (g.Evidence == nil || g.Evidence.Start != g.Line) {
			t.Fatalf("line gap lacks its anchor: %+v", g)
		}
	}
	if _, ok := extractSubject(inv, "app:data_entity:orders.stray"); ok {
		t.Fatal("a field outside an entity was guessed into one")
	}
}

func TestDCXV1396JobAndEventAreGenericSubjectKinds(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"job", "event"} {
		if !subjectKinds[kind] {
			t.Fatalf("%s is not a generic subject kind", kind)
		}
	}
}

func TestDCXV1398ExtractorIsDeterministicAndReadOnly(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, true, nil)
	_, first := extractInventory(t, root, rev)
	before := git(t, root, "status", "--porcelain", "--ignored")
	if err := os.WriteFile(filepath.Join(root, "app/config/routes.rb"), []byte("get '/changed', to: 'x'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, second := extractInventory(t, root, rev)
	if string(first) != string(second) {
		t.Fatal("extraction read the working tree or is not deterministic")
	}
	if err := os.WriteFile(filepath.Join(root, "app/config/routes.rb"), []byte(extractFiles["app/config/routes.rb"]), 0600); err != nil {
		t.Fatal(err)
	}
	if after := git(t, root, "status", "--porcelain", "--ignored"); after != before {
		t.Fatalf("extraction wrote files: %q -> %q", before, after)
	}
}

// extractWithin fails the test when f does not return within a generous wall-clock bound, so an
// exponential or quadratic regression fails instead of hanging the package.
func extractWithin(t *testing.T, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("extraction did not finish within its bound")
	}
}

func extractAnchored(anchors []Anchor, path string, line int) bool {
	return slices.ContainsFunc(anchors, func(a Anchor) bool { return a.Path == path && a.Start == line })
}

func TestDCXV1390ExtractorGlobMatchIsLinear(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		pattern, name string
		want          bool
	}{
		{"app/**/*.rb", "app/x.rb", true},
		{"app/**/*.rb", "app/a/b/x.rb", true},
		{"app/**/*.rb", "lib/x.rb", false},
		{"app/*.rb", "app/a/x.rb", false},
		{"**/x.rb", "x.rb", true},
		{"**", "a/b/c", true},
		{"app/**/jobs/**/*.rb", "app/a/jobs/b/c.rb", true},
		{"app/**/jobs/**/*.rb", "app/a/b/c.rb", false},
	} {
		if got := extractorMatch(c.pattern, c.name); got != c.want {
			t.Fatalf("extractorMatch(%q, %q) = %v", c.pattern, c.name, got)
		}
	}
	// 32 `**` segments against a deep path with a nonexistent filename backtracked exponentially.
	pattern := strings.Repeat("**/", 32) + "absent.rb"
	name := strings.Repeat("d/", 256) + "present.rb"
	extractWithin(t, func() {
		if extractorMatch(pattern, name) {
			t.Error("an adversarial pattern matched a different filename")
		}
	})
}

func TestDCXV1395ExtractorDenseFileIsRefusedWhileScanning(t *testing.T) {
	t.Parallel()
	// About 3.6 MiB of declarations: under the byte bound, far over the record bound. Anchoring
	// each site used to re-split and re-hash the whole file, and the bound was checked only after it.
	dense := strings.Repeat("def a\n", 600000)
	root, rev := extractFixtureFiles(t, false, map[string]string{"app/dense.rb": dense}, nil)
	extractWithin(t, func() {
		_, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: "corvint/extract.json"})
		tkRefused(t, err, "exceeds the record bound")
	})
}

func TestDCXV1392ExtractorRelationsCiteBothEnds(t *testing.T) {
	t.Parallel()
	root, rev := extractFixture(t, false, nil)
	inv, _ := extractInventory(t, root, rev)
	want := []struct {
		from, kind, to string
		ends           [2][2]any
	}{
		{"app:symbol:list_orders", "implements", "app:endpoint:GET /orders", [2][2]any{{"app/config/routes.rb", 2}, {"app/handlers.rb", 1}}},
		{"app:interaction:enqueue:app/handlers.rb:4", "calls", "app:job:OrderMailerJob", [2][2]any{{"app/handlers.rb", 4}, {"app/jobs/order_mailer_job.rb", 1}}},
		{"app:interaction:publish:app/handlers.rb:5", "calls", "app:event:order.created", [2][2]any{{"app/handlers.rb", 5}, {"app/config/events.yml", 1}}},
		{"app:data_entity:orders.status", "related_to", "app:data_entity:orders", [2][2]any{{"app/db/schema.rb", 2}, {"app/db/schema.rb", 1}}},
	}
	for _, w := range want {
		found := false
		for _, r := range inv.Provider.Relations {
			if r.From != w.from || r.Type != w.kind || r.To != w.to {
				continue
			}
			found = true
			for _, end := range w.ends {
				if !extractAnchored(r.Evidence.Anchors, end[0].(string), end[1].(int)) {
					t.Fatalf("relation %s %s %s lacks the anchor %v: %+v", w.from, w.kind, w.to, end, r.Evidence.Anchors)
				}
			}
		}
		if !found {
			t.Fatalf("missing relation %+v", w)
		}
	}
}

func TestDCXV1395AmbiguousEntityDeclarationClosesTheEntity(t *testing.T) {
	t.Parallel()
	more := "create_table \"users\"\n" +
		"  t.string \"name\"\n" +
		"create_table \"a\" create_table \"b\"\n" +
		"  t.string \"ghost\"\n" +
		"create_table \"people\"\n" +
		"  t.string \"age\"\n" +
		"create_table \"huge\" " + strings.Repeat("x", maxSpanBytes) + "\n" +
		"  t.string \"phantom\"\n" +
		"end\n"
	root, rev := extractFixtureFiles(t, false, map[string]string{"app/db/more.rb": more}, func(p map[string]any) {
		ex := p["extractors"].([]any)[8].(map[string]any)
		ex["files"] = []string{"app/db/*.rb"}
		ex["pattern"] = `create_table "(?P<entity>\w+)"`
	})
	inv, _ := extractInventory(t, root, rev)
	for _, id := range []string{"app:data_entity:users.name", "app:data_entity:people.age", "app:data_entity:orders.status"} {
		if _, ok := extractSubject(inv, id); !ok {
			t.Fatalf("missing field %s", id)
		}
	}
	for _, id := range []string{"app:data_entity:users.ghost", "app:data_entity:people.phantom"} {
		if _, ok := extractSubject(inv, id); ok {
			t.Fatalf("a field after an unread declaration was guessed into the previous entity: %s", id)
		}
	}
	for _, g := range []struct {
		kind string
		line int
	}{{"multiple-matches", 3}, {"field-outside-entity", 4}, {"line-over-bound", 0}, {"field-outside-entity", 8}} {
		if !extractGap(inv, "schema", g.kind, "app/db/more.rb", g.line) {
			t.Fatalf("missing gap %+v in %+v", g, inv.Gaps)
		}
	}
}
