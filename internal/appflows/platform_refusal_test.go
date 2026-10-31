package appflows

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// PRS-V1-920 (V1-2683): a docs-maintain maintenance-file read refused by the
// receipt platform boundary is a typed gokernel error the flows CLI keeps,
// not an argument error; other read failures are unchanged.
func TestPRSV1920MaintenanceReadReportsThePlatformRefusal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/page.md", []byte("page\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	saved := readReceiptFile
	t.Cleanup(func() { readReceiptFile = saved })
	readReceiptFile = func(*os.Root, string) ([]byte, error) {
		return nil, fmt.Errorf("open: %w", testvaliditydoc.ErrPlatformUnsupported)
	}
	_, _, err = maintenanceFile(root, "page.md", false)
	var coded *gokernel.Error
	if !errors.As(err, &coded) || coded.Code != testvaliditydoc.PlatformUnsupportedCode || coded.Message != testvaliditydoc.ErrPlatformUnsupported.Error() || !isPlatformRefusal(err) {
		t.Fatalf("got %v, want %s", err, testvaliditydoc.PlatformUnsupportedCode)
	}
	other := errors.New("unreadable")
	readReceiptFile = func(*os.Root, string) ([]byte, error) { return nil, other }
	if _, _, err = maintenanceFile(root, "page.md", false); err != other || isPlatformRefusal(err) {
		t.Fatalf("got %v, want the unchanged read error", err)
	}
}
