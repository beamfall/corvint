package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// TestDCIV1001DocsCorpusEstateCommand: docs corpus estate reads an estate file naming two corpora
// built from separate checkouts and prints the receipt joining the consumer's npm dependency to
// the library corpus's package by exact name; an unreadable corpus is refused naming it
// (GitHub #1998).
func TestDCIV1001DocsCorpusEstateCommand(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	for name, manifest := range map[string]string{
		"lib": "{\n  \"name\": \"lib-a\",\n  \"version\": \"1.0.0\",\n  \"publishConfig\": {\"access\": \"public\"}\n}\n",
		"svc": "{\n  \"name\": \"svc\",\n  \"private\": true,\n  \"dependencies\": {\n    \"lib-a\": \"1.0.0\"\n  }\n}\n",
	} {
		root := filepath.Join(parent, name)
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		run(root, "init", "-q")
		run(root, "add", ".")
		run(root, "-c", "user.name=Corpus test", "-c", "user.email=corpus@example.invalid", "commit", "-qm", name)
		m, err := doccorpus.ManifestInventory(context.Background(), root, run(root, "rev-parse", "HEAD"), []string{"package.json"}, doccorpus.ManifestSchema, "2026-10-29T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		m.RepositoryInventory = &doccorpus.RepositoryInventory{}
		a, err := doccorpus.Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		data, err := doccorpus.Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "corpus.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// --root is a checkout beside the corpora, which the estate names by climbing out of it.
	estate := filepath.Join(parent, "estate")
	if err := os.MkdirAll(estate, 0o700); err != nil {
		t.Fatal(err)
	}
	run(estate, "init", "-q")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(estate, "estate.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"schema":"corvint-corpus-estate/1","corpora":[{"root":"../svc","artifact":"corpus.json"},{"root":"../lib","artifact":"corpus.json"}]}`)
	exit, out, stderr := corpusCLI(t, estate, "docs", "corpus", "estate", "--input", "estate.json")
	if exit != 0 {
		t.Fatalf("estate refused: %d %s", exit, stderr)
	}
	var receipt doccorpus.EstateReceipt
	if err := json.Unmarshal([]byte(out), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Schema != doccorpus.EstateReceiptSchema || len(receipt.Corpora) != 2 || len(receipt.Repositories) != 2 || len(receipt.References) != 1 {
		t.Fatalf("unexpected receipt: %s", out)
	}
	if ref := receipt.References[0]; ref.Corpus != "../svc/corpus.json" || ref.Name != "lib-a" || ref.State != "resolved" || ref.Targets[0].Corpus != "../lib/corpus.json" {
		t.Fatalf("the service dependency should join the library corpus: %+v", ref)
	}
	write(`{"schema":"corvint-corpus-estate/1","corpora":[{"root":"../svc","artifact":"missing.json"}]}`)
	exit, _, stderr = corpusCLI(t, estate, "docs", "corpus", "estate", "--input", "estate.json")
	if exit == 0 || !strings.Contains(stderr, "estate corpus ../svc/missing.json: ") {
		t.Fatalf("an unreadable corpus should be refused naming it: %d %s", exit, stderr)
	}
}
