// Package betarung expresses the GPK-V0-042 beta rung: the third compatibility
// label BETA, the per-command/per-surface admission record that decides it, and
// the disclosure obligation that makes an admission valid.
//
// The clause is deliberately narrow and so is this package. It decides one
// thing — which label a (command, surface) pair carries — from a checked-in
// record, and it refuses to mint BETA from anything weaker than the three named
// evidence sources. It grants no authority toward installing the candidate as
// `corvint` (GPK-V0-015), and it never produces a global label (GPK-V0-042:
// admission is per command and per integration surface, never global).
package betarung

import (
	_ "embed"
	"sync"
)

// Label is the compatibility label a report states for one command on one
// integration surface. GPK-V0-023 keeps a partial slice at FALLBACK and forbids
// FULL; GPK-V0-042 inserts BETA between them.
type Label string

const (
	Fallback Label = "FALLBACK"
	Beta     Label = "BETA"
	Full     Label = "FULL"
)

// Status is the verdict vocabulary GPK-V0-026 already requires of promotion
// reports. GPK-V0-017 adds the rule that fixes NOT_RUN's meaning here: "Any
// threshold not measured is NOT_RUN, not waived."
type Status string

const (
	Pass   Status = "PASS"
	Fail   Status = "FAIL"
	NotRun Status = "NOT_RUN"
)

//go:embed admissions.json
var embeddedRecord []byte

// defaultRecord parses the checked-in record once. A record that fails to parse
// or validate yields the zero Record, whose PriorLabel is empty and whose
// admission set is empty, so every lookup degrades to FALLBACK. A broken record
// can therefore never mint a BETA claim; that is the only direction in which
// degrading is safe.
var defaultRecord = sync.OnceValues(func() (Record, error) {
	return Parse(embeddedRecord)
})

// Default returns the checked-in admission record and the error, if any, from
// parsing it. Callers that need the label use Support; Default exists so tests
// can assert the shipped record is well formed.
func Default() (Record, error) {
	return defaultRecord()
}

// Support resolves the compatibility label one command states on one
// integration surface. An unadmitted pair keeps its prior label unchanged,
// which GPK-V0-042 requires and GPK-V0-023 pins at FALLBACK for a partial
// slice.
func Support(command, surface string) Label {
	record, err := defaultRecord()
	if err != nil {
		return Fallback
	}
	return record.Label(command, surface)
}

// Label resolves one (command, surface) pair against this record.
func (record Record) Label(command, surface string) Label {
	admission, found := record.admission(command, surface)
	if !found {
		return record.priorLabel()
	}
	if !admission.Admitted {
		return record.priorLabel()
	}
	return Beta
}

func (record Record) priorLabel() Label {
	if record.PriorLabel == "" {
		return Fallback
	}
	return record.PriorLabel
}

func (record Record) admission(command, surface string) (Admission, bool) {
	for _, admission := range record.Admissions {
		if admission.Command != command {
			continue
		}
		if !contains(admission.Surfaces, surface) {
			continue
		}
		return admission, true
	}
	return Admission{}, false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
