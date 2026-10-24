package companionrelease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// StableGateOutcomeProfile names the readiness verdict one cem-release-gate
// run or aggregation writes (CEM-V1-613). It is unauthenticated run output:
// it records what the gate decided over one revision, never who ran it.
const StableGateOutcomeProfile = "cem-release-gate-outcome/1"

// StableRefusalGateOutcome refuses a gate outcome that is not one exact
// cem-release-gate-outcome/1 document (CEM-V1-613).
const StableRefusalGateOutcome = "release-gate-outcome"

// The two verdicts a gate outcome records.
const (
	StableGateReady   = "READY"
	StableGateRefused = "REFUSED"
)

const gateOutcomeLimit = 64 << 10

var (
	gateOutcomeMembers = []string{"profile", "revision", "declarationSha256", "records", "verdict", "reason"}
	gateRevision       = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	gateDigest         = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// StableGateOutcome is the cem-release-gate verdict over one revision: the
// declaration it judged by digest, the digests of the per-platform run
// records it aggregated (empty for a single run that wrote none), and READY or
// REFUSED with the refusal's text.
type StableGateOutcome struct {
	Profile           string   `json:"profile"`
	Revision          string   `json:"revision"`
	DeclarationSha256 string   `json:"declarationSha256"`
	Records           []string `json:"records"`
	Verdict           string   `json:"verdict"`
	Reason            string   `json:"reason"`
}

// EncodeStableGateOutcome returns the one canonical encoding of o, LF
// terminated, after checking it as ReadStableGateOutcome would.
func EncodeStableGateOutcome(o StableGateOutcome) ([]byte, error) {
	if o.Records == nil {
		o.Records = []string{}
	}
	if err := checkGateOutcome(o); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// ReadStableGateOutcome reads raw as one exact cem-release-gate-outcome/1
// document: strict CEM JSON with exactly these six members in this order, a
// full Git object name, digests of 64 lower-case hex, distinct record digests,
// and READY with an empty reason or REFUSED with a non-empty one. Only the
// canonical encoding is admitted.
func ReadStableGateOutcome(raw []byte) (StableGateOutcome, error) {
	var o StableGateOutcome
	if len(raw) == 0 || len(raw) > gateOutcomeLimit || !utf8.Valid(raw) {
		return o, stableRefuse(StableRefusalGateOutcome, "gate outcome exceeds UTF-8/size bounds")
	}
	parsed, err := wire.Parse(bytes.TrimSuffix(raw, []byte("\n")))
	if err != nil {
		return o, stableRefuse(StableRefusalGateOutcome, "%v", err)
	}
	if parsed.Kind != wire.KindObject || fmt.Sprint(parsed.Obj.Keys) != fmt.Sprint(gateOutcomeMembers) {
		return o, stableRefuse(StableRefusalGateOutcome, "gate outcome members are not exactly %v", gateOutcomeMembers)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&o); err != nil {
		return o, stableRefuse(StableRefusalGateOutcome, "%v", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return o, stableRefuse(StableRefusalGateOutcome, "%v", err)
	}
	if err := checkGateOutcome(o); err != nil {
		return o, err
	}
	canonical, err := EncodeStableGateOutcome(o)
	if err != nil || !bytes.Equal(canonical, raw) {
		return o, stableRefuse(StableRefusalGateOutcome, "gate outcome is not the canonical encoding")
	}
	return o, nil
}

func checkGateOutcome(o StableGateOutcome) error {
	if o.Profile != StableGateOutcomeProfile {
		return stableRefuse(StableRefusalGateOutcome, "profile %q is not %s", o.Profile, StableGateOutcomeProfile)
	}
	if !gateRevision.MatchString(o.Revision) || !gateDigest.MatchString(o.DeclarationSha256) {
		return stableRefuse(StableRefusalGateOutcome, "revision or declaration digest is malformed")
	}
	if o.Records == nil || len(o.Records) > declarationRowLimit {
		return stableRefuse(StableRefusalGateOutcome, "records is absent or names more than %d runs", declarationRowLimit)
	}
	seen := map[string]bool{}
	for _, d := range o.Records {
		if !gateDigest.MatchString(d) || seen[d] {
			return stableRefuse(StableRefusalGateOutcome, "record digest %q is malformed or repeated", d)
		}
		seen[d] = true
	}
	switch {
	case o.Verdict == StableGateReady && o.Reason == "":
	case o.Verdict == StableGateRefused && o.Reason != "" && len(o.Reason) <= gateOutcomeLimit/2:
	default:
		return stableRefuse(StableRefusalGateOutcome, "verdict %q with reason %q is not READY with no reason or REFUSED with one", o.Verdict, o.Reason)
	}
	return nil
}
