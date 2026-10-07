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
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
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
	// InfrastructureRetry is the optional ESC-V0-007 infrastructure retry
	// policy. Without it an infrastructure session is ordinary no-progress.
	InfrastructureRetry *InfraRetryConfig `json:"infrastructureRetry,omitempty"`
}

// InfraRetryConfig bounds automatic retries of a ticket's infrastructure
// sessions per acceptance revision. An absent member takes its default.
type InfraRetryConfig struct {
	MaxRetries         *int `json:"maxRetries,omitempty"`
	CooldownSeconds    *int `json:"cooldownSeconds,omitempty"`
	MaxCooldownSeconds *int `json:"maxCooldownSeconds,omitempty"`
}

// Limits returns maxRetries, cooldownSeconds and maxCooldownSeconds with
// the enabled defaults 3, 30 and 300.
func (r *InfraRetryConfig) Limits() (maxRetries, cooldown, maxCooldown int) {
	maxRetries, cooldown, maxCooldown = 3, 30, 300
	if r.MaxRetries != nil {
		maxRetries = *r.MaxRetries
	}
	if r.CooldownSeconds != nil {
		cooldown = *r.CooldownSeconds
	}
	if r.MaxCooldownSeconds != nil {
		maxCooldown = *r.MaxCooldownSeconds
	}
	return maxRetries, cooldown, maxCooldown
}

// RetryCooldown is the cooldown before retry ordinal n (1-based):
// min(maxCooldown, cooldown*2^(n-1)), saturating.
func RetryCooldown(cooldown, maxCooldown, n int) time.Duration {
	wait := cooldown
	for i := 1; i < n && wait < maxCooldown; i++ {
		wait *= 2
	}
	return time.Duration(min(wait, maxCooldown)) * time.Second
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
	// Model, Escalate and DeescalateOnProgress are the CAL-V0-057
	// escalation ladder. The role's host must render {model}.
	Model                string `json:"model,omitempty"`
	Escalate             []Tier `json:"escalate,omitempty"`
	DeescalateOnProgress *bool  `json:"deescalateOnProgress,omitempty"`
}

// Tier is one escalation step: from After consecutive no-progress sessions
// on a ticket, the role launches Model. Cap 0 leaves only the role cap.
type Tier struct {
	After int    `json:"after"`
	Model string `json:"model"`
	Cap   int    `json:"cap,omitempty"`
	Notes string `json:"notes,omitempty"`
}

// Deescalates reports whether progress returns the role to its base model
// (the default).
func (r *Role) Deescalates() bool { return r.DeescalateOnProgress == nil || *r.DeescalateOnProgress }

// TierFor is the ladder tier for a no-progress streak: 0 is the base model.
func (r *Role) TierFor(streak int) int {
	tier := 0
	for i, t := range r.Escalate {
		if streak >= t.After {
			tier = i + 1
		}
	}
	return tier
}

// ModelAt is the model the role launches at a tier.
func (r *Role) ModelAt(tier int) string {
	if tier < 1 || tier > len(r.Escalate) {
		return r.Model
	}
	return r.Escalate[tier-1].Model
}

// Escalates reports whether any role has a ladder.
func (c *Config) Escalates() bool {
	for _, r := range c.Roles {
		if len(r.Escalate) > 0 {
			return true
		}
	}
	return false
}

// TicketPools is the sorted set of pools the ticket roles' workers claim:
// the only pools whose tickets the dispatcher's plan may select (CAL-V0-097).
// It is empty, never nil, when no role names a pool. A disabled (cap 0)
// role claims nothing, so its pool is not planned (CAL-V0-128).
func (c *Config) TicketPools() []string {
	out := []string{}
	for _, r := range c.Roles {
		if r.Cap > 0 && r.Match != nil && r.Match.Pool != "" && !slices.Contains(out, r.Match.Pool) {
			out = append(out, r.Match.Pool)
		}
	}
	slices.Sort(out)
	return out
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
	// Pool is the pool the role's workers claim with `--pool {pool}`. A
	// ticket that requires a pool matches only a role naming that pool, and
	// a role naming a pool matches only its tickets (CAL-V0-029, CAL-V0-097).
	Pool string `json:"pool,omitempty"`
	// Gates is a conjunction of ERG-V0-009 native external review predicates.
	Gates []GateMatch `json:"gates,omitempty"`
}

// GateMatch requires one gate's routing state (Ticket.GateState) to be one
// of States: PASS, RETURN, RESUBMITTED or NONE.
type GateMatch struct {
	Gate   string   `json:"gate"`
	States []string `json:"states"`
}

// Lane selects pool members in one native pool state (QUARANTINED by default).
// MinAgeSeconds, when positive, admits a member only after the dispatcher has
// observed it continuously in the same matching state episode that long
// (CAL-V0-129).
type Lane struct {
	Pool          string   `json:"pool"`
	States        []string `json:"states,omitempty"`
	MinAgeSeconds int      `json:"minAgeSeconds,omitempty"`
}

type Backoff struct {
	CooldownSeconds int `json:"cooldownSeconds"`
	ParkAfter       int `json:"parkAfter"`
}

type Heal struct {
	Handoff bool `json:"handoff"`
	Reap    bool `json:"reap"`
	// ExitRecovery (CAL-V0-104, default true) retries a refused hand-off of
	// an ended worker's attempt with backoff and reaps it once its lease
	// expires, before reporting needs-owner. It acts only with Handoff.
	ExitRecovery *bool `json:"exitRecovery,omitempty"`
}

// ExitRecoveryOn reports whether CAL-V0-104 recovery applies; an absent
// switch is on.
func (h Heal) ExitRecoveryOn() bool { return h.Handoff && (h.ExitRecovery == nil || *h.ExitRecovery) }

// Placeholders are the only substitutions in argv, env and prompts.
// {operatorNote} renders untrusted operator prose, so only a role prompt may
// use it (ON-V0-011).
var Placeholders = []string{"{program}", "{role}", "{slot}", "{worker}", "{holder}", "{ticket}", "{ticketLocal}", "{state}", "{pool}", "{member}", "{workRoot}", "{prompt}", "{model}", "{nextStage}", "{operatorNote}"}

// operatorNotePlaceholder is refused in host argv, env and activity paths.
const operatorNotePlaceholder = "{operatorNote}"

var (
	namePattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)
	placeholder  = regexp.MustCompile(`\{[A-Za-z]+\}`)
	envKeyFormat = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
	modelFormat  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+-]{0,127}$`)
)

// ValidName is the bound on program and role names; worker IDs and holder
// labels are built from them and must stay inside a 64-byte label.
func ValidName(s string) bool { return namePattern.MatchString(s) }

// DecodeConfig parses and validates closed configuration bytes.
func DecodeConfig(raw []byte) (*Config, error) {
	if len(raw) > MaxConfig {
		return nil, fmt.Errorf("dispatch config exceeds %d bytes", MaxConfig)
	}
	if err := wire.RawProfileVersion("/profile", raw, ConfigProfile); err != nil {
		return nil, err
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
	if r := c.InfrastructureRetry; r != nil {
		n, cool, maxCool := r.Limits()
		if n < 0 || n > 10 || cool < 1 || cool > 3600 || maxCool < cool || maxCool > 86400 {
			return fail("infrastructureRetry needs maxRetries 0..10, cooldownSeconds 1..3600 and maxCooldownSeconds cooldownSeconds..86400")
		}
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
			if err := placeholdersKnown(a); err != nil || strings.Contains(a, operatorNotePlaceholder) {
				return fail("host %q argv: unknown placeholder or {operatorNote}", name)
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
			if err := placeholdersKnown(v); err != nil || strings.Contains(v, "{prompt}") || strings.Contains(v, operatorNotePlaceholder) {
				return fail("host %q env %s: unknown placeholder, {prompt} or {operatorNote}", name, k)
			}
		}
		for _, p := range h.ActivityPaths {
			if !clean(p) || strings.Contains(p, operatorNotePlaceholder) {
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
		// CAL-V0-128: cap 0 keeps a configured role but launches nothing.
		if r.Cap < 0 || r.Cap > 64 || r.Priority < 0 || r.Priority > 1000 || r.IdleSeconds < 30 || r.IdleSeconds > 86400 || r.WallSeconds < 60 || r.WallSeconds > 7*86400 {
			return fail("role %s needs cap 0..64, priority 0..1000, idleSeconds 30..86400 and wallSeconds 60..604800", r.Name)
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
		if strings.Contains(r.Prompt, operatorNotePlaceholder) {
			if err := noteSafeHost(c.Hosts[r.Host]); err != nil {
				return fail("role %s uses {operatorNote} but host %q %v", r.Name, r.Host, err)
			}
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
			if err := validGates(m.Gates); err != nil {
				return fail("role %s gates: %v", r.Name, err)
			}
			if len(m.Pool) > 64 {
				return fail("role %s match pool is longer than 64 bytes", r.Name)
			}
		}
		if l := r.Lane; l != nil && (l.Pool == "" || len(l.Pool) > 64) {
			return fail("role %s lane needs a pool", r.Name)
		}
		if l := r.Lane; l != nil && (l.MinAgeSeconds < 0 || l.MinAgeSeconds > 7*86400) {
			return fail("role %s lane minAgeSeconds must be 0..604800", r.Name)
		}
		if err := c.validateLadder(r); err != nil {
			return fail("role %s %v", r.Name, err)
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

// validateLadder refuses a model the role's host cannot deliver: a role
// with a model needs a host that renders {model}, and such a host serves
// only roles that name one.
func (c *Config) validateLadder(r Role) error {
	uses := false
	h := c.Hosts[r.Host]
	for _, a := range h.Argv {
		uses = uses || strings.Contains(a, "{model}")
	}
	for _, v := range h.Env {
		uses = uses || strings.Contains(v, "{model}")
	}
	// An activity path can name the model but does not deliver it.
	renders := uses
	for _, p := range h.ActivityPaths {
		renders = renders || strings.Contains(p, "{model}")
	}
	if r.Model == "" {
		if len(r.Escalate) > 0 || r.DeescalateOnProgress != nil {
			return fmt.Errorf("escalate and deescalateOnProgress need a base model")
		}
		if renders || strings.Contains(r.Prompt, "{model}") {
			return fmt.Errorf("uses a {model} placeholder but names no model")
		}
		return nil
	}
	if !modelFormat.MatchString(r.Model) {
		return fmt.Errorf("model must match %s", modelFormat)
	}
	if !uses {
		return fmt.Errorf("names a model but host %q renders no {model}: unsupported model configuration", r.Host)
	}
	if len(r.Escalate) == 0 {
		if r.DeescalateOnProgress != nil {
			return fmt.Errorf("deescalateOnProgress needs escalate")
		}
		return nil
	}
	if r.Lane != nil || len(r.Escalate) > 8 {
		return fmt.Errorf("escalate needs a match role and at most 8 tiers")
	}
	prev, after := r.Model, 0
	for i, t := range r.Escalate {
		if t.After <= after || t.After > 100 {
			return fmt.Errorf("escalate[%d].after must be 1..100 and strictly increasing", i)
		}
		if !modelFormat.MatchString(t.Model) || t.Model == prev {
			return fmt.Errorf("escalate[%d].model must be a valid model that differs from the tier below", i)
		}
		if t.Cap < 0 || t.Cap > r.Cap {
			return fmt.Errorf("escalate[%d].cap must be 0..%d (0 leaves only the role cap)", i, r.Cap)
		}
		if len(t.Notes) > 1024 || strings.IndexFunc(t.Notes, func(c rune) bool { return c < 0x20 || c == 0x7f }) >= 0 {
			return fmt.Errorf("escalate[%d].notes must be at most 1024 bytes without control characters", i)
		}
		prev, after = t.Model, t.After
	}
	return nil
}

func clean(p string) bool { return filepath.IsAbs(p) && filepath.Clean(p) == p }

// ConfigStateDir recovers the state directory from configuration bytes that
// DecodeConfig refused, so `dispatch status` can still read the ledger and
// show a recorded reload refusal (CAL-V0-127). It reads the top-level object
// up to the first syntax error and reports only a single, exactly spelled,
// clean absolute stateDir string; anything ambiguous recovers nothing.
func ConfigStateDir(raw []byte) (string, bool) {
	if len(raw) > MaxConfig {
		return "", false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return "", false
	}
	dir, seen := "", 0
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			break
		}
		if key, _ := t.(string); strings.EqualFold(key, "stateDir") {
			seen++
			v, err := dec.Token()
			s, ok := v.(string)
			if err != nil || !ok || key != "stateDir" {
				return "", false
			}
			dir = s
			continue
		}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			break
		}
	}
	if seen != 1 || !clean(dir) {
		return "", false
	}
	return dir, true
}

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

// validGates bounds gate predicates: at most 16 distinct gate labels, each
// with 1..4 distinct states.
func validGates(gates []GateMatch) error {
	if len(gates) > 16 {
		return fmt.Errorf("at most 16 predicates")
	}
	seen := map[string]bool{}
	for _, g := range gates {
		if _, err := wire.ParseLabel("gate", g.Gate); err != nil || seen[g.Gate] {
			return fmt.Errorf("gate %q is not a label or is repeated", g.Gate)
		}
		seen[g.Gate] = true
		if len(g.States) == 0 || len(g.States) > 4 {
			return fmt.Errorf("gate %s needs 1..4 states", g.Gate)
		}
		states := map[string]bool{}
		for _, s := range g.States {
			if states[s] || !contains([]string{GatePass, GateReturn, GateResubmitted, GateNone}, s) {
				return fmt.Errorf("gate %s state %q is not PASS, RETURN, RESUBMITTED or NONE, or is repeated", g.Gate, s)
			}
			states[s] = true
		}
	}
	return nil
}

// noteSafeHost admits a host for a role prompt carrying {operatorNote}
// (ON-V0-011) only when the rendered prompt reaches it as one whole argv
// element, every other argv element is literal (no placeholder, so the
// checks below see the launched argv), no argv element names a shell or script interpreter or takes a
// code-string option (a single-dash or single-plus cluster containing c or e,
// or --command, --eval, --exec, --execute), and no activity path renders it.
// Untrusted note prose is then passed as data rather than spliced into, or
// offered as, a command string; a host that needs a shell uses a wrapper
// executable. A program that evaluates a plain argument as code remains
// outside what this check can see.
func noteSafeHost(h Host) error {
	if !slices.Contains(h.Argv, "{prompt}") {
		return fmt.Errorf("never passes {prompt}, so the note would be dropped")
	}
	for i, a := range h.Argv {
		if a != "{prompt}" && placeholder.MatchString(a) {
			return fmt.Errorf("argv element %d holds a placeholder other than a whole {prompt}", i)
		}
		if interpreters[strings.TrimRight(filepath.Base(a), "0123456789.")] {
			return fmt.Errorf("runs the interpreter %s", a)
		}
		if i > 0 && codeOption(a) {
			return fmt.Errorf("takes the code-string option %s", a)
		}
	}
	for _, p := range h.ActivityPaths {
		if strings.Contains(p, "{prompt}") {
			return fmt.Errorf("renders {prompt} into an activity path")
		}
	}
	return nil
}

// interpreters are argv names (version suffix trimmed) that run their
// arguments as code, directly or through another program.
var interpreters = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "mksh": true, "fish": true, "csh": true, "tcsh": true, "busybox": true, "env": true, "xargs": true, "eval": true, "exec": true,
	"python": true, "node": true, "deno": true, "bun": true, "perl": true, "ruby": true, "php": true, "lua": true, "osascript": true, "awk": true, "gawk": true, "pwsh": true, "powershell": true, "cmd": true}

func codeOption(a string) bool {
	if name, _, _ := strings.Cut(a, "="); name == "--command" || name == "--eval" || name == "--exec" || name == "--execute" {
		return true
	}
	return len(a) > 1 && (a[0] == '-' || a[0] == '+') && a[1] != '-' && strings.ContainsAny(a[1:], "ce")
}
