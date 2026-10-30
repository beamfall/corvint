package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RoleCwd is a ticket role's optional ATR-V0-340 per-ticket working
// directory. Template renders to the launch directory; {worktree} in it
// resolves per ticket (ATR-V0-341) and Fallback says what a launch does when
// it does not resolve (ATR-V0-342). Label, when set, is the prefix of the
// ticket label whose remainder names the worktree.
type RoleCwd struct {
	Template string `json:"template"`
	Fallback string `json:"fallback"`
	Label    string `json:"label,omitempty"`
}

// ATR-V0-342 fallbacks, the refusal code and the ATR-V0-343 sources.
const (
	CwdFallbackWorkRoot = "WORK_ROOT"
	CwdFallbackRefuse   = "REFUSE"
	CwdUnresolved       = "CWD_UNRESOLVED"

	CwdSourceLiveAttempt    = "LIVE_ATTEMPT"
	CwdSourceTicketWorktree = "TICKET_WORKTREE"
	CwdSourceTicketLabel    = "TICKET_LABEL"
	CwdSourceTemplate       = "TEMPLATE"
	CwdSourceWorkRoot       = "FALLBACK_WORK_ROOT"
	CwdSourceNone           = "NONE"
)

// cwdPlaceholders are the placeholders a cwd template may hold.
var cwdPlaceholders = []string{"{worktree}", "{workRoot}", "{ticketLocal}", "{role}", "{program}"}

const maxCwdTemplateBytes = 4096

func (c *RoleCwd) validate() error {
	if c.Template == "" || len(c.Template) > maxCwdTemplateBytes {
		return fmt.Errorf("cwd template must be 1..%d bytes", maxCwdTemplateBytes)
	}
	for _, p := range placeholder.FindAllString(c.Template, -1) {
		if !contains(cwdPlaceholders, p) {
			return fmt.Errorf("cwd template holds %s; it may hold only %s", p, strings.Join(cwdPlaceholders, ", "))
		}
	}
	probe := Render(c.Template, map[string]string{"{worktree}": "/w", "{workRoot}": "/r", "{ticketLocal}": "t", "{role}": "r", "{program}": "p"})
	if !clean(probe) {
		return fmt.Errorf("cwd template must render to a clean absolute path; start it with {worktree}, {workRoot} or /")
	}
	if c.Fallback != CwdFallbackWorkRoot && c.Fallback != CwdFallbackRefuse {
		return fmt.Errorf("cwd fallback must be %s or %s", CwdFallbackWorkRoot, CwdFallbackRefuse)
	}
	if c.Label != "" && (len(c.Label) > 63 || strings.ContainsAny(c.Label, "\t\n\r")) {
		return fmt.Errorf("cwd label must be a label prefix of 1..63 bytes")
	}
	if c.Label != "" && !strings.Contains(c.Template, "{worktree}") {
		return fmt.Errorf("cwd label needs a template that holds {worktree}")
	}
	return nil
}

// isDir reports whether p is a clean absolute path naming a directory.
func isDir(p string) bool {
	if !clean(p) {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// ticketWorktree resolves {worktree} for ticket (ATR-V0-341): the ticket's
// live attempt worktree, then the worktree the policy's `worktrees` branch
// has checked out, then the single ticket label with the declared prefix.
// Each candidate must be a clean absolute directory; "" means none resolved.
func ticketWorktree(obs *Observation, c *RoleCwd, workRoot, ticket string, t *Ticket) (string, string) {
	for _, a := range obs.Attempts {
		if a.Ticket == ticket && a.Live && isDir(a.Worktree) {
			return a.Worktree, CwdSourceLiveAttempt
		}
	}
	if t == nil {
		return "", CwdSourceNone
	}
	if obs.TicketWorktree != nil {
		if p := obs.TicketWorktree(t.Local); isDir(p) {
			return p, CwdSourceTicketWorktree
		}
	}
	if c.Label != "" {
		found, matched := "", false
		for _, l := range t.Labels {
			v, ok := strings.CutPrefix(l, c.Label)
			if !ok {
				continue
			}
			if matched {
				return "", CwdSourceNone // two declarations are ambiguous
			}
			found, matched = v, true
		}
		if found != "" && !filepath.IsAbs(found) {
			if !filepath.IsLocal(found) {
				return "", CwdSourceNone
			}
			found = filepath.Join(workRoot, found)
		}
		if isDir(found) {
			return found, CwdSourceTicketLabel
		}
	}
	return "", CwdSourceNone
}

// launchCwd is the ATR-V0-340 launch directory of a ticket launch of role
// and its source. A role without cwd, and a lane launch, start in workRoot
// with no source. An unresolved directory under fallback REFUSE returns an
// error, which the launch treats as proved before spawn (ATR-V0-342).
func (d *Dispatcher) launchCwd(obs *Observation, role *Role, ticket, local string) (string, string, error) {
	c := role.Cwd
	if c == nil || ticket == "" {
		return d.Config.WorkRoot, "", nil
	}
	var t *Ticket
	for i := range obs.Tickets {
		if obs.Tickets[i].ID == ticket {
			t = &obs.Tickets[i]
		}
	}
	values := map[string]string{"{workRoot}": d.Config.WorkRoot, "{ticketLocal}": local, "{role}": role.Name, "{program}": d.Program}
	source := CwdSourceTemplate
	if strings.Contains(c.Template, "{worktree}") {
		values["{worktree}"], source = ticketWorktree(obs, c, d.Config.WorkRoot, ticket, t)
	}
	if source != CwdSourceNone {
		if p := Render(c.Template, values); isDir(p) {
			return p, source, nil
		}
	}
	if c.Fallback == CwdFallbackWorkRoot {
		return d.Config.WorkRoot, CwdSourceWorkRoot, nil
	}
	return "", CwdSourceNone, fmt.Errorf("%s: role %s cwd template %q resolves to no existing directory for %s", CwdUnresolved, role.Name, c.Template, ticket)
}

// cwdDetail records the ATR-V0-343 launch directory and its source in an
// event detail; an unresolved directory records only the source NONE.
func cwdDetail(detail map[string]string, cwd, source string) map[string]string {
	if cwd != "" {
		detail["cwd"] = cwd
	}
	detail["cwdSource"] = source
	return detail
}
