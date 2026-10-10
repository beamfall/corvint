package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// playwrightCorpusRepository commits the generic Playwright project of the doccorpus fixture
// and writes its caller-run listing, rebased onto the repository, beside the migration record.
func playwrightCorpusRepository(t *testing.T) (string, []byte) {
	t.Helper()
	root := t.TempDir()
	fixture := func(name string) string {
		raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "doccorpus", "testdata", "playwright-behavior", name))
		if err != nil {
			t.Fatal(err)
		}
		return strings.ReplaceAll(string(raw), "@ROOT@", root)
	}
	cemGit(t, root, "init", "-q")
	cemWrite(t, root, "playwright.config.ts", fixture("playwright.config.ts"))
	cemWrite(t, root, "e2e/items.spec.ts", fixture("items.spec.ts"))
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "source")
	revision := cemGit(t, root, "rev-parse", "HEAD")
	repository := cemGit(t, root, "rev-list", "--max-parents=0", "HEAD")
	migration, err := doccorpus.Encode(doccorpus.BehaviorMigration{Revisions: doccorpus.BehaviorRevisions{App: doccorpus.Repository{ID: strings.Repeat("1", 40), Revision: revision}, E2E: doccorpus.Repository{ID: repository, Revision: revision}, Docs: doccorpus.Repository{ID: strings.Repeat("2", 40), Revision: revision}}, Schema: 2, ContractID: "items-contract", SourceRevision: revision, DocumentationRevision: revision})
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "evidence/migration.json", string(migration))
	listing := fixture("list.json")
	cemWrite(t, root, "evidence/list.json", listing)
	cemWrite(t, root, "evidence/report.json", fixture("report.json"))
	return root, migration
}

func TestDocsCorpusPlaywrightProducersCLI(t *testing.T) {
	t.Run("DCP-V1-046 discover-playwright emits the library record without mutation", func(t *testing.T) {
		root, migration := playwrightCorpusRepository(t)
		before := treeDigest(t, root)
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "discover-playwright", "--migration", "evidence/migration.json", "--config", "playwright.config.ts", "--playwright-list", "evidence/list.json")
		if code != 0 {
			t.Fatalf("%d %s", code, stderr)
		}
		listing, _ := os.ReadFile(filepath.Join(root, "evidence", "list.json"))
		want, err := doccorpus.BuildPlaywrightDiscovery(context.Background(), doccorpus.PlaywrightDiscoveryInput{Root: root, Migration: migration, ConfigPath: "playwright.config.ts", Listing: listing})
		if err != nil || !bytes.Equal([]byte(out), want) || !strings.Contains(out, `"`+doccorpus.PlaywrightDiscoveryMode+`"`) {
			t.Fatalf("cli output differs from the library record: %v\n%s", err, out)
		}
		if treeDigest(t, root) != before {
			t.Fatal("discover-playwright mutated the repository")
		}
	})
	t.Run("DCP-V1-048 discover-playwright refuses a filtered listing", func(t *testing.T) {
		root, _ := playwrightCorpusRepository(t)
		listing, _ := os.ReadFile(filepath.Join(root, "evidence", "list.json"))
		filtered := bytes.Replace(listing, []byte(`"--list",`), []byte(`"--list", "--grep", "items",`), 1)
		if bytes.Equal(filtered, listing) {
			t.Fatal("fixture argv has no --list entry to filter")
		}
		cemWrite(t, root, "evidence/list.json", string(filtered))
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "discover-playwright", "--migration", "evidence/migration.json", "--config", "playwright.config.ts", "--playwright-list", "evidence/list.json")
		if code != 2 || out != "" || !strings.Contains(stderr, "--grep") {
			t.Fatalf("%d %q %s", code, out, stderr)
		}
	})
	t.Run("DCP-V1-049 witness-playwright requires its inputs and refuses an unretained receipt", func(t *testing.T) {
		root, _ := playwrightCorpusRepository(t)
		for _, args := range [][]string{
			{"witness-playwright", "--input", "request.json", "--report", "evidence/report.json"},
			{"witness-playwright", "--input", "request.json", "--receipt-input", "receipt", "--report", "evidence/report.json", "--previous", "x.json"},
			{"discover-playwright", "--migration", "evidence/migration.json", "--config", "playwright.config.ts"},
		} {
			if code, _, _ := corpusCLI(t, root, append([]string{"docs", "corpus"}, args...)...); code != 2 {
				t.Fatalf("%v admitted", args)
			}
		}
		cemWrite(t, root, "request.json", `{"schema":"corvint-behavior-adapter-request/1"}`)
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "witness-playwright", "--input", "request.json", "--receipt-input", "receipt", "--report", "evidence/report.json")
		if code != 2 || out != "" || stderr == "" {
			t.Fatalf("%d %q %s", code, out, stderr)
		}
	})
}
