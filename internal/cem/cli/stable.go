package cli

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// RunStableInvocation handles only the explicit Core stable command, before
// generic Core root resolution, worktree opening or observation side effects.
func RunStableInvocation(ctx context.Context, args []string, out io.Writer) (int, bool) {
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
		if key != "--repository" && key != "--map" && key != "--expected-base" && key != "--target" && key != "--artifacts" {
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
		return exit, true
	}
	if bad || !filepath.IsAbs(options.Repository) || !filepath.IsAbs(values["--map"]) || globalRoot != "" && (!filepath.IsAbs(globalRoot) || filepath.Clean(globalRoot) != globalRoot || filepath.Clean(options.Repository) != globalRoot) {
		exit = r.Refuse("arguments", "invalid-arguments", false)
		return emit()
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
