//go:build !unix

package docmaintain

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchUnsupportedPlatformNeverWrites(t *testing.T) {
	for _, existing := range []bool{false, true} {
		root := t.TempDir()
		page := filepath.Join(root, "page.md")
		if existing {
			if err := os.WriteFile(page, []byte("human"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		receipt, err := Watch(context.Background(), root, "page.md", Selector{"owner.md", "widget"}, Policy{Enabled: true, Apply: true, MaxWrites: 1, MaxWallClock: time.Second})
		if err == nil || receipt.StoppedReason != "unsupported-watch-platform" || receipt.Writes != 0 {
			t.Fatalf("%+v %v", receipt, err)
		}
		data, readErr := os.ReadFile(page)
		if existing && (readErr != nil || string(data) != "human") {
			t.Fatal("existing page changed")
		}
		if !existing && !os.IsNotExist(readErr) {
			t.Fatal("missing page created")
		}
	}
}
