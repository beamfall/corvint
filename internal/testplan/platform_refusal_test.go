package testplan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// PRS-V1-920 (V1-2683): a --tests provider read refused by the receipt
// platform boundary reports the typed platform code, not an invalid receipt;
// any other read failure keeps invalid-test-validity-receipt.
func TestPRSV1920ProviderReadReportsThePlatformRefusal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	saved := readReceiptFile
	t.Cleanup(func() { readReceiptFile = saved })
	for _, tc := range []struct {
		cause      error
		code, text string
	}{
		{fmt.Errorf("open: %w", testvaliditydoc.ErrPlatformUnsupported), testvaliditydoc.PlatformUnsupportedCode, testvaliditydoc.ErrPlatformUnsupported.Error()},
		{errors.New("not a regular file"), "invalid-test-validity-receipt", "--tests must name one bounded provider document test-validity accepts"},
	} {
		readReceiptFile = func(*os.Root, string) ([]byte, error) { return nil, tc.cause }
		_, err := readProvider(path)
		var coded *gokernel.Error
		if !errors.As(err, &coded) || coded.Code != tc.code || coded.Message != tc.text {
			t.Fatalf("cause %v: got %v, want %s %q", tc.cause, err, tc.code, tc.text)
		}
	}
}
