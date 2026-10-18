package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
	"github.com/Beamfall/corvint/internal/testvaliditydoc/externalorigins"
)

// errExternalOriginsRefused marks an external-origins refusal already written to stderr.
var errExternalOriginsRefused = errors.New("external-origins refused")

// parseExternalOrigins reads the external-origins flags: one project policy and 1..16 network
// evidence documents, in run order (PWP-V0-027).
func parseExternalOrigins(args []string) (string, []string, error) {
	fs := newSubcommandFlags("external-origins", flag.ContinueOnError)
	if usageSink == nil {
		fs.SetOutput(io.Discard)
	}
	policy := fs.String("policy", "", "corvint-external-origins-policy/0 file: the declared test environment and the deterministic external-origin allow-list (PWP-V0-025)")
	var network stringList
	fs.Var(&network, "network", "corvint-network-evidence/0 file of one run, repeatable in run order (PWP-V0-022)")
	if err := fs.Parse(args); err != nil {
		return "", nil, err
	}
	if fs.NArg() != 0 || *policy == "" || len(network) == 0 {
		return "", nil, errors.New("external-origins needs --policy FILE and at least one --network FILE, and takes no arguments")
	}
	return *policy, network, nil
}

// runExternalOrigins prints the corvint-external-origins/0 report. It runs no test and reads no
// network: the evidence is producer-stated (PWP-V0-027).
func runExternalOrigins(args []string, stdout, stderr io.Writer) error {
	policyPath, networkPaths, err := parseExternalOrigins(args)
	if err != nil {
		fmt.Fprintln(stderr, "external-origins:", err)
		return errExternalOriginsRefused
	}
	report, err := buildExternalOriginsReport(policyPath, networkPaths)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return errExternalOriginsRefused
	}
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = stdout.Write(append(data, '\n'))
	return err
}

func buildExternalOriginsReport(policyPath string, networkPaths []string) (externalorigins.ExternalOriginsReport, error) {
	data, err := readBoundedInput(policyPath, externalorigins.MaxExternalOriginsPolicyBytes)
	if err != nil {
		return externalorigins.ExternalOriginsReport{}, fmt.Errorf("%s: %v", externalorigins.ExternalOriginsPolicyInvalid, err)
	}
	policy, err := externalorigins.DecodeExternalOriginsPolicy(data)
	if err != nil {
		return externalorigins.ExternalOriginsReport{}, err
	}
	if len(networkPaths) > externalorigins.MaxNetworkEvidenceRuns {
		return externalorigins.ExternalOriginsReport{}, fmt.Errorf("%s: more than %d --network files", externalorigins.NetworkEvidenceInvalid, externalorigins.MaxNetworkEvidenceRuns)
	}
	runs := make([]externalorigins.NetworkEvidence, 0, len(networkPaths))
	for _, path := range networkPaths {
		data, err := readBoundedInput(path, externalorigins.MaxNetworkEvidenceBytes)
		if err != nil {
			return externalorigins.ExternalOriginsReport{}, fmt.Errorf("%s: %v", externalorigins.NetworkEvidenceInvalid, err)
		}
		run, err := externalorigins.DecodeNetworkEvidence(data)
		if err != nil {
			return externalorigins.ExternalOriginsReport{}, fmt.Errorf("%s: %w", path, err)
		}
		runs = append(runs, run)
	}
	return externalorigins.BuildExternalOriginsReport(policy, runs)
}

// readBoundedInput reads one regular file without following a final symlink, refusing it above
// limit bytes.
func readBoundedInput(path string, limit int64) ([]byte, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return testvaliditydoc.ReadFileBounded(root, filepath.Base(absolute), limit)
}
