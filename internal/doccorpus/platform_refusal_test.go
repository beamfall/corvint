package doccorpus

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// PRS-V1-022 (V1-2484, decision 0559): on a platform whose receipt reader is
// not qualified, every docs corpus input read refuses up front with the typed
// corpus-platform-unsupported code, never corpus-input-unavailable, and opens
// nothing; on darwin and linux the same reads succeed.
func TestPRSV1022CorpusReadsRefuseAnUnsupportedPlatformWithATypedCode(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "corpus.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	reads := map[string]func() error{
		"read": func() error { _, err := ReadCorpusFile(root, "docs/corpus.json"); return err },
		"stat": func() error { _, err := StatCorpusFile(root, "docs/corpus.json"); return err },
		"stream": func() error {
			return streamCorpusFile(root, "docs/corpus.json", func(r io.Reader) error { _, err := io.ReadAll(r); return err })
		},
	}
	qualified := runtime.GOOS == "darwin" || runtime.GOOS == "linux"
	if qualified {
		for name, read := range reads {
			if err := read(); err != nil {
				t.Fatalf("%s on %s: %v, want the qualified read to succeed", name, runtime.GOOS, err)
			}
		}
	}
	saved := receiptPlatformRefusal
	t.Cleanup(func() { receiptPlatformRefusal = saved })
	receiptPlatformRefusal = func() error { return testvaliditydoc.ErrPlatformUnsupported }
	// A root that does not exist proves the refusal precedes every open.
	missing := filepath.Join(root, "absent")
	for name, read := range map[string]func() error{
		"read": reads["read"], "stat": reads["stat"], "stream": reads["stream"],
		"read-missing-root": func() error { _, err := ReadCorpusFile(missing, "docs/corpus.json"); return err },
	} {
		err := read()
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != PlatformUnsupportedCode || typed.Message != "docs corpus reads are unsupported on this platform" {
			t.Fatalf("%s: %v, want %s", name, err, PlatformUnsupportedCode)
		}
	}
	if !qualified {
		receiptPlatformRefusal = saved
		if _, err := ReadCorpusFile(root, "docs/corpus.json"); err == nil {
			t.Fatalf("%s read a corpus input without a qualified receipt reader", runtime.GOOS)
		}
	}
}
