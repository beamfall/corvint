package main

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func manifestBytes(t testing.TB, sources []any) []byte {
	t.Helper()
	value := map[string]any{
		"configuredSources": sources,
		"generatedAt":       "2026-08-23T20:00:00.000000000Z",
		"schema":            traceManifestSchema,
	}
	raw, err := canonical(value, true)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTraceCLIArguments(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("unqualified platform")
	}
	root := filepath.Join(string(filepath.Separator), "tmp", "café")
	arguments := []string{"verify", "--root", root, "--manifest", filepath.Join(root, "manifest.json"), "--snapshot", filepath.Join(root, "snapshot.json")}
	parsed, err := parseTraceCLIArguments(arguments)
	if err != nil || parsed.root != root {
		t.Fatalf("parsed = %+v, %v", parsed, err)
	}
	bound := "/" + strings.Repeat("a", 4095)
	if runtime.GOOS == "windows" {
		bound = `C:\` + strings.Repeat("a", 4093)
	}
	if len(bound) != 4096 || !safeAbsoluteArgument(bound) || safeAbsoluteArgument(bound+"a") {
		t.Fatal("absolute argument 4096/4097 byte boundary violated")
	}
	for _, hostile := range [][]string{
		arguments[:6],
		{"verify", "--root", "relative", "--manifest", arguments[4], "--snapshot", arguments[6]},
		{"verify", "--root", root, "--manifest", filepath.Join(root, strings.Repeat("a", 4097)), "--snapshot", arguments[6]},
	} {
		if _, err := parseTraceCLIArguments(hostile); err == nil {
			t.Fatalf("hostile arguments accepted: %q", hostile)
		}
	}
}

func traceSource(adapter, ordinal string, relativePath any) map[string]any {
	return map[string]any{"adapterId": adapter, "configuredOrdinal": ordinal, "relativePath": relativePath}
}

func TestTraceManifestValid(t *testing.T) {
	raw := manifestBytes(t, []any{
		traceSource("local-trace-v1", "0", ".context-corvint/traces"),
		traceSource("query-envelope-v1", "0", nil),
	})
	manifest, err := parseTraceManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.configuredSources) != 2 || manifest.configuredSources[0].relativePath != ".context-corvint/traces" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestTraceManifestAcceptsSafeUTF8Path(t *testing.T) {
	raw := manifestBytes(t, []any{traceSource("local-trace-v1", "0", "café/traces")})
	manifest, err := parseTraceManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.configuredSources[0].relativePath; got != "café/traces" {
		t.Fatalf("relative path = %q", got)
	}
}

func TestTraceManifestRejectsHostileInputs(t *testing.T) {
	valid := manifestBytes(t, []any{traceSource("local-trace-v1", "0", ".context-corvint/traces")})
	tests := []struct {
		name string
		raw  []byte
		code rejectCode
	}{
		{"absolute", manifestBytes(t, []any{traceSource("local-trace-v1", "0", "/tmp/traces")}), rejectPrivacyText},
		{"traversal", manifestBytes(t, []any{traceSource("local-trace-v1", "0", "safe/../traces")}), rejectPrivacyText},
		{"empty-component", manifestBytes(t, []any{traceSource("local-trace-v1", "0", "safe//traces")}), rejectPrivacyText},
		{"control", manifestBytes(t, []any{traceSource("local-trace-v1", "0", "safe/\u0085traces")}), rejectPrivacyText},
		{"volume", manifestBytes(t, []any{traceSource("local-trace-v1", "0", "C:/traces")}), rejectPrivacyText},
		{"over-bound", manifestBytes(t, []any{traceSource("local-trace-v1", "0", strings.Repeat("a", 4097))}), rejectPrivacyText},
		{"unsupported-path", manifestBytes(t, []any{traceSource("query-envelope-v1", "0", "query.json")}), rejectEnum},
		{"missing-supported-path", manifestBytes(t, []any{traceSource("local-trace-v1", "0", nil)}), rejectPrivacyText},
		{"ordinal-gap", manifestBytes(t, []any{traceSource("local-trace-v1", "1", ".context-corvint/traces")}), rejectOrdering},
		{"duplicate", manifestBytes(t, []any{traceSource("local-trace-v1", "0", "a"), traceSource("local-trace-v1", "0", "b")}), rejectOrdering},
		{"wrong-order", manifestBytes(t, []any{traceSource("query-envelope-v1", "0", nil), traceSource("local-trace-v1", "0", ".context-corvint/traces")}), rejectOrdering},
		{"json-number", bytes.Replace(valid, []byte(`"configuredOrdinal":"0"`), []byte(`"configuredOrdinal":0`), 1), rejectType},
		{"noncanonical", append([]byte{' '}, valid...), rejectCanonical},
		{"unknown-field", bytes.Replace(valid, []byte(`,"generatedAt":`), []byte(`,"extra":null,"generatedAt":`), 1), rejectFieldSet},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseTraceManifest(test.raw)
			if got := rejectionCode(err); got != test.code {
				t.Fatalf("rejection = %s (%v), want %s", got, err, test.code)
			}
		})
	}
}
