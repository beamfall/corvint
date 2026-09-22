package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// The publication receipt reader (ARTIFACT-RDY-V0-006..009, decision 0141). It binds a committed
// owner receipt to the exact release tag's revision, not to HEAD, and cross-checks every attached
// digest against the private archive witness bound to that revision. It is read-only and offline.

const (
	publicationReceiptSchema     = "corvint.release-publication-receipt.v1"
	maxPublicationReceiptBytes   = 64 << 10
	maxPublicationReceiptFiles   = 16
	publicationDestinationPrefix = "https://github.com/Beamfall/corvint/releases/tag/"
	publicationChecksumsName     = "SHA256SUMS"

	publicationPass        = "PASS"
	publicationAbsent      = "ABSENT"
	publicationStale       = "STALE"
	publicationUnwitnessed = "UNWITNESSED"
	publicationMismatch    = "MISMATCH"
	publicationInvalid     = "INVALID"
)

var (
	releaseTagPattern       = regexp.MustCompile(`^v[0-9A-Za-z.+-]{1,64}$`)
	decisionRecordPattern   = regexp.MustCompile(`^docs/decisions/[0-9]{4}-[a-z0-9-]+\.md$`)
	publicationFilePattern  = regexp.MustCompile(`^[A-Za-z0-9._+-]{1,128}$`)
	publicationArchiveNames = regexp.MustCompile(`^corvint_(darwin|linux)_(amd64|arm64)\.tar\.gz$|^corvint_windows_amd64\.zip$`)
)

type PublicationReceipt struct {
	Schema            string                   `json:"schema"`
	Approval          string                   `json:"approval"`
	Decision          string                   `json:"decision"`
	Destination       string                   `json:"destination"`
	Tag               string                   `json:"tag"`
	Revision          string                   `json:"revision"`
	Tree              string                   `json:"tree"`
	PublisherIdentity string                   `json:"publisherIdentity"`
	Files             []PublicationReceiptFile `json:"files"`
}

type PublicationReceiptFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

type gitReader func(arguments ...string) ([]byte, error)

func runPublicationStatusCLI(arguments []string, stdout, stderr *os.File) int {
	flags := flag.NewFlagSet("release-artifact-v0 publication-status", flag.ContinueOnError)
	flags.SetOutput(stdout)
	witnessPath := flags.String("witness", "", "private archive witness path")
	revision := flags.String("revision", "", "full HEAD revision the checklist evaluates")
	tag := flags.String("tag", "", "exact release tag name")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *witnessPath == "" || *revision == "" || *tag == "" {
		fmt.Fprintln(stderr, "release-artifact-v0 publication-status: invalid invocation")
		return 2
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "release-artifact-v0 publication-status:", err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	status, err := publicationStatus(ctx, root, *revision, *tag, *witnessPath)
	fmt.Fprintln(stdout, status)
	if err != nil {
		fmt.Fprintln(stderr, "release-artifact-v0 publication-status:", err)
	}
	if status == publicationInvalid {
		return 1
	}
	return 0
}

func publicationStatus(ctx context.Context, root, revision, tag, witnessPath string) (string, error) {
	if !gitObjectPattern.MatchString(revision) {
		return publicationInvalid, errors.New("revision argument is not a full object name")
	}
	if !releaseTagPattern.MatchString(tag) {
		return publicationInvalid, errors.New("tag argument is invalid")
	}
	home, err := os.MkdirTemp("", "corvint-release-receipt-")
	if err != nil {
		return publicationInvalid, err
	}
	defer os.RemoveAll(home)
	git := func(arguments ...string) ([]byte, error) { return closedGit(ctx, root, home, arguments...) }
	raw, present, err := committedBlob(git, revision, "docs/releases/"+tag+"/publication-receipt.json", maxPublicationReceiptBytes)
	if err != nil {
		return publicationInvalid, err
	}
	if !present {
		return publicationAbsent, nil
	}
	receipt, err := decodePublicationReceipt(raw, tag)
	if err != nil {
		return publicationInvalid, err
	}
	_, decisionPresent, err := committedBlob(git, revision, receipt.Decision, 1<<20)
	if err != nil {
		return publicationInvalid, err
	}
	if !decisionPresent {
		return publicationInvalid, errors.New("receipt decision record is not committed at the evaluated revision")
	}
	binding, err := publicationBinding(git, revision, receipt)
	if binding != publicationPass {
		return binding, err
	}
	return publicationWitnessStatus(witnessPath, receipt)
}

// committedBlob reads path from the commit object, never the worktree or index. An absent path is
// (nil, false, nil); a present non-regular or oversized entry is an error.
func committedBlob(git gitReader, revision, path string, limit int) ([]byte, bool, error) {
	listing, err := git("ls-tree", "-z", revision, "--", path)
	if err != nil {
		return nil, false, err
	}
	if len(listing) == 0 {
		return nil, false, nil
	}
	fields := strings.Fields(strings.SplitN(strings.TrimSuffix(string(listing), "\x00"), "\t", 2)[0])
	if len(fields) != 3 || fields[0] != "100644" || fields[1] != "blob" {
		return nil, true, fmt.Errorf("%s is not a regular non-executable blob", path)
	}
	content, err := git("cat-file", "blob", fields[2])
	if err != nil {
		return nil, true, err
	}
	if len(content) > limit {
		return nil, true, fmt.Errorf("%s exceeds its size bound", path)
	}
	return content, true, nil
}

func decodePublicationReceipt(raw []byte, tag string) (PublicationReceipt, error) {
	var receipt PublicationReceipt
	if err := decodeStrictJSON(raw, &receipt); err != nil {
		return PublicationReceipt{}, err
	}
	if err := validatePublicationReceipt(receipt, tag); err != nil {
		return PublicationReceipt{}, err
	}
	canonical, err := canonicalJSON(receipt)
	if err != nil || string(canonical) != string(raw) {
		return PublicationReceipt{}, errors.New("receipt is not canonical compact JSON with one LF")
	}
	return receipt, nil
}

func validatePublicationReceipt(receipt PublicationReceipt, tag string) error {
	identity := receipt.Schema == publicationReceiptSchema && receipt.Approval == "repository-owner" &&
		receipt.PublisherIdentity == "NOT_VERIFIED" && decisionRecordPattern.MatchString(receipt.Decision)
	if !identity {
		return errors.New("receipt schema, approval, publisher identity, or decision record is invalid")
	}
	binding := receipt.Tag == tag && receipt.Destination == publicationDestinationPrefix+tag &&
		gitObjectPattern.MatchString(receipt.Revision) && gitObjectPattern.MatchString(receipt.Tree)
	if !binding {
		return errors.New("receipt tag, destination, or Git identity is invalid")
	}
	return validatePublicationFiles(receipt.Files)
}

func validatePublicationFiles(files []PublicationReceiptFile) error {
	if len(files) == 0 || len(files) > maxPublicationReceiptFiles {
		return errors.New("receipt file count is outside the bound")
	}
	checksums, archives := 0, 0
	for index, file := range files {
		if !publicationFilePattern.MatchString(file.Name) || !digestPattern.MatchString(file.SHA256) {
			return fmt.Errorf("receipt file row %d is invalid", index)
		}
		if index > 0 && files[index-1].Name >= file.Name {
			return errors.New("receipt files are not strictly sorted by name")
		}
		if file.Name == publicationChecksumsName {
			checksums++
		}
		if publicationArchiveNames.MatchString(file.Name) {
			archives++
		}
	}
	if checksums != 1 || archives == 0 {
		return errors.New("receipt must attach SHA256SUMS and at least one Go archive")
	}
	return nil
}

// publicationBinding resolves the conflict decision 0141 settles: the release revision is the one
// the receipt records, which the exact tag must name and HEAD must descend from.
func publicationBinding(git gitReader, head string, receipt PublicationReceipt) (string, error) {
	refs, err := git("for-each-ref", "--format=%(refname)", "refs/tags/"+receipt.Tag)
	if err != nil {
		return publicationInvalid, err
	}
	if !containsLine(string(refs), "refs/tags/"+receipt.Tag) {
		return publicationStale, errors.New("exact release tag is absent")
	}
	tagCommit, err := git("rev-parse", "--verify", "refs/tags/"+receipt.Tag+"^{commit}")
	if err != nil {
		return publicationInvalid, err
	}
	if strings.TrimSpace(string(tagCommit)) != receipt.Revision {
		return publicationStale, errors.New("release tag does not name the receipt revision")
	}
	tree, err := git("rev-parse", "--verify", receipt.Revision+"^{tree}")
	if err != nil {
		return publicationInvalid, err
	}
	if strings.TrimSpace(string(tree)) != receipt.Tree {
		return publicationInvalid, errors.New("receipt tree does not match its revision")
	}
	return ancestryBinding(git, receipt.Revision, head)
}

func ancestryBinding(git gitReader, ancestor, head string) (string, error) {
	_, err := git("merge-base", "--is-ancestor", ancestor, head)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return publicationStale, errors.New("receipt revision is not an ancestor of the evaluated revision")
	}
	if err != nil {
		return publicationInvalid, err
	}
	return publicationPass, nil
}

func containsLine(text, want string) bool {
	for _, line := range strings.Split(text, "\n") {
		if line == want {
			return true
		}
	}
	return false
}

func publicationWitnessStatus(witnessPath string, receipt PublicationReceipt) (string, error) {
	if _, err := os.Lstat(witnessPath); errors.Is(err, os.ErrNotExist) {
		return publicationUnwitnessed, errors.New("private archive witness is absent")
	}
	witness, err := readArchiveWitness(witnessPath)
	if err != nil {
		return publicationInvalid, err
	}
	if witness.Revision != receipt.Revision || witness.Tree != receipt.Tree {
		return publicationUnwitnessed, errors.New("archive witness is not bound to the receipt revision")
	}
	if witness.Verdict != statusPass {
		return publicationMismatch, errors.New("archive witness at the receipt revision is FAIL")
	}
	witnessed := witnessedPublicationDigests(witness)
	for _, file := range receipt.Files {
		if digest, known := witnessed[file.Name]; known && digest != file.SHA256 {
			return publicationMismatch, fmt.Errorf("%s digest differs from the archive witness", file.Name)
		}
	}
	for _, file := range receipt.Files {
		if _, known := witnessed[file.Name]; !known {
			return publicationUnwitnessed, fmt.Errorf("%s has no local witness", file.Name)
		}
	}
	return publicationPass, nil
}

// witnessedPublicationDigests maps each witnessed archive to its digest and SHA256SUMS to the digest
// of the canonical checksum file the gate writes from those same five rows.
func witnessedPublicationDigests(witness ArchiveWitness) map[string]string {
	digests := make(map[string]string, len(witness.Archives)+1)
	var sums strings.Builder
	for _, archive := range witness.Archives {
		digests[archive.Name] = archive.SHA256
		sums.WriteString(archive.SHA256 + "  " + archive.Name + "\n")
	}
	digests[publicationChecksumsName] = digestBytes([]byte(sums.String()))
	return digests
}
