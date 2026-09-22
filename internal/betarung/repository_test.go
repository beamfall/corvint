package betarung

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func shippedRecord(t *testing.T) Record {
	t.Helper()
	record, err := Default()
	if err != nil {
		t.Fatalf("shipped beta admission record is invalid: %v", err)
	}
	return record
}

// The shipped record must move no receipt bytes. Every command is unadmitted at
// this revision, so every surface still states FALLBACK exactly as before the
// beta rung existed.
func TestShippedRecordAdmitsNothingAndStatesFallback(t *testing.T) {
	record := shippedRecord(t)
	if admitted := record.AdmittedCommands(); len(admitted) != 0 {
		t.Fatalf("shipped record admits %v; flipping a label moves receipt bytes and needs oracle-authored expectations", admitted)
	}
	for _, surface := range []string{"cli", "plugin", "extension", "unknown-surface"} {
		if label := Support("harness", surface); label != Fallback {
			t.Fatalf("harness on %s states %q, want %q", surface, label, Fallback)
		}
	}
}

// The disclosure obligation is checked against the register at this revision,
// not against a copy taken when the record was written.
func TestShippedRecordMatchesTheDivergenceRegister(t *testing.T) {
	markdown, err := os.ReadFile(filepath.Join(repositoryRoot(t), "conformance", "divergence-register.md"))
	if err != nil {
		t.Fatalf("read divergence register: %v", err)
	}
	if err := shippedRecord(t).CheckRegister(markdown); err != nil {
		t.Fatal(err)
	}
}

func TestEveryCitedEvidenceFileExists(t *testing.T) {
	root := repositoryRoot(t)
	for _, admission := range shippedRecord(t).Admissions {
		for _, finding := range admission.Evidence.findings() {
			for _, source := range finding.Sources {
				if _, err := os.Stat(filepath.Join(root, source)); err != nil {
					t.Errorf("%s cites %s, which does not exist: %v", admission.Command, source, err)
				}
			}
		}
	}
}

// A command that enters the parity corpus without a beta assessment would be
// invisible to this rung. The parity manifest is the machine-readable list of
// commands the candidate claims, so the record must cover all of it.
func TestRecordCoversEveryParityCorpusCommand(t *testing.T) {
	path := filepath.Join(repositoryRoot(t), "conformance", "cli-parity-v0", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read parity manifest: %v", err)
	}
	var manifest struct {
		Commands []struct {
			Command string `json:"command"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode parity manifest: %v", err)
	}
	if len(manifest.Commands) == 0 {
		t.Fatal("parity manifest lists no commands")
	}
	record := shippedRecord(t)
	assessed := map[string]bool{}
	for _, admission := range record.Admissions {
		assessed[admission.Command] = true
	}
	for _, entry := range manifest.Commands {
		if assessed[entry.Command] {
			continue
		}
		t.Errorf("parity corpus command %q has no beta admission assessment", entry.Command)
	}
}
