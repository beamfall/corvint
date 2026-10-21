package snapshot

import (
	"slices"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfileCIRun is the retained CI run record (TCD-V0-155): the facts of one
// finished GitHub Actions run an operator recorded, kept as an
// evidence/<sha256> blob that a CI-backed gate result names.
const ProfileCIRun = "taskman-ci-run/0"

// MaxCIRunBytes bounds one CI run record; maxCIPassedChecks its check list.
const (
	MaxCIRunBytes     = 32 * wire.KiB
	maxCIPassedChecks = 256
	maxCICheckBytes   = 200
)

// CIConclusions is the GitHub Actions run conclusion set; only success
// satisfies a gate.
var CIConclusions = []string{"success", "failure", "cancelled", "skipped", "timed_out", "action_required", "neutral", "stale", "startup_failure"}

// CIRun is one retained CI run: the repository, run id, the commit it ran
// at, its conclusion, the names of the checks that passed in it and when it
// completed. Every field is a recorded claim; only HeadSha's ancestry is
// re-observed, from local git.
type CIRun struct {
	Repository   string
	RunID        wire.Size
	HeadSha      string
	Conclusion   string
	PassedChecks []string
	CompletedAt  wire.Timestamp
}

var ciRunFields = []string{"profile", "repository", "runId", "headSha", "conclusion", "passedChecks", "completedAt"}

// DecodeCIRun decodes a CI run record. The repository text is checked by
// the policy layer that compares it.
func DecodeCIRun(data []byte) (*CIRun, error) {
	if len(data) > MaxCIRunBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "CI run record larger than %d bytes", MaxCIRunBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	if err := wire.ProfileVersion("/profile", v, ProfileCIRun); err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed(ciRunFields...)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileCIRun); err != nil {
		return nil, err
	}
	c := &CIRun{}
	c.Repository = r.Field("repository").Prose(3, 141)
	c.RunID = r.Field("runId").Size()
	c.HeadSha = r.Field("headSha").OID()
	c.Conclusion = r.Field("conclusion").Enum(CIConclusions...)
	c.PassedChecks = r.Field("passedChecks").Strings(maxCIPassedChecks, false, func(x *wire.Reader) string { return x.Prose(1, maxCICheckBytes) })
	c.CompletedAt = r.Field("completedAt").Timestamp()
	if err := r.Err(); err != nil {
		return nil, err
	}
	return c, nil
}

// Encode is the canonical record bytes: passedChecks sorted and
// duplicate-free.
func (c *CIRun) Encode() ([]byte, error) {
	checks := slices.Clone(c.PassedChecks)
	slices.Sort(checks)
	checks = slices.Compact(checks)
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileCIRun))
	o.Set("repository", wire.String(c.Repository))
	o.Set("runId", wire.String(string(c.RunID)))
	o.Set("headSha", wire.String(c.HeadSha))
	o.Set("conclusion", wire.String(c.Conclusion))
	o.Set("passedChecks", wire.Strings(checks))
	o.Set("completedAt", wire.String(string(c.CompletedAt)))
	raw := wire.EncodeFile(wire.ObjectValue(o))
	if _, err := DecodeCIRun(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
