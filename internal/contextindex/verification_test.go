package contextindex

import (
	"context"
	"reflect"
	"testing"
)

func verificationEvidence(paths ...string) []map[string]any {
	evidence := make([]any, 0, len(paths))
	for _, path := range paths {
		evidence = append(evidence, map[string]any{"path": path})
	}
	return []map[string]any{{"evidence": evidence}}
}

// TestVerificationDerivesCommandsFromWhatTheRepositoryProves covers
// docs/agent-memory/bugs.md's "Beamfall's verification commands are
// prescribed in every repository": a repository must never receive a
// verification command whose prerequisite file it does not actually have,
// one shape at a time.
func TestVerificationDerivesCommandsFromWhatTheRepositoryProves(t *testing.T) {
	for _, test := range []struct {
		name     string
		sources  map[string]Source
		paths    []string
		expected []string
	}{
		{
			name: "makefile-only repository uses its own declared gate target",
			sources: map[string]Source{
				"Makefile": {Data: []byte("gate: build\n\t@true\nbuild:\n\t@true\n")},
			},
			paths:    []string{"README.md"},
			expected: []string{"make gate"},
		},
		{
			// A Beamfall-shaped path (internal/<pkg>/...) must not borrow
			// Beamfall's Command table without Beamfall's own signals
			// (testing/features.yaml + testing/scenarios.yaml) present --
			// that shape-only match is exactly the recorded bug.
			name: "go.mod-only repository scopes go test to the touched package",
			sources: map[string]Source{
				"go.mod": {Data: []byte("module example.test\n\ngo 1.24\n")},
			},
			paths:    []string{"internal/widget/widget.go"},
			expected: []string{"go test ./internal/widget/...", "git diff --check"},
		},
		{
			name: "package.json declaring a test script",
			sources: map[string]Source{
				"package.json": {Data: []byte(`{"scripts":{"test":"jest"}}`)},
			},
			paths:    []string{"lib/widget.js"},
			expected: []string{"npm test", "git diff --check"},
		},
		{
			name: "package.json with no test script never prescribes npm test",
			sources: map[string]Source{
				"package.json": {Data: []byte(`{"scripts":{"build":"tsc"}}`)},
			},
			paths:    []string{"lib/widget.js"},
			expected: []string{"git diff --check"},
		},
		{
			// DR-0022: nine distinct package hints would push the gate past
			// the eight-command ceiling; the gate survives, the ninth hint goes.
			name: "eight or more hints drop a hint, never the gate",
			sources: map[string]Source{
				"go.mod":   {Data: []byte("module example.test\n\ngo 1.24\n")},
				"Makefile": {Data: []byte("gate: build\n\t@true\nbuild:\n\t@true\n")},
			},
			paths: []string{
				"internal/a/a.go", "internal/b/b.go", "internal/c/c.go", "internal/d/d.go",
				"internal/e/e.go", "internal/f/f.go", "internal/g/g.go", "internal/h/h.go",
				"internal/i/i.go",
			},
			expected: []string{
				"go test ./internal/a/...", "go test ./internal/b/...", "go test ./internal/c/...",
				"go test ./internal/d/...", "go test ./internal/e/...", "go test ./internal/f/...",
				"go test ./internal/g/...", "make gate",
			},
		},
		{
			name:     "bare repository falls back to git diff --check alone",
			sources:  map[string]Source{"README.md": {Data: []byte("hello\n")}},
			paths:    []string{"README.md"},
			expected: []string{"git diff --check"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			index := &Index{Sources: test.sources}
			got := verification(index, verificationEvidence(test.paths...))
			gotStrings := make([]string, len(got))
			for i, value := range got {
				gotStrings[i] = value.(string)
			}
			if !reflect.DeepEqual(gotStrings, test.expected) {
				t.Fatalf("verification() = %v, want %v", gotStrings, test.expected)
			}
		})
	}
}

func TestReceiptNamesUnknownLanguageVerification(t *testing.T) {
	for _, test := range []struct {
		name        string
		sources     map[string]Source
		path        string
		wantUnknown bool
	}{
		{
			name: "Rust repository",
			sources: map[string]Source{
				"Cargo.toml": {Data: []byte("[package]\nname = \"fixture\"\n")},
				"src/lib.rs": {Data: []byte("pub fn value() -> usize { 1 }\n")},
			},
			path:        "src/lib.rs",
			wantUnknown: true,
		},
		{
			name: "Kotlin repository",
			sources: map[string]Source{
				"build.gradle.kts": {Data: []byte("plugins { kotlin(\"jvm\") }\n")},
				"src/Main.kt":      {Data: []byte("fun main() = Unit\n")},
			},
			path:        "src/Main.kt",
			wantUnknown: true,
		},
		{
			name: "known Go verification",
			sources: map[string]Source{
				"go.mod":  {Data: []byte("module example.test\n")},
				"main.go": {Data: []byte("package main\n")},
			},
			path: "main.go",
		},
		{
			name:    "documentation-only repository",
			sources: map[string]Source{"README.md": {Data: []byte("hello\n")}},
			path:    "README.md",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			index := &Index{Sources: test.sources}
			receipt, err := receipt(index, "query", map[string]any{"text": "task"}, verificationEvidence(test.path), 10, "")
			if err != nil {
				t.Fatal(err)
			}
			uncertainty := anySlice(receipt["coverage"].(map[string]any)["uncertainty"])
			gotUnknown := anyContains(uncertainty, "no language-specific verification command is known for this repository")
			if gotUnknown != test.wantUnknown {
				t.Fatalf("uncertainty=%v want unknown-language=%v", uncertainty, test.wantUnknown)
			}
		})
	}
}

// The standalone query index records every admitted path but loads content
// only for what it ranks, and an unloaded source's Text() is an empty string.
// So the Makefile scan behind gateCommand found no target and closed the plan
// with the signal-free "git diff --check", while the oracle read the same
// repository's Makefile and closed it with "make gate" - the divergence that
// made query-authority-start unequal-stdout in the 2026-09-03 packet-5 run.
// The pure-function tests above could not see it: they hand gateCommand a
// Source whose Data is already populated.
func TestQueryIndexLoadsTheGateFileItDerivesTheClosingCommandFrom(t *testing.T) {
	root := t.TempDir()
	benchmarkGit(t, root, "init", "-q")
	benchmarkGit(t, root, "config", "user.email", "corvint@example.test")
	benchmarkGit(t, root, "config", "user.name", "Corvint Test")
	benchmarkWriteFile(t, root, "AGENTS.md", "# Project instructions\n\nThe roadmap is the only active work queue.\n")
	benchmarkWriteFile(t, root, "Makefile", "gate: build\n\t@true\nbuild:\n\t@true\n")
	benchmarkWriteFile(t, root, "internal/widget/widget.go", "package widget\n\nvar Payload = 1\n")
	benchmarkGit(t, root, "add", ".")
	benchmarkGit(t, root, "commit", "-qm", "gate fixture")

	index, err := BuildQuery(context.Background(), root, "Identify the active work queue and the required workflow gates")
	if err != nil {
		t.Fatal(err)
	}
	source, present := index.Sources["Makefile"]
	if !present {
		t.Fatal("the query index did not admit the Makefile at all")
	}
	if text, valid, loaded := source.Text(); !loaded || !valid || text == "" {
		t.Fatalf("the Makefile is not loaded valid text: text=%q valid=%v loaded=%v", text, valid, loaded)
	}
	command, found, complete := makefileTargetCommand(index.Sources)
	if !complete || !found || command != "make gate" {
		t.Fatalf("makefileTargetCommand = %q, found=%v complete=%v; want \"make gate\", true, true", command, found, complete)
	}
	if got, complete := gateCommand(verificationProfile(index.Sources), index.Sources); !complete || got != "make gate" {
		t.Fatalf("gateCommand = %q, complete=%v; want \"make gate\", true", got, complete)
	}
}

func TestGateCommandRefusesAnUnloadedMakefile(t *testing.T) {
	projectFiles := map[string]Source{"Makefile": {Path: "Makefile", BlobHash: "b1", Mode: "100644"}}
	if command, found, complete := makefileTargetCommand(projectFiles); complete || found || command != "" {
		t.Fatalf("makefileTargetCommand = %q, found=%v complete=%v; want refusal", command, found, complete)
	}
	if command, complete := gateCommand(verificationProfile(projectFiles), projectFiles); complete || command != "" {
		t.Fatalf("gateCommand = %q, complete=%v; want refusal", command, complete)
	}
	if got := verification(&Index{Sources: projectFiles}, verificationEvidence("README.md")); len(got) != 0 {
		t.Fatalf("verification answered from an unloaded Makefile: %v", got)
	}
}

func TestGateCommandAcceptsAnEmptyLoadedMakefile(t *testing.T) {
	projectFiles := map[string]Source{"Makefile": {Path: "Makefile", Checked: true, Valid: true}}
	if command, found, complete := makefileTargetCommand(projectFiles); !complete || found || command != "" {
		t.Fatalf("makefileTargetCommand = %q, found=%v complete=%v; want complete no-target result", command, found, complete)
	}
	if command, complete := gateCommand(verificationProfile(projectFiles), projectFiles); !complete || command != "git diff --check" {
		t.Fatalf("gateCommand = %q, complete=%v; want \"git diff --check\", true", command, complete)
	}
}

func TestToolchainCommandRefusesAnUnloadedPackageJSON(t *testing.T) {
	projectFiles := map[string]Source{"package.json": {Path: "package.json", BlobHash: "b1", Mode: "100644"}}
	if hasTest, complete := packageJSONHasTestScript(projectFiles["package.json"]); complete || hasTest {
		t.Fatalf("packageJSONHasTestScript = hasTest=%v complete=%v; want refusal", hasTest, complete)
	}
	if command, complete := toolchainCommand("lib/widget.js", projectFiles); complete || command != "" {
		t.Fatalf("toolchainCommand = %q, complete=%v; want refusal", command, complete)
	}
}
