package intent

import (
	"regexp"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// MaxCIEvidenceGates bounds policy ciEvidence.gates, and MaxCIRequiredChecks
// each entry's requiredChecks (TCD-V0-154).
const (
	MaxCIEvidenceGates  = 16
	MaxCIRequiredChecks = 64
	// MaxCICheckNameBytes bounds one CI check name.
	MaxCICheckNameBytes = 200
)

// ciRepositoryText is a GitHub owner/name pair.
var ciRepositoryText = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9._-]{1,100}$`)

// ValidCIRepository reports whether s is a GitHub owner/name pair.
func ValidCIRepository(s string) bool { return ciRepositoryText.MatchString(s) }

// CIEvidencePolicy is the optional TCD-V0-154 declaration that lets a
// retained green CI run satisfy a command gate. Nil (the key absent) keeps
// every gate locally executed or reused; removing the key is the rollback.
type CIEvidencePolicy struct {
	Gates []CIEvidenceGate
}

// CIEvidenceGate names one command gate a CI run of Repository may satisfy
// when every RequiredChecks name passed in that run.
type CIEvidenceGate struct {
	GateID         string
	Repository     string
	RequiredChecks []string
}

// CIEvidenceGate is the ciEvidence entry for gateID, nil when the policy
// declares none.
func (p *Policy) CIEvidenceGate(gateID string) *CIEvidenceGate {
	if p == nil || p.CIEvidence == nil {
		return nil
	}
	for i := range p.CIEvidence.Gates {
		if p.CIEvidence.Gates[i].GateID == gateID {
			return &p.CIEvidence.Gates[i]
		}
	}
	return nil
}

// readCIEvidence decodes policy ciEvidence: each entry names a declared
// COMMAND gate once, a GitHub repository and at least one required check.
func readCIEvidence(r *wire.Reader, gates []GateDefinition) *CIEvidencePolicy {
	r.Closed("gates")
	out := &CIEvidencePolicy{}
	seen := map[string]bool{}
	for _, e := range r.Field("gates").Array(MaxCIEvidenceGates, true) {
		e.Closed("gateId", "repository", "requiredChecks")
		g := CIEvidenceGate{GateID: e.Field("gateId").Label()}
		repo := e.Field("repository")
		g.Repository = repo.String()
		g.RequiredChecks = e.Field("requiredChecks").Strings(MaxCIRequiredChecks, false, func(x *wire.Reader) string { return x.Prose(1, MaxCICheckNameBytes) })
		if r.Err() != nil {
			return nil
		}
		if !ValidCIRepository(g.Repository) {
			repo.Fail(wire.CodeMalformed, "repository must be a GitHub owner/name pair")
			return nil
		}
		if len(g.RequiredChecks) == 0 {
			e.Field("requiredChecks").Fail(wire.CodeMalformed, "requiredChecks needs 1..%d check names", MaxCIRequiredChecks)
			return nil
		}
		command := false
		for _, d := range gates {
			command = command || (d.GateID == g.GateID && d.Kind == "COMMAND")
		}
		if !command || seen[g.GateID] {
			e.Field("gateId").Fail(wire.CodeMalformed, "ciEvidence gate %s must name a declared COMMAND gate once", g.GateID)
			return nil
		}
		seen[g.GateID] = true
		out.Gates = append(out.Gates, g)
	}
	return out
}
