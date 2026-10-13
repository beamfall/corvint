package wire

import (
	"regexp"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// Stable map provenance and role (CEM-V1-093, CEM-V1-095, decision 0525).
// Both are optional root members. Verification reads neither for authority,
// ranking or any result axis, and neither is echoed into the stable result.

// StableRequiredRootKeys is the closed required root member list, in the
// refusal order of the stable schema.
var StableRequiredRootKeys = []string{"spec", "baseRevision", "patchSha256", "excludedPath", "evidence", "hunks", "criterionBindings", "runnerReceipts", "criterionLinks", "artifacts"}

// StableOptionalRootKeys is the closed optional root member list.
var StableOptionalRootKeys = []string{"provenance", "role"}

// StableProducerOnlyKeys is the closed CEM-V1-094 list of members a producer
// without that authority (such as a converted 0.x map) emits as empty arrays.
var StableProducerOnlyKeys = []string{"criterionBindings", "runnerReceipts", "criterionLinks", "artifacts"}

// StableProvenanceKeys is the closed provenance member list, in refusal order.
var StableProvenanceKeys = []string{"sourceSpec", "sourceBlobOid", "sourceSha256", "bindCommit", "converter"}

// StableRoles is the closed role enum. An absent role reads as StableRoleChange.
const (
	StableRoleChange    = "change"
	StableRoleBindChore = "bind-chore"
	StableRoleSealChore = "seal-chore"
)

var stableConverterPattern = regexp.MustCompile(`^corvint-cem-convert/[0-9]+(\.[0-9]+)*$`)

// StableProvenance names the 0.x map a stable map was converted from.
type StableProvenance struct {
	SourceSpec    string `json:"sourceSpec"`
	SourceBlobOid string `json:"sourceBlobOid"`
	SourceSha256  string `json:"sourceSha256"`
	BindCommit    string `json:"bindCommit"`
	Converter     string `json:"converter"`
}

// EffectiveRole returns the declared role, or "change" when it is absent.
func (d *StableMap) EffectiveRole() string {
	if d.Role == "" {
		return StableRoleChange
	}
	return d.Role
}

// requireStableRootKeys refuses a missing required member, then an unknown one.
func requireStableRootKeys(o *Object) error {
	allowed := map[string]bool{}
	for _, k := range StableRequiredRootKeys {
		if _, ok := o.Get(k); !ok {
			return cemcode.New(cemcode.MissingField, "%s is required", k)
		}
		allowed[k] = true
	}
	for _, k := range StableOptionalRootKeys {
		allowed[k] = true
	}
	for _, k := range o.Keys {
		if !allowed[k] {
			return cemcode.New(cemcode.UnknownField, "unknown field %q", k)
		}
	}
	return nil
}

func isSha1Oid(text string) bool { return isLowerHex(text) && len(text) == 40 }

// parseStableProvenanceRole validates the optional members after every
// required member, so their refusals never mask an earlier one.
func parseStableProvenanceRole(o *Object, d *StableMap) error {
	if v, ok := o.Get("provenance"); ok {
		if v.Kind != KindObject {
			return fieldError("stable provenance must be object")
		}
		if e := requireClosedKeys(v.Obj, StableProvenanceKeys); e != nil {
			return e
		}
		checks := map[string]func(string) bool{
			"sourceSpec":    RetiredSpec,
			"sourceBlobOid": isSha1Oid,
			"sourceSha256":  IsSha256,
			"bindCommit":    isSha1Oid,
			"converter":     stableConverterPattern.MatchString,
		}
		values := map[string]string{}
		for _, k := range StableProvenanceKeys {
			x, _ := v.Obj.Get(k)
			if x.Kind != KindString || !checks[k](x.Str) {
				return fieldError("stable provenance %s", k)
			}
			values[k] = x.Str
		}
		d.Provenance = &StableProvenance{values["sourceSpec"], values["sourceBlobOid"], values["sourceSha256"], values["bindCommit"], values["converter"]}
	}
	if v, ok := o.Get("role"); ok {
		if v.Kind != KindString || (v.Str != StableRoleChange && v.Str != StableRoleBindChore && v.Str != StableRoleSealChore) {
			return fieldError("stable role")
		}
		d.Role = v.Str
	}
	return nil
}
