package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testsource"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// PRS-V1-920 (V1-2683): test-validity --receipt and tests example-join
// --requirements report the receipt platform refusal with the typed platform
// code, not invalid-test-validity-receipt or tests-source-unavailable.
func TestPRSV1920ReceiptAndRequirementReadsReportThePlatformRefusal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	saved := readReceiptFile
	t.Cleanup(func() { readReceiptFile = saved })
	readReceiptFile = func(*os.Root, string) ([]byte, error) {
		return nil, fmt.Errorf("open: %w", testvaliditydoc.ErrPlatformUnsupported)
	}
	want := testvaliditydoc.ErrPlatformUnsupported.Error()

	_, err := readProviderReceipt(path)
	var coded *gokernel.Error
	if !errors.As(err, &coded) || coded.Code != testvaliditydoc.PlatformUnsupportedCode || coded.Message != want {
		t.Fatalf("readProviderReceipt: got %v", err)
	}

	var out, diagnostic bytes.Buffer
	code := runTestsExampleJoin(testsource.Repo{}, []string{"--receipt", path, "--requirements", path}, &out, &diagnostic)
	var refusal struct{ Code, Error string }
	if code != 2 || json.Unmarshal(diagnostic.Bytes(), &refusal) != nil || refusal.Code != testvaliditydoc.PlatformUnsupportedCode || refusal.Error != want {
		t.Fatalf("example-join: exit %d diagnostic %q", code, diagnostic.String())
	}

	readReceiptFile = func(*os.Root, string) ([]byte, error) { return nil, errors.New("unreadable") }
	if _, err := readProviderReceipt(path); !errors.As(err, &coded) || coded.Code != "invalid-test-validity-receipt" {
		t.Fatalf("readProviderReceipt other failure: got %v", err)
	}
	diagnostic.Reset()
	if code := runTestsExampleJoin(testsource.Repo{}, []string{"--receipt", path, "--requirements", path}, &out, &diagnostic); code != 2 || !bytes.Contains(diagnostic.Bytes(), []byte("tests-source-unavailable")) {
		t.Fatalf("example-join other failure: exit %d diagnostic %q", code, diagnostic.String())
	}
}
