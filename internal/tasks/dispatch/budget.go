package dispatch

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"sort"
	"strconv"
	"time"
)

// BudgetWindow is the rolling window of every CAL-V0-155 budget.
const BudgetWindow = 24 * time.Hour

// maxSpendSessions bounds the recorded launch history (CAL-V0-156); the
// oldest records beyond it are dropped and the history marks itself
// truncated.
const maxSpendSessions = 4096

// Budget hold limits and scopes (CAL-V0-155).
const (
	LimitSessions  = "sessionsPerDay"
	LimitTokens    = "tokensPerDay"
	LimitTruncated = "historyTruncated"
	ScopeRole      = "role"
	ScopeTicket    = "ticket"
)

// SpendRecord is the CAL-V0-156 launch history of the rolling window and
// the budget holds in force. It is derived dispatcher state, never a
// native-store input.
type SpendRecord struct {
	Sessions []SpendSession `json:"sessions"`
	// Truncated is the newest launch time of a record dropped by the
	// history cap: until it leaves the window, budgeted scopes are held.
	Truncated time.Time `json:"truncated"`
	// HistoryFrom is when this history began after an adopted older
	// ledger, whose sessions were never recorded; zero when the history is
	// complete since the ledger was created.
	HistoryFrom time.Time    `json:"historyFrom"`
	Held        []BudgetHold `json:"held"`
}

// SpendSession is one launch in the window and its token account so far.
type SpendSession struct {
	Worker   string    `json:"worker"`
	Role     string    `json:"role"`
	Ticket   string    `json:"ticket,omitempty"`
	Launched time.Time `json:"launched"`
	// Usage is RUNNING, KNOWN, PARTIAL or UNKNOWN; Input and Output are
	// the counters observed, meaningful only when Usage is not UNKNOWN.
	Usage  string `json:"usage"`
	Input  uint64 `json:"input"`
	Output uint64 `json:"output"`
}

// BudgetHold is one exhausted budgeted scope that held a launch.
type BudgetHold struct {
	Scope    string    `json:"scope"`
	Name     string    `json:"name"`
	Limit    string    `json:"limit"`
	ResetsAt time.Time `json:"resetsAt"`
}

func (s SpendSession) tokens() uint64 {
	if s.Usage == UsageUnknown {
		return 0
	}
	return s.Input + s.Output
}

// tokenSum is a scope's observed tokens as a 128-bit sum, so up to
// maxSpendSessions sessions of any representable total neither wrap nor
// saturate while the window's oldest sessions are subtracted.
type tokenSum struct{ hi, lo uint64 }

func (t *tokenSum) add(n uint64) {
	var c uint64
	t.lo, c = bits.Add64(t.lo, n, 0)
	t.hi += c
}

func (t *tokenSum) sub(n uint64) {
	var b uint64
	t.lo, b = bits.Sub64(t.lo, n, 0)
	t.hi -= b
}

func (t tokenSum) atLeast(n uint64) bool { return t.hi > 0 || t.lo >= n }

// value is the sum for display, saturated at the largest counter.
func (t tokenSum) value() uint64 {
	if t.hi > 0 {
		return math.MaxUint64
	}
	return t.lo
}

func (r *SpendRecord) validate() error {
	if len(r.Sessions) > maxSpendSessions || len(r.Held) > maxSpendSessions {
		return errors.New("budget history exceeds its bound")
	}
	for i, s := range r.Sessions {
		if s.Worker == "" || !ValidName(s.Role) || s.Launched.IsZero() || !slices.Contains([]string{UsageRunning, UsageKnown, UsagePartial, UsageUnknown}, s.Usage) {
			return fmt.Errorf("budget session %d is malformed", i)
		}
		if s.Usage == UsageUnknown && (s.Input != 0 || s.Output != 0) {
			return fmt.Errorf("budget session %d carries counters for an UNKNOWN total", i)
		}
		if s.Input+s.Output < s.Input {
			return fmt.Errorf("budget session %d overflows", i)
		}
	}
	for i, h := range r.Held {
		if !slices.Contains([]string{ScopeRole, ScopeTicket}, h.Scope) || h.Name == "" || !slices.Contains([]string{LimitSessions, LimitTokens, LimitTruncated}, h.Limit) || h.ResetsAt.IsZero() {
			return fmt.Errorf("budget hold %d is malformed", i)
		}
	}
	return nil
}

// spend returns the ledger's history, creating it.
func (d *Dispatcher) spend() *SpendRecord {
	if d.ledger.Budget == nil {
		d.ledger.Budget = &SpendRecord{Sessions: []SpendSession{}, Held: []BudgetHold{}}
	}
	return d.ledger.Budget
}

// recordSession charges one launch to the history (CAL-V0-156). A launch
// that may have started is charged whether or not it was identified.
func (d *Dispatcher) recordSession(a Assignment, worker, usage string, now time.Time) {
	r := d.spend()
	r.Sessions = append(r.Sessions, SpendSession{Worker: worker, Role: a.Role, Ticket: a.Ticket, Launched: now, Usage: usage})
	if n := len(r.Sessions) - maxSpendSessions; n > 0 {
		for _, s := range r.Sessions[:n] {
			if s.Launched.After(r.Truncated) {
				r.Truncated = s.Launched
			}
		}
		r.Sessions = slices.Delete(r.Sessions, 0, n)
	}
}

// syncSpend prunes sessions launched before the window and copies each
// recorded worker's running usage into its session.
func (d *Dispatcher) syncSpend(now time.Time) {
	r := d.ledger.Budget
	if r == nil {
		return
	}
	cutoff := now.Add(-BudgetWindow)
	r.Sessions = slices.DeleteFunc(r.Sessions, func(s SpendSession) bool { return !s.Launched.After(cutoff) })
	if !r.Truncated.IsZero() && !r.Truncated.After(cutoff) {
		r.Truncated = time.Time{}
	}
	if !r.HistoryFrom.IsZero() && !r.HistoryFrom.After(cutoff) {
		r.HistoryFrom = time.Time{}
	}
	for _, w := range d.ledger.Workers {
		if w.Usage != nil {
			d.settleSession(w.ID, UsageRunning, w.Usage)
		}
	}
}

// settleSession records a worker's usage state and counters in its session.
func (d *Dispatcher) settleSession(worker, state string, u *WorkerUsage) {
	r := d.ledger.Budget
	if r == nil {
		return
	}
	for i := range r.Sessions {
		if s := &r.Sessions[i]; s.Worker == worker {
			s.Usage, s.Input, s.Output = state, 0, 0
			if state != UsageUnknown && u != nil {
				s.Input, s.Output = u.Input, u.Output
			}
			return
		}
	}
}

// spendGate is one roster's CAL-V0-155 budget check: the window's history
// plus this roster's own admissions.
type spendGate struct {
	c       *Config
	now     time.Time
	r       *SpendRecord
	charged map[[2]string]int
	holds   map[[2]string]BudgetHold
}

func (d *Dispatcher) spendGate(now time.Time) *spendGate {
	r := d.ledger.Budget
	if r == nil {
		r = &SpendRecord{}
	}
	return &spendGate{c: d.Config, now: now, r: r, charged: map[[2]string]int{}, holds: map[[2]string]BudgetHold{}}
}

// scopes are the budgeted scopes a launch of a would charge.
func (g *spendGate) scopes(a Assignment) [][2]string {
	var out [][2]string
	if r := g.c.roleNamed(a.Role); r != nil && r.Budget != nil {
		out = append(out, [2]string{ScopeRole, a.Role})
	}
	if g.c.TicketBudget != nil && a.Ticket != "" {
		out = append(out, [2]string{ScopeTicket, a.Ticket})
	}
	return out
}

func (g *spendGate) budget(scope [2]string) *Budget {
	if scope[0] == ScopeTicket {
		return g.c.TicketBudget
	}
	if r := g.c.roleNamed(scope[1]); r != nil {
		return r.Budget
	}
	return nil
}

func (s SpendSession) in(scope [2]string) bool {
	if scope[0] == ScopeTicket {
		return s.Ticket == scope[1]
	}
	return s.Role == scope[1]
}

// budgetHeld names the tickets a budget holds outside the selection window
// (CAL-V0-155): a ticket whose ticket scope is exhausted, or for which every
// enabled ticket role that would match it, were it selected, has an
// exhausted role scope. A held ticket left SELECTED would otherwise keep an
// unspent ticket out of a planSelected role's window until the hold ends.
func (d *Dispatcher) budgetHeld(ts []Ticket, now time.Time) map[string]bool {
	c := d.Config
	budgeted := c.TicketBudget != nil
	for _, r := range c.Roles {
		budgeted = budgeted || r.Budget != nil
	}
	if !budgeted {
		return nil
	}
	g := d.spendGate(now)
	exhausted := func(scope [2]string) bool { _, ok := g.exhausted(scope); return ok }
	var held map[string]bool
	for _, t := range ts {
		hold := c.TicketBudget != nil && exhausted([2]string{ScopeTicket, t.ID})
		if !hold {
			selected := t
			selected.Plan = "SELECTED"
			roles := 0
			hold = true
			for _, r := range c.Roles {
				if r.Match == nil || r.Cap == 0 || !matches(r.Match, selected) {
					continue
				}
				roles++
				if r.Budget == nil || !exhausted([2]string{ScopeRole, r.Name}) {
					hold = false
					break
				}
			}
			hold = hold && roles > 0
		}
		if hold {
			if held == nil {
				held = map[string]bool{}
			}
			held[t.ID] = true
		}
	}
	return held
}

// admits reports whether a may launch, recording the first exhausted
// scope's hold when it may not.
func (g *spendGate) admits(a Assignment) bool {
	for _, scope := range g.scopes(a) {
		if h, held := g.exhausted(scope); held {
			if _, ok := g.holds[scope]; !ok {
				g.holds[scope] = h
			}
			return false
		}
	}
	return true
}

// charge counts an admitted launch against its scopes for the rest of the
// roster.
func (g *spendGate) charge(a Assignment) {
	for _, scope := range g.scopes(a) {
		g.charged[scope]++
	}
}

// exhausted reports the hold of a budgeted scope that may not launch now:
// an incomplete history, its sessions or its observed tokens. ResetsAt is
// the earliest time the window alone releases it; tokens of a session still
// running may move it later.
func (g *spendGate) exhausted(scope [2]string) (BudgetHold, bool) {
	b := g.budget(scope)
	if b == nil {
		return BudgetHold{}, false
	}
	h := BudgetHold{Scope: scope[0], Name: scope[1]}
	cutoff := g.now.Add(-BudgetWindow)
	if g.r.Truncated.After(cutoff) {
		h.Limit, h.ResetsAt = LimitTruncated, g.r.Truncated.Add(BudgetWindow)
		return h, true
	}
	var in []SpendSession
	var tokens tokenSum
	for _, s := range g.r.Sessions {
		if s.in(scope) && s.Launched.After(cutoff) {
			in = append(in, s)
			tokens.add(s.tokens())
		}
	}
	sort.SliceStable(in, func(i, j int) bool { return in[i].Launched.Before(in[j].Launched) })
	if L := b.SessionsPerDay; L > 0 {
		if n := len(in) + g.charged[scope]; n >= L {
			// n-L+1 sessions must leave the window; this roster's own
			// charges leave it last.
			h.Limit, h.ResetsAt = LimitSessions, g.now.Add(BudgetWindow)
			if e := n - L + 1; e <= len(in) {
				h.ResetsAt = in[e-1].Launched.Add(BudgetWindow)
			}
			return h, true
		}
	}
	if L := b.TokensPerDay; L > 0 && tokens.atLeast(L) {
		h.Limit = LimitTokens
		for _, s := range in {
			tokens.sub(s.tokens())
			h.ResetsAt = s.Launched.Add(BudgetWindow)
			if !tokens.atLeast(L) {
				break
			}
		}
		return h, true
	}
	return BudgetHold{}, false
}

// recordBudget keeps the holds this roster found, and earlier holds whose
// scope is still exhausted, and emits one budget event per new hold, so a
// hold that persists across ticks is reported once (CAL-V0-158).
func (d *Dispatcher) recordBudget(g *spendGate) {
	prev := map[[2]string]BudgetHold{}
	if d.ledger.Budget != nil {
		for _, h := range d.ledger.Budget.Held {
			prev[[2]string{h.Scope, h.Name}] = h
		}
	}
	for scope := range prev {
		if _, ok := g.holds[scope]; !ok {
			g.charged[scope] = 0 // the roster did not reach it: judge history alone
			if h, held := g.exhausted(scope); held {
				g.holds[scope] = h
			}
		}
	}
	if len(g.holds) == 0 && len(prev) == 0 {
		return
	}
	next := make([]BudgetHold, 0, len(g.holds))
	for _, h := range g.holds {
		next = append(next, h)
	}
	sort.Slice(next, func(i, j int) bool {
		if next[i].Scope != next[j].Scope {
			return next[i].Scope < next[j].Scope
		}
		return next[i].Name < next[j].Name
	})
	if len(next) > maxSpendSessions {
		next = next[:maxSpendSessions]
	}
	d.spend().Held = next
	for _, h := range next {
		if p, ok := prev[[2]string{h.Scope, h.Name}]; ok && p.Limit == h.Limit {
			continue
		}
		sessions, tokens, unknown := d.scopeSpend([2]string{h.Scope, h.Name}, g.now)
		reset := h.ResetsAt.UTC().Format(time.RFC3339)
		e := Event{Kind: "budget", Message: fmt.Sprintf("%s %s is held by its %s budget until %s", h.Scope, h.Name, h.Limit, reset), Detail: map[string]string{"scope": h.Scope, "name": h.Name, "limit": h.Limit, "resetsAt": reset, "sessions": strconv.Itoa(sessions), "observedTokens": strconv.FormatUint(tokens, 10), "unknownSessions": strconv.Itoa(unknown)}}
		if h.Scope == ScopeRole {
			e.Role = h.Name
		} else {
			e.Ticket = h.Name
		}
		d.emit(e)
	}
}

// scopeSpend is a scope's sessions in the window, its observed tokens and
// how many of its sessions have an UNKNOWN total.
func (d *Dispatcher) scopeSpend(scope [2]string, now time.Time) (sessions int, tokens uint64, unknown int) {
	if d.ledger.Budget == nil {
		return 0, 0, 0
	}
	cutoff := now.Add(-BudgetWindow)
	var sum tokenSum
	for _, s := range d.ledger.Budget.Sessions {
		if s.in(scope) && s.Launched.After(cutoff) {
			sessions++
			sum.add(s.tokens())
			if s.Usage == UsageUnknown {
				unknown++
			}
		}
	}
	return sessions, sum.value(), unknown
}

// ScopeSpend is one scope's spend in the window, for dispatch status
// (CAL-V0-158): sessions launched, the tokens observed (a lower bound while
// any session is PARTIAL or RUNNING), the sessions whose total is UNKNOWN,
// its budget if any, and the recorded hold.
type ScopeSpend struct {
	Scope, Name                         string
	Budget                              *Budget
	Sessions, Unknown, Partial, Running int
	Tokens                              uint64
	Hold                                *BudgetHold
}

// SpendReport is every configured role's spend and, with a ticket budget,
// each ticket's that launched in the window. It reads the ledger only.
func SpendReport(c *Config, l *Ledger, now time.Time) []ScopeSpend {
	var out []ScopeSpend
	add := func(scope [2]string, b *Budget) {
		s := ScopeSpend{Scope: scope[0], Name: scope[1], Budget: b}
		var sum tokenSum
		if l.Budget != nil {
			cutoff := now.Add(-BudgetWindow)
			for _, x := range l.Budget.Sessions {
				if !x.in(scope) || !x.Launched.After(cutoff) {
					continue
				}
				s.Sessions++
				sum.add(x.tokens())
				s.Tokens = sum.value()
				switch x.Usage {
				case UsageUnknown:
					s.Unknown++
				case UsagePartial:
					s.Partial++
				case UsageRunning:
					s.Running++
				}
			}
			for i, h := range l.Budget.Held {
				if h.Scope == scope[0] && h.Name == scope[1] {
					s.Hold = &l.Budget.Held[i]
				}
			}
		}
		out = append(out, s)
	}
	for _, r := range c.Roles {
		add([2]string{ScopeRole, r.Name}, r.Budget)
	}
	if c.TicketBudget != nil && l.Budget != nil {
		seen, tickets := map[string]bool{}, []string{}
		for _, x := range l.Budget.Sessions {
			if x.Ticket != "" && !seen[x.Ticket] && x.Launched.After(now.Add(-BudgetWindow)) {
				seen[x.Ticket] = true
				tickets = append(tickets, x.Ticket)
			}
		}
		sort.Strings(tickets)
		for _, t := range tickets {
			add([2]string{ScopeTicket, t}, c.TicketBudget)
		}
	}
	return out
}
