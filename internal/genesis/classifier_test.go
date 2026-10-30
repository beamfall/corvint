package genesis

import (
	"encoding/json"
	"reflect"
	"testing"
)

const oracleOID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestClassifyEntryOrderedCascade(t *testing.T) {
	// Expected classification/reason pairs were captured from the Python oracle at 06a6a09.
	text := []byte("text")
	cases := []struct {
		name       string
		entry      Entry
		blobs      map[string][]byte
		exhausted  map[string]struct{}
		exclusions []string
		wantClass  string
		wantReason string
	}{
		{"unsafe path precedes gitlink", testEntry(nil, "160000", "commit", nil), nil, nil, nil, "UNSUPPORTED", "unsafe-or-non-utf8-path"},
		{"caller exclusion precedes binary asset", testEntry(pointer("assets/logo.PNG"), "100644", "blob", int64Pointer(4)), map[string][]byte{oracleOID: text}, nil, []string{"assets"}, "EXCLUDED", "caller-declared-prefix"},
		{"gitlink", testEntry(pointer("deps/sub"), "160000", "commit", nil), nil, nil, nil, "UNSUPPORTED", "gitlink"},
		{"symlink", testEntry(pointer("link.txt"), "120000", "blob", int64Pointer(4)), map[string][]byte{oracleOID: text}, nil, nil, "UNSUPPORTED", "symlink"},
		{"special mode", testEntry(pointer("src/app.py"), "100600", "blob", int64Pointer(4)), map[string][]byte{oracleOID: text}, nil, nil, "UNSUPPORTED", "special-tree-entry"},
		{"special object type", testEntry(pointer("src/app.py"), "100644", "commit", int64Pointer(4)), map[string][]byte{oracleOID: text}, nil, nil, "UNSUPPORTED", "special-tree-entry"},
		{"blob size unavailable", testEntry(pointer("src/app.py"), "100644", "blob", nil), nil, nil, nil, "UNKNOWN", "blob-size-unavailable"},
		{"binary asset uses Python lowercase", testEntry(pointer("assets/logo.PNG"), "100644", "blob", int64Pointer(4)), map[string][]byte{oracleOID: text}, nil, nil, "UNSUPPORTED", "binary-asset"},
		{"blob too large", testEntry(pointer("src/app.py"), "100644", "blob", int64Pointer(5)), map[string][]byte{oracleOID: []byte("text!")}, nil, nil, "UNSUPPORTED", "blob-too-large"},
		{"blob budget exhausted", testEntry(pointer("src/app.py"), "100644", "blob", int64Pointer(4)), map[string][]byte{oracleOID: text}, map[string]struct{}{oracleOID: {}}, nil, "UNKNOWN", "blob-budget-exhausted"},
		{"blob unavailable", testEntry(pointer("src/app.py"), "100644", "blob", int64Pointer(4)), nil, nil, nil, "UNKNOWN", "blob-unavailable"},
		{"invalid UTF-8", testEntry(pointer("src/app.py"), "100644", "blob", int64Pointer(2)), map[string][]byte{oracleOID: {0xff, 0xfe}}, nil, nil, "UNSUPPORTED", "non-utf8-or-binary"},
		{"embedded zero octet", testEntry(pointer("src/app.py"), "100644", "blob", int64Pointer(3)), map[string][]byte{oracleOID: {'a', 0, 'b'}}, nil, nil, "UNSUPPORTED", "binary-content"},
		{"tracked text", testEntry(pointer("src/app.py"), "100755", "blob", int64Pointer(4)), map[string][]byte{oracleOID: text}, nil, nil, "INCLUDED", "tracked-text"},
		{"present empty blob", testEntry(pointer("empty.txt"), "100644", "blob", int64Pointer(0)), map[string][]byte{oracleOID: nil}, nil, nil, "INCLUDED", "tracked-text"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := ClassifyEntry(test.entry, test.blobs, test.exhausted, test.exclusions, Limits{MaxBlobBytes: 4})
			if got.Classification != test.wantClass || got.Reason != test.wantReason {
				t.Fatalf("classification/reason = %s/%s, want %s/%s", got.Classification, got.Reason, test.wantClass, test.wantReason)
			}
		})
	}
}

func TestClassifyEntryRecordAndPythonPathSemantics(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		wantClasses []string
	}{
		{"Unicode full lowercase does not invent ASCII suffix", "x.GİF", []string{"OTHER_TEXT"}},
		{"hidden name has no suffix", ".png", []string{"OTHER_TEXT"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			entry := testEntry(pointer(test.path), "100644", "blob", int64Pointer(4))
			got := ClassifyEntry(entry, map[string][]byte{oracleOID: []byte("text")}, nil, nil, Limits{MaxBlobBytes: 4})
			if got.Classification != "INCLUDED" || got.Reason != "tracked-text" || !reflect.DeepEqual(got.Classes, test.wantClasses) {
				t.Fatalf("record = %#v", got)
			}
		})
	}

	unsafe := ClassifyEntry(testEntry(nil, "160000", "commit", nil), nil, nil, nil, Limits{MaxBlobBytes: 4})
	serialized, err := json.Marshal(unsafe)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"boundaries":[],"classification":"UNSUPPORTED","classes":[],"mode":"160000","objectType":"commit","oid":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","path":null,"pathSha256":"path-sha","reason":"unsafe-or-non-utf8-path","roles":[],"size":null}`
	if string(serialized) != want {
		t.Fatalf("serialized record = %s, want %s", serialized, want)
	}

	path := "src/app.py"
	size := int64(4)
	record := ClassifyEntry(testEntry(&path, "100644", "blob", &size), map[string][]byte{oracleOID: []byte("text")}, nil, nil, Limits{MaxBlobBytes: 4})
	path = "mutated"
	size = 99
	if *record.Path != "src/app.py" || *record.Size != 4 {
		t.Fatalf("record aliases caller metadata: path=%q size=%d", *record.Path, *record.Size)
	}
}

func TestSourceClassesMatchPythonOracleOrder(t *testing.T) {
	cases := map[string][]string{
		"AGENTS.md":                       {"INSTRUCTIONS", "DOCUMENTATION"},
		"docs/rfcs/auth-rfc.md":           {"DOCUMENTATION", "SPECIFICATION"},
		"tests/e2e/login.spec.ts":         {"TEST", "E2E_TEST", "CODE"},
		".github/workflows/ci.yml":        {"CI", "CONFIGURATION"},
		"pyproject.toml":                  {"MANIFEST", "CONFIGURATION"},
		"uv.lock":                         {"LOCKFILE", "CONFIGURATION"},
		"CODEOWNERS":                      {"OWNERSHIP"},
		"docs/runbooks/login-incident.md": {"DOCUMENTATION", "RUNBOOK", "INCIDENT"},
		"ops/runbooks/deploy.sh":          {"DOCUMENTATION", "RUNBOOK", "CODE"},
		"incidents/2026.sh":               {"DOCUMENTATION", "INCIDENT", "CODE"},
		"schema/api.proto":                {"SCHEMA"},
		"config/app.yaml":                 {"CONFIGURATION"},
		"assets/icon.svg":                 {"ASSET"},
		"src/example.test.txt":            {"DOCUMENTATION", "TEST"},
		"src/example.spec.txt":            {"DOCUMENTATION", "SPECIFICATION"},
		"src/file.KT":                     {"CODE"},
		"plain":                           {"OTHER_TEXT"},
	}
	for path, want := range cases {
		if got := sourceClasses(path); !reflect.DeepEqual(got, want) {
			t.Errorf("sourceClasses(%q) = %v, want %v", path, got, want)
		}
	}
	if got := SourceClassOrder(); !reflect.DeepEqual(got, []string{"INSTRUCTIONS", "DOCUMENTATION", "SPECIFICATION", "TEST", "E2E_TEST", "CI", "MANIFEST", "LOCKFILE", "OWNERSHIP", "RUNBOOK", "INCIDENT", "SCHEMA", "CONFIGURATION", "CODE", "ASSET", "OTHER_TEXT"}) {
		t.Fatalf("source class order = %v", got)
	}
	if got := Classifications(); !reflect.DeepEqual(got, []string{"INCLUDED", "EXCLUDED", "UNSUPPORTED", "UNKNOWN"}) {
		t.Fatalf("classifications = %v", got)
	}
}

func TestBoundariesRolesAndExclusionsMatchPythonOracle(t *testing.T) {
	path := "vendor/generated/client_generated.go"
	if got, want := pathBoundaries(path), []string{"GENERATED_CANDIDATE", "VENDOR_CANDIDATE"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pathBoundaries(%q) = %v, want %v", path, got, want)
	}
	if got := BoundaryOrder(); !reflect.DeepEqual(got, []string{"GENERATED_CANDIDATE", "VENDOR_CANDIDATE"}) {
		t.Fatalf("boundary order = %v", got)
	}
	path = "cmd/worker_route.go"
	if got, want := pathRoles(path), []string{"ENTRYPOINT_CANDIDATE", "JOB_CANDIDATE", "ROUTE_CANDIDATE"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pathRoles(%q) = %v, want %v", path, got, want)
	}
	for _, test := range []struct {
		path     string
		prefixes []string
		want     bool
	}{
		{"scratch", []string{"scratch"}, true},
		{"scratch/ignored.py", []string{"scratch"}, true},
		{"scratchpad/kept.py", []string{"scratch"}, false},
		{"Scratch/ignored.py", []string{"scratch"}, false},
		{"src/app.py", nil, false},
	} {
		if got := matchesExclusion(test.path, test.prefixes); got != test.want {
			t.Errorf("matchesExclusion(%q, %v) = %t, want %t", test.path, test.prefixes, got, test.want)
		}
	}
}

func testEntry(path *string, mode, objectType string, size *int64) Entry {
	return Entry{Mode: mode, ObjectType: objectType, OID: oracleOID, Path: path, PathSHA256: "path-sha", Size: size}
}

func pointer(value string) *string {
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}
