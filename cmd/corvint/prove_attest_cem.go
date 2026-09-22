package main

import (
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/attest"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// maxAttestationEnvelopeBytes bounds the envelope verify-cem mode reads; it
// matches the prove-observe stdin bound (FPK-V0-031, experimental).
const maxAttestationEnvelopeBytes = 8 << 20

// cemAttestation names the map bytes the CEM verifier read, for the second
// statement `--attest-cem` emits.
type cemAttestation struct {
	name  string
	bytes []byte
}

// cemAttestationInput returns the CEM statement input when --attest-cem was
// given, and nil otherwise so the FPK-V0-015 output is untouched.
func cemAttestationInput(prove proveOptions, mapBytes []byte) *cemAttestation {
	if !prove.attestCEM {
		return nil
	}
	return &cemAttestation{name: filepath.ToSlash(prove.mapPath), bytes: mapBytes}
}

// proveVerifiesCEMAttestation reports whether --verify-cem-attestation appears
// before --, which selects verify-cem mode wherever it is placed.
func proveVerifiesCEMAttestation(rest []string) bool {
	for _, argument := range rest {
		if argument == "--" {
			return false
		}
		if argument == "--verify-cem-attestation" || strings.HasPrefix(argument, "--verify-cem-attestation=") {
			return true
		}
	}
	return false
}

// parseProveVerifyCEMArguments reads `--verify-cem-attestation ENVELOPE
// --attest-public-key PEM [--cem MAP]`; every flag takes one value and none
// repeats. The root is resolved only when a map is to be read.
func parseProveVerifyCEMArguments(rootArguments, rest []string) (options, error) {
	result := options{command: "prove", proveMode: "verify-cem"}
	root := "."
	for index := 0; index < len(rootArguments); index++ {
		if rootArguments[index] == "--root" {
			root, index = rootArguments[index+1], index+1
		} else {
			root = strings.TrimPrefix(rootArguments[index], "--root=")
		}
	}
	fields := map[string]*string{
		"--verify-cem-attestation": &result.prove.envelopePath,
		"--attest-public-key":      &result.prove.publicKeyPath,
		"--cem":                    &result.prove.mapPath,
	}
	seen := map[string]bool{}
	for index := 0; index < len(rest); {
		name, value, inline := strings.Cut(rest[index], "=")
		field := fields[name]
		if field == nil {
			return result, argumentError("unrecognized arguments: " + rest[index])
		}
		if seen[name] {
			return result, argumentError("argument " + name + ": may not be repeated")
		}
		seen[name] = true
		if !inline {
			if index+1 >= len(rest) || argparseOptionLike(rest[index+1]) {
				return result, argumentError("argument " + name + ": expected one argument")
			}
			value, index = rest[index+1], index+2
		} else {
			index++
		}
		if value == "" {
			return result, argumentError("argument " + name + ": expected one argument")
		}
		*field = value
	}
	if result.prove.envelopePath == "" {
		return result, argumentError("argument --verify-cem-attestation: expected one argument")
	}
	if result.prove.publicKeyPath == "" {
		return result, argumentError("argument --attest-public-key: expected one argument")
	}
	if result.prove.mapPath == "" {
		return result, nil
	}
	resolved, err := resolveExplicitRoot(root)
	result.root = resolved
	return result, err
}

type cemAttestationReceipt struct {
	CEM           cemAttestationClaim `json:"cem"`
	Mutates       bool                `json:"mutates"`
	OK            bool                `json:"ok"`
	PredicateType string              `json:"predicateType"`
	Reason        string              `json:"reason,omitempty"`
	Status        string              `json:"status"`
	Tool          string              `json:"tool"`
}

type cemAttestationClaim struct {
	Digest map[string]string `json:"digest"`
	Name   string            `json:"name"`
	Size   int64             `json:"size"`
	Spec   string            `json:"spec"`
}

// runVerifyCEMAttestation verifies a CEM DSSE envelope offline and prints the
// signed claim with VERIFIED or, without --cem, NOT_RUN. It runs no Git and
// writes nothing.
func runVerifyCEMAttestation(options options, stdout, stderr io.Writer) int {
	verification, err := verifyCEMAttestation(options)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(cemAttestationReceipt{
		CEM: cemAttestationClaim{
			Digest: map[string]string{"sha256": verification.SHA256}, Name: verification.Name,
			Size: verification.Size, Spec: verification.Spec,
		},
		OK: true, PredicateType: attest.CEMPredicateType, Reason: verification.Reason,
		Status: verification.Status, Tool: "prove",
	})
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot encode the CEM attestation verification"})
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write the CEM attestation verification"})
		return 2
	}
	return 0
}

func verifyCEMAttestation(options options) (attest.CEMVerification, error) {
	publicKey, err := attest.ReadPublicKey(options.prove.publicKeyPath)
	if err != nil {
		return attest.CEMVerification{}, attestPublicKeyRefusal(options.prove.publicKeyPath)
	}
	envelope, err := readBoundedFile(options.prove.envelopePath, maxAttestationEnvelopeBytes)
	if err != nil {
		return attest.CEMVerification{}, attestEnvelopeRefusal(options.prove.envelopePath)
	}
	mapBytes, err := readAttestedMap(options.root, options.prove.mapPath)
	if err != nil {
		return attest.CEMVerification{}, err
	}
	verification, err := attest.VerifyCEM(envelope, publicKey, mapBytes)
	if errors.Is(err, attest.ErrCEMBytesMismatch) {
		return attest.CEMVerification{}, &gokernel.Error{Code: "attest-cem-mismatch", Message: "the CEM bytes differ from the attested sha256 or size"}
	}
	if err != nil {
		return attest.CEMVerification{}, &gokernel.Error{Code: "attest-verification-failed", Message: "the envelope is not a verified CEM attestation for this key"}
	}
	return verification, nil
}

// readAttestedMap reads the map bytes to check, unparsed so a changed map is
// reported as a digest mismatch; no --cem returns nil, which VerifyCEM
// discloses as NOT_RUN.
func readAttestedMap(root, mapPath string) ([]byte, error) {
	if mapPath == "" {
		return nil, nil
	}
	return readCEMMapBytes(root, mapPath)
}

// readCEMMapBytes is the one bounded map read prove --cem and the attestation verifier share: a
// path that is absolute, climbs out of the root or names .git is refused before any read.
func readCEMMapBytes(root, mapPath string) ([]byte, error) {
	if !containedRelativePath(mapPath) {
		return nil, cemMapPathRefusal(mapPath)
	}
	data, err := readBoundedFileUnderRoot(root, mapPath, wire.MaxMapBytes)
	if err != nil {
		return nil, cemMapReadRefusal(mapPath)
	}
	return data, nil
}
