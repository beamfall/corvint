package companionrelease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// StableRefusalDeclaration refuses a release-readiness declaration that is
// not one exact cem-release-readiness/1 document (CEM-V1-600).
const StableRefusalDeclaration = "release-declaration"

// StableReleaseDeclarationProfile names the committed release-readiness
// declaration a release gate reads (CEM-V1-600).
const StableReleaseDeclarationProfile = "cem-release-readiness/1"

// declarationMembers is the exact member order of a declaration.
var declarationMembers = []string{"profile", "runner", "platform", "external"}

const (
	declarationLimit    = 64 << 10
	declarationRowLimit = 64
	declarationRowBytes = 128
)

// StableReleaseDeclaration lists every runner, platform and external row a
// release needs. It names what must be observed; it observes nothing.
type StableReleaseDeclaration struct {
	Profile  string   `json:"profile"`
	Runner   []string `json:"runner"`
	Platform []string `json:"platform"`
	External []string `json:"external"`
}

// ReadStableReleaseDeclaration reads raw as one exact cem-release-readiness/1
// declaration: strict CEM JSON, exactly these four members in this order, and 1 to 64
// distinct row names per class, each 1 to 128 bytes of [A-Za-z0-9._/:-].
func ReadStableReleaseDeclaration(raw []byte) (StableReleaseDeclaration, error) {
	var d StableReleaseDeclaration
	if len(raw) == 0 || len(raw) > declarationLimit || !utf8.Valid(raw) {
		return d, stableRefuse(StableRefusalDeclaration, "declaration exceeds UTF-8/size bounds")
	}
	parsed, err := wire.Parse(bytes.TrimSuffix(raw, []byte("\n")))
	if err != nil {
		return d, stableRefuse(StableRefusalDeclaration, "%v", err)
	}
	if parsed.Kind != wire.KindObject {
		return d, stableRefuse(StableRefusalDeclaration, "declaration is not a JSON object")
	}
	// encoding/json matches member names case-insensitively, so a later
	// "Runner" would replace "runner". Require the exact names first.
	if fmt.Sprint(parsed.Obj.Keys) != fmt.Sprint(declarationMembers) {
		return d, stableRefuse(StableRefusalDeclaration, "members %v are not exactly %v", parsed.Obj.Keys, declarationMembers)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return d, stableRefuse(StableRefusalDeclaration, "%v", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return d, stableRefuse(StableRefusalDeclaration, "%v", err)
	}
	if d.Profile != StableReleaseDeclarationProfile {
		return d, stableRefuse(StableRefusalDeclaration, "profile %q is not %s", d.Profile, StableReleaseDeclarationProfile)
	}
	for _, class := range []struct {
		name string
		rows []string
	}{{"runner", d.Runner}, {"platform", d.Platform}, {"external", d.External}} {
		if len(class.rows) == 0 || len(class.rows) > declarationRowLimit {
			return d, stableRefuse(StableRefusalDeclaration, "%s declares %d rows, not 1 to %d", class.name, len(class.rows), declarationRowLimit)
		}
		seen := map[string]bool{}
		for _, row := range class.rows {
			if !declarationRow(row) || seen[row] {
				return d, stableRefuse(StableRefusalDeclaration, "%s row %q is malformed or repeated", class.name, row)
			}
			seen[row] = true
		}
	}
	return d, nil
}

func declarationRow(row string) bool {
	if row == "" || len(row) > declarationRowBytes {
		return false
	}
	for _, c := range []byte(row) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || bytes.IndexByte([]byte("._/:-"), c) >= 0) {
			return false
		}
	}
	return true
}

// StableReleaseReadiness judges one release run (CEM-V1-601): every accepted
// stable result line the run produced must be an exact admitted ACCEPT within
// its authority, runtime and envelope limits, and every row the declaration
// requires must be observed PASSED. Every result is checked before any
// outcome, so a malformed result is never hidden behind a missing row. A run
// with no accepted result is refused: reference integrity elsewhere, a binary
// version or an empty run never stands in for one.
func StableReleaseReadiness(d StableReleaseDeclaration, results [][]byte, runner, platform, external map[string]string) error {
	if len(results) == 0 {
		return stableRefuse(StableRefusalNotAccept, "the release run produced no accepted stable result")
	}
	for _, raw := range results {
		if err := validateStableResult(raw); err != nil {
			return err
		}
	}
	return completeReleaseOutcomes(StableReleaseOutcomes{Required: d.Runner, Observed: runner}, StableReleaseOutcomes{Required: d.Platform, Observed: platform}, StableReleaseOutcomes{Required: d.External, Observed: external})
}
