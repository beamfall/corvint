package appflows

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// Observe runs only the explicitly selected optional provider, never a core read path.
func Observe(ctx context.Context, root, manifestPath, assets string, live bool) (Evidence, error) {
	e := Evidence{}
	in, err := Capture(ctx, root, manifestPath)
	if err != nil {
		return e, err
	}
	in.Observe = live
	var nonce [32]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return e, err
	}
	in.RunID = hex.EncodeToString(nonce[:])
	raw, err := json.Marshal(in)
	if err != nil {
		return e, err
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return e, errors.New("optional Node runtime unavailable")
	}
	assets, err = filepath.Abs(assets)
	if err != nil {
		return e, err
	}
	providerDigest, err := providerIdentity(assets)
	if err != nil {
		return e, err
	}
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.Getenv("TMPDIR"), "LANG=C"}
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	o := procgroup.Run(runCtx, procgroup.Spec{Argv: []string{node, "--max-old-space-size=256", filepath.Join(assets, "observe.mjs")}, Dir: root,
		Env: env, Stdin: raw, InputLimit: MaxBytes + 65536, OutputLimit: MaxBytes, StderrLimit: 8192,
		Timeout: 5 * time.Minute, BeforeStop: func(hook context.Context, pid int) error {
			if runCtx.Err() == nil {
				return nil
			}
			p, findErr := os.FindProcess(pid)
			if findErr == nil {
				_ = p.Signal(os.Interrupt)
			}
			// Give the trusted observer time to close Chromium's separately owned group
			// before procgroup terminates its Node/server group. This is not an escape proof.
			select {
			case <-time.After(500 * time.Millisecond):
			case <-hook.Done():
			}
			return nil
		}})
	if o.Cancelled || o.TimedOut {
		return e, errors.New("flow observation interrupted or timed out")
	}
	if !o.OwnedProcessGroupCleanup || o.DescendantCleanupQualification == "PARTIAL" {
		return e, errors.New("flow observer cleanup unqualified")
	}
	if o.Err != nil || o.ExitStatus != 0 {
		return e, errors.New("optional flow observer failed; verify installed dependencies and manifest")
	}
	if err = Decode(o.Stdout, &e); err != nil {
		return e, err
	}
	afterProvider, err := providerIdentity(assets)
	if err != nil || afterProvider != providerDigest {
		return e, errors.New("flow provider changed during observation")
	}
	e.ProviderDigest = providerDigest
	if !reflect.DeepEqual(e.Binding, in.Binding) || e.RunID != in.RunID {
		return e, errors.New("observer binding mismatch")
	}
	e.Cleanup = true
	if err = ValidateEvidence(in, e); err != nil {
		return e, err
	}
	after, err := Capture(ctx, root, manifestPath)
	if err != nil {
		return e, errors.New("flow source changed during observation")
	}
	if !reflect.DeepEqual(in.Binding, after.Binding) {
		return e, errors.New("flow binding changed during observation")
	}
	return e, nil
}

func providerIdentity(assets string) (string, error) {
	hashes := map[string]string{}
	paths := []string{"observe.mjs", "scan.mjs", "lifecycle.mjs", "package-lock.json"}
	for _, p := range paths {
		b, err := readSource(assets, p)
		if err != nil {
			return "", errors.New("flow provider assets or lockfile unavailable")
		}
		hashes[p] = Digest(b)
	}
	return buildDigest(paths, hashes), nil
}
