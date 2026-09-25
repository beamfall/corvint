package console

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const chainSpec = "# Fixture spec\n\n## Requirements\n\n" +
	"- `FIX-V0-001`: The widget MUST frob.\n" +
	"- `FIX-V0-002`: The widget MUST log.\n" +
	"- `FIX-V0-003`: The widget MUST NOT be linked by text.\n\n## Non-goals\n"

const (
	chainHunkID     = "hunk:sha256:1111111111111111111111111111111111111111111111111111111111111111"
	chainEvidenceID = "evidence:sha256:2222222222222222222222222222222222222222222222222222222222222222"
	chainClaimID    = "claim:sha256:3333333333333333333333333333333333333333333333333333333333333333"
)

// chainFixture is a repository holding one sealed change built the way
// script/dogfood-seal.sh builds one: a bind commit holding
// .corvint/change.cem.json, then a seal commit moving it to
// .corvint/changes/<bind>.cem.json.
type chainFixture struct {
	root, base, change, specOid, testOid string
	cem                                  []byte
}

func chainGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	argv := append([]string{"-C", root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test"}, args...)
	cmd := exec.Command("git", argv...)
	cmd.Env = gitEnvironment()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s %v", args, out, err)
	}
	return strings.TrimSpace(string(out))
}

func chainWrite(t *testing.T, root, name string, content []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func spanRef(id, path, oid, text, within string) map[string]any {
	start := strings.Index(within, text)
	return map[string]any{"id": id, "path": path, "blobOid": oid,
		"span": map[string]any{"start": start, "end": start + len(text)}, "spanSha256": sha256Hex(text)}
}

// newChainFixture commits the fixture. addedLine is the line the change adds
// to widget.go; edit may rewrite the change map before it is committed.
func newChainFixture(t *testing.T, addedLine string, edit func(cem map[string]any)) *chainFixture {
	t.Helper()
	root := t.TempDir()
	chainGit(t, root, "init", "-q")
	chainWrite(t, root, "spec.md", []byte(chainSpec))
	chainWrite(t, root, "widget.go", []byte("package widget\n"))
	chainWrite(t, root, "widget_test.go", []byte("package widget\n\nfunc TestFrob() {}\n"))
	chainGit(t, root, "add", ".")
	chainGit(t, root, "commit", "-q", "--no-gpg-sign", "-m", "base")
	fixture := &chainFixture{root: root, base: chainGit(t, root, "rev-parse", "HEAD"),
		specOid: chainGit(t, root, "rev-parse", "HEAD:spec.md"), testOid: chainGit(t, root, "rev-parse", "HEAD:widget_test.go")}
	cem := map[string]any{
		"spec": "cem/0.2", "baseRevision": fixture.base, "excludedPath": boundMapPath,
		"evidence": []any{spanRef(chainEvidenceID, "spec.md", fixture.specOid, "- `FIX-V0-001`: The widget MUST frob.\n", chainSpec)},
		"hunks": []any{map[string]any{"id": chainHunkID, "path": "widget.go", "disposition": "supported", "reason": "",
			"oldRange": map[string]any{"start": 1, "count": 0}, "newRange": map[string]any{"start": 2, "count": 1},
			"basis": []any{map[string]any{"evidenceId": chainEvidenceID, "relation": "specification"}}}},
	}
	if edit != nil {
		edit(cem)
	}
	encoded, err := json.Marshal(cem)
	if err != nil {
		t.Fatal(err)
	}
	fixture.cem = encoded
	chainWrite(t, root, "widget.go", []byte("package widget\n"+addedLine+"\n"))
	chainWrite(t, root, boundMapPath, encoded)
	chainGit(t, root, "add", ".")
	chainGit(t, root, "commit", "-q", "--no-gpg-sign", "-m", "bind")
	fixture.change = chainGit(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(changesDir)), 0o755); err != nil {
		t.Fatal(err)
	}
	chainGit(t, root, "mv", boundMapPath, changesDir+"/"+fixture.change+".cem.json")
	chainGit(t, root, "commit", "-q", "--no-gpg-sign", "-m", "seal")
	return fixture
}

// ocm is an OCM map bound to the sealed change: FIX-V0-001 linked to the hunk
// and the test claim.
func (f *chainFixture) ocm() map[string]any {
	scope := spanRef("", "spec.md", f.specOid, chainSpec[strings.Index(chainSpec, "## Requirements"):strings.Index(chainSpec, "## Non-goals")], chainSpec)
	delete(scope, "id")
	testText := "package widget\n\nfunc TestFrob() {}\n"
	claim := spanRef(chainClaimID, "widget_test.go", f.testOid, "func TestFrob() {}\n", testText)
	claim["extractor"], claim["selector"] = "go-test", "TestFrob"
	return map[string]any{
		"spec": ocmProfile, "targetRevision": f.change, "intentScope": scope,
		"cem":    map[string]any{"mapSha256": sha256Hex(string(f.cem)), "patchSha256": strings.Repeat("0", 64)},
		"claims": []any{claim},
		"obligations": []any{map[string]any{"id": "FIX-V0-001", "disposition": "linked", "reason": "",
			"hunkIds": []any{chainHunkID}, "claimIds": []any{chainClaimID}}},
	}
}

func (f *chainFixture) writeOCM(t *testing.T, name string, doc map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	chainWrite(t, f.root, ocmDir+"/"+name, encoded)
}

func (f *chainFixture) traceRow(revision, task string, commands ...string) string {
	encoded, _ := json.Marshal(map[string]any{"schema_version": 1, "revision": revision, "trace_id": "trace-" + revision[:8],
		"task": task, "outcome": "passed", "changed_paths": []any{"widget.go"}, "opened_paths": []any{}, "verification": commands})
	return string(encoded) + "\n"
}

func (f *chainFixture) writeTrace(t *testing.T, revision string, rows ...string) {
	t.Helper()
	chainWrite(t, f.root, traceDir+"/"+revision+".jsonl", []byte(strings.Join(rows, "")))
}

// mainOf is the page's main element, so a failure prints the pane and not the
// stylesheet.
func mainOf(page string) string {
	_, rest, _ := strings.Cut(page, "<main>")
	body, _, _ := strings.Cut(rest, "</main>")
	return body
}

func (f *chainFixture) page(t *testing.T, query string) string {
	t.Helper()
	server, err := New(Options{Addr: "127.0.0.1:0", Repo: f.root, Binary: "atm"})
	if err != nil {
		t.Fatal(err)
	}
	return get(t, server, "/chain?change="+f.change+query)
}

// TestConsoleChainComplete covers LAC-V0-033 and LAC-V0-034: a sealed change
// whose map, bound OCM map and trace establish every edge renders the whole
// chain, and every edge names the artifact and field that justify it.
func TestConsoleChainComplete(t *testing.T) {
	fixture := newChainFixture(t, "func Frob() {}", nil)
	fixture.writeOCM(t, "change.ocm.001.json", fixture.ocm())
	fixture.writeTrace(t, fixture.change, fixture.traceRow(fixture.change, "frob the widget", "go test ./widget"))

	body := fixture.page(t, "&hunk="+chainHunkID)
	if strings.Contains(body, "<b>gap:") || strings.Contains(body, ">gap: ") {
		t.Fatalf("a complete chain rendered a gap:\n%s", mainOf(body))
	}
	sealed := changesDir + "/" + fixture.change + ".cem.json"
	ocmPath := ocmDir + "/change.ocm.001.json"
	for _, want := range []string{
		// change binding: the sealed name is confirmed by the bind commit.
		"<code>" + sealed + "</code> · <code>file name",
		// hunk -> evidence
		"<code>hunks[0].basis[0].evidenceId = evidence[0].id</code>",
		"pinned <code>spec.md @ object " + fixture.specOid,
		// hunk -> requirement
		`<a href="#req-FIX-V0-001"><code>FIX-V0-001</code></a>`,
		"<code>" + ocmPath + "</code> · <code>obligations[0].hunkIds[0]</code>",
		// requirement -> clause, test claim
		`id="req-FIX-V0-001"`,
		"The widget MUST frob.",
		"<code>obligations[0].claimIds[0] = claims[0].id</code>",
		"structural test claim",
		// change -> recorded verification
		"<code>" + traceDir + "/" + fixture.change + ".jsonl</code> · <code>line 1 revision</code>",
		"recorded outcome &#34;passed&#34;",
		// hunk detail: the added line at the change commit and the cited span.
		"<pre>func Frob() {}</pre>",
		"<pre>- `FIX-V0-001`: The widget MUST frob.\n</pre>",
		// axes: the Git-read edge states its axes; the local files state none.
		"<b>authorityClass</b> OWNING_VERIFIER",
		"NOT_STATED",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("complete chain missing %q:\n%s", want, mainOf(body))
		}
	}
}

// TestConsoleChainGaps covers LAC-V0-035: each gap class renders as a gap row
// with its reason, never as a link.
func TestConsoleChainGaps(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(cem map[string]any)
		setup func(t *testing.T, f *chainFixture)
		want  []string
	}{
		{name: "missing", edit: func(cem map[string]any) {
			hunk := cem["hunks"].([]any)[0].(map[string]any)
			hunk["basis"] = []any{map[string]any{"evidenceId": "evidence:sha256:" + strings.Repeat("9", 64), "relation": "specification"}}
		}, want: []string{"<b>gap: missing</b>", "no evidence[] entry has this id",
			"no OCM map bound to this sealed map by digest and revision was found"}},
		{name: "stale", edit: func(cem map[string]any) {
			cem["evidence"].([]any)[0].(map[string]any)["spanSha256"] = strings.Repeat("a", 64)
		}, setup: func(t *testing.T, f *chainFixture) {
			doc := f.ocm()
			doc["cem"] = map[string]any{"mapSha256": strings.Repeat("b", 64)}
			f.writeOCM(t, "change.ocm.json", doc)
			f.writeTrace(t, f.change, f.traceRow(f.base, "recorded against the base"))
		}, want: []string{"<b>gap: stale</b>", "spanSha256 " + strings.Repeat("a", 64) + " does not match",
			"targetRevision names this change but cem.mapSha256", "the row names revision"}},
		{name: "ambiguous", edit: func(cem map[string]any) {
			evidence := cem["evidence"].([]any)
			cem["evidence"] = append(evidence, evidence[0])
		}, setup: func(t *testing.T, f *chainFixture) {
			f.writeTrace(t, f.change, f.traceRow(f.change, "first"), f.traceRow(f.change, "second"))
		}, want: []string{"<b>gap: ambiguous</b>", "2 evidence[] entries share this id; none is chosen",
			"2 rows record a result for this revision; every one is shown and none is chosen"}},
		{name: "unverified", want: []string{"<b>gap: unverified</b>",
			"no recorded verification result names revision"}},
		{name: "unsupported", edit: func(cem map[string]any) {
			hunk := cem["hunks"].([]any)[0].(map[string]any)
			hunk["disposition"], hunk["reason"], hunk["basis"] = "unknown", "no-evidence", []any{}
		}, setup: func(t *testing.T, f *chainFixture) {
			doc := f.ocm()
			doc["obligations"] = []any{map[string]any{"id": "FIX-V0-001", "disposition": "unknown", "reason": "unassessed",
				"hunkIds": []any{}, "claimIds": []any{}}}
			f.writeOCM(t, "change.ocm.001.json", doc)
			f.writeTrace(t, f.change, "{not json}\n")
		}, want: []string{"<b>gap: unsupported</b>", "states disposition &#34;unknown&#34;, reason &#34;no-evidence&#34;",
			"the obligation lists no hunk", "the obligation lists no test claim",
			"the row is not a schema_version 1 trace record"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newChainFixture(t, "func Frob() {}", tc.edit)
			if tc.setup != nil {
				tc.setup(t, fixture)
			}
			body := fixture.page(t, "")
			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Fatalf("%s gap missing %q:\n%s", tc.name, want, mainOf(body))
				}
			}
		})
	}
}

// TestConsoleChainPanelsAgreeOnObligationHunkEdges is V1-0152: the
// requirement panel and the hunk panel give one obligation's hunk edge the same
// state, for a non-linked disposition and for an id two bound maps share.
func TestConsoleChainPanelsAgreeOnObligationHunkEdges(t *testing.T) {
	cases := []struct {
		name, want string
		setup      func(t *testing.T, f *chainFixture)
	}{
		{name: "disposition", want: GapUnsupported, setup: func(t *testing.T, f *chainFixture) {
			doc := f.ocm()
			doc["obligations"].([]any)[0].(map[string]any)["disposition"] = "unknown"
			f.writeOCM(t, "change.ocm.001.json", doc)
		}},
		{name: "duplicate", want: GapAmbiguous, setup: func(t *testing.T, f *chainFixture) {
			f.writeOCM(t, "change.ocm.001.json", f.ocm())
			f.writeOCM(t, "change.ocm.002.json", f.ocm())
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newChainFixture(t, "func Frob() {}", nil)
			tc.setup(t, fixture)
			head := chainGit(t, fixture.root, "rev-parse", "HEAD")
			chain := Worktree{Root: fixture.root}.ReadChain(context.Background(), head, fixture.change)
			edges := append([]ChainEdge(nil), chain.Hunks[0].Requirements...)
			for _, row := range chain.Requirements {
				edges = append(edges, row.Hunks...)
			}
			if len(edges) < 2 {
				t.Fatalf("want edges on both panels, got %+v", edges)
			}
			for _, edge := range edges {
				if edge.Gap != tc.want || edge.Anchor != "" && tc.want == GapAmbiguous {
					t.Fatalf("edge %s %s: gap %q anchor %q, want %q on both panels", edge.Artifact, edge.Field, edge.Gap, edge.Anchor, tc.want)
				}
			}
		})
	}
}

// TestConsoleChainOCMCapRendersAPartialGapRow is V1-0153: more OCM maps than
// the pane reads render a gap row naming the cap instead of being dropped
// silently (LAC-V0-035).
func TestConsoleChainOCMCapRendersAPartialGapRow(t *testing.T) {
	fixture := newChainFixture(t, "func Frob() {}", nil)
	for index := 0; index <= maxChainOCMs; index++ {
		chainWrite(t, fixture.root, fmt.Sprintf("%s/change.ocm.%03d.json", ocmDir, index), []byte("{}"))
	}
	body := fixture.page(t, "")
	want := fmt.Sprintf("<td>missing</td><td>%d OCM maps exist and the pane reads only the first %d by name, so this list is PARTIAL", maxChainOCMs+1, maxChainOCMs)
	if !strings.Contains(body, want) {
		t.Fatalf("no PARTIAL cap row %q:\n%s", want, mainOf(body))
	}
}

// TestConsoleChainTextMatchDecoy covers LAC-V0-034's no-inference rule: a
// requirement ID written in the changed line and in a cited span, an OCM map
// that lists the hunk but is bound to another map, and a trace that names the
// changed path under another revision link nothing.
func TestConsoleChainTextMatchDecoy(t *testing.T) {
	fixture := newChainFixture(t, "// FIX-V0-003 frob widget.go", nil)
	decoy := fixture.ocm()
	decoy["targetRevision"] = fixture.base
	decoy["cem"] = map[string]any{"mapSha256": strings.Repeat("c", 64)}
	decoy["obligations"] = []any{map[string]any{"id": "FIX-V0-003", "disposition": "linked",
		"hunkIds": []any{chainHunkID}, "claimIds": []any{chainClaimID}}}
	fixture.writeOCM(t, "change.ocm.002.json", decoy)
	fixture.writeTrace(t, fixture.base, fixture.traceRow(fixture.base, "widget.go FIX-V0-003 "+fixture.change, "go test ./..."))

	body := fixture.page(t, "&hunk="+chainHunkID)
	if !strings.Contains(body, "<pre>// FIX-V0-003 frob widget.go</pre>") {
		t.Fatalf("the decoy line is not the rendered hunk:\n%s", mainOf(body))
	}
	for _, banned := range []string{`href="#req-FIX-V0-003"`, `id="req-FIX-V0-003"`, `href="#req-FIX-V0-001"`, "recorded outcome"} {
		if strings.Contains(body, banned) {
			t.Fatalf("a decoy was linked (%q):\n%s", banned, mainOf(body))
		}
	}
	for _, want := range []string{"<td>unrelated</td>", "no OCM map bound to this sealed map by digest and revision was found",
		"<b>gap: unverified</b>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("decoy page missing %q:\n%s", want, mainOf(body))
		}
	}
}

// TestConsoleChainHostileContent covers LAC-V0-020 for the chain pane: markup
// in any artifact's free text renders escaped and inert.
func TestConsoleChainHostileContent(t *testing.T) {
	hostile := "<script>alert(1)</script>"
	fixture := newChainFixture(t, hostile, func(cem map[string]any) {
		hunk := cem["hunks"].([]any)[0].(map[string]any)
		hunk["disposition"], hunk["reason"], hunk["basis"] = "unknown", hostile, []any{}
	})
	doc := fixture.ocm()
	doc["obligations"] = []any{map[string]any{"id": `"><img src=x onerror=alert(1)>`, "disposition": "unknown", "reason": hostile,
		"hunkIds": []any{chainHunkID}, "claimIds": []any{}}}
	fixture.writeOCM(t, "change.ocm.001.json", doc)
	fixture.writeTrace(t, fixture.change, fixture.traceRow(fixture.change, hostile, hostile))

	body := fixture.page(t, "&hunk="+chainHunkID)
	for _, banned := range []string{"<script>alert", "<img src=x"} {
		if strings.Contains(body, banned) {
			t.Fatalf("hostile artifact text reached the page as markup (%q):\n%s", banned, mainOf(body))
		}
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("hostile text is not rendered escaped:\n%s", mainOf(body))
	}
}

// TestConsoleChainKeyboardNavigation covers LAC-V0-036 against the
// accessibility matrix of LAC-V0-029: plain anchors only, no pointer-only
// affordance, the pane in the primary navigation, links pinned to the commit,
// and a request the listing did not name reads nothing.
func TestConsoleChainKeyboardNavigation(t *testing.T) {
	fixture := newChainFixture(t, "func Frob() {}", nil)
	fixture.writeOCM(t, "change.ocm.001.json", fixture.ocm())
	server, err := New(Options{Addr: "127.0.0.1:0", Repo: fixture.root, Binary: "atm"})
	if err != nil {
		t.Fatal(err)
	}
	head := chainGit(t, fixture.root, "rev-parse", "HEAD")
	index := get(t, server, "/chain")
	chain := get(t, server, "/chain?change="+fixture.change+"&hunk="+chainHunkID)
	for _, page := range []string{index, chain} {
		lower := strings.ToLower(page)
		for _, banned := range []string{"onclick", "onmousedown", "onmouseup", "onkeypress=", `tabindex="-1"`, "<script"} {
			if strings.Contains(lower, banned) {
				t.Fatalf("page has a pointer-only or keyboard-trap affordance (%q):\n%s", banned, mainOf(page))
			}
		}
		if !strings.Contains(page, `<a href="/chain" aria-current="page">Chain</a>`) {
			t.Fatalf("the chain pane is not current in the primary navigation:\n%s", mainOf(page))
		}
	}
	for _, want := range []string{
		`<a href="/chain?change=` + fixture.change + `&amp;at=` + head + `"`,
	} {
		if !strings.Contains(index, want) {
			t.Fatalf("index missing plain pinned anchor %q:\n%s", want, mainOf(index))
		}
	}
	for _, want := range []string{`&amp;at=` + head + `#hunk-detail"`, `<a href="#req-FIX-V0-001">`, `id="req-FIX-V0-001"`, `id="hunk-detail"`} {
		if !strings.Contains(chain, want) {
			t.Fatalf("chain missing keyboard-reachable anchor %q:\n%s", want, mainOf(chain))
		}
	}

	stale := get(t, server, "/chain?change="+fixture.change+"&at="+fixture.base)
	if !strings.Contains(stale, "gap: This link is pinned to commit "+fixture.base) || strings.Contains(stale, "Change <code>") {
		t.Fatalf("a link pinned to another commit rendered content:\n%s", mainOf(stale))
	}
	for _, unlisted := range []string{fixture.base, "../../spec.md", "HEAD"} {
		if page := get(t, server, "/chain?change="+unlisted); strings.Contains(page, "Change <code>") {
			t.Fatalf("an unlisted change %q was read:\n%s", unlisted, mainOf(page))
		}
	}
}
