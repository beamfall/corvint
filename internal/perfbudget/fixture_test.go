package perfbudget

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// PRB-V0-020, PRB-V0-021, PRB-V0-022: the generator is deterministic and produces the declared
// shape: a primary repository, at least three additional repositories, a previous and a current
// artifact, and a Ruby tree.
func TestPRBV0020FixtureIsDeterministicAndShaped(t *testing.T) {
	if testing.Short() {
		t.Skip("generates two Git fixtures")
	}
	ctx := context.Background()
	first, err := Generate(ctx, t.TempDir(), Smoke, LibraryBuilder)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(ctx, t.TempDir(), Smoke, LibraryBuilder)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != second.Revision || first.PreviousRevision != second.PreviousRevision || first.Records != second.Records {
		t.Fatalf("not deterministic: %+v vs %+v", first, second)
	}
	for _, name := range []string{first.Previous, first.PreviousManifest, first.Current, first.Manifest} {
		a, err := os.ReadFile(filepath.Join(first.Primary, name))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(second.Primary, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("%s differs between identical generations", name)
		}
	}
	// Every repository's commit is identical, not only the primary's: additional-repository
	// commits are anchor identities, and the Ruby commit is what affected plans at.
	for i, dir := range append(append([]string{}, first.Additional...), first.Ruby) {
		other := append(append([]string{}, second.Additional...), second.Ruby)[i]
		a, errA := gitOutput(ctx, dir, "rev-parse", "HEAD")
		b, errB := gitOutput(ctx, other, "rev-parse", "HEAD")
		if errA != nil || errB != nil || a != b {
			t.Fatalf("%s HEAD differs between identical generations: %s vs %s (%v %v)", filepath.Base(dir), a, b, errA, errB)
		}
	}
	if len(first.Additional) < 3 || first.Inputs != Smoke.PrimaryPages+Smoke.AdditionalRepositories*Smoke.AdditionalPages {
		t.Fatalf("shape: %d additional repositories, %d inputs", len(first.Additional), first.Inputs)
	}
	// Records per input is what Full scales: about ten, so 10,000 inputs give about 100,000.
	perInput := float64(first.Records) / float64(first.Inputs)
	if perInput < 8 || perInput > 10.5 {
		t.Fatalf("%d records over %d inputs (%.2f per input) would not scale Full to about 100,000", first.Records, first.Inputs, perInput)
	}
	t.Logf("smoke fixture: %d inputs, %d records (%.2f per input), %d artifact bytes", first.Inputs, first.Records, perInput, first.ArtifactBytes)
	rubyFiles := 0
	_ = filepath.WalkDir(first.Ruby, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(path) == ".rb" {
			rubyFiles++
		}
		return nil
	})
	if rubyFiles < Smoke.RubyFiles {
		t.Fatalf("ruby tree holds %d files, want at least %d", rubyFiles, Smoke.RubyFiles)
	}
	if _, err := os.Stat(filepath.Join(first.Ruby, filepath.FromSlash(first.RubyPath))); err != nil {
		t.Fatal(err)
	}
}

// PRB-V0-021: the full scale is the realistic size GitHub #1602 names.
func TestPRBV0021FullScaleIsRealistic(t *testing.T) {
	inputs := Full.PrimaryPages + Full.AdditionalRepositories*Full.AdditionalPages
	if inputs != 10000 || Full.AdditionalRepositories < 3 || Full.RubyFiles < 20000 || Full.ChangedPages < 1 {
		t.Fatalf("full scale %+v: %d inputs", Full, inputs)
	}
	if Smoke.ParagraphsPerPage != Full.ParagraphsPerPage || Smoke.ClaimsPerPage != Full.ClaimsPerPage || Smoke.AdditionalRepositories != Full.AdditionalRepositories {
		t.Fatal("smoke must keep the full shape so its records-per-input ratio predicts full")
	}
}

// PRB-V0-023: an invalid scale or a non-empty directory is refused before anything is written.
func TestPRBV0023InvalidScaleRefused(t *testing.T) {
	dir := t.TempDir()
	bad := Smoke
	bad.AdditionalRepositories = 8
	if _, err := Generate(context.Background(), dir, bad, LibraryBuilder); err == nil {
		t.Fatal("eight additional repositories admitted")
	}
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(context.Background(), dir, Smoke, LibraryBuilder); err == nil {
		t.Fatal("non-empty directory admitted")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("refusal wrote %v", entries)
	}
}
