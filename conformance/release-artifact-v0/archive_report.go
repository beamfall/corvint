package main

import (
	"encoding/json"
	"fmt"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivewire"
)

const (
	archiveReportSchema        = "corvint.release-go-archive-report.v2"
	legacyArchiveReportSchema  = "corvint.release-go-archive-report.v1"
	archiveWitnessSchema       = "corvint.release-go-archive-witness.v2"
	legacyArchiveWitnessSchema = "corvint.release-go-archive-witness.v1"
	archiveReportName          = "verification-report.json"
	maxWitnessBytes            = 64 << 10
)

type ArchiveReport struct {
	Schema         string                `json:"schema"`
	Revision       string                `json:"revision"`
	Tree           string                `json:"tree"`
	ManifestSHA256 string                `json:"manifestSha256"`
	Toolchain      string                `json:"toolchain"`
	Verdict        string                `json:"verdict"`
	Limits         ArchiveLimits         `json:"limits"`
	Targets        []ArchiveTargetReport `json:"targets"`
	Reasons        []ArchiveReason       `json:"reasons"`
}

type ArchiveLimits struct {
	Targets             int   `json:"targets"`
	MembersPerArchive   int   `json:"membersPerArchive"`
	MaximumArchiveBytes int64 `json:"maximumArchiveBytes"`
	MaximumTotalBytes   int64 `json:"maximumTotalArchiveBytes"`
	MaximumReportBytes  int64 `json:"maximumReportBytes"`
	MaximumWitnessBytes int64 `json:"maximumWitnessBytes"`
}

type ArchiveDigest struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ArchiveTargetReport struct {
	GOOS            string        `json:"goos"`
	GOARCH          string        `json:"goarch"`
	BinaryName      string        `json:"binaryName"`
	ArchiveName     string        `json:"archiveName"`
	BuildA          ArchiveDigest `json:"buildA"`
	BuildB          ArchiveDigest `json:"buildB"`
	AssemblyA       ArchiveDigest `json:"assemblyA"`
	AssemblyB       ArchiveDigest `json:"assemblyB"`
	RetainedBinary  ArchiveDigest `json:"retainedBinary"`
	RetainedArchive ArchiveDigest `json:"retainedArchive"`
}

type ArchiveReason struct {
	Code   string `json:"code"`
	Target string `json:"target,omitempty"`
}

type ArchiveWitness struct {
	Schema   string                 `json:"schema"`
	Revision string                 `json:"revision"`
	Tree     string                 `json:"tree"`
	Verdict  string                 `json:"verdict"`
	Archives []ArchiveWitnessDigest `json:"archives"`
	Reasons  []string               `json:"reasons"`
}

type ArchiveWitnessDigest struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

func canonicalJSON(value any) ([]byte, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func (report ArchiveReport) witness() ArchiveWitness {
	witness := ArchiveWitness{Schema: archiveWitnessSchema, Revision: report.Revision, Tree: report.Tree, Verdict: report.Verdict, Archives: []ArchiveWitnessDigest{}, Reasons: []string{}}
	if report.Schema == legacyArchiveReportSchema {
		witness.Schema = legacyArchiveWitnessSchema
	}
	for _, target := range report.Targets {
		if report.Verdict == statusPass && target.RetainedArchive.SHA256 != "" {
			witness.Archives = append(witness.Archives, ArchiveWitnessDigest{Name: target.ArchiveName, SHA256: target.RetainedArchive.SHA256, Bytes: target.RetainedArchive.Bytes})
		}
	}
	for _, reason := range report.Reasons {
		witness.Reasons = append(witness.Reasons, reason.Code)
	}
	return witness
}

func validateArchiveReport(report ArchiveReport) error {
	if (report.Schema != archiveReportSchema && report.Schema != legacyArchiveReportSchema) || (report.Verdict != statusPass && report.Verdict != statusFail) {
		return fmt.Errorf("invalid archive report schema or verdict")
	}
	if !gitObjectPattern.MatchString(report.Revision) || !gitObjectPattern.MatchString(report.Tree) || !digestPattern.MatchString(report.ManifestSHA256) || report.Toolchain != "go1.27.1" {
		return fmt.Errorf("invalid archive report immutable identity")
	}
	expected := []struct{ goos, goarch, binary, archive string }{
		{"darwin", "amd64", "corvint", "corvint_darwin_amd64.tar.gz"},
		{"darwin", "arm64", "corvint", "corvint_darwin_arm64.tar.gz"},
		{"linux", "amd64", "corvint", "corvint_linux_amd64.tar.gz"},
		{"linux", "arm64", "corvint", "corvint_linux_arm64.tar.gz"},
		{"windows", "amd64", "corvint.exe", "corvint_windows_amd64.zip"},
	}
	if report.Verdict == statusPass && (len(report.Targets) != len(expected) || len(report.Reasons) != 0) {
		return fmt.Errorf("PASS report requires five targets and no reasons")
	}
	limits := report.Limits
	if limits.Targets != len(expected) || limits.MembersPerArchive != archivewire.MaxMembers ||
		limits.MaximumArchiveBytes <= 0 || limits.MaximumArchiveBytes > archivewire.HardMaxArchiveBytes ||
		limits.MaximumTotalBytes < limits.MaximumArchiveBytes || limits.MaximumTotalBytes > archivewire.HardMaxArchiveBytes*int64(len(expected)) ||
		limits.MaximumReportBytes != maximumArchiveReportBytes || limits.MaximumWitnessBytes != maxWitnessBytes {
		return fmt.Errorf("archive report limits are invalid")
	}
	var retainedArchiveBytes int64
	for index, target := range report.Targets {
		if index >= len(expected) || target.GOOS != expected[index].goos || target.GOARCH != expected[index].goarch || target.BinaryName != expected[index].binary || target.ArchiveName != expected[index].archive {
			return fmt.Errorf("report target order or identity is invalid")
		}
		for _, value := range []string{target.BinaryName, target.ArchiveName} {
			if value == "" || value[0] == '/' {
				return fmt.Errorf("report contains an invalid path-bearing name")
			}
		}
		for _, digest := range []ArchiveDigest{target.BuildA, target.BuildB, target.AssemblyA, target.AssemblyB, target.RetainedBinary, target.RetainedArchive} {
			if !digestPattern.MatchString(digest.SHA256) || digest.Bytes <= 0 {
				return fmt.Errorf("report contains an invalid digest row")
			}
		}
		if target.BuildA != target.BuildB || target.AssemblyA != target.AssemblyB || target.BuildA != target.RetainedBinary || target.AssemblyA != target.RetainedArchive {
			return fmt.Errorf("report double-build or double-assembly evidence disagrees")
		}
		if target.RetainedArchive.Bytes > limits.MaximumArchiveBytes || target.RetainedBinary.Bytes > archivewire.HardMaxContentBytes {
			return fmt.Errorf("report digest row exceeds limits")
		}
		retainedArchiveBytes += target.RetainedArchive.Bytes
	}
	if retainedArchiveBytes > limits.MaximumTotalBytes {
		return fmt.Errorf("report retained bytes exceed total limit")
	}
	return nil
}
