// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const (
	// coreGateBinary is the one binary the corvint archive gate builds and qualifies.
	coreGateBinary = "corvint"
	// coreGateReportName is the gate's report, which corvint-update reads as the Core
	// qualification of every corvint archive (ARTIFACT-BIN-V0-526).
	coreGateReportName   = "verification-report.json"
	coreGateReportSchema = "corvint.release-go-archive-report.v2"
	coreGateReportLimit  = 1 << 20
	// releaseChecksumsName is the release's one checksum file (ARTIFACT-BIN-V0-525).
	releaseChecksumsName = "SHA256SUMS"
)

// coreGateDigest is one digest object of the gate report.
type coreGateDigest struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// coreGateReport is the part of the gate report the binding reads; the copy written into the
// release is the gate's bytes, unchanged.
type coreGateReport struct {
	Schema    string            `json:"schema"`
	Revision  string            `json:"revision"`
	Toolchain string            `json:"toolchain"`
	Verdict   string            `json:"verdict"`
	Reasons   []json.RawMessage `json:"reasons"`
	Targets   []struct {
		ArchiveName     string         `json:"archiveName"`
		RetainedBinary  coreGateDigest `json:"retainedBinary"`
		RetainedArchive coreGateDigest `json:"retainedArchive"`
	} `json:"targets"`
}

// coreGate is a loaded gate output directory.
type coreGate struct {
	directory string
	raw       []byte
	report    coreGateReport
}

// checkCoreGateUse refuses before anything is built when the tag's selection and the
// presence of --core-gate disagree (ARTIFACT-BIN-V0-526).
func checkCoreGateUse(selection Selection, directory string) error {
	builds := slices.Contains(selection.Binaries, coreGateBinary)
	switch {
	case builds && directory == "":
		return fmt.Errorf("core-gate-required: %s builds %s; pass --core-gate DIR, the output of `go run ./conformance/release-artifact-v0 archive` at the same commit, so the release carries the Core qualification corvint-update requires", selection.Tag, coreGateBinary)
	case !builds && directory != "":
		return fmt.Errorf("core-gate-unused: %s does not build %s; drop --core-gate", selection.Tag, coreGateBinary)
	}
	return nil
}

// loadCoreGate reads and shape-checks the gate report and binds it to the commit.
func loadCoreGate(directory, commit string) (coreGate, error) {
	path := filepath.Join(directory, coreGateReportName)
	info, err := os.Stat(path)
	if err != nil {
		return coreGate{}, fmt.Errorf("core-gate-mismatch: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > coreGateReportLimit {
		return coreGate{}, fmt.Errorf("core-gate-mismatch: %s is not a regular file of at most %d bytes", path, coreGateReportLimit)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return coreGate{}, fmt.Errorf("core-gate-mismatch: %w", err)
	}
	var report coreGateReport
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&report); err != nil {
		return coreGate{}, fmt.Errorf("core-gate-mismatch: %s: %w", path, err)
	}
	switch {
	case report.Schema != coreGateReportSchema:
		return coreGate{}, fmt.Errorf("core-gate-mismatch: %s schema %q, want %q", coreGateReportName, report.Schema, coreGateReportSchema)
	case report.Verdict != "PASS" || len(report.Reasons) != 0:
		return coreGate{}, fmt.Errorf("core-gate-mismatch: %s verdict %q with %d reasons, want PASS with none", coreGateReportName, report.Verdict, len(report.Reasons))
	case report.Revision != commit:
		return coreGate{}, fmt.Errorf("core-gate-mismatch: %s revision %s, but release builds %s", coreGateReportName, report.Revision, commit)
	}
	return coreGate{directory: directory, raw: raw, report: report}, nil
}

// bind requires every corvint archive the release built to be one of the gate's retained
// archives, byte for byte, and the report to name the toolchain the release built with.
func (g coreGate) bind(entry Entry) error {
	if g.report.Toolchain != entry.GoVersion {
		return fmt.Errorf("core-gate-mismatch: %s toolchain %q, but release built with %q", coreGateReportName, g.report.Toolchain, entry.GoVersion)
	}
	var problems []string
	for _, archive := range entry.Archives {
		matches := 0
		for _, target := range g.report.Targets {
			if target.ArchiveName != archive.Name {
				continue
			}
			matches++
			if target.RetainedArchive.SHA256 != archive.SHA256 || target.RetainedArchive.Bytes != archive.Bytes {
				problems = append(problems, archive.Name+": retainedArchive differs from the archive release built")
			}
			if target.RetainedBinary.SHA256 != archive.BinarySHA256 {
				problems = append(problems, archive.Name+": retainedBinary differs from the binary release built")
			}
		}
		if matches != 1 {
			problems = append(problems, fmt.Sprintf("%s: %d report targets, want 1", archive.Name, matches))
			continue
		}
		raw, err := readBoundedFile(filepath.Join(g.directory, archive.Name), archive.Bytes)
		if err != nil {
			problems = append(problems, archive.Name+": gate archive: "+err.Error())
		} else if digestOf(raw) != archive.SHA256 {
			problems = append(problems, archive.Name+": gate archive sha256 differs")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New("core-gate-mismatch: " + strings.Join(problems, "; "))
	}
	return nil
}

// releaseChecksums is one row per archive in the manifest's records that is present in
// directory, sorted by archive name (ARTIFACT-BIN-V0-525).
func releaseChecksums(manifest Manifest, directory string) []byte {
	var rows []string
	for _, record := range manifest.Binaries {
		for _, archive := range record.Archives {
			if _, err := os.Lstat(filepath.Join(directory, archive.Name)); err == nil {
				rows = append(rows, archive.Name+"\x00"+archive.SHA256)
			}
		}
	}
	sort.Strings(rows)
	var out strings.Builder
	for _, row := range rows {
		name, sum, _ := strings.Cut(row, "\x00")
		out.WriteString(sum + "  " + name + "\n")
	}
	return []byte(out.String())
}
