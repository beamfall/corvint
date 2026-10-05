package trace

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"slices"
	"sort"
	"strconv"
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

// decodeTypedRow decodes the strict json/v2 row shapes: schema 2, and schema 3,
// which adds the closed producer member and admits either verification shape.
func decodeTypedRow(data []byte, schema int) (Record, error) {
	fail := typedFailure(schema)
	if len(data) > MaxTraceRowBytes {
		return Record{}, fail("row exceeds byte bound")
	}
	var raw map[string]jsontext.Value
	if err := json.Unmarshal(data, &raw); err != nil {
		return Record{}, fail(fmt.Sprintf("invalid schema-%d JSON", schema))
	}
	want := len(requiredFields)
	if schema == SchemaVersionV3 {
		want++
	}
	if len(raw) != want {
		return Record{}, fail("invalid row fields")
	}
	for key := range raw {
		if _, ok := requiredFields[key]; !ok && (schema != SchemaVersionV3 || key != "producer") {
			return Record{}, fail("unknown row field")
		}
	}
	if string(bytes.TrimSpace(raw["schema_version"])) != strconv.Itoa(schema) {
		return Record{}, fail("invalid schema version")
	}
	record := Record{SchemaVersion: schema}
	fields := map[string]*string{"revision": &record.Revision, "trace_id": &record.TraceID, "task": &record.Task, "outcome": &record.Outcome}
	if schema == SchemaVersionV3 {
		fields["producer"] = &record.Producer
	}
	for key, dst := range fields {
		if err := json.Unmarshal(raw[key], dst); err != nil || bytes.Equal(raw[key], []byte("null")) {
			return Record{}, fail("invalid string field")
		}
	}
	for key, dst := range map[string]*[]string{"opened_paths": &record.OpenedPaths, "changed_paths": &record.ChangedPaths} {
		if err := json.Unmarshal(raw[key], dst); err != nil || *dst == nil {
			return Record{}, fail("invalid path array")
		}
	}
	var items []jsontext.Value
	if err := json.Unmarshal(raw["verification"], &items); err != nil || items == nil || len(items) > MaxVerificationCommands {
		return Record{}, fail("invalid verification array")
	}
	if schema == SchemaVersionV3 && (len(items) == 0 || items[0][0] == '"') {
		record.Verification = make([]string, len(items))
		for i, item := range items {
			if item[0] != '"' || json.Unmarshal(item, &record.Verification[i]) != nil {
				return Record{}, fail("invalid verification command")
			}
		}
		return record, nil
	}
	for _, item := range items {
		var value map[string]jsontext.Value
		if err := json.Unmarshal(item, &value); err != nil || len(value) != 2 {
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

// typedFailure keeps schema-2 refusals in the argv reason class they always had;
// schema-3 row-shape refusals are plain unsupported-row errors.
func typedFailure(schema int) func(string) error {
	if schema == SchemaVersionV2 {
		return argvFailure
	}
	return func(reason string) error { return fmt.Errorf("schema-3 local trace: %s", reason) }
}

// DecodeTyped validates a schema-2 or schema-3 row's structure, bounds,
// screening, canonical values and identity. Repository witnesses remain the
// caller's responsibility.
func DecodeTyped(data []byte, revision string) (Record, error) {
	schema := SchemaVersionV2
	if typedSchema(data) == SchemaVersionV3 {
		schema = SchemaVersionV3
	}
	record, err := decodeTypedRow(data, schema)
	if err != nil {
		return Record{}, err
	}
	if err := validateTypedRecord(record, revision); err != nil {
		return Record{}, err
	}
	return record, nil
}

func validateTypedRecord(record Record, revision string) error {
	fail := typedFailure(record.SchemaVersion)
	switch record.SchemaVersion {
	case SchemaVersionV2:
		if record.Producer != "" || record.Verification != nil {
			return fail("schema or revision mismatch")
		}
	case SchemaVersionV3:
		if !ValidProducer(record.Producer) {
			return fail("invalid producer")
		}
		if (record.Verification == nil) == (record.TypedVerification == nil) {
			return fail("invalid verification shape")
		}
	default:
		return fail("schema or revision mismatch")
	}
	if record.Revision != revision {
		return fail("schema or revision mismatch")
	}
	if !validOutcome(record.Outcome) {
		return fail("invalid outcome")
	}
	task, err := validTask(safeText(record.Task, "trace task", MaxTaskCharacters))
	if err != nil || task != record.Task {
		return fail("invalid or noncanonical task")
	}
	for _, paths := range [][]string{record.OpenedPaths, record.ChangedPaths} {
		normalized, err := normalizePaths(paths, "trace paths")
		if err != nil || paths == nil || !equalStrings(normalized, paths) {
			return fail("invalid or noncanonical paths")
		}
	}
	if record.TypedVerification != nil {
		if err := validateEntries(record.TypedVerification); err != nil {
			return err
		}
	} else {
		commands, err := normalizeCommands(record.Verification)
		if err != nil {
			return err
		}
		if !equalStrings(commands, record.Verification) {
			return fail("verification commands are not normalized")
		}
	}
	expected, err := traceID(record)
	if err != nil || record.TraceID != expected {
		return fail("trace digest mismatch")
	}
	return nil
}

func writeVerification(b *bytes.Buffer, record Record) error {
	switch {
	case record.SchemaVersion == SchemaVersion, record.SchemaVersion == SchemaVersionV3 && record.TypedVerification == nil:
		return writeStringArray(b, record.Verification)
	case record.SchemaVersion != SchemaVersionV2 && record.SchemaVersion != SchemaVersionV3:
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
	if record.TypedVerification == nil {
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
	if record.TypedVerification == nil {
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

// VerificationCommands retains command entries when revalidating a typed input.
func (record Record) VerificationCommands() []string {
	if record.TypedVerification == nil {
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

// IsTypedJSON reports only the exact schema-2 or schema-3 discriminator; full
// admission needs DecodeTyped.
func IsTypedJSON(data []byte) bool {
	return typedSchema(data) != 0
}

func typedSchema(data []byte) int {
	var raw map[string]jsontext.Value
	if json.Unmarshal(data, &raw) != nil {
		return 0
	}
	switch strings.TrimSpace(string(raw["schema_version"])) {
	case "2":
		return SchemaVersionV2
	case "3":
		return SchemaVersionV3
	}
	return 0
}
