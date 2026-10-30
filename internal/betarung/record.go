package betarung

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Profile names the record shape this package accepts.
const Profile = "corvint-beta-rung/0"

// Record is the checked-in beta admission record. It states, per command and
// per integration surface, whether that pair meets the GPK-V0-042 bar, citing
// the evidence file that carries each verdict.
type Record struct {
	Profile string `json:"profile"`
	// Authority names the clauses this record executes. It is documentation
	// inside the machine record so a reader never has to guess which ruling
	// produced an admission.
	Authority  []string `json:"authority"`
	CapturedAt string   `json:"capturedAt"`
	// PriorLabel is the label an unadmitted pair keeps. GPK-V0-023 pins a
	// partial slice at FALLBACK, so nothing else is accepted here.
	PriorLabel Label `json:"priorLabel"`
	// OpenDivergences mirrors the entries in conformance/divergence-register.md
	// whose Status line reads OPEN, together with the commands each affects.
	// CheckRegister proves this set still equals the register's own.
	OpenDivergences []OpenDivergence `json:"openDivergences"`
	Admissions      []Admission      `json:"admissions"`
	Notes           []string         `json:"notes"`
}

// OpenDivergence is one open entry in the divergence register, resolved to the
// commands it names.
type OpenDivergence struct {
	ID       string   `json:"id"`
	Commands []string `json:"commands"`
	Summary  string   `json:"summary"`
}

// Admission is one command's standing on one or more integration surfaces.
type Admission struct {
	Command  string   `json:"command"`
	Surfaces []string `json:"surfaces"`
	Admitted bool     `json:"admitted"`
	Evidence Evidence `json:"evidence"`
	// DisclosedDivergences names the open register entries this command carries.
	// GPK-V0-042 makes an admission INVALID without them; it does not make the
	// open entry itself a blocker.
	DisclosedDivergences []string `json:"disclosedDivergences"`
	Blockers             []string `json:"blockers"`
}

// Evidence is exactly the three sources GPK-V0-042 names. No additional
// evidence class is created and no weaker one is accepted, so this struct is
// closed by design.
type Evidence struct {
	GPKV0017Criteria      Finding `json:"gpkV0017Criteria"`
	W11ReproducibleBuild  Finding `json:"w11ReproducibleBuild"`
	W12DogfoodAndRollback Finding `json:"w12DogfoodAndRollback"`
}

// Finding is one evidence source's verdict for one command, with the
// repository-relative files that carry it.
type Finding struct {
	Verdict Status   `json:"verdict"`
	Sources []string `json:"sources"`
	Detail  string   `json:"detail"`
}

func (evidence Evidence) findings() []Finding {
	return []Finding{
		evidence.GPKV0017Criteria,
		evidence.W11ReproducibleBuild,
		evidence.W12DogfoodAndRollback,
	}
}

// Parse decodes and validates one admission record. Unknown fields are refused:
// a record this package does not fully understand must not decide a label.
func Parse(data []byte) (Record, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("beta admission record: %w", err)
	}
	if err := record.validate(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (record Record) validate() error {
	if record.Profile != Profile {
		return fmt.Errorf("beta admission record: profile %q is not %q", record.Profile, Profile)
	}
	if record.PriorLabel != Fallback {
		return fmt.Errorf("beta admission record: priorLabel %q is not %q; GPK-V0-023 keeps a partial slice at FALLBACK", record.PriorLabel, Fallback)
	}
	if err := record.validateOpenDivergences(); err != nil {
		return err
	}
	return record.validateAdmissions()
}

func (record Record) validateOpenDivergences() error {
	seen := map[string]bool{}
	for _, entry := range record.OpenDivergences {
		if entry.ID == "" {
			return fmt.Errorf("beta admission record: an open divergence has no id")
		}
		if seen[entry.ID] {
			return fmt.Errorf("beta admission record: open divergence %s is declared twice", entry.ID)
		}
		if len(entry.Commands) == 0 {
			return fmt.Errorf("beta admission record: open divergence %s names no command", entry.ID)
		}
		seen[entry.ID] = true
	}
	return nil
}

func (record Record) validateAdmissions() error {
	seen := map[string]bool{}
	for _, admission := range record.Admissions {
		if admission.Command == "" {
			return fmt.Errorf("beta admission record: an admission has no command")
		}
		if len(admission.Surfaces) == 0 {
			return fmt.Errorf("beta admission record: %s names no integration surface; GPK-V0-042 admits no global label", admission.Command)
		}
		if err := claimEachSurfaceOnce(seen, admission); err != nil {
			return err
		}
		if err := admission.validateEvidence(); err != nil {
			return err
		}
		if err := admission.validateBar(); err != nil {
			return err
		}
		if err := admission.validateDisclosure(record.OpenDivergences); err != nil {
			return err
		}
	}
	return nil
}

func claimEachSurfaceOnce(seen map[string]bool, admission Admission) error {
	for _, surface := range admission.Surfaces {
		key := admission.Command + "\x00" + surface
		if seen[key] {
			return fmt.Errorf("beta admission record: %s is admitted twice on surface %s", admission.Command, surface)
		}
		seen[key] = true
	}
	return nil
}

func (admission Admission) validateEvidence() error {
	for _, finding := range admission.Evidence.findings() {
		if finding.Verdict != Pass && finding.Verdict != Fail && finding.Verdict != NotRun {
			return fmt.Errorf("beta admission record: %s carries verdict %q, which is not PASS, FAIL, or NOT_RUN", admission.Command, finding.Verdict)
		}
		if len(finding.Sources) == 0 {
			return fmt.Errorf("beta admission record: %s states a verdict with no evidence file", admission.Command)
		}
	}
	return nil
}

// validateBar enforces the admission bar itself. GPK-V0-042 names exactly three
// evidence sources and accepts no weaker one, so an admitted command must carry
// PASS on all three.
func (admission Admission) validateBar() error {
	if !admission.Admitted {
		return nil
	}
	for _, finding := range admission.Evidence.findings() {
		if finding.Verdict == Pass {
			continue
		}
		return fmt.Errorf("beta admission record: %s is admitted to beta on a %s evidence verdict; GPK-V0-042 requires PASS on all three named sources", admission.Command, finding.Verdict)
	}
	return nil
}

// validateDisclosure enforces the clause's disclosure obligation: a command
// admitted to beta while holding an open register entry MUST disclose that
// entry, and admission without that disclosure is invalid. The converse is
// checked too — a disclosure naming an entry that is not open is stale, and a
// stale disclosure is how a record silently stops describing reality.
func (admission Admission) validateDisclosure(open []OpenDivergence) error {
	declared := map[string]bool{}
	for _, entry := range open {
		declared[entry.ID] = true
	}
	for _, disclosed := range admission.DisclosedDivergences {
		if declared[disclosed] {
			continue
		}
		return fmt.Errorf("beta admission record: %s discloses %s, which is not a declared open divergence", admission.Command, disclosed)
	}
	if !admission.Admitted {
		return nil
	}
	for _, entry := range open {
		if !contains(entry.Commands, admission.Command) {
			continue
		}
		if contains(admission.DisclosedDivergences, entry.ID) {
			continue
		}
		return fmt.Errorf("beta admission record: %s is admitted to beta holding open divergence %s without disclosing it; GPK-V0-042 makes that admission invalid", admission.Command, entry.ID)
	}
	return nil
}

// AdmittedCommands lists, in sorted order, the commands this record admits to
// beta on at least one surface.
func (record Record) AdmittedCommands() []string {
	var admitted []string
	for _, admission := range record.Admissions {
		if !admission.Admitted {
			continue
		}
		admitted = append(admitted, admission.Command)
	}
	sort.Strings(admitted)
	return admitted
}
