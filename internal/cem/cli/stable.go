package cli

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// RunStableInvocation handles only the explicit Core stable command, before
// generic Core root resolution, worktree opening or observation side effects.
// The closed result goes to out; an invalid-arguments refusal caused by a
// relative path option also names the absolute-path fix on errout
// (CEM-V1-117), never inside the result.
func RunStableInvocation(ctx context.Context, args []string, out, errout io.Writer) (int, bool) {
	at := -1
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "cem" && args[i+1] == "verify-stable" {
			at = i
			break
		}
	}
	if at < 0 {
		return 0, false
	}
	// Only global-option-shaped tokens may precede the command. A bare word first
	// means another command owns the arguments and merely carries these two values.
	for lead := 0; lead < at; {
		switch {
		case args[lead] == "--root" && lead+1 < at:
			lead += 2
		case strings.HasPrefix(args[lead], "--"):
			lead++
		default:
			return 0, false
		}
	}
	options := verify.StableOptions{}
	values := map[string]string{}
	bad := false
	globalRoot := ""
	for i := 0; i < at; i++ {
		if args[i] != "--root" || globalRoot != "" || i+1 >= at {
			bad = true
			break
		}
		i++
		globalRoot = args[i]
		if globalRoot == "" {
			bad = true
		}
	}
	for i := at + 2; i < len(args); i++ {
		key := args[i]
		if key != "--repository" && key != "--map" && key != "--expected-base" && key != "--target" && key != "--artifacts" && key != "--trusted-keys" {
			bad = true
			break
		}
		if _, ok := values[key]; ok || i+1 >= len(args) {
			bad = true
			break
		}
		i++
		values[key] = args[i]
		if strings.HasPrefix(args[i], "--") {
			bad = true
			break
		}
	}
	options.Repository = values["--repository"]
	options.ExpectedBase = values["--expected-base"]
	options.Target = values["--target"]
	options.ArtifactRoot = values["--artifacts"]
	r := verify.NewStableResult(options)
	exit := 2
	emit := func() (int, bool) {
		if e := json.NewEncoder(out).Encode(r); e != nil {
			return 2, true
		}
		if r.Code != nil && *r.Code == "invalid-arguments" {
			// Scan the supplied argv, not the parsed values, so a relative path in
			// a repeated option is still named, as the portable consumer does.
			relative := func(name string, from, to int) [2]string {
				for i := from; i+1 < to; i += 2 {
					if args[i] == name && args[i+1] != "" && !filepath.IsAbs(args[i+1]) {
						return [2]string{name, args[i+1]}
					}
				}
				return [2]string{name, ""}
			}
			paths := [][2]string{relative("--root", 0, at), relative("--repository", at+2, len(args)), relative("--map", at+2, len(args)), relative("--artifacts", at+2, len(args)), relative("--trusted-keys", at+2, len(args))}
			if fix := verify.RelativePathFix(paths...); fix != "" {
				_, _ = io.WriteString(errout, "corvint cem verify-stable: "+fix+"\n")
			}
		}
		return exit, true
	}
	if bad || !filepath.IsAbs(options.Repository) || !filepath.IsAbs(values["--map"]) || globalRoot != "" && (!filepath.IsAbs(globalRoot) || filepath.Clean(globalRoot) != globalRoot || filepath.Clean(options.Repository) != globalRoot) {
		exit = r.Refuse("arguments", "invalid-arguments", true)
		return emit()
	}
	if path, ok := values["--trusted-keys"]; ok {
		keys, e := readTrustedKeys(path)
		if e != nil {
			exit = r.Refuse("arguments", "invalid-arguments", true)
			return emit()
		}
		options.TrustedKeys = keys
	}
	// Read once into the private bounded byte copy. A regular file is required so
	// FIFO/device inputs cannot hang before the verifier's process deadline.
	f, e := openStableMap(values["--map"])
	if e != nil {
		exit = r.Refuse("input", "map-unavailable", true)
		return emit()
	}
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() > wire.MaxMapBytes {
		f.Close()
		exit = r.Refuse("input", "map-unavailable", true)
		return emit()
	}
	raw, e := io.ReadAll(io.LimitReader(f, wire.MaxMapBytes+1))
	closeErr := f.Close()
	if e != nil || closeErr != nil || len(raw) > wire.MaxMapBytes {
		exit = r.Refuse("input", "map-unavailable", true)
		return emit()
	}
	r, exit = verify.Stable(ctx, raw, options)
	return emit()
}

// readTrustedKeys reads the consumer --trusted-keys file (CEM-V1-084) once,
// as a bounded regular file at an absolute path, and parses its closed form.
func readTrustedKeys(path string) (map[string]ed25519.PublicKey, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("trusted keys path must be absolute")
	}
	f, e := openStableMap(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() > wire.MaxTrustedKeysBytes {
		return nil, errors.New("trusted keys file unavailable")
	}
	raw, e := io.ReadAll(io.LimitReader(f, wire.MaxTrustedKeysBytes+1))
	if e != nil {
		return nil, e
	}
	return wire.ParseTrustedKeys(raw)
}
