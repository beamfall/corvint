package migrationratchet

import (
	"reflect"
	"sort"
	"strings"
)

// SchemaDescriptionSchema names the self-description emitted by
// `corvint migration-ratchet schema` (MER-V0-016).
const SchemaDescriptionSchema = "corvint-migration-evidence-ratchet-schema/1"

// CanonicalEncoding documents the byte encoding every self-digest hashes, so
// producers outside Go can reproduce it (MER-V0-013).
var CanonicalEncoding = []string{
	"digest = lowercase hex SHA-256 of the canonical JSON encoding of the object with its own digest member set to the empty string",
	"objects: members in the declared field order below, no insignificant whitespace, omitempty members omitted when empty",
	`strings: UTF-8 written raw except: quotation mark and backslash as \" and \\; U+0008, U+000C, LF, CR and TAB as \b, \f, \n, \r, \t; other U+0000..U+001F as \u00xx (lowercase hex); '<', '>', '&', U+2028 and U+2029 as <, >, &,  ,  ; '/' and U+007F unescaped; invalid UTF-8 decodes to U+FFFD before hashing`,
	"arrays: an absent or null array encodes as null and an empty array as []; the two produce different digests",
	"integers: decimal without exponent or fraction; booleans: true or false",
	"order: each record's links sorted by relation, target_kind, target_id, target_sha256, reverse_relation; records by kind, id, sha256; exceptions by id, sha256 (byte order, NUL separated)",
	"seal: `corvint migration-ratchet seal --snapshot FILE` or `--profile FILE` fills every self-digest and canonical order without changing any other value",
}

// Refusals lists every exit-2 refusal the comparison emits. '*' stands for a
// side (baseline or candidate), a field or a decoder detail.
var Refusals = []string{
	"migration-ratchet * * record refused: */*",
	"migration-ratchet * artifact digest refused",
	"migration-ratchet * bound refused",
	"migration-ratchet * digest refused: * expected *",
	"migration-ratchet * duplicate link refused",
	"migration-ratchet * exception owner refused",
	"migration-ratchet * exception refused",
	"migration-ratchet * exception reviewer refused",
	"migration-ratchet * exceptions are not canonical",
	"migration-ratchet * identity refused",
	"migration-ratchet * link refused",
	"migration-ratchet * policy accept_incomparable refused",
	"migration-ratchet * policy digest refused",
	"migration-ratchet * policy states refused",
	"migration-ratchet * provider identity refused",
	"migration-ratchet * record links are not canonical",
	"migration-ratchet * record refused",
	"migration-ratchet * record source refused",
	"migration-ratchet * records are not canonical",
	"migration-ratchet * source refused: * content_sha256 does not match Git span digest *",
	"migration-ratchet * source unreadable: *: *",
	"migration-ratchet * source unverified: * has no Git reader",
	"migration-ratchet evaluation date refused",
	"migration-ratchet identity mapping refused",
	"migration-ratchet identity mapping reuses candidate */* for */* and */*",
	"migration-ratchet identity mapping source missing",
	"migration-ratchet identity mapping target missing",
	"migration-ratchet incomparable * refused",
	"migration-ratchet input refused: *",
	"migration-ratchet input refused: trailing JSON",
	"migration-ratchet input size refused",
	"migration-ratchet migration rule allows refused",
	"migration-ratchet migration rule owner refused",
	"migration-ratchet migration rule refused",
	"migration-ratchet migration rule reviewer refused",
	"migration-ratchet migration rule selection has no duplicate",
	"migration-ratchet migration rule selection refused",
	"migration-ratchet profile schema refused",
}

type FieldDescription struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Optional bool   `json:"optional,omitempty"`
}

type SchemaDescription struct {
	Schema               string                        `json:"schema"`
	ProfileSchema        string                        `json:"profile_schema"`
	SnapshotSchema       string                        `json:"snapshot_schema"`
	ReceiptSchema        string                        `json:"receipt_schema"`
	Limits               map[string]int                `json:"limits"`
	CanonicalEncoding    []string                      `json:"canonical_encoding"`
	Objects              map[string][]FieldDescription `json:"objects"`
	RecordKinds          []string                      `json:"record_kinds"`
	ComparableDimensions []string                      `json:"comparable_dimensions"`
	PolicyAcceptable     []string                      `json:"policy_accept_incomparable"`
	ExceptionDeltas      []string                      `json:"exception_deltas"`
	Verdicts             map[string]string             `json:"verdicts"`
	Refusals             []string                      `json:"refusals"`
	Example              Profile                       `json:"example"`
}

// Describe returns the input contract derived from the decoder's own types,
// so the description cannot drift from the fields Decode accepts.
func Describe() SchemaDescription {
	objects := map[string][]FieldDescription{}
	for _, value := range []any{Profile{}, Snapshot{}, RepositoryBinding{}, ProviderBinding{}, Policy{}, StateRule{}, Record{}, SourceSpan{}, Link{}, Exception{}, ExceptionScope{}, ComparabilityRule{}, IdentityMapping{}, RecordSelection{}} {
		kind := reflect.TypeOf(value)
		objects[kind.Name()] = describeFields(kind)
	}
	return SchemaDescription{
		Schema: SchemaDescriptionSchema, ProfileSchema: ProfileSchema, SnapshotSchema: SnapshotSchema, ReceiptSchema: ReceiptSchema,
		Limits:            map[string]int{"max_bytes": MaxBytes, "max_records_per_snapshot": MaxRecords, "max_exceptions_per_snapshot": MaxRecords, "max_policy_states": 64},
		CanonicalEncoding: CanonicalEncoding, Objects: objects,
		RecordKinds: sortedSet(recordKinds), ComparableDimensions: sortedSet(comparableDimensions), PolicyAcceptable: sortedSet(policyAcceptable), ExceptionDeltas: sortedSet(exceptionDeltas),
		Verdicts: map[string]string{"pass": "exit 0", "fail": "exit 1: policy failure", "unknown": "exit 1: unexcepted record-level incomparability", "refused": "exit 2: typed refusal on stderr, no receipt"},
		Refusals: Refusals, Example: ExampleProfile(),
	}
}

// ExampleProfile is a minimal sealed profile that compares as pass.
func ExampleProfile() Profile {
	policy := Policy{ID: "policy:example", States: []StateRule{{Name: "unresolved", Rank: 0, Unresolved: true}, {Name: "reviewed", Rank: 1}}, UnresolvedDenominatorCannotGrow: true}
	owner := Record{Kind: "owner", ID: "owner:example", ContentSHA256: Digest("owner:example"), State: "reviewed", Links: []Link{}}
	snapshot := func(revision, tree string) Snapshot {
		return Snapshot{Schema: SnapshotSchema, Repository: RepositoryBinding{ID: "repository:example", Revision: revision, Tree: tree}, Policy: policy,
			Provider: ProviderBinding{ID: "provider:example", Schema: "example-inventory/1", SHA256: Digest("provider:example")}, Complete: true, Fresh: true,
			Records: []Record{owner}, Exceptions: []Exception{}}
	}
	return SealProfile(Profile{Schema: ProfileSchema, EvaluatedOn: "2026-10-27",
		Baseline:  snapshot(strings.Repeat("1", 40), strings.Repeat("2", 40)),
		Candidate: snapshot(strings.Repeat("3", 40), strings.Repeat("4", 40))})
}

func describeFields(kind reflect.Type) []FieldDescription {
	fields := []FieldDescription{}
	for i := 0; i < kind.NumField(); i++ {
		field := kind.Field(i)
		tag := field.Tag.Get("json")
		name, options, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		optional := strings.Contains(options, "omitempty") || field.Type.Kind() == reflect.Pointer
		fields = append(fields, FieldDescription{Name: name, Type: typeName(field.Type), Optional: optional})
	}
	return fields
}

func typeName(kind reflect.Type) string {
	switch kind.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int:
		return "integer"
	case reflect.Slice:
		return "array<" + typeName(kind.Elem()) + ">"
	case reflect.Pointer:
		return typeName(kind.Elem())
	default:
		return "object<" + kind.Name() + ">"
	}
}

func sortedSet(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
