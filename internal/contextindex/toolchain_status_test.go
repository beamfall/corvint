package contextindex

import (
	"strings"
	"testing"
)

func toolchainTestRecord(t *testing.T, path, body string, extra ...string) Record {
	t.Helper()
	source := Source{Path: path, BlobHash: strings.Repeat("0", 40), Data: []byte(body)}
	sources := map[string]Source{path: source}
	for _, other := range extra {
		sources[other] = Source{Path: other, BlobHash: strings.Repeat("1", 40)}
	}
	text, valid, loaded := source.Text()
	if !loaded || !valid {
		t.Fatalf("%s: test source is not loaded valid text", path)
	}
	record, ok := documentRecord(source, text, sources)
	if !ok {
		t.Fatalf("%s: document was not recognized", path)
	}
	return record
}

func TestSTAV0007_ToolchainPathClassification(t *testing.T) {
	const specify = ".specify/templates/spec-template.md"
	cases := []struct {
		name, path, body  string
		extra             []string
		kind, status      string
		native, authority string
		authoritative     bool
	}{
		{"openspec current", "openspec/specs/auth/spec.md", "# Auth\n\nStatus: rejected\n", nil,
			"spec", "current", "", "accepted-spec", true},
		{"openspec nested current", "openspec/specs/auth/login/spec.md", "# Login\n", nil,
			"spec", "current", "", "accepted-spec", true},
		{"openspec proposed delta", "openspec/changes/add-2fa/specs/auth/spec.md", "# Delta\n\nStatus: accepted\n", nil,
			"spec", "proposed", "", "repository-spec", false},
		{"openspec archived", "openspec/changes/archive/2026-01-02-add-2fa/specs/auth/spec.md", "# Old\n\nStatus: accepted\n", nil,
			"spec", "archived", "2026-01-02", "repository-spec", false},
		{"openspec archive without date", "openspec/changes/archive/add-2fa/specs/auth/spec.md", "# Old\n\nStatus: current\n", nil,
			"spec", "archived", "", "repository-spec", false},
		{"spec kit status header", "specs/001-login/spec.md", "# Feature\n\n**Feature Branch**: `001-login`\n**Status**: Approved\n", []string{specify},
			"spec", "approved", "Approved", "accepted-spec", true},
		{"spec kit draft header", "specs/001-login/spec.md", "# Feature\n\n**Status**: Draft\n", []string{specify},
			"spec", "draft", "Draft", "repository-spec", false},
		{"spec kit header in fence ignored", "specs/001-login/spec.md", "# Feature\n\n```\n**Status**: Approved\n```\n", []string{specify},
			"spec", "draft", "", "repository-spec", false},
		{"spec kit absent header defaults draft", "specs/001-login/spec.md", "# Feature\n\nStatus: accepted\n", []string{specify},
			"spec", "draft", "", "repository-spec", false},
		{"specs without .specify keeps content status", "specs/001-login/spec.md", "# Feature\n\nStatus: accepted\n", nil,
			"spec", "accepted", "", "accepted-spec", true},
		{"spec kit body status below level-2 ignored", "specs/001-login/spec.md", "# Feature\n\n## Requirements\n\n**Status**: Approved\n", []string{specify},
			"spec", "draft", "", "repository-spec", false},
		{"openspec archived instructions not binding", "openspec/changes/archive/2026-10-01-old/AGENTS.md", "# Old agents\n", nil,
			"instructions", "archived", "2026-10-01", "project-instructions", false},
		{"openspec proposed instructions not binding", "openspec/changes/add-2fa/AGENTS.md", "# Agents\n", nil,
			"instructions", "proposed", "", "project-instructions", false},
		{"spec kit constitution", ".specify/memory/constitution.md", "# Constitution\n", nil,
			"instructions", "", "", "project-instructions", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			record := toolchainTestRecord(t, test.path, test.body, test.extra...)
			if record.Kind != test.kind || record.Fields["status"] != test.status {
				t.Fatalf("kind/status = %q/%q, want %q/%q", record.Kind, record.Fields["status"], test.kind, test.status)
			}
			native, _ := record.Fields["native_state"].(string)
			if native != test.native {
				t.Fatalf("native_state = %q, want %q", native, test.native)
			}
			authority, confidence := documentAuthority(record)
			if authority != test.authority || (confidence == "authoritative") != test.authoritative {
				t.Fatalf("authority = %q/%q, want %q binding=%v", authority, confidence, test.authority, test.authoritative)
			}
			result := documentResult(&Index{Sources: map[string]Source{}}, record, 1, "test")
			if got, _ := result["native_state"].(string); got != test.native {
				t.Fatalf("result native_state = %q, want %q", got, test.native)
			}
			if _, present := result["native_state"]; present != (test.native != "") {
				t.Fatalf("result native_state presence = %v", present)
			}
		})
	}
}

func TestSTAV0007_NonToolchainPathsUnchanged(t *testing.T) {
	record := toolchainTestRecord(t, "docs/specs/example.md", "# Example\n\nStatus: proposed\n", ".specify/memory/constitution.md")
	if record.Fields["status"] != "proposed" {
		t.Fatalf("status = %q", record.Fields["status"])
	}
	if _, present := record.Fields["native_state"]; present {
		t.Fatal("native_state set on a non-toolchain path")
	}
	// Only a root-level specs/<feature>/spec.md is a Spec Kit feature spec.
	nested := toolchainTestRecord(t, "docs/specs/feature/spec.md", "# Nested\n\nStatus: accepted\n", ".specify/memory/constitution.md")
	if nested.Fields["status"] != "accepted" {
		t.Fatalf("nested status = %q", nested.Fields["status"])
	}
	if documentKind("docs/.specify/memory/constitution.md") == "instructions" {
		t.Fatal("constitution outside the repository root is instructions")
	}
}
