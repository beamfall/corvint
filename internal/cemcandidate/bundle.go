package cemcandidate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
)

// RunBundleInvocation handles the supported out-of-tree bundle command
// (CSB-V0-001):
//
//	corvint cem bundle --request FILE --out DIR
//
// FILE is a cem-stable-plain-assembly/1 request; DIR is the new bundle
// directory. The command writes only DIR, never the request's repository, and
// prints the plain assembly result with its verify-stable next action. Each
// absolute path is accepted after its ancestor directories are resolved to
// their real paths (CSB-V0-004). It reports false when the arguments are not a
// cem bundle invocation.
func RunBundleInvocation(ctx context.Context, args []string, out, errout io.Writer) (int, bool) {
	at := -1
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "cem" && args[i+1] == "bundle" {
			at = i
			break
		}
	}
	if at < 0 {
		return 0, false
	}
	// As for verify-stable, only global-option-shaped tokens may precede the
	// command; a bare word first means another command owns the arguments.
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
	if at > 0 {
		return refuseBundle(errout, 2, cemcode.New("invalid-arguments", "cem bundle takes no global options; the request names the repository"))
	}
	values := map[string]string{}
	for i := at + 2; i < len(args); i += 2 {
		key := args[i]
		if key != "--request" && key != "--out" {
			if message, paired := cemcli.PairedChangeRefusal([]string{key}); paired {
				return refuseBundle(errout, 2, cemcode.New("invalid-arguments", "%s", message))
			}
			return refuseBundle(errout, 2, cemcode.New("invalid-arguments", "unknown cem bundle argument %q", key))
		}
		if _, seen := values[key]; seen {
			return refuseBundle(errout, 2, cemcode.New("invalid-arguments", "%s given more than once", key))
		}
		if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
			return refuseBundle(errout, 2, cemcode.New("invalid-arguments", "%s needs a value", key))
		}
		values[key] = args[i+1]
	}
	for _, key := range []string{"--request", "--out"} {
		p, ok := values[key]
		if !ok {
			return refuseBundle(errout, 2, cemcode.New("invalid-arguments", "%s is required", key))
		}
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return refuseBundle(errout, 2, cemcode.New("invalid-arguments", "%s must be an absolute clean path: %q", key, p))
		}
	}
	result, next, e := bundle(ctx, values["--request"], values["--out"])
	if e != nil {
		return refuseBundle(errout, 1, e)
	}
	if e := EncodeOK(out, result, next); e != nil {
		return refuseBundle(errout, 1, e)
	}
	return 0, true
}

// bundle reads the request, resolves its paths and assembles the bundle.
func bundle(ctx context.Context, request, out string) (PlainAssemblyResult, [][]string, error) {
	var result PlainAssemblyResult
	request, e := resolveAncestors(request)
	if e != nil {
		return result, nil, e
	}
	if out, e = resolveAncestors(out); e != nil {
		return result, nil, e
	}
	raw, e := Read(request)
	if e != nil {
		return result, nil, e
	}
	var head struct {
		Profile string `json:"profile"`
	}
	if e := json.Unmarshal(raw, &head); e != nil {
		return result, nil, cemcode.New("bundle-request-invalid", "request %q is not a JSON object: %v", request, e)
	}
	if head.Profile != PlainAssemblyProfile {
		return result, nil, cemcode.New("bundle-profile-unsupported", "request profile %q is not %s", head.Profile, PlainAssemblyProfile)
	}
	var r PlainRequest
	if e := Decode(raw, &r); e != nil {
		return result, nil, cemcode.New("bundle-request-invalid", "request %q: %v", request, e)
	}
	if e := r.resolvePaths(); e != nil {
		return result, nil, e
	}
	result, e = AssemblePlain(ctx, r, out)
	if e != nil {
		return result, nil, verifierRefusal(result.Verification, e)
	}
	return result, PlainNextActions(r, result.StablePath, out), nil
}

// resolvePaths replaces the repository by its real path and each absolute,
// clean pin path by the same path under its real parent directory. Other
// paths stay as written, so AssemblePlain refuses them as before.
func (r *PlainRequest) resolvePaths() error {
	if filepath.IsAbs(r.Repository) && filepath.Clean(r.Repository) == r.Repository {
		resolved, e := filepath.EvalSymlinks(r.Repository)
		if e != nil {
			return unresolved(r.Repository, e)
		}
		r.Repository = resolved
	}
	pins := []*FileRef{&r.SourceMap}
	if r.multi() {
		for i := range r.Runners {
			pins = append(pins, &r.Runners[i].RunnerPlan, &r.Runners[i].RunnerReceipt)
		}
	} else {
		pins = append(pins, &r.RunnerPlan, &r.RunnerReceipt)
	}
	for _, pin := range pins {
		if !filepath.IsAbs(pin.Path) || filepath.Clean(pin.Path) != pin.Path {
			continue
		}
		resolved, e := resolveAncestors(pin.Path)
		if e != nil {
			return e
		}
		pin.Path = resolved
	}
	return nil
}

// resolveAncestors resolves the ancestor directories of the absolute clean
// path p and keeps its last element literal, so a symlinked final element is
// still refused by the no-follow reader or the bundle writer (CSB-V0-004).
func resolveAncestors(p string) (string, error) {
	parent, e := filepath.EvalSymlinks(filepath.Dir(p))
	if e != nil {
		return "", unresolved(p, e)
	}
	return filepath.Join(parent, filepath.Base(p)), nil
}

// verifierRefusal types the untyped error AssemblePlain returns when its own
// stable verification does not accept the bundle, from the result it keeps:
// the verifier's refusal code, or bundle-verification-rejected with the issue
// codes of a REJECT (CSB-V0-005). Any other error is returned unchanged.
func verifierRefusal(v verify.StableResult, e error) error {
	if cemcode.CodeOf(e) != "" {
		return e
	}
	if v.Code != nil && *v.Code != "" {
		return cemcode.New(*v.Code, "%v", e)
	}
	if v.Outcome == "REJECT" {
		return cemcode.New("bundle-verification-rejected", "%vREJECT, issue codes %s", e, strings.Join(v.IssueCodes, ","))
	}
	return e
}

func unresolved(p string, e error) error {
	return cemcode.New("bundle-path-unresolved", "path %q cannot be resolved to a real path: %v", p, e)
}

// refuseBundle writes one closed refusal line, as the other cem commands do,
// and returns status. An untyped assembly error carries bundle-refused.
func refuseBundle(errout io.Writer, status int, e error) (int, bool) {
	code := cemcode.CodeOf(e)
	if code == "" {
		code = "bundle-refused"
	}
	_, _ = fmt.Fprintf(errout, "{\"code\": %s, \"error\": %s, \"ok\": false}\n", cw.CanonicalString(code), cw.CanonicalString(cemcode.MessageOf(e)))
	return status, true
}
