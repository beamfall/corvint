//go:build !darwin

package main

import (
	"errors"
	"github.com/Beamfall/corvint/internal/authoritystore"
	"os"
)

func stageAndPublishEvidence(*os.File, string, authoritystore.EvidenceManifest, map[string][]byte, []byte, uint32, uint32) error {
	return errors.New("Darwin evidence publication required")
}
func writeEvidenceStageFD(*os.File, authoritystore.EvidenceManifest, map[string][]byte, []byte, uint32, uint32) error {
	return errors.New("Darwin evidence publication required")
}
func removeEvidenceStageFD(*os.File) error { return errors.New("Darwin evidence publication required") }
