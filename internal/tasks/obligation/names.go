package obligation

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Names resolves the tokens that name a ledger's obligations: the native id
// (TOL-V0-010) and its optional adopter alias (TOL-V0-034). Every lookup
// returns native ids, so everything a run records stays keyed to them.
type Names struct {
	Prefix  string
	aliases map[string]string // alias -> native id
	byID    map[string]string // native id -> alias
}

// NamesOf returns the names of l's obligations; a nil ledger has none.
func NamesOf(l *ticket.ObligationLedger) Names {
	n := Names{aliases: map[string]string{}, byID: map[string]string{}}
	if l == nil {
		return n
	}
	n.Prefix = l.Prefix
	for alias, id := range l.Aliases() {
		n.aliases[alias] = id
		n.byID[id] = alias
	}
	return n
}

// aliasSpans calls each with the native id and byte span of every alias in
// s that occurs as a whole token: a maximal run of [A-Za-z0-9._:-] equal to
// the alias once trailing `.` and `:` bytes are dropped (TOL-V0-036).
func (n Names) aliasSpans(s string, each func(id string, at, end int)) {
	if len(n.aliases) == 0 {
		return
	}
	for i := 0; i < len(s); {
		if !wire.ObligationAliasByte(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && wire.ObligationAliasByte(s[j]) {
			j++
		}
		end := j
		for end > i && (s[end-1] == '.' || s[end-1] == ':') {
			end--
		}
		if id, ok := n.aliases[s[i:end]]; ok {
			each(id, i, end)
		}
		i = j
	}
}

// Find returns the distinct native ids that s names: native ids of the
// prefix as whole tokens (FindIDs), then the ids whose alias s holds as a
// whole token, each once.
func (n Names) Find(s string) []string {
	out := FindIDs(s, n.Prefix)
	seen := map[string]bool{}
	for _, id := range out {
		seen[id] = true
	}
	n.aliasSpans(s, func(id string, _, _ int) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	})
	return out
}

// Contains reports whether content names id as a whole token, by its native
// id (TOL-V0-012) or by its alias (TOL-V0-036).
func (n Names) Contains(content []byte, id string) bool {
	if ContainsID(content, id) {
		return true
	}
	alias, ok := n.byID[id]
	if !ok {
		return false
	}
	a := []byte(alias)
	for i := 0; ; {
		k := bytes.Index(content[i:], a)
		if k < 0 {
			return false
		}
		at := i + k
		end := at + len(a)
		for end < len(content) && (content[end] == '.' || content[end] == ':') {
			end++
		}
		if (at == 0 || !wire.ObligationAliasByte(content[at-1])) && (end == len(content) || !wire.ObligationAliasByte(content[end])) {
			return true
		}
		i = at + 1
	}
}

// DefectIDs is DefectIDs(s, prefix) with every alias span masked, so an
// alias shaped like a defect id (`ABC-12.V4`) never names a defect
// (TOL-V0-036).
func (n Names) DefectIDs(s string) []string {
	if len(n.aliases) == 0 {
		return DefectIDs(s, n.Prefix)
	}
	b := []byte(s)
	n.aliasSpans(s, func(_ string, at, end int) {
		for k := at; k < end; k++ {
			b[k] = ' '
		}
	})
	return DefectIDs(string(b), n.Prefix)
}

// contractAlias matches a `<contract>:<token>` annotation whose token is a
// maximal run of the alias alphabet; contractAliases keeps the tokens that are
// aliases.
var contractAlias = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.\-])[A-Za-z][A-Za-z0-9_.\-]{0,63}:([A-Za-z0-9_][A-Za-z0-9_.:\-]*)`)

// contractAliases returns the native ids whose alias s names as
// `<contract>:<alias>` (TOL-V0-036, the alias form of TOL-V0-025).
func (n Names) contractAliases(s string) []string {
	if len(n.aliases) == 0 {
		return nil
	}
	var out []string
	for _, m := range contractAlias.FindAllStringSubmatch(s, -1) {
		if id, ok := n.aliases[strings.TrimRight(m[1], ".:")]; ok {
			out = append(out, id)
		}
	}
	return out
}
