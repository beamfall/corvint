package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localauthority"
)

const piReleaseProfile = "corvint-pi-authority-release/0"

func releaseFileLimit(path string) int {
	if path == "pi-protected" {
		return 256 << 20
	}
	return 64 << 20
}

// Build metadata is reviewed input, never a qualification or admission token.
func validatePiBuild(dir, consumer string) error {
	return validatePiBuildFiles(dir, consumer, "manifest.json")
}
func validatePiBuildFiles(dir, consumer, manifest string) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || consumer != filepath.Join(dir, "corvint") {
		return errors.New("absolute Pi build directory required")
	}
	raw, err := readRegular(filepath.Join(dir, manifest), 128<<10)
	if err != nil {
		return err
	}
	var m struct {
		Schema             string            `json:"schema"`
		Status             string            `json:"status"`
		Pi                 string            `json:"pi"`
		Node               string            `json:"node"`
		Bun                string            `json:"bun"`
		NodeArchiveSHA256  string            `json:"nodeArchiveSHA256"`
		LockSHA256         string            `json:"lockSHA256"`
		BundleSHA256       string            `json:"bundleSHA256"`
		WorkerSHA256       string            `json:"workerSHA256"`
		PhotonSHA256       string            `json:"photonSHA256"`
		EntitlementsSHA256 string            `json:"entitlementsSHA256"`
		Assets             map[string]string `json:"assets"`
		Images             map[string]string `json:"images"`
	}
	if !uniqueHookJSON(raw) || strictDecode(raw, &m) != nil || m.Schema != "corvint-pi-protected-build/0" || m.Status != "EXPERIMENTAL_UNQUALIFIED" || m.Pi != "0.85.1" || m.Node != "22.23.2" || m.Bun != "1.3.11" || len(m.Images) != 2 || len(m.Assets) == 0 {
		return errors.New("invalid experimental Pi build manifest")
	}
	for _, sum := range []string{m.NodeArchiveSHA256, m.LockSHA256, m.BundleSHA256, m.WorkerSHA256, m.PhotonSHA256, m.EntitlementsSHA256} {
		if _, err = decodeHex(sum, 32); err != nil {
			return err
		}
	}
	for _, sum := range m.Assets {
		if _, err = decodeHex(sum, 32); err != nil {
			return err
		}
	}
	for _, name := range []string{"pi-protected", "corvint"} {
		image, err := readRegular(filepath.Join(dir, name), releaseFileLimit(name))
		if err != nil || digest(image) != m.Images[name] {
			return errors.New("Pi build image drift")
		}
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode()&0111 == 0 {
			return errors.New("Pi image is not executable")
		}
	}
	return nil
}

func requirePiStore() error {
	raw, err := readRootFile(filepath.Join(authoritystore.RootPath, "accepted-root.json"), 128<<10)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return piStoreAdmission(raw)
}
func piStoreAdmission(raw []byte) error {
	var root authoritystore.RootDocument
	if localauthority.Decode(raw, &root) != nil || root.Profile != authoritystore.PiRootProfile {
		return errors.New("existing non-Pi or invalid admission; store retained")
	}
	// A profile-looking fragment is not an accepted root. Leave incomplete or
	// unknown state for its independent operator rather than repairing it here.
	if root.Admission != "OPERATOR_ACCEPTED" || root.RootID == "" || root.RepositoryRoot == "" {
		return errors.New("incomplete Pi admission; store retained")
	}
	encoded, err := localauthority.Canonical(root)
	if err != nil || !bytes.Equal(raw, encoded) {
		return errors.New("noncanonical Pi admission; store retained")
	}
	return nil
}
