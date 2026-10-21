//go:build darwin || linux

package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ATR-V0-133: with carryOver on, argv elements after the first may hold any
// argv placeholder, store-derived ones included; {prompt} spliced into
// another element, a placeholder in argv[0], an interpreter and a
// code-string option stay refused, each named in the refusal.
func TestATRV0133_CarryOverHostArgvPlaceholders(t *testing.T) {
	exe := "/usr/local/bin/agent"
	for name, tc := range map[string]struct {
		argv []string
		want string // "" accepts; otherwise a refusal substring
	}{
		"session title":    {[]string{exe, "--title", "lane {role} {ticketLocal} {worker}", "{prompt}"}, ""},
		"bare ticketLocal": {[]string{exe, "{ticketLocal}", "{prompt}"}, ""},
		"store values":     {[]string{exe, "--meta={ticket}/{state}/{pool}/{member}/{nextStage}", "{prompt}"}, ""},
		"config values":    {[]string{exe, "--model", "{model}", "--root", "{workRoot}", "{prompt}"}, ""},
		"spliced prompt":   {[]string{exe, "--prompt={prompt}"}, "splices {prompt} into argv element 1"},
		"no prompt":        {[]string{exe, "--title", "{role}"}, "never passes {prompt} as a whole argv element"},
		"argv0 template":   {[]string{"/opt/{role}/agent", "{prompt}"}, "argv element 0 holds the placeholder {role}"},
		"interpreter":      {[]string{"/bin/sh", exe, "{prompt}"}, "runs the interpreter /bin/sh"},
		"code option":      {[]string{exe, "-c", "{prompt}"}, "takes the code-string option -c"},
		// Only the literal text is checked at load: a placeholder name is
		// not option text, and its rendered value is checked at launch.
		"option glued to placeholder":  {[]string{exe, "-t{ticketLocal}", "{prompt}"}, ""},
		"bare dash placeholder":        {[]string{exe, "-{ticketLocal}", "{prompt}"}, ""},
		"code option and placeholder":  {[]string{exe, "-c{ticketLocal}", "{prompt}"}, "takes the code-string option -c"},
		"eval option with placeholder": {[]string{exe, "--eval={role}", "{prompt}"}, "takes the code-string option --eval="},
		"interpreter in literal text":  {[]string{exe, "/usr/bin/{role}/python3", "{prompt}"}, "runs the interpreter /usr/bin//python3"},
	} {
		c := testConfig(t, "exit 0")
		c.Hosts["sh"] = Host{Argv: tc.argv}
		c.Roles[0].CarryOver = &CarryOver{}
		if strings.Contains(strings.Join(tc.argv, " "), "{model}") {
			c.Roles[0].Model = "m1"
		}
		raw, _ := json.Marshal(c)
		_, err := DecodeConfig(raw)
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s: refused: %v", name, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%s: err %v, want %q", name, err, tc.want)
		}
	}
	// {operatorNote} keeps the literal-argv rule (ON-V0-011).
	c := testConfig(t, "exit 0")
	c.Hosts["sh"] = Host{Argv: []string{exe, "--title", "{ticketLocal}", "{prompt}"}}
	c.Roles[0].Prompt = "work {operatorNote}"
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err == nil || !strings.Contains(err.Error(), "argv element 2 holds a placeholder other than a whole {prompt}") {
		t.Fatalf("operatorNote host err %v", err)
	}

	values := map[string]string{"{ticketLocal}": "t1", "{role}": "impl", "{prompt}": "-c rm"}
	for _, tc := range []struct {
		argv   []string
		local  string
		unsafe bool
	}{
		{[]string{exe, "--title", "lane {role} {ticketLocal}", "{prompt}"}, "sh", false},
		{[]string{exe, "{ticketLocal}", "{prompt}"}, "t1", false},
		{[]string{exe, "{ticketLocal}", "{prompt}"}, "sh", true},
		{[]string{exe, "{ticketLocal}", "{prompt}"}, "python3.12", true},
		{[]string{exe, "-{ticketLocal}", "{prompt}"}, "x1", false},
		{[]string{exe, "-{ticketLocal}", "{prompt}"}, "ce", true},
		{[]string{exe, "--{ticketLocal}", "{prompt}"}, "eval", true},
		{[]string{exe, "/opt/bin/{ticketLocal}", "{prompt}"}, "node", true},
	} {
		values["{ticketLocal}"] = tc.local
		if err := renderedArgvSafe(tc.argv, values); (err != nil) != tc.unsafe {
			t.Errorf("%v with %s: err %v, want unsafe %v", tc.argv, tc.local, err, tc.unsafe)
		}
	}
}

// lastArgScript saves the host's last argument, the prompt, as evidence.
const lastArgScript = `for a; do last=$a; done; printf '%s' "$last" > "$CORVINT_DISPATCH_EVIDENCE_DIR/prompt.txt"`

// ATR-V0-133: a launch whose rendered argv is safe carries the block with a
// templated title; one whose store-derived value renders an interpreter
// name launches without it and alerts.
// TestATRV0163_LoadMatchesOnlyALiteralProgramName: at load an element whose
// last path segment holds a placeholder is not matched against the
// interpreter list ({ticketLocal}sh is accepted); a literal last segment
// still is, and a code-string option is refused either way (V1-1857).
func TestATRV0163_LoadMatchesOnlyALiteralProgramName(t *testing.T) {
	exe := "/usr/local/bin/agent"
	for _, tc := range []struct {
		argv []string
		want string
	}{
		{[]string{exe, "{ticketLocal}sh", "{prompt}"}, ""},
		{[]string{exe, "--name={ticketLocal}python3", "{prompt}"}, ""},
		{[]string{exe, "{workRoot}/bin/python3", "{prompt}"}, "runs the interpreter /bin/python3"},
		{[]string{exe, "{ticketLocal}/sh", "{prompt}"}, "runs the interpreter /sh"},
		{[]string{exe, "-c{ticketLocal}", "{prompt}"}, "takes the code-string option -c"},
		{[]string{exe, "{workRoot}/tool", "-{model}e", "{prompt}"}, "takes the code-string option -e"},
	} {
		err := promptSafeHost(Host{Argv: tc.argv}, carryArgvPlaceholders)
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%q: err %v, want %q", tc.argv, err, tc.want)
		}
	}
	// The rendered value is still checked at launch.
	if renderedArgvSafe([]string{exe, "{ticketLocal}sh", "{prompt}"}, map[string]string{"{ticketLocal}": ""}) == nil {
		t.Fatal("{ticketLocal}sh rendering sh passed the launch recheck")
	}
	if err := renderedArgvSafe([]string{exe, "{ticketLocal}sh", "{prompt}"}, map[string]string{"{ticketLocal}": "t1"}); err != nil {
		t.Fatalf("t1sh refused at launch: %v", err)
	}
}

// TestATRV0164_HostCheckIsNameBased records the ATR-V0-164 limit: an
// env-prefixed interpreter is refused because env is on the list, while a
// wrapper or renamed program that evaluates a plain argument passes, which
// the operator must not configure (V1-1853).
func TestATRV0164_HostCheckIsNameBased(t *testing.T) {
	for _, tc := range []struct {
		argv []string
		want string
	}{
		{[]string{"/usr/bin/env", "python3", "{prompt}"}, "runs the interpreter /usr/bin/env"},
		{[]string{"/usr/local/bin/agent", "env", "{prompt}"}, "runs the interpreter env"},
		{[]string{"/opt/wrap/run-agent", "{prompt}"}, ""},
		{[]string{"/usr/local/bin/py3", "{prompt}"}, ""},
		{[]string{"/usr/local/bin/agent", "-x", "{prompt}"}, ""},
	} {
		for name, err := range map[string]error{"operatorNote": noteSafeHost(Host{Argv: tc.argv}), "carryOver": promptSafeHost(Host{Argv: tc.argv}, carryArgvPlaceholders)} {
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Errorf("%s %q: err %v, want %q", name, tc.argv, err, tc.want)
			}
		}
	}
}

func TestATRV0133_LaunchRechecksRenderedArgv(t *testing.T) {
	for _, tc := range []struct {
		name, local string
		argv        []string
		carried     bool
	}{
		{"title", "t1", []string{"--title", "lane {role} {ticketLocal} {worker}", "{prompt}"}, true},
		{"interpreter", "sh", []string{"{ticketLocal}", "{prompt}"}, false},
		{"code option", "ce", []string{"-{ticketLocal}", "{prompt}"}, false},
	} {
		q := &fakeQueue{}
		c := testConfig(t, lastArgScript)
		h := c.Hosts["sh"]
		h.Argv = append([]string{h.Argv[0]}, tc.argv...)
		c.Hosts["sh"] = h
		c.Roles[0].CarryOver = &CarryOver{}
		d, now := openCtx(t, q, c)
		q.obs.Tickets = []Ticket{ticket(tc.local, "P1", 1)}
		id := "ticket:a:q:" + tc.local
		noteWith(t, d, &Worker{ID: "w0", Role: "impl", Key: id, Ticket: id}, *now, SessionNoProgress, "earlier words")
		tick(t, d)
		if len(d.ledger.Workers) != 1 {
			t.Fatalf("%s: workers %d", tc.name, len(d.ledger.Workers))
		}
		w := d.ledger.Workers[0]
		killOnCleanup(t, w)
		tickUntil(t, d, func() bool { return d.Running() == 0 })
		raw, err := os.ReadFile(filepath.Join(EvidenceDir(d.dir, w.ID), "prompt.txt"))
		if err != nil {
			t.Fatal(err)
		}
		prompt := string(raw)
		if !strings.HasPrefix(prompt, "work on "+tc.local+" as "+w.ID) || strings.Contains(prompt, "earlier words") != tc.carried {
			t.Fatalf("%s: prompt %q", tc.name, prompt)
		}
		e := launchedOf(t, d, "launched", w.ID)
		var alerted bool
		for _, a := range eventsOf(t, d, "alert") {
			alerted = alerted || strings.Contains(a.Message, "carry-over not offered: host \"sh\" argv element 1, as rendered for this launch")
		}
		if (e.Detail["carryOver"] == "1") != tc.carried || alerted == tc.carried {
			t.Fatalf("%s: launched detail %v, alerted %v", tc.name, e.Detail, alerted)
		}
	}
}

// TestATRV0165_UnsafeArgvAlertsOncePerTicketAndConfig: relaunches of a
// ticket whose rendered argv stays unsafe alert once per applied
// configuration; another ticket or a configuration change alerts again, and
// no launch carries the block (V1-1854).
func TestATRV0165_UnsafeArgvAlertsOncePerTicketAndConfig(t *testing.T) {
	q := &fakeQueue{}
	c := testConfig(t, lastArgScript)
	h := c.Hosts["sh"]
	h.Argv = append([]string{h.Argv[0]}, "{ticketLocal}", "{prompt}")
	c.Hosts["sh"] = h
	c.Roles[0].CarryOver = &CarryOver{}
	d, now := openCtx(t, q, c)
	alerts := func() int {
		n := 0
		for _, a := range eventsOf(t, d, "alert") {
			if strings.Contains(a.Message, "carry-over not offered") {
				n++
			}
		}
		return n
	}
	seq := 0
	launch := func(local string, want int) {
		t.Helper()
		seq++
		q.obs.Tickets = []Ticket{ticket(local, "P1", 1)}
		id := "ticket:a:q:" + local
		noteWith(t, d, &Worker{ID: "prior" + strconv.Itoa(seq), Role: "impl", Key: id, Ticket: id}, *now, SessionNoProgress, "earlier words")
		tickUntil(t, d, func() bool { return len(d.ledger.Workers) == 1 })
		w := d.ledger.Workers[0]
		killOnCleanup(t, w)
		tickUntil(t, d, func() bool { return d.Running() == 0 })
		if e := launchedOf(t, d, "launched", w.ID); e.Detail["carryOver"] == "1" {
			t.Fatalf("launch %d carried the block: %v", seq, e.Detail)
		}
		if got := alerts(); got != want {
			t.Fatalf("launch %d (%s): %d unsafe-argv alerts, want %d", seq, local, got, want)
		}
		*now = now.Add(2 * time.Hour)
	}
	launch("sh", 1)
	launch("sh", 1)
	launch("bash", 2)
	d.configSha256 = "another-applied-config"
	launch("bash", 3)
}
