package main

import (
	"flag"
	"fmt"
	"os"
)

func runArchiveStatusCLI(arguments []string, stdout, stderr *os.File) int {
	flags := flag.NewFlagSet("release-artifact-v0 archive-status", flag.ContinueOnError)
	flags.SetOutput(stdout)
	witnessPath := flags.String("witness", "", "private witness path")
	revision := flags.String("revision", "", "expected full revision")
	tree := flags.String("tree", "", "expected full tree")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *witnessPath == "" || *revision == "" || *tree == "" {
		fmt.Fprintln(stderr, "release-artifact-v0 archive-status: invalid invocation")
		return 2
	}
	witness, err := readArchiveWitness(*witnessPath)
	if err != nil {
		fmt.Fprintln(stdout, "INVALID")
		fmt.Fprintln(stderr, "release-artifact-v0 archive-status:", err)
		return 1
	}
	if witness.Revision != *revision || witness.Tree != *tree {
		fmt.Fprintln(stdout, "STALE")
		return 0
	}
	fmt.Fprintln(stdout, witness.Verdict)
	return 0
}

func readArchiveWitness(path string) (ArchiveWitness, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return ArchiveWitness{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return ArchiveWitness{}, fmt.Errorf("witness must be a regular 0600 file")
	}
	if err := invokingUserOwns(info); err != nil {
		return ArchiveWitness{}, err
	}
	if info.Size() <= 0 || info.Size() > maxWitnessBytes {
		return ArchiveWitness{}, fmt.Errorf("witness size is outside the bound")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ArchiveWitness{}, err
	}
	var witness ArchiveWitness
	if err := decodeStrictJSON(raw, &witness); err != nil {
		return ArchiveWitness{}, err
	}
	if err := validateArchiveWitness(witness); err != nil {
		return ArchiveWitness{}, err
	}
	canonical, err := canonicalJSON(witness)
	if err != nil || string(canonical) != string(raw) {
		return ArchiveWitness{}, fmt.Errorf("witness is not canonical compact JSON with one LF")
	}
	return witness, nil
}

func validateArchiveWitness(witness ArchiveWitness) error {
	if (witness.Schema != archiveWitnessSchema && witness.Schema != legacyArchiveWitnessSchema) || !gitObjectPattern.MatchString(witness.Revision) || !gitObjectPattern.MatchString(witness.Tree) {
		return fmt.Errorf("witness schema or Git identity is invalid")
	}
	if witness.Verdict != statusPass && witness.Verdict != statusFail {
		return fmt.Errorf("witness verdict is invalid")
	}
	if witness.Verdict == statusFail {
		if len(witness.Archives) != 0 || len(witness.Reasons) == 0 {
			return fmt.Errorf("FAIL witness must have reasons and no retained archives")
		}
		return nil
	}
	expected := []string{
		"corvint_darwin_amd64.tar.gz", "corvint_darwin_arm64.tar.gz",
		"corvint_linux_amd64.tar.gz", "corvint_linux_arm64.tar.gz",
		"corvint_windows_amd64.zip",
	}
	if len(witness.Archives) != len(expected) || len(witness.Reasons) != 0 {
		return fmt.Errorf("PASS witness requires exactly five archives and no reasons")
	}
	for index, archive := range witness.Archives {
		if archive.Name != expected[index] || !digestPattern.MatchString(archive.SHA256) || archive.Bytes <= 0 {
			return fmt.Errorf("archive witness row %d is invalid", index)
		}
	}
	return nil
}
