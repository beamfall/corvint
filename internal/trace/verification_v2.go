package trace

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	SchemaVersionV2  = 2
	MaxArgvArguments = 32
	MaxArgvBytes     = 4096
)

// VerificationEntry is a closed schema-v2 union. Exactly the field selected by
// Kind is populated; Argv retains literal argument boundaries, never shell text.
type VerificationEntry struct {
	Kind    string
	Command string
	Argv    []string
}

func argvFailure(reason string) error {
	return verificationFailure("unsupported-verify-argv", fmt.Errorf("schema-2 typed verification: %s", reason))
}

// ParseVerificationArgv decodes the public JSON transport without shell parsing.
// json/v2 rejects invalid UTF-8, unpaired surrogates and trailing JSON values.
func ParseVerificationArgv(data []byte) ([]string, error) {
	if len(data) > MaxTraceRowBytes {
		return nil, argvFailure("JSON input exceeds row bound")
	}
	var values []jsontext.Value
	if err := json.Unmarshal(data, &values); err != nil || values == nil {
		return nil, argvFailure("expected one JSON array of strings")
	}
	argv := make([]string, len(values))
	for i, value := range values {
		if len(value) == 0 || value[0] != '"' || json.Unmarshal(value, &argv[i]) != nil {
			return nil, argvFailure("expected string argument")
		}
	}
	if err := validateArgv(argv); err != nil {
		return nil, err
	}
	return argv, nil
}

// CanonicalArgv returns the compact ASCII-escaped JSON array used in v2 identity
// and displays. It never joins arguments into an executable command.
func CanonicalArgv(argv []string) ([]byte, error) {
	var b bytes.Buffer
	if err := writeStringArray(&b, argv); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func validateArgv(argv []string) error {
	if len(argv) < 1 || len(argv) > MaxArgvArguments || argv[0] == "" {
		return argvFailure("argv requires 1..32 arguments and a nonempty first argument")
	}
	for _, arg := range argv {
		if !utf8.ValidString(arg) || utf8.RuneCountInString(arg) > MaxCommandCharacters {
			return argvFailure("invalid argument encoding or character bound")
		}
		for _, r := range arg {
			if unicode.IsControl(r) {
				return argvFailure("argument contains a control character")
			}
		}
	}
	encoded, err := CanonicalArgv(argv)
	if err != nil || len(encoded) > MaxArgvBytes {
		return argvFailure("argv exceeds canonical byte bound")
	}
	if secretscreen.MatchArgv(argv) || secretscreen.MatchString(string(encoded)) {
		return argvFailure("secret-shaped content")
	}
	return nil
}

func entryBytes(entry VerificationEntry) ([]byte, error) {
	var b bytes.Buffer
	switch entry.Kind {
	case "command":
		if entry.Argv != nil {
			return nil, argvFailure("command entry contains argv")
		}
		b.WriteString(`{"command":`)
		if err := writePythonString(&b, entry.Command); err != nil {
			return nil, err
		}
		b.WriteString(`,"kind":"command"}`)
	case "argv":
		if entry.Command != "" {
			return nil, argvFailure("argv entry contains command")
		}
		b.WriteString(`{"argv":`)
		if err := writeStringArray(&b, entry.Argv); err != nil {
			return nil, err
		}
		b.WriteString(`,"kind":"argv"}`)
	default:
		return nil, argvFailure("unknown entry kind")
	}
	return b.Bytes(), nil
}

func normalizeEntries(entries []VerificationEntry) ([]VerificationEntry, error) {
	if len(entries) == 0 || len(entries) > MaxVerificationCommands {
		return nil, argvFailure("verification entry count out of bounds")
	}
	unique := make(map[string]VerificationEntry, len(entries))
	hasArgv := false
	for _, entry := range entries {
		switch entry.Kind {
		case "command":
			if entry.Argv != nil {
				return nil, argvFailure("command entry contains argv")
			}
			commands, err := normalizeCommands([]string{entry.Command})
			if err != nil {
				return nil, argvFailure("invalid command entry")
			}
			entry.Command = commands[0]
		case "argv":
			if entry.Command != "" {
				return nil, argvFailure("argv entry contains command")
			}
			if err := validateArgv(entry.Argv); err != nil {
				return nil, err
			}
			entry.Argv = slices.Clone(entry.Argv)
			hasArgv = true
		default:
			return nil, argvFailure("unknown entry kind")
		}
		encoded, err := entryBytes(entry)
		if err != nil {
			return nil, err
		}
		unique[string(encoded)] = entry
	}
	if !hasArgv {
		return nil, argvFailure("schema 2 requires an argv entry")
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]VerificationEntry, 0, len(keys))
	for _, key := range keys {
		result = append(result, unique[key])
	}
	return result, nil
}

func validateEntries(entries []VerificationEntry) error {
	normalized, err := normalizeEntries(entries)
	if err != nil {
		return err
	}
	if len(normalized) != len(entries) {
		return argvFailure("duplicate entries")
	}
	for i := range normalized {
		a, _ := entryBytes(entries[i])
		b, _ := entryBytes(normalized[i])
		if !bytes.Equal(a, b) {
			return argvFailure("entries are not normalized and sorted")
		}
	}
	return nil
}

func decodeV2Record(data []byte) (Record, error) {
	if len(data) > MaxTraceRowBytes {
		return Record{}, argvFailure("row exceeds byte bound")
	}
	var raw map[string]jsontext.Value
	if err := json.Unmarshal(data, &raw); err != nil {
		return Record{}, argvFailure("invalid schema-2 JSON")
	}
	if len(raw) != len(requiredFields) {
		return Record{}, argvFailure("invalid row fields")
	}
	for key := range raw {
		if _, ok := requiredFields[key]; !ok {
			return Record{}, argvFailure("unknown row field")
		}
	}
	if string(bytes.TrimSpace(raw["schema_version"])) != "2" {
		return Record{}, argvFailure("invalid schema version")
	}
	record := Record{SchemaVersion: SchemaVersionV2}
	for key, dst := range map[string]*string{"revision": &record.Revision, "trace_id": &record.TraceID, "task": &record.Task, "outcome": &record.Outcome} {
		if err := json.Unmarshal(raw[key], dst); err != nil || bytes.Equal(raw[key], []byte("null")) {
			return Record{}, argvFailure("invalid string field")
		}
	}
	for key, dst := range map[string]*[]string{"opened_paths": &record.OpenedPaths, "changed_paths": &record.ChangedPaths} {
		if err := json.Unmarshal(raw[key], dst); err != nil || *dst == nil {
			return Record{}, argvFailure("invalid path array")
		}
	}
	var values []map[string]jsontext.Value
	if err := json.Unmarshal(raw["verification"], &values); err != nil || values == nil || len(values) > MaxVerificationCommands {
		return Record{}, argvFailure("invalid verification array")
	}
	for _, value := range values {
		if len(value) != 2 {
			return Record{}, argvFailure("invalid entry fields")
		}
		var kind string
		if err := json.Unmarshal(value["kind"], &kind); err != nil {
			return Record{}, argvFailure("invalid entry kind")
		}
		entry := VerificationEntry{Kind: kind}
		switch kind {
		case "command":
			rawCommand, ok := value["command"]
			if !ok || json.Unmarshal(rawCommand, &entry.Command) != nil || bytes.Equal(rawCommand, []byte("null")) {
				return Record{}, argvFailure("invalid command entry")
			}
		case "argv":
			var err error
			entry.Argv, err = ParseVerificationArgv(value["argv"])
			if err != nil {
				return Record{}, err
			}
		default:
			return Record{}, argvFailure("unknown entry kind")
		}
		record.TypedVerification = append(record.TypedVerification, entry)
	}
	if err := validateEntries(record.TypedVerification); err != nil {
		return Record{}, err
	}
	return record, nil
}

// DecodeV2 validates a schema-2 row's structure, bounds, screening, canonical
// values and identity. Repository witnesses remain the caller's responsibility.
func DecodeV2(data []byte, revision string) (Record, error) {
	record, err := decodeV2Record(data)
	if err != nil {
		return Record{}, err
	}
	if err := validateV2Record(record, revision); err != nil {
		return Record{}, err
	}
	return record, nil
}

func validateV2Record(record Record, revision string) error {
	if record.SchemaVersion != SchemaVersionV2 || record.Revision != revision || record.Verification != nil {
		return argvFailure("schema or revision mismatch")
	}
	if !validOutcome(record.Outcome) {
		return argvFailure("invalid outcome")
	}
	task, err := validTask(safeText(record.Task, "trace task", MaxTaskCharacters))
	if err != nil || task != record.Task {
		return argvFailure("invalid or noncanonical task")
	}
	for _, paths := range [][]string{record.OpenedPaths, record.ChangedPaths} {
		normalized, err := normalizePaths(paths, "trace paths")
		if err != nil || paths == nil || !equalStrings(normalized, paths) {
			return argvFailure("invalid or noncanonical paths")
		}
	}
	if err := validateEntries(record.TypedVerification); err != nil {
		return err
	}
	expected, err := traceID(record)
	if err != nil || record.TraceID != expected {
		return argvFailure("trace digest mismatch")
	}
	return nil
}

func writeVerification(b *bytes.Buffer, record Record) error {
	if record.SchemaVersion == SchemaVersion {
		return writeStringArray(b, record.Verification)
	}
	if record.SchemaVersion != SchemaVersionV2 {
		return fmt.Errorf("unsupported local trace schema")
	}
	b.WriteByte('[')
	for i, entry := range record.TypedVerification {
		if i > 0 {
			b.WriteByte(',')
		}
		encoded, err := entryBytes(entry)
		if err != nil {
			return err
		}
		b.Write(encoded)
	}
	b.WriteByte(']')
	return nil
}

// VerificationValue exposes the version's exact JSON shape for receipt and
// batch projections; callers use it only on validated records.
func (record Record) VerificationValue() any {
	if record.SchemaVersion != SchemaVersionV2 {
		return record.Verification
	}
	entries := make([]any, 0, len(record.TypedVerification))
	for _, entry := range record.TypedVerification {
		if entry.Kind == "argv" {
			entries = append(entries, map[string]any{"kind": "argv", "argv": entry.Argv})
		} else {
			entries = append(entries, map[string]any{"kind": "command", "command": entry.Command})
		}
	}
	return entries
}

// VerificationDisplay keeps argv explicitly labelled as JSON data.
func (record Record) VerificationDisplay() []string {
	if record.SchemaVersion != SchemaVersionV2 {
		return slices.Clone(record.Verification)
	}
	lines := make([]string, 0, len(record.TypedVerification))
	for _, entry := range record.TypedVerification {
		if entry.Kind == "argv" {
			encoded, _ := CanonicalArgv(entry.Argv)
			lines = append(lines, "argv "+string(encoded))
		} else {
			lines = append(lines, entry.Command)
		}
	}
	return lines
}

// VerificationArgv returns an owned copy for a second producer validation pass.
func (record Record) VerificationArgv() [][]string {
	var result [][]string
	for _, entry := range record.TypedVerification {
		if entry.Kind == "argv" {
			result = append(result, slices.Clone(entry.Argv))
		}
	}
	return result
}

// VerificationCommands retains command entries when revalidating a v2 input.
func (record Record) VerificationCommands() []string {
	if record.SchemaVersion != SchemaVersionV2 {
		return record.Verification
	}
	var result []string
	for _, entry := range record.TypedVerification {
		if entry.Kind == "command" {
			result = append(result, entry.Command)
		}
	}
	return result
}

// IsV2JSON reports only the exact discriminator; full admission needs DecodeV2.
func IsV2JSON(data []byte) bool {
	var raw map[string]jsontext.Value
	return json.Unmarshal(data, &raw) == nil && strings.TrimSpace(string(raw["schema_version"])) == "2"
}
