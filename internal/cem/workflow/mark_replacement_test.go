package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestMarkReplacementAtFirstRead is CEM-PILOT-013 (V1-0665). A real second
// mark replaces the map between Lstat and open if the observer reads before
// locking. With serialization, that writer completes before metadata capture.
func TestMarkReplacementAtFirstRead(t *testing.T) {
	for _, output := range []string{"", wire.ExcludedCEMPath} {
		t.Run("output="+output, func(t *testing.T) {
			root, base, target := makeRepo(t)
			writer := openSession(t, root)
			if _, err := writer.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
				t.Fatal(err)
			}
			mapPath := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
			before, err := os.Lstat(mapPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := writer.readProducerMap(wire.ExcludedCEMPath); err != nil {
				t.Fatal(err)
			}
			observer := openSession(t, root)
			held, err := writer.workRoot.Lock(wire.ExcludedCEMPath)
			if err != nil {
				t.Fatal(err)
			}
			var release sync.Once
			releaseWriter := func() { release.Do(held) }
			defer releaseWriter()
			done := make(chan error, 1)
			start := make(chan struct{})
			var begin sync.Once
			finishWriter := func() {
				begin.Do(func() { releaseWriter(); close(start) })
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
					done <- err // the other boundary can observe the same completed result
				case <-time.After(30 * time.Second):
					t.Fatal("replacement mark did not finish")
				}
			}
			go func() {
				<-start
				_, err := writer.Mark(ctx(), MarkOptions{MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "unknown", Reason: "conflicting-evidence"})
				done <- err
			}()
			previous := lockMap
			lockMap = func(r *publish.Root, relative string) (func(), error) {
				if r == observer.workRoot {
					finishWriter()
				}
				return previous(r, relative)
			}
			defer func() { lockMap = previous }()
			reached := false
			observer.workRoot.BeforeReadOpen = func(path string) {
				reached = true
				if path != mapPath {
					t.Fatalf("unexpected input %s", path)
				}
				finishWriter()
				after, err := os.Lstat(mapPath)
				if err != nil || os.SameFile(before, after) {
					t.Fatalf("writer did not atomically replace map: %v", err)
				}
			}
			_, err = observer.Mark(ctx(), MarkOptions{MapPath: wire.ExcludedCEMPath, Hunk: "2", Disposition: "unknown", Reason: "insufficient-evidence", Output: output})
			if !reached {
				t.Fatal("first-read injection was not reached")
			}
			if err != nil {
				t.Fatalf("mark across concurrent replacement failed: %v", err)
			}
			_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
			if err != nil {
				t.Fatal(err)
			}
			if document.Hunks[0].Reason != "conflicting-evidence" || document.Hunks[1].Reason != "insufficient-evidence" {
				t.Fatal("replacement or observer mark was lost")
			}
		})
	}
}

// TestMarkHostileInputPrecedence is CEM-PILOT-013: acquiring a lock before
// reading cannot turn an unsafe input refusal into a publication error.
func TestMarkHostileInputPrecedence(t *testing.T) {
	t.Parallel()
	t.Run("symlinked parent", func(t *testing.T) {
		root, base, target := makeRepo(t)
		session := openSession(t, root)
		if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, ".corvint"), filepath.Join(root, "alias")); err != nil {
			t.Fatal(err)
		}
		_, err := session.Mark(ctx(), MarkOptions{MapPath: "alias/change.cem.json", Hunk: "1", Disposition: "unknown", Reason: "insufficient-evidence"})
		if cemcode.CodeOf(err) != cemcode.MapUnavailable {
			t.Fatalf("unsafe ancestor returned %v", err)
		}
	})
	for _, malformed := range []bool{false, true} {
		t.Run(fmt.Sprintf("occupied lock malformed=%v", malformed), func(t *testing.T) {
			root, base, target := makeRepo(t)
			session := openSession(t, root)
			if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
				t.Fatal(err)
			}
			mapPath := filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
			if malformed {
				if err := os.WriteFile(mapPath, []byte("invalid"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(mapPath+".lock", []byte("someone's data"), 0o600); err != nil {
				t.Fatal(err)
			}
			prior, err := os.ReadFile(mapPath)
			if err != nil {
				t.Fatal(err)
			}
			_, _, readErr := session.readProducerMap(wire.ExcludedCEMPath)
			_, err = session.Mark(ctx(), MarkOptions{MapPath: wire.ExcludedCEMPath, Hunk: "1", Disposition: "unknown", Reason: "insufficient-evidence"})
			expected := cemcode.PublishFailed
			if malformed {
				if readErr == nil {
					t.Fatal("malformed fixture parsed")
				}
				expected = cemcode.CodeOf(readErr)
			}
			if cemcode.CodeOf(err) != expected {
				t.Fatalf("got %v; expected %s", err, expected)
			}
			after, readErr := os.ReadFile(mapPath)
			if readErr != nil || string(after) != string(prior) {
				t.Fatalf("input changed: %v", readErr)
			}
			lock, readErr := os.ReadFile(mapPath + ".lock")
			if readErr != nil || string(lock) != "someone's data" {
				t.Fatalf("lock data changed: %v", readErr)
			}
		})
	}
}
