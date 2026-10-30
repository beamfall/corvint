package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/jstestprovider"
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
	t.Run("DCP-V1-048 discover-playwright refuses a stale listing after a committed test file", func(t *testing.T) {
		root, _ := playwrightCorpusRepository(t)
		cemWrite(t, root, "e2e/extra.spec.ts", "import { test } from '@playwright/test';\n\ntest('extra', async () => {});\n")
		cemGit(t, root, "add", "e2e/extra.spec.ts")
		cemGit(t, root, "commit", "-qm", "add a test file")
		revision := cemGit(t, root, "rev-parse", "HEAD")
		repository := cemGit(t, root, "rev-list", "--max-parents=0", "HEAD")
		migration, err := doccorpus.Encode(doccorpus.BehaviorMigration{Revisions: doccorpus.BehaviorRevisions{App: doccorpus.Repository{ID: strings.Repeat("1", 40), Revision: revision}, E2E: doccorpus.Repository{ID: repository, Revision: revision}, Docs: doccorpus.Repository{ID: strings.Repeat("2", 40), Revision: revision}}, Schema: 2, ContractID: "items-contract", SourceRevision: revision, DocumentationRevision: revision})
		if err != nil {
			t.Fatal(err)
		}
		cemWrite(t, root, "evidence/migration.json", string(migration))
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "discover-playwright", "--migration", "evidence/migration.json", "--config", "playwright.config.ts", "--playwright-list", "evidence/list.json")
		if code != 2 || out != "" || !strings.Contains(stderr, "corpus-refused") || !strings.Contains(stderr, "e2e/extra.spec.ts") {
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

// DCP-V1-053: the docs help names corvint-js-test-provider e2e --external-server as the qualified
// receipt producer, and its end-to-end example runs: the provider's canonical stdout encoding
// (jstestprovider.EncodeQualified, the bytes e2e --external-server prints and --retain keeps) is
// accepted unchanged by discover-playwright --receipt, and witness-playwright takes the receipt
// as a named request input. The witness join itself is TestPlaywrightWitnessImporter's.
func TestDocsHelpNamesPlaywrightReceiptProducer(t *testing.T) {
	for _, want := range []string{"DCP-V1-053", "corvint-js-test-provider e2e\n--external-server", ".corvint/test-evidence/", "Managed-server e2e output is not a qualified receipt", "names a\nREQUEST.json input, not a file", "commit the receipt bytes unchanged"} {
		if !strings.Contains(docsHelp, want) {
			t.Errorf("docs help omits %q", want)
		}
	}
	root, migration := playwrightCorpusRepository(t)
	config, _ := os.ReadFile(filepath.Join(root, "playwright.config.ts"))
	spec, _ := os.ReadFile(filepath.Join(root, "e2e", "items.spec.ts"))
	adopted := strings.Repeat("a", 64)
	receipt, err := jstestprovider.EncodeQualified(jstestprovider.Receipt{
		Profile: jstestprovider.ExternalProfile, Kind: "e2e",
		Identity: jstestprovider.Identity{ConfigFile: root + "/playwright.config.ts", ConfigDigest: doccorpus.Digest(config), TestFileDigests: map[string]string{root + "/e2e/items.spec.ts": doccorpus.Digest(spec)}, RunnerName: "playwright", RunnerVersion: "1.64.0", NodeVersion: "v24.11.1"},
		External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", DeclaredAppIdentity: "web"},
		Tests:    []jstestprovider.TestOutcome{{ID: adopted, Name: "lists items", FullName: " > alpha > items.spec.ts > items > lists items", State: "passed", Anchor: &jstestprovider.Anchor{File: root + "/e2e/items.spec.ts", Line: 7}, Project: &jstestprovider.ProjectIdentity{Name: "alpha"}, Attempts: []jstestprovider.Attempt{{State: "passed", Retry: 0}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "evidence/receipt.json", string(receipt))
	ran := map[string]bool{}
	for _, line := range strings.Split(docsHelp, "\n") {
		fields := strings.Fields(line)
		if !strings.HasPrefix(line, "    corvint docs corpus ") {
			continue
		}
		if n := len(fields); n > 2 && fields[n-2] == ">" {
			fields = fields[:n-2]
		}
		code, out, stderr := corpusCLI(t, root, fields[1:]...)
		switch fields[3] {
		case "discover-playwright":
			listing, _ := os.ReadFile(filepath.Join(root, "evidence", "list.json"))
			want, err := doccorpus.BuildPlaywrightDiscovery(context.Background(), doccorpus.PlaywrightDiscoveryInput{Root: root, Migration: migration, ConfigPath: "playwright.config.ts", Listing: listing, Receipt: receipt})
			if code != 0 || err != nil || out != string(want) || !strings.Contains(out, adopted) {
				t.Fatalf("example discover-playwright: %d %v %s\n%s", code, err, stderr, out)
			}
		case "witness-playwright":
			// The example's request is the caller's; absent here, the read refuses only after
			// every argument was admitted.
			if code != 2 || !strings.Contains(stderr, "corpus-input-unavailable") {
				t.Fatalf("example witness-playwright arguments refused: %d %s", code, stderr)
			}
		}
		ran[fields[3]] = true
	}
	if !ran["discover-playwright"] || !ran["witness-playwright"] {
		t.Fatalf("help example lacks the discovery or witness step: %v", ran)
	}
}

// DCP-V1-080: docs help and the witness-playwright topic state the annotation contract the
// importer enforces: every annotation type, every event key, every kind and navigation value,
// the missing-annotation order and the shipped producer. The lists come from the code, so the
// help cannot drift from it silently.
func TestDocsHelpNamesBehaviorAnnotationContract(t *testing.T) {
	want := []string{"DCP-V1-080", "DCP-V1-081", "DCP-V1-082", "at most 4096 events", "exactly one, after the cleanup ran: passed",
		"in the order events, cleanup, fixture, role", "internal/doccorpus/behavior-events.cjs", "Playwright behavior witness walkthrough",
		doccorpus.PlaywrightEventAnnotation, doccorpus.PlaywrightCleanupAnnotation, doccorpus.PlaywrightFixtureAnnotation, doccorpus.PlaywrightRoleAnnotation,
		"page | assertion | negative-control", "main-frame | frame | redirect | popup |\n                            setup"}
	event := reflect.TypeFor[doccorpus.BehaviorEvent]()
	for index := range event.NumField() {
		want = append(want, event.Field(index).Tag.Get("json"))
	}
	for _, reason := range []string{"events-unobserved", "event-malformed", "cleanup-unobserved", "cleanup-failed", "fixture-role-mismatch"} {
		want = append(want, reason)
	}
	topic := docsCorpusOperationHelp("witness-playwright")
	for _, text := range want {
		if !strings.Contains(docsHelp, text) || !strings.Contains(topic, text) {
			t.Errorf("docs help or witness-playwright help omits %q", text)
		}
	}
	if strings.Join(doccorpus.PlaywrightMissingAnnotationTypes, ", ") != "events, cleanup, fixture, role" {
		t.Errorf("missing-annotation order %v differs from the help", doccorpus.PlaywrightMissingAnnotationTypes)
	}
	if strings.Contains(docsCorpusOperationHelp("discover-playwright"), "DCP-V1-080") {
		t.Error("discover-playwright help carries the witness annotation contract")
	}
	if _, err := os.Stat(filepath.Join("..", "..", "internal", "doccorpus", "behavior-events.cjs")); err != nil {
		t.Errorf("help names a producer that is not shipped: %v", err)
	}
}
