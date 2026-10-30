// archive-verifier is the separately compiled offline verifier process.
// It imports no assembler and invokes no subprocess, artifact, or network API.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archiveverify"
	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivewire"
)

const maximumRequestBytes = 2 << 20

func main() { os.Exit(run(os.Args[1:])) }

func run(arguments []string) int {
	flags := flag.NewFlagSet("archive-verifier", flag.ContinueOnError)
	requestPath := flags.String("request", "", "strict verifier request")
	resultPath := flags.String("result", "", "new verifier result")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *requestPath == "" || *resultPath == "" {
		return 2
	}
	requestBytes, _, err := readBounded(*requestPath, maximumRequestBytes)
	if err != nil {
		return 2
	}
	var request archivewire.Request
	if err := archivewire.DecodeStrict(requestBytes, &request); err != nil || request.Schema != archivewire.RequestSchema {
		return 2
	}
	canonicalRequest, err := json.Marshal(request)
	if err != nil || !bytesEqualLine(canonicalRequest, requestBytes) {
		return 2
	}
	result := archivewire.Result{Schema: archivewire.ResultSchema, Verdict: "FAIL"}
	input, err := materialize(request)
	if err == nil {
		result, err = archiveverify.Verify(input)
	}
	if err != nil {
		result.Schema, result.Verdict, result.Reason = archivewire.ResultSchema, "FAIL", reasonCode(err)
	}
	if err := writeCanonicalExclusive(*resultPath, result); err != nil {
		return 2
	}
	if result.Verdict != "PASS" {
		return 1
	}
	return 0
}

func materialize(request archivewire.Request) (archivewire.Input, error) {
	if request.MaximumArchiveBytes <= 0 || request.MaximumArchiveBytes > archivewire.HardMaxArchiveBytes || request.MaximumContentBytes <= 0 || request.MaximumContentBytes > archivewire.HardMaxContentBytes {
		return archivewire.Input{}, errors.New("resource-limit")
	}
	archiveA, archiveB, err := readDistinctPair(request.ArchiveAPath, request.ArchiveBPath, request.MaximumArchiveBytes)
	if err != nil {
		return archivewire.Input{}, err
	}
	binaryA, binaryB, err := readDistinctPair(request.BinaryAPath, request.BinaryBPath, request.MaximumContentBytes)
	if err != nil {
		return archivewire.Input{}, err
	}
	looseGateBinary, _, err := readBounded(request.LooseGateBinaryPath, request.MaximumContentBytes)
	if err != nil {
		return archivewire.Input{}, err
	}
	for _, candidate := range []string{request.BinaryAPath, request.BinaryBPath} {
		candidateInfo, statErr := os.Stat(candidate)
		gateInfo, gateErr := os.Stat(request.LooseGateBinaryPath)
		if statErr != nil || gateErr != nil || os.SameFile(candidateInfo, gateInfo) {
			return archivewire.Input{}, errors.New("loose-gate-file-not-distinct")
		}
	}
	return archivewire.Input{
		ArchiveName: request.ArchiveName, Format: request.Format, Root: request.Root, BinaryName: request.BinaryName,
		GOOS: request.GOOS, GOARCH: request.GOARCH, Commit: request.Commit, Tree: request.Tree,
		GoVersion: request.GoVersion, PackagePath: request.PackagePath, ManifestBytes: request.ManifestBytes,
		ArchiveBytes: archiveA, SecondArchive: archiveB, LooseBinary: binaryA, SecondBinary: binaryB, LooseGateBinary: looseGateBinary,
		LegalFiles: request.LegalFiles, MaximumArchiveBytes: request.MaximumArchiveBytes, MaximumContentBytes: request.MaximumContentBytes,
	}, nil
}

func readDistinctPair(firstPath, secondPath string, maximum int64) ([]byte, []byte, error) {
	firstPath, secondPath = filepath.Clean(firstPath), filepath.Clean(secondPath)
	if !filepath.IsAbs(firstPath) || !filepath.IsAbs(secondPath) || firstPath == secondPath || filepath.Dir(firstPath) == filepath.Dir(secondPath) {
		return nil, nil, errors.New("pair-paths-not-distinct")
	}
	firstDirectory, err := os.Stat(filepath.Dir(firstPath))
	if err != nil {
		return nil, nil, errors.New("pair-paths-not-distinct")
	}
	secondDirectory, err := os.Stat(filepath.Dir(secondPath))
	if err != nil || os.SameFile(firstDirectory, secondDirectory) {
		return nil, nil, errors.New("pair-paths-not-distinct")
	}
	first, firstInfo, err := readBounded(firstPath, maximum)
	if err != nil {
		return nil, nil, err
	}
	second, secondInfo, err := readBounded(secondPath, maximum)
	if err != nil {
		return nil, nil, err
	}
	if os.SameFile(firstInfo, secondInfo) {
		return nil, nil, errors.New("pair-files-not-distinct")
	}
	return first, second, nil
}

func readBounded(path string, maximum int64) ([]byte, os.FileInfo, error) {
	if maximum <= 0 {
		return nil, nil, errors.New("resource-limit")
	}
	file, err := openRegularNoFollow(path)
	if err != nil {
		return nil, nil, errors.New("resource-limit")
	}
	defer file.Close()
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maximum {
		return nil, nil, errors.New("resource-limit")
	}
	content, err := io.ReadAll(io.LimitReader(file, maximum+1))
	afterInfo, afterErr := file.Stat()
	if err != nil || afterErr != nil || !os.SameFile(info, afterInfo) || afterInfo.Size() != info.Size() || int64(len(content)) > maximum || int64(len(content)) != info.Size() {
		return nil, nil, errors.New("resource-limit")
	}
	return content, info, nil
}

func bytesEqualLine(canonical, raw []byte) bool {
	canonical = append(canonical, '\n')
	return string(canonical) == string(raw)
}

func writeCanonicalExclusive(path string, value any) error {
	content, err := json.Marshal(value)
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func reasonCode(err error) string {
	value := err.Error()
	if index := strings.IndexByte(value, ':'); index >= 0 {
		value = value[:index]
	}
	if value == "" || len(value) > 64 {
		return "verification-failed"
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return "verification-failed"
		}
	}
	return value
}
