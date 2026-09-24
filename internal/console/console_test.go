package console

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubTaskman writes a fake `atm` that replies with canned envelopes, so the
// console's own behaviour is tested without a real ticket store. The stub
// prints the envelope for the verb it was given and exits non-zero for a
// refusal, exactly as `atm` does.
func stubTaskman(t *testing.T, replies map[string]string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub tool is a POSIX shell script")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "atm-stub")
	var script strings.Builder
	script.WriteString("#!/bin/sh\nverb=\"$1 $2\"\ncase \"$verb\" in\n")
	for verb, envelope := range replies {
		script.WriteString("  '" + verb + "'*) cat <<'ENVELOPE'\n" + envelope + "\nENVELOPE\n  ;;\n")
	}
	script.WriteString("  *) echo '{\"profile\":\"taskman-command-result/0\",\"command\":[],\"outcome\":\"ERROR\",\"codes\":[],\"warnings\":[\"stub has no reply\"],\"items\":[],\"untrusted\":[]}' ;;\nesac\n")
	if err := os.WriteFile(path, []byte(script.String()), 0o755); err != nil {
		t.Fatalf("stub: %v", err)
	}
	return path
}

const helpEnvelope = `{"profile":"taskman-command-result/0","command":["help"],"outcome":"OK",
"codes":[],"warnings":[],"untrusted":[],"items":[{"implemented":["ticket list","ticket show","ticket hold"],
"omitted":["ticket archive","admit"],"statuses":["DRAFT","OPEN","HELD"],
"eligibility":["BLOCKED","UNKNOWN"],"note":"stub"}]}`

const listEnvelope = `{"profile":"taskman-command-result/0","command":["ticket","list"],"outcome":"OK",
"codes":[],"warnings":[],"untrusted":["title"],"items":[
{"ticketId":"ticket:acme:main:AT-1","title":"Ordinary","status":"OPEN","kind":"FEATURE","priority":"P2",
 "revision":"3","eligibility":"UNKNOWN","nextAction":"admit","blockers":[],"unknowns":[]},
{"ticketId":"ticket:acme:main:AT-2","title":"Strange","status":"WEIRD_STATUS","kind":"BUG","priority":"P1",
 "revision":"1","eligibility":"BLOCKED","nextAction":null,"blockers":[],
 "unknowns":[{"code":"ATTEMPT_LIVE","detail":"attempt liveness NOT_OBSERVED","ticketId":null}]}]}`

const refusalEnvelope = `{"profile":"taskman-command-result/0","command":["ticket","list"],"outcome":"REFUSED",
"codes":["UNINITIALIZED"],"warnings":["state dir does not exist; run ` + "`atm init`" + `"],
"items":[],"untrusted":[]}`

func newTestServer(t *testing.T, binary string) *Server {
	t.Helper()
	server, err := New(Options{Addr: "127.0.0.1:0", Repo: t.TempDir(), Binary: binary})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return server
}

func get(t *testing.T, server *Server, target string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Host = server.host
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", target, recorder.Code, recorder.Body.String())
	}
	return recorder.Body.String()
}

// TestConsoleBoundary covers LAC-V0-001..005: the console is opt-in, binds
// loopback only, and keeps nothing between requests.
func TestConsoleBoundary(t *testing.T) {
	t.Run("LAC-V0-001 current standalone presentation", func(t *testing.T) {
		server := newTestServer(t, stubTaskman(t, map[string]string{"help ": helpEnvelope, "ticket list": listEnvelope}))
		body := get(t, server, "/")
		for _, want := range []string{"<title>Board · Corvint Console</title>", "Corvint Console", "corvint-tasks", "corvint-dashboard-snapshot"} {
			if !strings.Contains(body, want) {
				t.Errorf("page omitted %q", want)
			}
		}
		if got := server.snapshotBinary(); got != "corvint-dashboard-snapshot" {
			t.Fatalf("snapshot binary = %q", got)
		}
	})
	t.Run("refuses a non-loopback address", func(t *testing.T) {
		for _, addr := range []string{"0.0.0.0:7777", "192.168.1.10:7777", "[::]:7777"} {
			if _, err := New(Options{Addr: addr, Repo: t.TempDir(), Binary: "atm"}); err == nil {
				t.Errorf("%s was accepted; the console has no authentication and must not be reachable off-host", addr)
			}
		}
	})
	t.Run("accepts loopback", func(t *testing.T) {
		for _, addr := range []string{"127.0.0.1:7777", "localhost:7777", "[::1]:7777"} {
			if _, err := New(Options{Addr: addr, Repo: t.TempDir(), Binary: "atm"}); err != nil {
				t.Errorf("%s was refused: %v", addr, err)
			}
		}
	})
	t.Run("holds no state between requests", func(t *testing.T) {
		// The board is read fresh each time: changing what the tool reports
		// changes the next response with no cache to invalidate (LAC-V0-004).
		dir := t.TempDir()
		binary := filepath.Join(dir, "atm-stub")
		write := func(list string) {
			script := "#!/bin/sh\ncase \"$1 $2\" in\n 'help '*) cat <<'E'\n" + helpEnvelope +
				"\nE\n ;;\n *) cat <<'E'\n" + list + "\nE\n ;;\nesac\n"
			if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		write(refusalEnvelope)
		server := newTestServer(t, binary)
		if body := get(t, server, "/"); !strings.Contains(body, "UNINITIALIZED") {
			t.Fatal("first read did not show the refusal")
		}
		write(listEnvelope)
		if body := get(t, server, "/"); !strings.Contains(body, "AT-1") {
			t.Error("the second read served a cached refusal instead of the new state")
		}
	})
}

// TestConsoleAxisPassthrough covers LAC-V0-006..011: attribution, the six
// axes as the source stated them, and weakest-authority derivation.
func TestConsoleAxisPassthrough(t *testing.T) {
	t.Run("an unstated axis is never defaulted", func(t *testing.T) {
		axes := UnstatedAxes()
		for _, pair := range axes.Pairs() {
			if pair.Stated() {
				t.Errorf("%s was %q before any source stated it", pair.Axis, pair.Value)
			}
		}
	})
	t.Run("a value outside the closed set is refused", func(t *testing.T) {
		axes := UnstatedAxes()
		if axes.Set(AxisValidity, "PROBABLY_FINE") {
			t.Error("a value outside the closed set was accepted as an axis")
		}
		if axes.Validity != Unstated {
			t.Errorf("validity = %q after a refused set", axes.Validity)
		}
		if !axes.Set(AxisValidity, "VALID") || axes.Validity != "VALID" {
			t.Error("a closed value was not accepted")
		}
	})
	t.Run("a derived value takes the weakest contribution", func(t *testing.T) {
		strong := UnstatedAxes()
		strong.Set(AxisAuthorityClass, "REPOSITORY_ACCEPTED")
		strong.Set(AxisCompleteness, "COMPLETE")
		weak := UnstatedAxes()
		weak.Set(AxisAuthorityClass, "ADVISORY")
		weak.Set(AxisCompleteness, "PARTIAL")

		combined := Weakest(strong, weak)
		if combined.AuthorityClass != "ADVISORY" {
			t.Errorf("authorityClass = %q, want the weaker ADVISORY", combined.AuthorityClass)
		}
		if combined.Completeness != "PARTIAL" {
			t.Errorf("completeness = %q, want the weaker PARTIAL", combined.Completeness)
		}
		// An unstated contribution cannot be improved by a stated one.
		if got := Weakest(strong, UnstatedAxes()).AuthorityClass; got != Unstated {
			t.Errorf("authorityClass = %q, want %s when one source stated nothing", got, Unstated)
		}
	})
	t.Run("the page carries its invocation and its boundary", func(t *testing.T) {
		binary := stubTaskman(t, map[string]string{"help ": helpEnvelope, "ticket list": listEnvelope})
		body := get(t, newTestServer(t, binary), "/")
		if !strings.Contains(body, "read by") || !strings.Contains(body, "ticket list") {
			t.Error("the board does not name the invocation that produced it")
		}
		if !strings.Contains(body, "compiled at") || !strings.Contains(body, "This page is historical") {
			t.Error("the page does not state its own observation boundary")
		}
		if !strings.Contains(body, Unstated) {
			t.Error("axes the tool did not state are not rendered as unstated")
		}
	})
}

// TestConsoleMutationDelegation covers LAC-V0-012..015: every change runs the
// owning verb, an unimplemented verb is a disabled control carrying the
// tool's reason, and the console never invokes one.
func TestConsoleMutationDelegation(t *testing.T) {
	caps := &Capabilities{
		Implemented: []string{"ticket list", "ticket hold"},
		Omitted:     []string{"ticket archive"},
	}
	t.Run("controls follow the tool's own capability report", func(t *testing.T) {
		byVerb := map[string]Control{}
		for _, control := range caps.Controls() {
			byVerb[control.Verb] = control
		}
		if !byVerb["hold"].Enabled {
			t.Error("hold is implemented but its control is disabled")
		}
		if byVerb["archive"].Enabled {
			t.Error("archive is omitted but its control is enabled")
		}
		if byVerb["archive"].Reason == "" {
			t.Error("a disabled control carries no reason")
		}
		if _, ok := byVerb["archive"]; !ok {
			t.Error("an unimplemented verb was hidden instead of disabled")
		}
	})
	t.Run("an unimplemented verb is never invoked", func(t *testing.T) {
		dir := t.TempDir()
		marker := filepath.Join(dir, "invoked")
		binary := filepath.Join(dir, "atm-stub")
		// Only a ticket mutation trips the marker: the console legitimately
		// runs `atm version` and `atm help` for attribution and capabilities.
		script := "#!/bin/sh\ncase \"$1 $2\" in\n 'help '*) cat <<'E'\n" + helpEnvelope +
			"\nE\n ;;\n 'ticket archive'*) touch " + marker + "; echo '{}' ;;\n *) echo '{}' ;;\nesac\n"
		if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		server := newTestServer(t, binary)
		recorder := httptest.NewRecorder()
		form := strings.NewReader("token=" + server.token + "&verb=archive&ticket=ticket:acme:main:AT-1&expected=3&payload=%7B%7D&requestId=r1")
		request := httptest.NewRequest(http.MethodPost, "/mutate", form)
		request.Host = server.host
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		server.ServeHTTP(recorder, request)

		if _, err := os.Stat(marker); err == nil {
			t.Fatal("the console invoked a verb the tool reported as unimplemented")
		}
		if !strings.Contains(recorder.Body.String(), "not implemented") {
			t.Error("the refusal does not carry the tool's reason")
		}
	})
	t.Run("a mutation sends the revision the operator was shown", func(t *testing.T) {
		dir := t.TempDir()
		argv := filepath.Join(dir, "argv")
		binary := filepath.Join(dir, "atm-stub")
		script := "#!/bin/sh\ncase \"$1 $2\" in\n 'help '*) cat <<'E'\n" + helpEnvelope +
			"\nE\n ;;\n *) echo \"$@\" > " + argv + "; cat <<'E'\n" + refusalEnvelope + "\nE\n ;;\nesac\n"
		if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		server := newTestServer(t, binary)
		recorder := httptest.NewRecorder()
		form := strings.NewReader("token=" + server.token + "&verb=hold&ticket=ticket:acme:main:AT-1&expected=3&payload=%7B%7D&requestId=r1")
		request := httptest.NewRequest(http.MethodPost, "/mutate", form)
		request.Host = server.host
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		server.ServeHTTP(recorder, request)

		recorded, err := os.ReadFile(argv)
		if err != nil {
			t.Fatalf("the owning verb was not invoked: %v", err)
		}
		for _, want := range []string{"ticket hold", "--expected-revision 3", "--request-id r1", "--target ticket:acme:main:AT-1"} {
			if !strings.Contains(string(recorded), want) {
				t.Errorf("invocation %q is missing %q", strings.TrimSpace(string(recorded)), want)
			}
		}
	})
	t.Run("a mutation is POST only", func(t *testing.T) {
		binary := stubTaskman(t, map[string]string{"help ": helpEnvelope})
		recorder := httptest.NewRecorder()
		server := newTestServer(t, binary)
		request := httptest.NewRequest(http.MethodGet, "/mutate", nil)
		request.Host = server.host
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET /mutate = %d, want 405", recorder.Code)
		}
	})
}

// TestConsoleRendering covers LAC-V0-016..020: columns from the tool's own
// enumeration, an unmapped column for a status it did not name, refusals as
// refusals, and untrusted text rendered inert.
func TestConsoleRendering(t *testing.T) {
	t.Run("columns come from the tool and an unknown status is not dropped", func(t *testing.T) {
		binary := stubTaskman(t, map[string]string{"help ": helpEnvelope, "ticket list": listEnvelope})
		body := get(t, newTestServer(t, binary), "/")
		for _, status := range []string{"DRAFT", "OPEN", "HELD"} {
			if !strings.Contains(body, status) {
				t.Errorf("column %s from the tool's enumeration is missing", status)
			}
		}
		if strings.Contains(body, "COMPLETED") {
			t.Error("a column the tool did not enumerate was invented")
		}
		if !strings.Contains(body, "WEIRD_STATUS") || !strings.Contains(body, "not enumerated by the tool") {
			t.Error("a ticket with an unenumerated status was dropped instead of shown as unmapped")
		}
		if !strings.Contains(body, "ATTEMPT_LIVE") {
			t.Error("an unobservable fact the tool reported was not rendered")
		}
	})
	t.Run("a refusal renders as a refusal, never as an empty board", func(t *testing.T) {
		binary := stubTaskman(t, map[string]string{"help ": helpEnvelope, "ticket list": refusalEnvelope})
		body := get(t, newTestServer(t, binary), "/")
		if !strings.Contains(body, "UNINITIALIZED") {
			t.Error("the refusal code is missing")
		}
		if !strings.Contains(body, "state dir does not exist") {
			t.Error("the tool's own warning text is missing")
		}
		if !strings.Contains(body, "not an empty result") {
			t.Error("the page does not say the refusal is not an empty result")
		}
		if strings.Contains(body, `class="col`) {
			t.Error("a board was drawn for a read that was refused")
		}
	})
	t.Run("untrusted text renders inert", func(t *testing.T) {
		hostile := strings.Replace(listEnvelope, `"title":"Ordinary"`,
			`"title":"<script>alert(1)</script><img src=x onerror=alert(2)>"`, 1)
		binary := stubTaskman(t, map[string]string{"help ": helpEnvelope, "ticket list": hostile})
		body := get(t, newTestServer(t, binary), "/")
		if strings.Contains(body, "<script>alert(1)</script>") || strings.Contains(body, "<img src=x") {
			t.Fatal("ticket text reached the page as live markup")
		}
		if !strings.Contains(body, "&lt;script&gt;") {
			t.Error("the hostile title was dropped rather than rendered inert")
		}
		if !strings.Contains(body, "untrusted") {
			t.Error("the envelope's untrusted field was not reported")
		}
	})
	t.Run("the code pane names the object id it read", func(t *testing.T) {
		root := repoRoot(t)
		blob := Worktree{Root: root}.Read(context.Background(), "HEAD", "go.mod")
		if blob.Err != "" {
			t.Fatalf("read: %s", blob.Err)
		}
		if len(blob.ObjectID) != 40 {
			t.Errorf("object id = %q, want a 40-character git object id", blob.ObjectID)
		}
		if !strings.Contains(blob.Text, "module ") {
			t.Error("the committed bytes were not returned")
		}
		if blob.Source.Axes.AuthorityClass != "OWNING_VERIFIER" {
			t.Errorf("authorityClass = %q for content read at an object id", blob.Source.Axes.AuthorityClass)
		}
	})
	t.Run("a requirement id reaches its clause and its spec status", func(t *testing.T) {
		clause := SpecLookup{Root: repoRoot(t)}.Resolve("LAC-V0-001")
		if clause.Err != "" {
			t.Fatalf("resolve: %s", clause.Err)
		}
		if clause.Requirement.File != "docs/specs/local-admin-console-v0.md" {
			t.Errorf("file = %q", clause.Requirement.File)
		}
		if !strings.Contains(clause.Text, "LAC-V0-001") {
			t.Errorf("clause text does not contain the requirement: %q", clause.Text)
		}
		if clause.Spec == nil || clause.Spec.Delivery == "" {
			t.Error("the owning spec's declared delivery status was not rendered alongside the clause")
		}
	})
}

// TestReadDirtyCheckUsesLiteralPathspec covers Read's dirty check (LAC-V0-019):
// a path containing Git pathspec glob syntax must be matched by name, not as a
// glob an unrelated dirty file can also satisfy, or a clean file at the
// requested path is mislabeled dirty.
func TestReadDirtyCheckUsesLiteralPathspec(t *testing.T) {
	root := t.TempDir()
	// "foo[bar].txt" as a glob matches a single-character class {b,a,r}, so it
	// also matches "foob.txt" unless the pathspec is literal.
	files := map[string]string{"foo[bar].txt": "clean\n", "foob.txt": "clean\n"}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-q", "--no-gpg-sign", "-m", "fixture"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = gitEnvironment()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "foob.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blob := Worktree{Root: root}.Read(context.Background(), "HEAD", "foo[bar].txt")
	if blob.Err != "" {
		t.Fatalf("read: %s", blob.Err)
	}
	if blob.Dirty {
		t.Fatalf("Read reported the untouched foo[bar].txt dirty because of an unrelated glob-matched file; DirtyNote=%q", blob.DirtyNote)
	}
}

// TestHeadKeepsFirstDirtyPath covers LAC-V0-019's dirty label: the porcelain
// status column of the first line is part of the record, so a one-character
// path must still be counted.
func TestHeadKeepsFirstDirtyPath(t *testing.T) {
	root := committedFixture(t, map[string]string{"a": "clean\n"})
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	revision := Worktree{Root: root}.Head(context.Background())
	if revision.Err != "" || !revision.Dirty || len(revision.DirtyPaths) != 1 || revision.DirtyPaths[0] != "a" {
		t.Fatalf("Head = %+v, want one dirty path \"a\"", revision)
	}
}

// TestReadLabelsUnobservedWorktree covers LAC-V0-019 when the dirty check
// itself fails: the committed bytes are still shown, but the page must say the
// worktree difference was not observed rather than imply a clean path.
func TestReadLabelsUnobservedWorktree(t *testing.T) {
	root := committedFixture(t, map[string]string{"a.txt": "clean\n"})
	if err := os.WriteFile(filepath.Join(root, ".git", "index"), []byte("not an index"), 0o644); err != nil {
		t.Fatal(err)
	}
	blob := Worktree{Root: root}.Read(context.Background(), "HEAD", "a.txt")
	if blob.Err != "" || blob.Text != "clean\n" {
		t.Fatalf("read: text %q err %q", blob.Text, blob.Err)
	}
	if !blob.Dirty || !strings.Contains(blob.DirtyNote, "not observed") {
		t.Fatalf("a failed worktree status was presented as clean: Dirty=%v DirtyNote=%q", blob.Dirty, blob.DirtyNote)
	}
}

func committedFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-q", "--no-gpg-sign", "-m", "fixture"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = gitEnvironment()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	return root
}

// repoRoot walks up to the repository this test file lives in.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("no repository root above the test directory")
	return ""
}

// stubSnapshot writes a fake `corvint-dashboard-snapshot` that prints one canned
// document, so the evidence pane's own behaviour is tested without compiling a
// real snapshot.
func stubSnapshot(t *testing.T, document string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub tool is a POSIX shell script")
	}
	path := filepath.Join(t.TempDir(), "snapshot-stub")
	script := "#!/bin/sh\ncat <<'DOCUMENT'\n" + document + "\nDOCUMENT\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("stub: %v", err)
	}
	return path
}

// snapshotDocument carries one metric whose axes are all stated, one whose
// value was never measured, and one whose authorityClass is a vocabulary the
// console does not know.
const snapshotDocument = `{"schema":"corvint-dashboard-snapshot/0",
"generatedAt":"2026-09-08T00:00:00.000000000Z","snapshotSha256":"sha256:abc",
"observation":{"scanState":"COMPLETE"},
"repository":{"headRevision":"deadbeef","treeRevision":"cafe","worktreeState":"MIXED","dirtyPathCount":"2"},
"privacy":{"outboundNetwork":"NONE"},
"sources":[{"id":"dashboard-source:sha256:1","displayLabel":"local-trace-v1#0","adapterId":"local-trace-v1",
"profile":"corvint-local-trace/1","verifierId":"go-local-trace-v1","contentSha256":"sha256:def","byteCount":"12",
"validity":"VALID","epistemicClass":"OBSERVED","authorityClass":"ADAPTER_QUALIFIED","completeness":"COMPLETE",
"currency":"VALIDATED_AT","deliveryStage":"NOT_STARTED","exclusions":[]}],
"issues":[{"code":"OBSERVATION_TIME_UNKNOWN","severity":"INFO","sourceId":"dashboard-source:sha256:1"}],
"data":[{"name":"data.artifact.bytes","unit":"BYTES","value":"4096","scopeClass":"COHORT",
"sourceIds":["dashboard-source:sha256:1"],"exclusions":[],"dimensions":[],
"validity":"VALID","epistemicClass":"OBSERVED","authorityClass":"ADAPTER_QUALIFIED",
"completeness":"COMPLETE","currency":"VALIDATED_AT","deliveryStage":"IMPLEMENTED"}],
"verification":[{"name":"verification.closure","unit":"COUNT","value":null,"scopeClass":"UNAVAILABLE",
"sourceIds":[],"exclusions":[],"dimensions":[],
"validity":"UNSUPPORTED","epistemicClass":"NOT_OBSERVED","authorityClass":"MADE_UP_CLASS",
"completeness":"UNKNOWN","currency":"UNKNOWN","deliveryStage":"UNSUPPORTED"}]}`

// TestConsoleEvidencePane covers the S3 evidence surface: the snapshot's axes
// reach the page as the snapshot stated them, an unmeasured value is not a
// zero, and the compiler's refusal is a refusal.
func TestConsoleEvidencePane(t *testing.T) {
	newEvidenceServer := func(t *testing.T, document string) *Server {
		t.Helper()
		server, err := New(Options{
			Addr: "127.0.0.1:0", Repo: repoRoot(t), Binary: "atm",
			Snapshot: stubSnapshot(t, document),
		})
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		return server
	}

	t.Run("axes reach the page as the snapshot stated them", func(t *testing.T) {
		body := get(t, newEvidenceServer(t, snapshotDocument), "/evidence")
		for _, stated := range []string{"ADAPTER_QUALIFIED", "VALIDATED_AT", "IMPLEMENTED", "NOT_OBSERVED"} {
			if !strings.Contains(body, stated) {
				t.Errorf("axis value %s the snapshot stated is missing from the page", stated)
			}
		}
		if !strings.Contains(body, "4096") {
			t.Error("the measured value is missing")
		}
	})
	t.Run("a vocabulary the console does not know stays unstated", func(t *testing.T) {
		body := get(t, newEvidenceServer(t, snapshotDocument), "/evidence")
		if strings.Contains(body, "MADE_UP_CLASS") {
			t.Error("an unrecognized string reached the page dressed as an axis")
		}
		if !strings.Contains(body, Unstated) {
			t.Errorf("the unrecognized axis was not rendered as %s", Unstated)
		}
	})
	t.Run("an unmeasured value is not rendered as a zero", func(t *testing.T) {
		body := get(t, newEvidenceServer(t, snapshotDocument), "/evidence")
		if !strings.Contains(body, "no value measured") {
			t.Error("a metric with no value was not marked unmeasured")
		}
	})
	t.Run("the compiler's refusal renders as a refusal", func(t *testing.T) {
		refusal := `{"code":"DASHBOARD_INVALID_ARGUMENT","profile":"corvint-dashboard-error/0"}`
		body := get(t, newEvidenceServer(t, refusal), "/evidence")
		if !strings.Contains(body, "DASHBOARD_INVALID_ARGUMENT") {
			t.Error("the compiler's own code is missing")
		}
		if !strings.Contains(body, "not an empty result") {
			t.Error("the refusal was not presented as a refusal")
		}
		if strings.Contains(body, "Metrics (") {
			t.Error("a metrics panel was drawn for a snapshot that was refused")
		}
	})
	t.Run("LAC-V0-009 the compiler's stderr refusal keeps its code", func(t *testing.T) {
		// corvint-dashboard-snapshot writes its refusal to stderr and exits 2.
		path := filepath.Join(t.TempDir(), "snapshot-stub")
		script := "#!/bin/sh\necho '{\"code\":\"DASHBOARD_REPOSITORY_UNAVAILABLE\",\"profile\":\"corvint-dashboard-error/0\"}' >&2\nexit 2\n"
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		evidence := Dashboard{Binary: path, Root: repoRoot(t)}.Read(context.Background())
		if !evidence.Refused || evidence.Code != "DASHBOARD_REPOSITORY_UNAVAILABLE" || evidence.Err != "" {
			t.Fatalf("refused=%v code=%q err=%q, want the compiler's own refusal code", evidence.Refused, evidence.Code, evidence.Err)
		}
	})
	t.Run("LAC-V0-022 an error object without a code is a failure, not a refusal", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "snapshot-stub")
		script := "#!/bin/sh\necho '{\"code\":\"\",\"profile\":\"corvint-dashboard-error/0\"}'\nexit 2\n"
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		evidence := Dashboard{Binary: path, Root: repoRoot(t)}.Read(context.Background())
		if evidence.Refused || evidence.Err == "" {
			t.Fatalf("refused=%v code=%q err=%q, want a failure for a code-less error object", evidence.Refused, evidence.Code, evidence.Err)
		}
	})
	t.Run("LAC-V0-008 an undecodable metric family is invalid, not absent", func(t *testing.T) {
		document := strings.Replace(snapshotDocument, `"value":"4096"`, `"value":4096`, 1)
		body := get(t, newEvidenceServer(t, document), "/evidence")
		if strings.Contains(body, "Metrics (") {
			t.Error("a metrics panel was drawn although the data family could not be decoded")
		}
		if !strings.Contains(body, `undecodable &#34;data&#34; metric family`) {
			t.Error("the undecodable family was not reported")
		}
	})
	t.Run("LAC-V0-008 a null metric family or metric is invalid, not empty", func(t *testing.T) {
		metric := strings.Index(snapshotDocument, `{"name":"data.artifact.bytes"`)
		for name, document := range map[string]string{
			"null family": strings.Replace(snapshotDocument, `"data":[`, `"data":null,"unrendered":[`, 1),
			"null metric": snapshotDocument[:metric] + "null," + snapshotDocument[metric:],
		} {
			body := get(t, newEvidenceServer(t, document), "/evidence")
			if strings.Contains(body, "Metrics (") {
				t.Errorf("%s: a metrics panel was drawn although the data family is not an array of metrics", name)
			}
			if !strings.Contains(body, `undecodable &#34;data&#34; metric family`) {
				t.Errorf("%s: the undecodable family was not reported", name)
			}
		}
	})
}

// TestConsoleDogfoodPane covers the S3 dogfood surface: an absent report is
// NOT_OBSERVED rather than a pass, and every NOT_PRODUCED reason is shown.
func TestConsoleDogfoodPane(t *testing.T) {
	t.Run("an absent report is NOT_OBSERVED, not an empty pass", func(t *testing.T) {
		server, err := New(Options{Addr: "127.0.0.1:0", Repo: t.TempDir(), Binary: "atm"})
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		body := get(t, server, "/dogfood")
		if !strings.Contains(body, "NOT_OBSERVED") {
			t.Error("a repository with no dogfood report did not render as NOT_OBSERVED")
		}
		if !strings.Contains(body, "not a pass") {
			t.Error("the page does not say an absent report is not a pass")
		}
	})
	t.Run("LAC-V0-006 shared report attribution preserves missing ownership", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		report := `{"profile":"corvint-dogfood-change/0","base":"aaa","target":"bbb","complete":false,
"steps":[{"name":"prechange-query","status":"PRODUCED","reason":"none"},
{"name":"ocm-aggregate","status":"NOT_PRODUCED","reason":"missing-intent-scope"}],
"anchor":{"state":"NOT_OBSERVED","mergeBase":null}}`
		if err := os.WriteFile(filepath.Join(root, dogfoodReportPath), []byte(report), 0o644); err != nil {
			t.Fatal(err)
		}
		server, err := New(Options{Addr: "127.0.0.1:0", Repo: root, Binary: "atm"})
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		body := get(t, server, "/dogfood")
		if !strings.Contains(body, "missing-intent-scope") {
			t.Error("a NOT_PRODUCED reason was summarised away")
		}
		if !strings.Contains(body, "1 of 2 steps did not produce") {
			t.Error("an incomplete loop was not reported as incomplete")
		}
		for _, want := range []string{"Last-written dogfood report", dogfoodReportPath,
			"It may belong to another session", "Session ownership is NOT_OBSERVED",
			"file modified at", "observed at", "aaa", "bbb", "NOT_STATED"} {
			if !strings.Contains(body, want) {
				t.Errorf("shared report attribution missing %q", want)
			}
		}
	})
}

// TestConsoleDogfoodPacketCoverage pins the DCW-V0-016 reader contract: a
// report carrying packetCoverage shows each packet's numbers, and a report
// written before the field existed still reads and says it was not reported.
func TestConsoleDogfoodPacketCoverage(t *testing.T) {
	writeReport := func(t *testing.T, raw []byte) string {
		t.Helper()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dogfoodReportPath), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	t.Run("DCW-V0-016 an older report without the block still reads", func(t *testing.T) {
		historical := filepath.Join(repoRoot(t), "conformance/use-cases-v0/receipts/UC-EVIDENCE-CARRYING-COMPLETION/corvint-dogfood/dogfood-report.json")
		raw, err := os.ReadFile(historical)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "packetCoverage") {
			t.Fatal("the historical fixture is expected to predate packetCoverage")
		}
		root := writeReport(t, raw)
		report := ReadDogfood(root)
		if report.Err != "" || report.PacketCoverage != nil {
			t.Fatalf("older report: err=%q packetCoverage=%v", report.Err, report.PacketCoverage)
		}
		server, err := New(Options{Addr: "127.0.0.1:0", Repo: root, Binary: "atm"})
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		if body := get(t, server, "/dogfood"); !strings.Contains(body, "this report predates packet coverage") {
			t.Error("an older report did not say packet coverage was not reported")
		}
	})
	t.Run("DCW-V0-016 each packet's coverage fields are shown", func(t *testing.T) {
		report := `{"profile":"corvint-dogfood-change/0","base":"aaa","target":"bbb","complete":true,"steps":[],
"packetCoverage":[{"step":"prechange-query","status":"PRODUCED","packet_bytes":3820,"budget_bytes":null,"within_budget":true,"included_results":1,"omitted_results":0},
{"step":"prechange-impact","status":"NOT_PRODUCED","reason":"packet-not-compiled"}]}`
		root := writeReport(t, []byte(report))
		read := ReadDogfood(root)
		if read.Err != "" || len(read.PacketCoverage) != 2 || read.PacketCoverage[0].PacketBytes != 3820 || read.PacketCoverage[0].BudgetBytes != nil {
			t.Fatalf("packet coverage read as err=%q %+v", read.Err, read.PacketCoverage)
		}
		server, err := New(Options{Addr: "127.0.0.1:0", Repo: root, Binary: "atm"})
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		body := get(t, server, "/dogfood")
		for _, want := range []string{"<td>3820</td><td>null</td><td>true</td><td>1</td><td>0</td>", "packet-not-compiled"} {
			if !strings.Contains(body, want) {
				t.Errorf("the packet coverage panel is missing %q", want)
			}
		}
	})
}

// TestConsoleListingPanes covers the S3 benchmark and backlog surfaces: each
// entry is named by the object id its bytes come from, a path the listing did
// not name is not readable, and backlog text renders inert.
func TestConsoleListingPanes(t *testing.T) {
	newListingServer := func(t *testing.T) *Server {
		t.Helper()
		server, err := New(Options{Addr: "127.0.0.1:0", Repo: repoRoot(t), Binary: "atm"})
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		return server
	}

	t.Run("each entry is named by its object id", func(t *testing.T) {
		listing := Worktree{Root: repoRoot(t)}.List(context.Background(), "HEAD", agentMemoryDir)
		if listing.Err != "" {
			t.Fatalf("list: %s", listing.Err)
		}
		if len(listing.Entries) == 0 {
			t.Fatal("the backlog directory listed no entry")
		}
		for _, entry := range listing.Entries {
			if len(entry.ObjectID) != 40 {
				t.Errorf("%s: object id = %q, want a 40-character git object id", entry.Path, entry.ObjectID)
			}
		}
		if listing.Source.Axes.AuthorityClass != "OWNING_VERIFIER" {
			t.Errorf("authorityClass = %q for a tree read at a revision", listing.Source.Axes.AuthorityClass)
		}
	})
	t.Run("a path the listing did not name is not read", func(t *testing.T) {
		body := get(t, newListingServer(t), "/backlogs?path=AGENTS.md")
		if strings.Contains(body, "<pre>") {
			t.Error("the backlog pane read a path outside the directory it listed")
		}
	})
	t.Run("backlog text renders inert", func(t *testing.T) {
		body := get(t, newListingServer(t), "/backlogs?path="+agentMemoryDir+"/bugs.md")
		if !strings.Contains(body, "<pre>") {
			t.Fatal("a listed backlog file was not rendered")
		}
		if strings.Contains(body, "untrusted text and renders inert") == false {
			t.Error("the page does not say backlog entries are untrusted")
		}
	})
	t.Run("a non-ASCII file name is listed and read", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, filepath.FromSlash(agentMemoryDir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "café.md"), []byte("accented backlog body\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", "-q"}, {"add", "."},
			{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-q", "--no-gpg-sign", "-m", "fixture"}} {
			cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
			cmd.Env = gitEnvironment()
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %s %v", args, out, err)
			}
		}
		server, err := New(Options{Addr: "127.0.0.1:0", Repo: root, Binary: "atm"})
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		body := get(t, server, "/backlogs?path="+agentMemoryDir+"/caf%C3%A9.md")
		if !strings.Contains(body, "accented backlog body") {
			t.Error("a committed backlog file with a non-ASCII name was not listed under its real path")
		}
	})
	t.Run("benchmark results reach the same surface", func(t *testing.T) {
		body := get(t, newListingServer(t), "/benchmarks")
		if !strings.Contains(body, "agent-retrieval-bench") {
			t.Error("the committed benchmark results were not listed")
		}
		if !strings.Contains(body, "states no evidence class") {
			t.Error("the page does not say a result file states no evidence class of its own")
		}
	})
}

// TestConsoleNotLinkedIntoDefaultBinary pins LAC-V0-001: the default local
// product is one native-Go binary with no UI. A loopback bind and
// statelessness test alone would still pass if cmd/corvint linked this
// package, so this checks the dependency edge directly.
func TestConsoleNotLinkedIntoDefaultBinary(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "list", "-deps", "./cmd/corvint")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), "internal/console") {
		t.Error("cmd/corvint reaches internal/console; the default binary must not link the console")
	}
}
