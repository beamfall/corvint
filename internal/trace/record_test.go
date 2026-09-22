package trace

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const (
	testRevision = "0123456789abcdef0123456789abcdef01234567"
	zeroRevision = "0000000000000000000000000000000000000000"
)

func TestPythonOracleCanonicalRows(t *testing.T) {
	// Goldens were emitted by src/context_corvint_trace.py at 06a6a09 using
	// _safe_text/_commands, _trace_id, and _canonical; Go output is never used
	// to derive the expected bytes.
	tests := []struct {
		name    string
		input   Input
		tracked []string
		wantID  string
		want    string
	}{
		{
			name: "BMP and astral strings",
			input: Input{Revision: testRevision, Task: "café ☃ 😀", OpenedPaths: []string{"z.go", "a.go", "a.go"},
				ChangedPaths: []string{"a.go"}, Verification: []string{"go test ./...", "git diff --check"}, Outcome: "passed"},
			tracked: []string{"a.go", "z.go"},
			wantID:  "5be94b69b7df0706b2aa769fcfc9d46d1fd439d7845023521681cd22ce0951bf",
			want:    "{\"changed_paths\":[\"a.go\"],\"opened_paths\":[\"a.go\",\"z.go\"],\"outcome\":\"passed\",\"revision\":\"0123456789abcdef0123456789abcdef01234567\",\"schema_version\":1,\"task\":\"caf\\u00e9 \\u2603 \\ud83d\\ude00\",\"trace_id\":\"5be94b69b7df0706b2aa769fcfc9d46d1fd439d7845023521681cd22ce0951bf\",\"verification\":[\"git diff --check\",\"go test ./...\"]}\n",
		},
		{
			name:   "DEL HTML and controls",
			input:  Input{Revision: zeroRevision, Task: "edge <&> \x7f \b\f\n\r\t end", Outcome: "blocked"},
			wantID: "a5b758b689435fbac4221e2acb0c7eb93b4aa1ee938a3066d44b4465b45ad611",
			want:   "{\"changed_paths\":[],\"opened_paths\":[],\"outcome\":\"blocked\",\"revision\":\"0000000000000000000000000000000000000000\",\"schema_version\":1,\"task\":\"edge <&> \\u007f \\b\\f\\n\\r\\t end\",\"trace_id\":\"a5b758b689435fbac4221e2acb0c7eb93b4aa1ee938a3066d44b4465b45ad611\",\"verification\":[]}\n",
		},
		{
			// Deliberate divergence from the retired Python writer, which sorted before
			// trimming and sealed ["z","a","a"] (trace ID 96f14057...), a row DecodeStore
			// refuses. Only this case pinned that ID. The ID is the sha256 of the basis.
			name:   "trim before command sort",
			input:  Input{Revision: zeroRevision, Task: "task", Verification: []string{" z", "a ", "a"}, Outcome: "passed"},
			wantID: "183c0fc18b04145f0f25a0c8e1c85521a8daaabf04b17f2cfa7c235ce467cbb3",
			want:   "{\"changed_paths\":[],\"opened_paths\":[],\"outcome\":\"passed\",\"revision\":\"0000000000000000000000000000000000000000\",\"schema_version\":1,\"task\":\"task\",\"trace_id\":\"183c0fc18b04145f0f25a0c8e1c85521a8daaabf04b17f2cfa7c235ce467cbb3\",\"verification\":[\"a\",\"z\"]}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, err := NewRecord(test.input, test.tracked)
			if err != nil {
				t.Fatal(err)
			}
			if record.TraceID != test.wantID {
				basis, _ := canonicalRecord(record, false)
				t.Fatalf("TraceID = %s, want %s; basis=%q", record.TraceID, test.wantID, basis)
			}
			got, err := Encode(record)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("row mismatch\n got: %q\nwant: %q", got, test.want)
			}
		})
	}
}

// LTPM-V0-011. One NewRecord pass trims verification commands before sorting
// and deduplicating them, so the row it writes is one DecodeStore accepts.
func TestNewRecordWritesAStoreAcceptedRowInOnePass(t *testing.T) {
	for _, verification := range [][]string{{" go test ./b", "go test ./a"}, {" go test ./a", "go test ./a"}} {
		record, err := NewRecord(Input{Revision: testRevision, Task: "task", Verification: verification, Outcome: "passed"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeStore(row, testRevision, nil); err != nil {
			t.Fatalf("DecodeStore refused single-pass row %q: %v", row, err)
		}
	}
}

// LTPM-V0-011 (decision 0246). Trace task text must be valid UTF-8: the
// recorder refuses a task holding an encoded lone surrogate, and the store
// reader refuses the escaped-surrogate row the retired Python writer produced.
func TestTraceTaskMustBeValidUTF8(t *testing.T) {
	for _, task := range []string{"lone-\xed\xa0\x80", "lone-\xed\xb2\x98"} {
		if _, err := NewRecord(Input{Revision: testRevision, Task: task, Outcome: "blocked"}, nil); err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
			t.Fatalf("NewRecord(%q) error = %v, want a valid UTF-8 refusal", task, err)
		}
	}
	stored := "{\"changed_paths\":[],\"opened_paths\":[],\"outcome\":\"blocked\",\"revision\":\"0000000000000000000000000000000000000000\",\"schema_version\":1,\"task\":\"lone-\\ud800\",\"trace_id\":\"30d46ff2b6588d7677328108e90a2f7357ec95f86e3069e580eefee654350cb8\",\"verification\":[]}\n"
	if _, err := DecodeStore([]byte(stored), zeroRevision, nil); err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
		t.Fatalf("DecodeStore accepted escaped lone surrogate task: %v", err)
	}
}

func TestNewRecordValidationBounds(t *testing.T) {
	paths200 := make([]string, MaxTracePaths)
	paths201 := make([]string, MaxTracePaths+1)
	for index := range paths201 {
		paths201[index] = "p/" + strings.Repeat("a", 3) + string(rune(0x100+index))
		if index < len(paths200) {
			paths200[index] = paths201[index]
		}
	}
	commands50 := make([]string, MaxVerificationCommands)
	commands51 := make([]string, MaxVerificationCommands+1)
	for index := range commands51 {
		commands51[index] = "go test ./case" + commandSuffix(index)
		if index < len(commands50) {
			commands50[index] = commands51[index]
		}
	}
	tests := []struct {
		name    string
		input   Input
		tracked []string
		wantErr string
	}{
		{"task boundary", Input{Revision: testRevision, Task: strings.Repeat("é", MaxTaskCharacters), Outcome: "passed"}, nil, ""},
		{"task overflow", Input{Revision: testRevision, Task: strings.Repeat("é", MaxTaskCharacters+1), Outcome: "passed"}, nil, "exceeds 2000"},
		{"path boundary", Input{Revision: testRevision, Task: "task", OpenedPaths: paths200, Outcome: "passed"}, paths200, ""},
		{"path overflow", Input{Revision: testRevision, Task: "task", OpenedPaths: paths201, Outcome: "passed"}, paths201, "exceeds 200 paths"},
		{"command boundary count", Input{Revision: testRevision, Task: "task", Verification: commands50, Outcome: "passed"}, nil, ""},
		{"command overflow count", Input{Revision: testRevision, Task: "task", Verification: commands51, Outcome: "passed"}, nil, "exceeds 50 commands"},
		{"command character boundary", Input{Revision: testRevision, Task: "task", Verification: []string{strings.Repeat("a", MaxCommandCharacters)}, Outcome: "passed"}, nil, ""},
		{"command character overflow", Input{Revision: testRevision, Task: "task", Verification: []string{strings.Repeat("a", MaxCommandCharacters+1)}, Outcome: "passed"}, nil, "exceeds 512"},
		{"secret task", Input{Revision: testRevision, Task: "token=abcdefghijklmnopqrstuvwxyz", Outcome: "passed"}, nil, "secret-shaped"},
		{"unicode-space secret", Input{Revision: testRevision, Task: "token\u00a0=\u2003abcdefghijklmnopqrstuvwxyz", Outcome: "passed"}, nil, "secret-shaped"},
		{"secret path", Input{Revision: testRevision, Task: "task", OpenedPaths: []string{"token=abcdefghijklmnopqrstuvwxyz"}, Outcome: "passed"}, []string{"token=abcdefghijklmnopqrstuvwxyz"}, "secret-shaped"},
		{"forbidden path", Input{Revision: testRevision, Task: "task", OpenedPaths: []string{".git/config"}, Outcome: "passed"}, []string{".git/config"}, "forbidden path"},
		{"untracked path", Input{Revision: testRevision, Task: "task", OpenedPaths: []string{"a.go"}, Outcome: "passed"}, nil, "not tracked"},
		{"unsafe command", Input{Revision: testRevision, Task: "task", Verification: []string{"go test; false"}, Outcome: "passed"}, nil, `unsupported shell syntax: byte 7 ';' is outside the admitted set of ASCII letters, digits, and "_./:@=+, -"`},
		{"bad outcome", Input{Revision: testRevision, Task: "task", Outcome: "unknown"}, nil, "one of"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, err := NewRecord(test.input, test.tracked)
			if test.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if record.OpenedPaths == nil || record.ChangedPaths == nil || record.Verification == nil {
					t.Fatal("nil slices were not normalized to []")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

// LTPM-V0-011.
// TestTracePathScreenIsTheIndexScreen pins LTPM-V0-011's 2026-09-12 amendment
// (decision 0102): supplied and stored trace paths are forbidden by exactly the
// IDX-SNAP-V0-018 screen. `.claude` discriminates it from the ten-member set the
// trace package used to copy; whole-component matching keeps look-alikes admitted.
func TestTracePathScreenIsTheIndexScreen(t *testing.T) {
	for _, test := range []struct {
		value     string
		forbidden bool
	}{
		{".claude/worktrees/a/main.go", true},
		{"build/main.go", true},
		{"internal/store/migrate/0001.sql", true},
		{"gen/api.go", true},
		{"internal/claude/main.go", false},
		{"builds/main.go", false},
	} {
		if got := contextindex.ForbiddenPathReason(test.value) != ""; got != test.forbidden {
			t.Fatalf("IDX-SNAP-V0-018 screen(%q) = %v, want %v", test.value, got, test.forbidden)
		}
		_, recordErr := NewRecord(Input{Revision: testRevision, Task: "task", OpenedPaths: []string{test.value}, Outcome: "passed"}, []string{test.value})
		_, storedErr := normalizeStoredPaths([]string{test.value}, "opened_paths")
		for surface, err := range map[string]error{"record": recordErr, "stored": storedErr} {
			if got := err != nil && strings.Contains(err.Error(), "forbidden path"); got != test.forbidden {
				t.Fatalf("%s trace screen(%q) forbidden = %v (err=%v), want %v", surface, test.value, got, err, test.forbidden)
			}
		}
	}
}

func TestAdmissibleCurrentPathsSharesRecordAdmission(t *testing.T) {
	tracked := []string{"internal/a.go", "internal/b.go"}
	admitted, err := AdmissibleCurrentPaths([]string{".gitignore", "internal/b.go", "internal/a.go"}, tracked)
	if err != nil || !reflect.DeepEqual(admitted, tracked) {
		t.Fatalf("admitted=%v error=%v", admitted, err)
	}
	if _, err := AdmissibleCurrentPaths([]string{"bad\xff.txt"}, tracked); err == nil {
		t.Fatal("malformed non-source candidate was filtered instead of rejected")
	} else if reason := AdmissionFailureReason(err); reason != "malformed-path" {
		t.Fatalf("malformed reason=%q error=%v", reason, err)
	}
	tooMany := make([]string, MaxTracePaths+1)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("internal/%03d.go", index)
	}
	if _, err := AdmissibleCurrentPaths(tooMany, tooMany); err == nil {
		t.Fatal("admitted subset above the record bound was accepted")
	} else if reason := AdmissionFailureReason(err); reason != "admitted-path-limit" {
		t.Fatalf("admitted limit reason=%q error=%v", reason, err)
	}
}

func TestDecodeStorePythonInputSemantics(t *testing.T) {
	canonical := "{\"changed_paths\":[],\"opened_paths\":[],\"outcome\":\"passed\",\"revision\":\"0000000000000000000000000000000000000000\",\"schema_version\":1,\"task\":\"task\",\"trace_id\":\"05fc51042e891a9dc394e3e91f58944c13c29dd947e9a3c4184de5e2f22f50b0\",\"verification\":[]}\n"
	tests := []struct {
		name string
		row  []byte
	}{
		{"canonical", []byte(canonical)},
		{"CRLF", []byte(strings.TrimSuffix(canonical, "\n") + "\r\n")},
		{"final no LF", []byte(strings.TrimSuffix(canonical, "\n"))},
		{"schema float equality", []byte(strings.Replace(canonical, "\"schema_version\":1", "\"schema_version\":1.0", 1))},
		{"schema boolean equality", []byte(strings.Replace(canonical, "\"schema_version\":1", "\"schema_version\":true", 1))},
		{"duplicate key last wins", []byte(strings.Replace(canonical, "\"task\":\"task\"", "\"task\":\"wrong\",\"task\":\"task\"", 1))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			records, err := DecodeStore(test.row, zeroRevision, nil)
			if err != nil || len(records) != 1 || records[0].Task != "task" {
				t.Fatalf("records = %+v, error = %v", records, err)
			}
		})
	}
}

func TestStoredV1RowsRemainValidAfterWriterSecretScreenExpansion(t *testing.T) {
	record := Record{
		SchemaVersion: SchemaVersion,
		Revision:      testRevision,
		Task:          "credential=abcdefghijklmnopqrstuvwxyz",
		OpenedPaths:   []string{"config/passphrase=correct-horse-battery-staple"},
		ChangedPaths:  []string{"config/credentials=abcdefghijklmnopqrstuvwxyz"},
		Verification:  []string{"echo database.passwd=abcdefghijklmnopqrstuvwxyz"},
		Outcome:       "passed",
	}
	var err error
	record.TraceID, err = traceID(record)
	if err != nil {
		t.Fatal(err)
	}
	tracked := []string{record.OpenedPaths[0], record.ChangedPaths[0]}
	encoded, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeStore(encoded, testRevision, tracked)
	if err != nil || len(decoded) != 1 || !reflect.DeepEqual(decoded[0], record) {
		t.Fatalf("DecodeStore() = (%+v, %v), want stored-v1 row", decoded, err)
	}
	if err := validateUnreachableRecord(record, testRevision); err != nil {
		t.Fatalf("validateUnreachableRecord() rejected stored-v1 row: %v", err)
	}

	writes := []Input{
		{Revision: testRevision, Task: record.Task, Outcome: "passed"},
		{Revision: testRevision, Task: "task", OpenedPaths: record.OpenedPaths, Outcome: "passed"},
		{Revision: testRevision, Task: "task", ChangedPaths: record.ChangedPaths, Outcome: "passed"},
		{Revision: testRevision, Task: "task", Verification: record.Verification, Outcome: "passed"},
	}
	for index, input := range writes {
		if _, err := NewRecord(input, tracked); err == nil || !strings.Contains(err.Error(), "secret-shaped") {
			t.Fatalf("NewRecord() case %d error=%v, want secret-shaped rejection", index, err)
		}
	}
}

// LTA-V0-004: record refuses each writer-only shape the retired oracle admitted.
func TestLTAV0004RecordRefusesWriterOnlySecretShapes(t *testing.T) {
	for _, task := range []string{
		`password = "correct horse battery"`,
		"export DB_PASS='synthetic-example-value'",
		"whsec_abcdefghijklmnopqrstuvwxyz0123456789",
		"hf_abcdefghijklmnopqrstuvwxyz0123",
		"dop_v1_0123456789abcdef0123456789abcdef",
		"xapp-1-A0123456789-0123456789012",
		"https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
		"AKIAABCDEFGHIJKLMNOP abcdefghijklmnopqrstuvwxyz0123456789ABCD",
	} {
		input := Input{Revision: testRevision, Task: task, Outcome: "passed"}
		if _, err := NewRecord(input, nil); err == nil || !strings.Contains(err.Error(), "secret-shaped") {
			t.Errorf("NewRecord(%q) error=%v, want secret-shaped rejection", task, err)
		}
	}
}

// EAF-V0-001: regression for the authorized audit follow-up.
func TestQuotedCredentialsRejectNewRecordsButRetainStoredV1(t *testing.T) {
	t.Run("EAF-V0-001", func(t *testing.T) {
		for _, task := range []string{`{"password":"synthetic-example-value"}`, `{"api_key":"synthetic-example-value"}`, `{"password":"top secret value"}`, `{"password":"top \"secret\" value"}`, `{"password":"top secret value`, `{"password":"top secret value\`} {
			input := Input{Revision: testRevision, Task: task, Outcome: "passed"}
			if _, err := NewRecord(input, nil); err == nil || !strings.Contains(err.Error(), "secret-shaped") {
				t.Errorf("NewRecord(%q) error=%v, want secret-shaped rejection", task, err)
			}
			record := Record{SchemaVersion: SchemaVersion, Revision: testRevision, Task: task,
				OpenedPaths: []string{}, ChangedPaths: []string{}, Verification: []string{}, Outcome: "passed"}
			var err error
			record.TraceID, err = traceID(record)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := Encode(record)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeStore(encoded, testRevision, nil)
			if err != nil || len(decoded) != 1 || !reflect.DeepEqual(decoded[0], record) {
				t.Fatalf("historical quoted row changed: %+v, %v", decoded, err)
			}
			if err := validateUnreachableRecord(record, testRevision); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := NewRecord(Input{Revision: testRevision, Task: `{"task":"repair config"}`, Outcome: "passed"}, nil); err != nil {
			t.Fatalf("benign JSON rejected: %v", err)
		}
	})
}

func TestDecodeStoreRejectsMalformedInput(t *testing.T) {
	base := "{\"changed_paths\":[],\"opened_paths\":[],\"outcome\":\"passed\",\"revision\":\"0000000000000000000000000000000000000000\",\"schema_version\":1,\"task\":\"task\",\"trace_id\":\"05fc51042e891a9dc394e3e91f58944c13c29dd947e9a3c4184de5e2f22f50b0\",\"verification\":[]}\n"
	tests := []struct {
		name    string
		row     []byte
		wantErr string
	}{
		{"invalid UTF-8", append([]byte("{\"task\":\""), 0xff), "encoding"},
		{"unknown field", []byte(strings.Replace(base, "\"task\"", "\"unknown\"", 1)), "fields"},
		{"wrong schema", []byte(strings.Replace(base, "\"schema_version\":1", "\"schema_version\":2", 1)), "schema"},
		{"mixed revision", []byte(strings.Replace(base, zeroRevision, strings.Repeat("1", 40), 1)), "mixed revision"},
		{"empty task", []byte(strings.Replace(base, "\"task\":\"task\"", "\"task\":\" \"", 1)), "non-empty"},
		{"invalid outcome", []byte(strings.Replace(base, "\"outcome\":\"passed\"", "\"outcome\":\"unknown\"", 1)), "one of"},
		{"untracked path", []byte(strings.Replace(base, "\"opened_paths\":[]", "\"opened_paths\":[\"a.go\"]", 1)), "not tracked"},
		{"unsafe command", []byte(strings.Replace(base, "\"verification\":[]", "\"verification\":[\"go test; false\"]", 1)), "unsupported shell"},
		{"digest", []byte(strings.Replace(base, "05fc51042e891a9dc394e3e91f58944c13c29dd947e9a3c4184de5e2f22f50b0", strings.Repeat("0", 64), 1)), "digest mismatch"},
		{"blank row", []byte("\n"), "invalid local trace JSON"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeStore(test.row, zeroRevision, nil)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

// LTPM-V0-011. Decision 0235: trace paths follow corvint-dashboard-trace-path-witness/0, so
// the recorder refuses, and the store reader refuses a planted row naming, a
// path the dashboard trace adapter would refuse.
func TestTracePathsFollowTheDashboardWitnessProfile(t *testing.T) {
	longest := strings.Repeat("a", 4_096)
	for _, value := range []string{"a\x7f.go", "a\u0085.go", "a\xed\xa0\x80.go", `a\b.go`, "C:a.go", longest + "a"} {
		_, err := NewRecord(Input{Revision: testRevision, Task: "task", OpenedPaths: []string{value}, Outcome: "passed"}, []string{value})
		if AdmissionFailureReason(err) != "malformed-path" {
			t.Fatalf("NewRecord(%q) error = %v, want malformed-path", value, err)
		}
		planted := Record{SchemaVersion: SchemaVersion, Revision: testRevision, Task: "task",
			OpenedPaths: []string{value}, ChangedPaths: []string{}, Verification: []string{}, Outcome: "passed"}
		planted.TraceID, err = traceID(planted)
		if err != nil {
			t.Fatal(err)
		}
		row, err := Encode(planted)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeStore(row, testRevision, []string{value}); err == nil || !strings.Contains(err.Error(), "invalid path") {
			t.Fatalf("DecodeStore accepted planted path %q: %v", value, err)
		}
	}
	if _, err := NewRecord(Input{Revision: testRevision, Task: "task", OpenedPaths: []string{longest}, Outcome: "passed"}, []string{longest}); err != nil {
		t.Fatalf("4,096-byte path refused: %v", err)
	}
}

func TestPhysicalRowAndStoreBounds(t *testing.T) {
	tests := []struct {
		name          string
		data          []byte
		remainingRows int
		remainingSize int
		wantErr       string
	}{
		{"row inclusive", append(bytes.Repeat([]byte{' '}, MaxTraceRowBytes-1), '\n'), 1, MaxTraceStoreBytes, ""},
		{"row overflow", bytes.Repeat([]byte{' '}, MaxTraceRowBytes+1), 1, MaxTraceStoreBytes, "row"},
		{"row count inclusive", bytes.Repeat([]byte("{}\n"), MaxTraces), MaxTraces, MaxTraceStoreBytes, ""},
		{"row count overflow", bytes.Repeat([]byte("{}\n"), MaxTraces+1), MaxTraces, MaxTraceStoreBytes, "1000 rows"},
		{"store byte overflow", bytes.Repeat(append(bytes.Repeat([]byte{' '}, MaxTraceRowBytes-1), '\n'), MaxTraceStoreBytes/MaxTraceRowBytes+1), MaxTraces, MaxTraceStoreBytes, "16777216 bytes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := splitRows(test.data, test.remainingRows, test.remainingSize, "fixture")
			if test.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestPythonOracleMigrationTransform(t *testing.T) {
	// source and want were emitted by the Python _canonical_commit_bytes oracle.
	source := "{\"changed_paths\":[\"a.go\"],\"opened_paths\":[\"a.go\"],\"outcome\":\"passed\",\"revision\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\",\"schema_version\":1,\"task\":\"migrate caf\\u00e9\",\"trace_id\":\"ce7e06a7e098700cf9cae733ae54202257e37cb31e70f1fbaef48305becf7bde\",\"verification\":[\"go test ./...\"]}\n"
	want := "{\"changed_paths\":[\"a.go\"],\"opened_paths\":[\"a.go\"],\"outcome\":\"passed\",\"revision\":\"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\",\"schema_version\":1,\"task\":\"migrate caf\\u00e9\",\"trace_id\":\"4831361487118fdc2df2d3e48c7ace5bee42f2245f7de939aa33d0e2d5dadd21\",\"verification\":[\"go test ./...\"]}\n"
	got, err := TransformLegacy([]byte(source), strings.Repeat("a", 40), strings.Repeat("b", 40), []string{"a.go"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("migration mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestMigrationTransformRejectsExpandedTargetStore(t *testing.T) {
	treeRevision := strings.Repeat("a", 40)
	commitRevision := strings.Repeat("b", 40)
	// Twenty 4,096-byte paths (the trace path bound) per row, each raw "é"
	// growing from two bytes to six when escaped.
	trackedPaths := make([]string, 20)
	for index := range trackedPaths {
		trackedPaths[index] = "p" + commandSuffix(index) + "/" + strings.Repeat("é", 2_046)
	}
	var source bytes.Buffer
	for index := 0; index < 75; index++ {
		record := mustRecord(t, Input{Revision: treeRevision, Task: "task " + commandSuffix(index), OpenedPaths: trackedPaths, Outcome: "passed"}, trackedPaths)
		row, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		source.Write(bytes.ReplaceAll(row, []byte(`\u00e9`), []byte("é")))
	}
	if source.Len() >= MaxTraceStoreBytes {
		t.Fatalf("source fixture = %d bytes, must be below source bound", source.Len())
	}
	_, err := TransformLegacy(source.Bytes(), treeRevision, commitRevision, trackedPaths)
	if err == nil || !strings.Contains(err.Error(), "exceeds 16777216 bytes") {
		t.Fatalf("error = %v", err)
	}
}

func commandSuffix(value int) string {
	return string(rune('a'+value/26)) + string(rune('a'+value%26))
}
