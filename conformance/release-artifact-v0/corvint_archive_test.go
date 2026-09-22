package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archiveverify"
	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivewire"
)

func TestArchiveIdentityVersions(t *testing.T) {
	t.Run("CRB-V0-004 closed current and legacy archive identities", testArchiveIdentityVersions)
}

func testArchiveIdentityVersions(t *testing.T) {
	current, _ := validArchiveReportForTest(gitState{strings.Repeat("a", 40), strings.Repeat("b", 40)})
	legacy := current
	legacy.Schema = legacyArchiveReportSchema
	// Both schema versions name the one `corvint` archive set (decision 0308);
	// the retired `corvint-go` names are rejected under either.
	for _, report := range []ArchiveReport{legacy, current} {
		if err := validateArchiveReport(report); err != nil {
			t.Fatal(err)
		}
		if err := validateArchiveWitness(report.witness()); err != nil {
			t.Fatal(err)
		}
		retired := report
		retired.Targets = append([]ArchiveTargetReport(nil), report.Targets...)
		retired.Targets[1].BinaryName = strings.Replace(retired.Targets[1].BinaryName, "corvint", "corvint-go", 1)
		retired.Targets[1].ArchiveName = strings.Replace(retired.Targets[1].ArchiveName, "corvint", "corvint-go", 1)
		if validateArchiveReport(retired) == nil || validateArchiveWitness(retired.witness()) == nil {
			t.Fatal("retired archive name accepted")
		}
	}
}

func TestPublicationIdentityPairs(t *testing.T) {
	t.Run("ARTIFACT-RDY-V0-006 matched publication identities", testPublicationIdentityPairs)
}

func testPublicationIdentityPairs(t *testing.T) {
	fixture := newPublicationFixture(t)
	receipt := validReceiptPublicationTest(fixture)
	if err := validatePublicationReceipt(receipt, receipt.Tag); err != nil {
		t.Fatal(err)
	}
	for i := range receipt.Files {
		receipt.Files[i].Name = strings.Replace(receipt.Files[i].Name, "corvint_", "corvint-go_", 1)
	}
	if validatePublicationReceipt(receipt, receipt.Tag) == nil {
		t.Fatal("pre-rename archive names accepted at the Corvint destination")
	}
}

// This host proof deliberately does not call the five-target report/witness writer.
func TestCorvintHostArchivePartialProof(t *testing.T) {
	t.Run("ARTIFACT-GO-V0-002 Corvint six member host archive", testCorvintHostArchivePartialProof)
}

func testCorvintHostArchivePartialProof(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("host proof requires Darwin arm64; five-target qualification NOT_RUN")
	}
	ctx := context.Background()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	state, err := resolveCleanState(ctx, ArchiveOptions{Root: root, Revision: "HEAD"}, scratch)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	gitdir, err := closedGit(ctx, root, scratch, "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatal(err)
	}
	target := Target{GOOS: "darwin", GOARCH: "arm64"}
	var binaries, archives [2][]byte
	var legal []archivewire.ExpectedFile
	for i, label := range []string{"a", "b"} {
		source := filepath.Join(scratch, "source-"+label)
		if err := exportRawCommit(ctx, root, scratch, state.commit, source); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(scratch, "build-"+label, "corvint")
		if err := archiveBuildOnce(ctx, source, strings.TrimSpace(string(gitdir)), state.commit, manifest, target, output, filepath.Join(scratch, "cache-"+label), filepath.Join(scratch, "tmp-"+label)); err != nil {
			t.Fatal(err)
		}
		binaries[i], err = os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		files, err := loadCommittedLegal(source, manifest)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			legal = files
		} else if !equalLegal(legal, files) {
			t.Fatal("legal export mismatch")
		}
		archives[i], err = assembleArchiveTo("tar.gz", archiveMembers("corvint_darwin_arm64", "corvint", binaries[i], files), filepath.Join(scratch, "assembly-"+label, "corvint_darwin_arm64.tar.gz"))
		if err != nil {
			t.Fatal(err)
		}
	}
	input := archiveverify.Input{ArchiveName: "corvint_darwin_arm64.tar.gz", Format: "tar.gz", Root: "corvint_darwin_arm64", BinaryName: "corvint", GOOS: target.GOOS, GOARCH: target.GOARCH, Commit: state.commit, Tree: state.tree, GoVersion: manifest.Toolchain.GoVersion, PackagePath: manifest.Profile.ModulePath + "/cmd/corvint", ManifestBytes: manifestBytes, ArchiveBytes: archives[0], SecondArchive: archives[1], LooseBinary: binaries[0], SecondBinary: binaries[1], LooseGateBinary: binaries[0], LegalFiles: legal, MaximumArchiveBytes: archivewire.HardMaxArchiveBytes, MaximumContentBytes: archivewire.HardMaxContentBytes}
	result, err := archiveverify.Verify(input)
	if err != nil || result.Verdict != statusPass {
		t.Fatalf("independent verification: %+v %v", result, err)
	}
	zipped, err := gzip.NewReader(bytes.NewReader(archives[0]))
	if err != nil {
		t.Fatal(err)
	}
	defer zipped.Close()
	reader := tar.NewReader(zipped)
	extracted := filepath.Join(scratch, "installed", "corvint")
	if err := os.MkdirAll(filepath.Dir(extracted), 0700); err != nil {
		t.Fatal(err)
	}
	count := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		count++
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == "corvint_darwin_arm64/corvint" {
			if err := os.WriteFile(extracted, data, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}
	if count != 6 {
		t.Fatalf("members=%d", count)
	}
	build, err := buildNumber(ctx, root, state.commit)
	if err != nil {
		t.Fatal(err)
	}
	smoke := smokeTest(ctx, extracted, manifest.Smoke, build, scratch)
	if smoke.Status != statusPass || !smoke.RepositoryUnchanged {
		t.Fatalf("extracted smoke: %+v", smoke)
	}
	if output, err := runCaptured(ctx, extracted, "help"); err != nil || !strings.Contains(output, "Corvint extraction alpha") || strings.Contains(output, "corvint-go ") {
		t.Fatalf("help: %v %s", err, output)
	}
	t.Logf("PARTIAL host-only commit=%s tree=%s archive=%s binary=%s members=6 version=%s query=%s; loose production gate and five-target PASS NOT_RUN", state.commit, state.tree, result.ArchiveSHA256, result.BinarySHA256, smoke.VersionOutput, smoke.Status)
}
