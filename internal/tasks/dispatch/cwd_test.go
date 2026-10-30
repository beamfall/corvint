//go:build darwin || linux

package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cwdScript records the worker's physical directory, CORVINT_DISPATCH_CWD,
// the host MCP configuration and every argv element.
const cwdScript = `{ pwd -P; printf '%s\n' "${CORVINT_DISPATCH_CWD-UNSET}" "$OPENCODE_CONFIG_CONTENT"; printf '%s\n' "$@"; } > "$CORVINT_DISPATCH_EVIDENCE_DIR/cwd.txt.tmp" && mv "$CORVINT_DISPATCH_EVIDENCE_DIR/cwd.txt.tmp" "$CORVINT_DISPATCH_EVIDENCE_DIR/cwd.txt"`

// openCodeConfig is the reporter's issue 1754 shape: the coordination
// repository holds the store at workRoot; the ticket's code changes live in
// a linked worktree of another repository, and the OpenCode host, which has
// no directory flag, starts its Corvint MCP server with --root {cwd}.
func openCodeConfig(t *testing.T, cwd *RoleCwd) *Config {
	t.Helper()
	c := testConfig(t, cwdScript)
	h := c.Hosts["sh"]
	h.Argv = []string{h.Argv[0], "run", "--root", "{cwd}", "{prompt}"}
	h.Env = map[string]string{"OPENCODE_CONFIG_CONTENT": `{"mcp":{"corvint":{"type":"local","command":["corvint-mcp","--root","{cwd}"]}}}`}
	c.Hosts["sh"] = h
	c.Roles[0].Cwd = cwd
	c.Roles[0].IdleSeconds, c.Roles[0].WallSeconds = 3600, 7200
	return c
}

// launchCwdRecord launches one worker and returns its launched event and the
// lines cwdScript recorded.
func launchCwdRecord(t *testing.T, c *Config, obs Observation) (Event, []string) {
	t.Helper()
	d := openPacket(t, &fakeQueue{obs: obs}, c)
	tick(t, d)
	if len(d.ledger.Workers) != 1 {
		t.Fatalf("workers %d, events %v", len(d.ledger.Workers), kinds(t, d))
	}
	w := d.ledger.Workers[0]
	killOnCleanup(t, w)
	var raw []byte
	tickUntil(t, d, func() bool {
		var err error
		raw, err = os.ReadFile(filepath.Join(EvidenceDir(d.dir, w.ID), "cwd.txt"))
		return err == nil
	})
	return launchedOf(t, d, "launched", w.ID), strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

func physical(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestATRV0340_OpenCodeMultiRepoLaunchesInTicketWorktree: a role whose cwd
// template is {worktree} starts its OpenCode worker in the ticket worktree
// of the code repository, renders {cwd} into the MCP --root of both argv and
// the host configuration, sets CORVINT_DISPATCH_CWD, and records the
// directory and its source in the launched event (ATR-V0-340, 341, 343, 344).
func TestATRV0340_OpenCodeMultiRepoLaunchesInTicketWorktree(t *testing.T) {
	code := filepath.Join(t.TempDir(), "api", ".worktrees", "t1")
	if err := os.MkdirAll(code, 0o700); err != nil {
		t.Fatal(err)
	}
	c := openCodeConfig(t, &RoleCwd{Template: "{worktree}", Fallback: CwdFallbackRefuse})
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("cwd role refused: %v", err)
	}
	var asked []string
	obs := Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}, TicketWorktree: func(local string) string {
		asked = append(asked, local)
		return code
	}}
	e, lines := launchCwdRecord(t, c, obs)
	want := []string{physical(t, code), code, `{"mcp":{"corvint":{"type":"local","command":["corvint-mcp","--root","` + code + `"]}}}`, "run", "--root", code}
	if len(lines) < len(want) || strings.Join(lines[:len(want)], "\n") != strings.Join(want, "\n") {
		t.Fatalf("recorded %q, want prefix %q", lines, want)
	}
	if e.Detail["cwd"] != code || e.Detail["cwdSource"] != CwdSourceTicketWorktree || len(asked) == 0 || asked[0] != "t1" {
		t.Fatalf("launched detail %v, lookups %v", e.Detail, asked)
	}
}

// TestATRV0342_UnresolvedWorktreeFallsBackOrRefuses: with no worktree,
// fallback WORK_ROOT launches in workRoot and records FALLBACK_WORK_ROOT;
// fallback REFUSE launches nothing and emits launch-failed with code
// CWD_UNRESOLVED and source NONE.
func TestATRV0342_UnresolvedWorktreeFallsBackOrRefuses(t *testing.T) {
	missing := func(string) string { return "/nonexistent/corvint-gh1754" }
	obs := Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}, TicketWorktree: missing}

	c := openCodeConfig(t, &RoleCwd{Template: "{worktree}", Fallback: CwdFallbackWorkRoot})
	e, lines := launchCwdRecord(t, c, obs)
	if lines[0] != physical(t, c.WorkRoot) || lines[1] != c.WorkRoot || e.Detail["cwd"] != c.WorkRoot || e.Detail["cwdSource"] != CwdSourceWorkRoot {
		t.Fatalf("fallback recorded %q detail %v", lines, e.Detail)
	}

	c = openCodeConfig(t, &RoleCwd{Template: "{worktree}", Fallback: CwdFallbackRefuse})
	q := &fakeQueue{obs: obs}
	d := openPacket(t, q, c)
	tick(t, d)
	if len(d.ledger.Workers) != 0 {
		t.Fatalf("a refused cwd launched %d workers", len(d.ledger.Workers))
	}
	failed := eventsOf(t, d, "launch-failed")
	if len(failed) != 1 || failed[0].Detail["code"] != CwdUnresolved || failed[0].Detail["cwdSource"] != CwdSourceNone || failed[0].Detail["cwd"] != "" || !strings.Contains(failed[0].Message, CwdUnresolved) {
		t.Fatalf("launch-failed %+v", failed)
	}

	// The refusal cools the ticket down like any pre-spawn failure; once the
	// worktree exists and the cooldown has passed, the next tick launches.
	now := time.Now()
	d.Now = func() time.Time { return now }
	code := t.TempDir()
	q.obs.TicketWorktree = func(string) string { return code }
	tick(t, d)
	if len(d.ledger.Workers) != 0 {
		t.Fatalf("a cooling-down ticket launched %d workers", len(d.ledger.Workers))
	}
	now = now.Add(time.Duration(c.TickSeconds) * time.Second * 11)
	tick(t, d)
	if len(d.ledger.Workers) != 1 {
		t.Fatalf("after the cooldown %d workers launched, want 1", len(d.ledger.Workers))
	}
	if launched := eventsOf(t, d, "launched"); len(launched) != 1 || launched[0].Detail["cwd"] != code || launched[0].Detail["cwdSource"] != CwdSourceTicketWorktree {
		t.Fatalf("retried launch %+v", launched)
	}
}

// TestATRV0344_CwdIsRecheckedAsRendered: {cwd} is a store-derived argv value
// under the ATR-V0-133 recheck, so a resolved directory whose name is an
// interpreter fails the launch-time check, and an ordinary one passes.
func TestATRV0344_CwdIsRecheckedAsRendered(t *testing.T) {
	argv := []string{"/usr/local/bin/agent", "{cwd}", "{prompt}"}
	if renderedArgvSafe(argv, map[string]string{"{cwd}": "/tmp/repos/python"}) == nil {
		t.Fatal("a {cwd} rendered as an interpreter passed the recheck")
	}
	if err := renderedArgvSafe(argv, map[string]string{"{cwd}": "/tmp/repos/api"}); err != nil {
		t.Fatalf("an ordinary {cwd} failed the recheck: %v", err)
	}
	c := testConfig(t, "exit 0")
	c.Hosts["sh"] = Host{Argv: argv}
	c.Roles[0].CarryOver = &CarryOver{}
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("carryOver host argv with {cwd} refused: %v", err)
	}
}

// TestATRV0340_NoCwdKeepsWorkRoot: a role without cwd starts in workRoot,
// renders {cwd} as workRoot, sets no CORVINT_DISPATCH_CWD and records no
// cwd detail, as before.
func TestATRV0340_NoCwdKeepsWorkRoot(t *testing.T) {
	c := openCodeConfig(t, nil)
	e, lines := launchCwdRecord(t, c, Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}})
	if lines[0] != physical(t, c.WorkRoot) || lines[1] != "UNSET" || lines[5] != c.WorkRoot {
		t.Fatalf("recorded %q", lines)
	}
	if _, ok := e.Detail["cwd"]; ok {
		t.Fatalf("launched detail %v", e.Detail)
	}
	if _, ok := e.Detail["cwdSource"]; ok {
		t.Fatalf("launched detail %v", e.Detail)
	}
}

// TestATRV0341_WorktreeSourcesInOrder: {worktree} comes from the live
// attempt, then the policy ticket worktree, then the single declared label;
// a missing directory, a non-live attempt, two labels or an escaping
// relative label resolve nothing.
func TestATRV0341_WorktreeSourcesInOrder(t *testing.T) {
	root := t.TempDir()
	dirs := map[string]string{}
	for _, n := range []string{"live", "policy", "label"} {
		dirs[n] = filepath.Join(root, n)
		if err := os.Mkdir(dirs[n], 0o700); err != nil {
			t.Fatal(err)
		}
	}
	id := "ticket:a:q:t1"
	c := &RoleCwd{Template: "{worktree}", Fallback: CwdFallbackRefuse, Label: "worktree="}
	for _, tc := range []struct {
		name     string
		attempts []Attempt
		policy   string
		labels   []string
		want     string
		source   string
	}{
		{"live first", []Attempt{{Ticket: id, Live: true, Worktree: dirs["live"]}}, dirs["policy"], []string{"worktree=label"}, dirs["live"], CwdSourceLiveAttempt},
		{"ended attempt skipped", []Attempt{{Ticket: id, Live: false, Worktree: dirs["live"]}}, dirs["policy"], nil, dirs["policy"], CwdSourceTicketWorktree},
		{"missing policy worktree", nil, filepath.Join(root, "gone"), []string{"worktree=label"}, dirs["label"], CwdSourceTicketLabel},
		{"absolute label", nil, "", []string{"worktree=" + dirs["label"]}, dirs["label"], CwdSourceTicketLabel},
		{"two labels", nil, "", []string{"worktree=label", "worktree=live"}, "", CwdSourceNone},
		{"empty then named label", nil, "", []string{"worktree=", "worktree=label"}, "", CwdSourceNone},
		{"escaping label", nil, "", []string{"worktree=../x"}, "", CwdSourceNone},
		{"nothing", nil, "", nil, "", CwdSourceNone},
	} {
		obs := &Observation{Attempts: tc.attempts}
		if tc.policy != "" {
			p := tc.policy
			obs.TicketWorktree = func(string) string { return p }
		}
		tk := ticket("t1", "P1", 1, tc.labels...)
		got, source := ticketWorktree(obs, c, root, id, &tk)
		if got != tc.want || source != tc.source {
			t.Fatalf("%s: %q %s, want %q %s", tc.name, got, source, tc.want, tc.source)
		}
	}
	// A template suffix applies to the resolved worktree; a rendered path
	// that does not exist falls back.
	d := &Dispatcher{Program: "p", Config: &Config{WorkRoot: root}}
	obs := &Observation{Tickets: []Ticket{ticket("t1", "P1", 1, "worktree=label")}}
	role := &Role{Name: "impl", Cwd: &RoleCwd{Template: "{worktree}/svc", Fallback: CwdFallbackWorkRoot, Label: "worktree="}}
	if got, source, err := d.launchCwd(obs, role, id, "t1"); got != root || source != CwdSourceWorkRoot || err != nil {
		t.Fatalf("missing suffix: %q %s %v", got, source, err)
	}
	if err := os.Mkdir(filepath.Join(dirs["label"], "svc"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, source, err := d.launchCwd(obs, role, id, "t1"); got != filepath.Join(dirs["label"], "svc") || source != CwdSourceTicketLabel || err != nil {
		t.Fatalf("suffix: %q %s %v", got, source, err)
	}
	role.Cwd = &RoleCwd{Template: "{workRoot}", Fallback: CwdFallbackRefuse}
	if got, source, err := d.launchCwd(obs, role, id, "t1"); got != root || source != CwdSourceTemplate || err != nil {
		t.Fatalf("plain template: %q %s %v", got, source, err)
	}
}

// TestATRV0340_CwdConfigIsValidated: the template must use only the cwd
// placeholders and render a clean absolute path, the fallback is closed, a
// label needs {worktree}, and a lane role cannot declare cwd.
func TestATRV0340_CwdConfigIsValidated(t *testing.T) {
	for _, tc := range []struct {
		cwd  RoleCwd
		want string
	}{
		{RoleCwd{Template: "", Fallback: CwdFallbackRefuse}, "cwd template must be"},
		{RoleCwd{Template: "{worktree}/{slot}", Fallback: CwdFallbackRefuse}, "cwd template holds {slot}"},
		{RoleCwd{Template: "{ticketLocal}", Fallback: CwdFallbackRefuse}, "clean absolute path"},
		{RoleCwd{Template: "{worktree}/../x", Fallback: CwdFallbackRefuse}, "clean absolute path"},
		{RoleCwd{Template: "{worktree}", Fallback: "workRoot"}, "cwd fallback must be"},
		{RoleCwd{Template: "{workRoot}/x", Fallback: CwdFallbackRefuse, Label: "worktree="}, "needs a template that holds {worktree}"},
	} {
		c := testConfig(t, "exit 0")
		cwd := tc.cwd
		c.Roles[0].Cwd = &cwd
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%+v: %v, want %q", tc.cwd, err, tc.want)
		}
	}
	c := testConfig(t, "exit 0")
	c.Roles[0].Cwd = &RoleCwd{Template: "{workRoot}/repos/{ticketLocal}", Fallback: CwdFallbackWorkRoot}
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("valid cwd refused: %v", err)
	}
	c.Roles[0].Match, c.Roles[0].Lane = nil, &Lane{Pool: "p"}
	raw, _ = json.Marshal(c)
	if _, err := DecodeConfig(raw); err == nil || !strings.Contains(err.Error(), "cwd applies to ticket roles only") {
		t.Fatalf("a lane role with cwd: %v", err)
	}
}
