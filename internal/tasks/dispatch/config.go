// Package dispatch is the CAL-V0-052..058 continuous dispatcher: a
// deterministic roster over native queue state that launches, supervises and
// heals independent host worker processes. It owns only its own state
// directory; every queue change goes through the existing lease transactions.
package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	ConfigProfile = "taskman-dispatch/0"
	StateProfile  = "taskman-dispatch-state/0"
	EventProfile  = "taskman-dispatch-event/0"
	MaxConfig     = 256 << 10
)

// Config is the closed taskman-dispatch/0 operator configuration.
type Config struct {
	PoolSweep        *PoolSweepConfig `json:"poolSweep,omitempty"`
	Profile          string           `json:"profile"`
	StateDir         string           `json:"stateDir"`
	WorkRoot         string           `json:"workRoot"`
	TickSeconds      int              `json:"tickSeconds"`
	GlobalCap        int              `json:"globalCap"`
	KillGraceSeconds int              `json:"killGraceSeconds"`
	Hosts            map[string]Host  `json:"hosts"`
	WorkState        *WorkState       `json:"workState,omitempty"`
	Roles            []Role           `json:"roles"`
	Pinned           []string         `json:"pinned,omitempty"`
	Backoff          Backoff          `json:"backoff"`
	Heal             Heal             `json:"heal"`
	// Pressure is the optional CAL-V0-068 host-pressure launch throttle.
	Pressure *PressureConfig `json:"pressure,omitempty"`
}

// Host is one worker runtime. argv[0] is an absolute executable; every argv
// and env value may use the placeholders in Placeholders.
type Host struct {
	Argv          []string          `json:"argv"`
	Env           map[string]string `json:"env,omitempty"`
	IdleIgnore    []string          `json:"idleIgnore,omitempty"`
	ActivityPaths []string          `json:"activityPaths,omitempty"`
}

// WorkState reads a program-defined per-ticket state. Kind status-line reads
// the first "key: value" line of a per-ticket file; kind command runs argv
// once per tick and expects one JSON object of ticket ID to state.
type WorkState struct {
	Kind string   `json:"kind"`
	Path string   `json:"path,omitempty"`
	Key  string   `json:"key,omitempty"`
	Argv []string `json:"argv,omitempty"`
}

// Role is one roster rule. Exactly one of Match (ticket work) and Lane
// (pool member work) is present.
type Role struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Cap         int    `json:"cap"`
	Priority    int    `json:"priority"`
	Match       *Match `json:"match,omitempty"`
	Lane        *Lane  `json:"lane,omitempty"`
	Prompt      string `json:"prompt"`
	IdleSeconds int    `json:"idleSeconds"`
	WallSeconds int    `json:"wallSeconds"`
}

// Match is a conjunction; an empty list matches anything. Statuses defaults
// to OPEN. States and ExcludeStates name work states (NONE when the reader
// found none); UNKNOWN never matches a rule that names states. IDGlob matches
// the local ticket ID. A ticket with any live attempt is never a candidate;
// an expired lease becomes free only after heal reaps it.
type Match struct {
	Labels        []string `json:"labels,omitempty"`
	Kinds         []string `json:"kinds,omitempty"`
	IDGlob        string   `json:"idGlob,omitempty"`
	States        []string `json:"states,omitempty"`
	ExcludeStates []string `json:"excludeStates,omitempty"`
	Statuses      []string `json:"statuses,omitempty"`
	PlanSelected  bool     `json:"planSelected,omitempty"`
}

// Lane selects pool members in one native pool state (QUARANTINED by default).
type Lane struct {
	Pool   string   `json:"pool"`
	States []string `json:"states,omitempty"`
}

type Backoff struct {
	CooldownSeconds int `json:"cooldownSeconds"`
	ParkAfter       int `json:"parkAfter"`
}

type Heal struct {
	Handoff bool `json:"handoff"`
	Reap    bool `json:"reap"`
}

// Placeholders are the only substitutions in argv, env and prompts.
var Placeholders = []string{"{program}", "{role}", "{slot}", "{worker}", "{holder}", "{ticket}", "{ticketLocal}", "{state}", "{pool}", "{member}", "{workRoot}", "{prompt}"}

var (
	namePattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)
	placeholder  = regexp.MustCompile(`\{[A-Za-z]+\}`)
	envKeyFormat = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
)

// ValidName is the bound on program and role names; worker IDs and holder
// labels are built from them and must stay inside a 64-byte label.
func ValidName(s string) bool { return namePattern.MatchString(s) }

// DecodeConfig parses and validates closed configuration bytes.
func DecodeConfig(raw []byte) (*Config, error) {
	if len(raw) > MaxConfig {
		return nil, fmt.Errorf("dispatch config exceeds %d bytes", MaxConfig)
	}
	if !strictSweepConfig(raw) {
		return nil, fmt.Errorf("dispatch config: malformed poolSweep JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var c Config
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("dispatch config: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("dispatch config: trailing input")
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) validate() error {
	fail := func(f string, a ...any) error { return fmt.Errorf("dispatch config: "+f, a...) }
	if c.PoolSweep != nil && (c.PoolSweep.TimeoutSeconds < 1 || c.PoolSweep.TimeoutSeconds > 1800 || c.PoolSweep.IntervalSeconds < 1 || c.PoolSweep.IntervalSeconds > 3600) {
		return fail("poolSweep requires timeoutSeconds 1..1800 and intervalSeconds 1..3600")
	}
	if c.Profile != ConfigProfile {
		return fail("profile must be %s", ConfigProfile)
	}
	if !clean(c.StateDir) || !clean(c.WorkRoot) {
		return fail("stateDir and workRoot must be clean absolute paths")
	}
	if c.TickSeconds < 1 || c.TickSeconds > 3600 || c.GlobalCap < 1 || c.GlobalCap > 64 || c.KillGraceSeconds < 1 || c.KillGraceSeconds > 120 {
		return fail("tickSeconds 1..3600, globalCap 1..64 and killGraceSeconds 1..120 are required")
	}
	if c.Backoff.CooldownSeconds < 0 || c.Backoff.CooldownSeconds > 86400 || c.Backoff.ParkAfter < 1 || c.Backoff.ParkAfter > 100 {
		return fail("backoff needs cooldownSeconds 0..86400 and parkAfter 1..100")
	}
	if len(c.Hosts) == 0 || len(c.Hosts) > 8 {
		return fail("hosts needs 1..8 entries")
	}
	for name, h := range c.Hosts {
		if !ValidName(name) || len(h.Argv) == 0 || len(h.Argv) > 64 || !clean(h.Argv[0]) || len(h.Env) > 64 || len(h.IdleIgnore) > 16 || len(h.ActivityPaths) > 16 {
			return fail("host %q needs a short name, 1..64 argv with an absolute executable, and bounded env/idleIgnore/activityPaths", name)
		}
		prompt := 0
		for _, a := range h.Argv {
			if err := placeholdersKnown(a); err != nil {
				return fail("host %q argv: %v", name, err)
			}
			prompt += strings.Count(a, "{prompt}")
		}
		if prompt > 1 {
			return fail("host %q argv uses {prompt} more than once", name)
		}
		for k, v := range h.Env {
			if !envKeyFormat.MatchString(k) || strings.HasPrefix(k, "CORVINT_DISPATCH_") {
				return fail("host %q env key %q is invalid or reserved", name, k)
			}
			if err := placeholdersKnown(v); err != nil || strings.Contains(v, "{prompt}") {
				return fail("host %q env %s: unknown placeholder or {prompt}", name, k)
			}
		}
		for _, p := range h.ActivityPaths {
			if !clean(p) {
				return fail("host %q activityPaths must be clean absolute paths", name)
			}
		}
	}
	if w := c.WorkState; w != nil {
		switch w.Kind {
		case "status-line":
			if !clean(strings.ReplaceAll(w.Path, "{ticketLocal}", "t")) || !strings.Contains(w.Path, "{ticketLocal}") || placeholder.ReplaceAllString(strings.ReplaceAll(w.Path, "{ticketLocal}", ""), "") != strings.ReplaceAll(w.Path, "{ticketLocal}", "") || w.Key == "" || len(w.Key) > 64 || strings.ContainsAny(w.Key, ":\n") || len(w.Argv) != 0 {
				return fail("status-line workState needs an absolute path with {ticketLocal} and a key")
			}
		case "command":
			if len(w.Argv) == 0 || len(w.Argv) > 64 || !clean(w.Argv[0]) || w.Path != "" || w.Key != "" {
				return fail("command workState needs argv with an absolute executable")
			}
		default:
			return fail("workState kind must be status-line or command")
		}
	}
	if len(c.Roles) == 0 || len(c.Roles) > 32 {
		return fail("roles needs 1..32 entries")
	}
	seen := map[string]bool{}
	for _, r := range c.Roles {
		if !ValidName(r.Name) || seen[r.Name] {
			return fail("role name %q is invalid or repeated", r.Name)
		}
		seen[r.Name] = true
		if _, ok := c.Hosts[r.Host]; !ok {
			return fail("role %s names unknown host %q", r.Name, r.Host)
		}
		if r.Cap < 1 || r.Cap > 64 || r.Priority < 0 || r.Priority > 1000 || r.IdleSeconds < 30 || r.IdleSeconds > 86400 || r.WallSeconds < 60 || r.WallSeconds > 7*86400 {
			return fail("role %s needs cap 1..64, priority 0..1000, idleSeconds 30..86400 and wallSeconds 60..604800", r.Name)
		}
		if (r.Match == nil) == (r.Lane == nil) {
			return fail("role %s needs exactly one of match and lane", r.Name)
		}
		if r.Prompt == "" || len(r.Prompt) > 64<<10 {
			return fail("role %s prompt must be 1..65536 bytes", r.Name)
		}
		if err := placeholdersKnown(r.Prompt); err != nil || strings.Contains(r.Prompt, "{prompt}") {
			return fail("role %s prompt: unknown placeholder or {prompt}", r.Name)
		}
		if m := r.Match; m != nil {
			if m.IDGlob != "" {
				if _, err := filepath.Match(m.IDGlob, ""); err != nil {
					return fail("role %s idGlob: %v", r.Name, err)
				}
			}
			if len(m.States)+len(m.ExcludeStates) > 0 && c.WorkState == nil {
				return fail("role %s matches work states without a workState reader", r.Name)
			}
		}
		if l := r.Lane; l != nil && (l.Pool == "" || len(l.Pool) > 64) {
			return fail("role %s lane needs a pool", r.Name)
		}
	}
	if len(c.Pinned) > 256 {
		return fail("pinned is limited to 256 tickets")
	}
	if p := c.Pressure; p != nil {
		if err := p.Validate(); err != nil {
			return fail("%v", err)
		}
		for _, r := range p.ExemptRoles {
			if !seen[r] {
				return fail("pressure exempt role %q is not a configured role", r)
			}
		}
	}
	return nil
}

func clean(p string) bool { return filepath.IsAbs(p) && filepath.Clean(p) == p }

func placeholdersKnown(s string) error {
	for _, p := range placeholder.FindAllString(s, -1) {
		if !contains(Placeholders, p) {
			return fmt.Errorf("unknown placeholder %s", p)
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Render substitutes placeholders in one pass; values never re-expand.
func Render(s string, values map[string]string) string {
	pairs := make([]string, 0, 2*len(values))
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		pairs = append(pairs, k, values[k])
	}
	return strings.NewReplacer(pairs...).Replace(s)
}
