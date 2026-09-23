package extevidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Pin is an exact authoring-kit consumer contract, not a Core authority claim.
// RepositoryID and Origin select the root repository in /1 and /2 records.
// ExecutableSHA256 is mandatory for command sources and forbidden for files.
type Pin struct {
	Schema, ProviderID, ProviderRevision     string
	RepositoryRevision, RepositoryID, Origin string
	ExecutableSHA256                         string
}

// ReadPinned reads one file or runs one contained command, then returns the
// original bytes only after strict decoding and exact pin agreement. Callers
// must keep the trusted local executable immutable between hashing and launch.
// This helper does not compose evidence or change the Core receipt contract.
func ReadPinned(ctx context.Context, root, source string, pin Pin) ([]byte, error) {
	if err := pin.validate(); err != nil {
		return nil, err
	}
	data, err := readPinnedSource(ctx, root, source, pin.ExecutableSHA256)
	if err != nil {
		return nil, err
	}
	if err := pin.check(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (pin Pin) validate() error {
	if pin.Schema != Schema && pin.Schema != Schema1 && pin.Schema != Schema2 {
		return errors.New("unsupported pinned schema")
	}
	if checkIdentifier("provider.id", pin.ProviderID) != nil || pin.ProviderID == "path" {
		return errors.New("invalid pinned provider id")
	}
	if checkIdentifier("provider.revision", pin.ProviderRevision) != nil {
		return errors.New("invalid pinned provider revision")
	}
	if !blobPattern.MatchString(pin.RepositoryRevision) {
		return errors.New("repository revision pin must be a full commit id")
	}
	if pin.Schema == Schema {
		if pin.RepositoryID != "" || pin.Origin != "" {
			return errors.New("schema /0 does not carry repository identity")
		}
		return nil
	}
	if checkIdentifier("repository.id", pin.RepositoryID) != nil || !blobPattern.MatchString(pin.Origin) {
		return errors.New("repository id and full origin pin required")
	}
	return nil
}

func (pin Pin) check(data []byte) error {
	if err := profileReason(data, pin.Schema); err != nil {
		return err
	}
	if pin.Schema == Schema {
		record, err := Decode(data)
		if err != nil {
			return err
		}
		return pin.checkIdentity(record.Provider, record.Repository.Revision)
	}
	record, err := Decode1(data)
	if err != nil {
		return err
	}
	claims := 0
	for _, repository := range record.Repositories {
		if repository.Origin == pin.Origin {
			claims++
		}
	}
	if claims > 1 {
		return errors.New("ambiguous pinned repository origin")
	}
	for _, repository := range record.Repositories {
		if repository.ID == pin.RepositoryID {
			if repository.Origin != pin.Origin {
				return errors.New("repository origin differs from pin")
			}
			return pin.checkIdentity(record.Provider, repository.Revision)
		}
	}
	return errors.New("pinned repository absent")
}

// maxPinnedDepth exceeds every legal record nesting; deeper input is refused
// before the member walk rather than recursed into.
const maxPinnedDepth = 32

// profileReason gives an ambiguous, unsupported and mismatched record profile
// distinct reasons (EEP-V0-020). A repeated member is ambiguous because Go's
// decoder would silently keep its last value.
func profileReason(data []byte, pinned string) error {
	repeated, err := repeatedMember(json.NewDecoder(bytes.NewReader(data)), "", 0)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		return fmt.Errorf("record is not a strict JSON document: %s", trimJSONError(err))
	}
	if repeated == "schema" {
		return errors.New("ambiguous record profile: repeated schema member")
	}
	if repeated != "" {
		return fmt.Errorf("ambiguous record: repeated member %q", repeated)
	}
	declared := declaredSchema(data)
	if declared != Schema && declared != Schema1 && declared != Schema2 {
		return errors.New("unsupported record profile")
	}
	if declared != pinned {
		return errors.New("record schema differs from pin")
	}
	return nil
}

// repeatedMember returns the dotted path of the first member name repeated
// within one object of the next JSON value, or "" when every name is unique.
func repeatedMember(decoder *json.Decoder, prefix string, depth int) (string, error) {
	if depth > maxPinnedDepth {
		return "", fmt.Errorf("nesting exceeds %d", maxPinnedDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return "", err
	}
	delim, container := token.(json.Delim)
	if !container {
		return "", nil
	}
	seen := map[string]bool{}
	for decoder.More() {
		child := prefix
		if delim == '{' {
			key, err := decoder.Token()
			if err != nil {
				return "", err
			}
			child = prefix + key.(string)
			if seen[child] {
				return child, nil
			}
			seen[child] = true
			child += "."
		}
		if found, err := repeatedMember(decoder, child, depth+1); found != "" || err != nil {
			return found, err
		}
	}
	_, err = decoder.Token()
	return "", err
}

func (pin Pin) checkIdentity(identity Identity, revision string) error {
	if identity.ID != pin.ProviderID || identity.Revision != pin.ProviderRevision {
		return errors.New("provider identity differs from pin")
	}
	if revision != pin.RepositoryRevision {
		return errors.New("repository revision differs from pin")
	}
	return nil
}

func readPinnedSource(ctx context.Context, root, source, digest string) ([]byte, error) {
	if argv, command := commandArgv(source); command {
		if err := checkExecutable(argv[0], digest); err != nil {
			return nil, err
		}
		data, _, reason := runCommand(ctx, root, argv)
		if reason != "" {
			return nil, errors.New(reason)
		}
		return data, nil
	}
	if digest != "" {
		return nil, errors.New("executable digest requires command source")
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(root, source)
	}
	file, err := openRegular(source)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxRecordBytes+1))
	if len(data) > MaxRecordBytes {
		return nil, errors.New("record exceeds 1048576 bytes")
	}
	return data, err
}

func checkExecutable(path, digest string) error {
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != digest {
		return errors.New("command requires lowercase executable SHA256 pin")
	}
	file, err := openRegular(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != digest {
		return errors.New("executable SHA256 differs from pin")
	}
	return nil
}

func openRegular(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("source must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err = file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("source must be a regular file")
	}
	return file, nil
}
