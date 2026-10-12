package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

const legacyCasesCLIExport = "ID,Title,Section Hierarchy,Suite,Steps,Expected Result\n" +
	"C7,Manager approves request,Invoices > Refund approvals,Billing,<b>Open</b> the STEPQUEUE,EXPECTEDBANNER\n"

// DCP-V1-070/075/078: import-cases and search-cases print the library
// records unchanged, never mutate the repository and refuse misuse.
func TestDocsCorpusLegacyCasesCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	cemWrite(t, root, "exports/cases.csv", legacyCasesCLIExport)
	before := treeDigest(t, root)
	raw, err := os.ReadFile(filepath.Join(root, "exports", "cases.csv"))
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "import-cases", "--format", "csv", "--input", "exports/cases.csv")
	want, err := doccorpus.ImportLegacyCases("csv", "", "exports/cases.csv", raw)
	if code != 0 || err != nil || !bytes.Equal([]byte(out), want) || !strings.Contains(out, `"state":"manual"`) || strings.Contains(out, "STEPQUEUE") || strings.Contains(out, "EXPECTEDBANNER") {
		t.Fatalf("import-cases: %d %v %s %s", code, err, stderr, out)
	}
	code, out, stderr = corpusCLI(t, root, "docs", "corpus", "search-cases", "--format", "csv", "--input", "exports/cases.csv", "--query", "refund", "--limit", "5")
	want, err = doccorpus.SearchLegacyCases("csv", "exports/cases.csv", raw, "refund", 5)
	if code != 0 || err != nil || !bytes.Equal([]byte(out), want) || !strings.Contains(out, `"id":"C7"`) || strings.Contains(out, "STEPQUEUE") {
		t.Fatalf("search-cases: %d %v %s %s", code, err, stderr, out)
	}
	code, out, _ = corpusCLI(t, root, "docs", "corpus", "import-cases", "--format", "csv", "--input", "exports/cases.csv", "--content", "full")
	if code != 0 || !strings.Contains(out, `"local_only":true`) || !strings.Contains(out, "Open the STEPQUEUE") {
		t.Fatalf("full content: %d %s", code, out)
	}
	if treeDigest(t, root) != before {
		t.Fatal("legacy case commands mutated the repository")
	}
	for _, args := range [][]string{
		{"import-cases", "--input", "exports/cases.csv"},
		{"import-cases", "--format", "csv"},
		{"search-cases", "--format", "csv", "--input", "exports/cases.csv"},
		{"search-cases", "--format", "csv", "--input", "exports/cases.csv", "--query", "x", "--offset", "1"},
		{"import-cases", "--format", "csv", "--input", "exports/cases.csv", "--query", "x"},
		{"import-cases", "--format", "xlsx", "--input", "exports/cases.csv"},
		{"import-cases", "--format", "csv", "--input", "/etc/passwd"},
	} {
		if code, out, _ := corpusCLI(t, root, append([]string{"docs", "corpus"}, args...)...); code != 2 || out != "" {
			t.Fatalf("%v admitted: %d %s", args, code, out)
		}
	}
	for _, text := range []string{"import-cases --format testrail-json|csv", "search-cases --format", "DCP-V1-070..079", "never count as runtime parity"} {
		if !strings.Contains(docsHelp, text) {
			t.Errorf("docs help omits %q", text)
		}
	}
}
