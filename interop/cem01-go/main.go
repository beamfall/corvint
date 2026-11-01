package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// retiredFix is the remediation printed with every refusal of a retired cem/0.x map.
const retiredFix = "cem/0.x is retired (decision 0525); regenerate the map as cem/1.0 and run verify-stable-canonical"

// invocationFix is the remediation printed for every other invocation outside the cem/1.0 commands.
const invocationFix = "use verify-stable-canonical, verify-stable or verify-candidate"

func main() {
	if len(os.Args) == 3 && os.Args[1] == stableKeeperProtocol && stableKeeperControlPresent() {
		stableKeeperMain()
	}
	if len(os.Args) > 1 && os.Args[1] == "verify-stable-canonical" {
		runStableCanonicalCLI(os.Args[2:])
	}
	if len(os.Args) > 1 && os.Args[1] == "verify-stable" {
		runStableCLI(os.Args[2:])
	}
	if len(os.Args) > 1 && os.Args[1] == "verify-candidate" {
		runCandidateCLI(os.Args[2:])
	}
	if len(os.Args) > 1 && os.Args[1] == "read-verdict-ledger" {
		runVerdictLedgerCLI(os.Args[2:])
	}
	code, status := retiredRoute(os.Args[1:])
	_, _ = fmt.Fprintf(os.Stderr, "cem01-go: %s; fix: %s\n", code, refusalFix(code))
	_, _ = fmt.Fprintln(os.Stdout, `{"accept":false,"spec":"`+stableSpec+`","code":"`+code+`"}`)
	os.Exit(status)
}

// refusalFix names the retired-map remediation only for a cem/0.x map; any other invocation is
// pointed at the cem/1.0 commands.
func refusalFix(code string) string {
	if code == "unsupported-spec" {
		return retiredFix
	}
	return invocationFix
}

// retiredRoute refuses every invocation outside the cem/1.0 commands (CEM-V1-222). The cem/0.1
// default verify route, verify-03 and ci are retired: an invocation naming a map whose spec is a
// cem/0.x string is refused with the closed code unsupported-spec (exit 1); anything else is an
// invocation error (exit 2).
func retiredRoute(argv []string) (string, int) {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] != "--map" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), verificationBudget)
		defer cancel()
		raw, e := readBounded(ctx, argv[i+1], maxJSONBytes)
		if e == nil && retiredSpec(raw) {
			return "unsupported-spec", 1
		}
		break
	}
	return "invocation", 2
}

// retiredSpec reports a strictly parsed JSON object whose spec member is a cem/0.x string.
func retiredSpec(raw []byte) bool {
	if strictJSON03(raw) != nil {
		return false
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return false
	}
	s, ok := text03(root["spec"])
	return ok && strings.HasPrefix(s, "cem/0.")
}

func runCandidateCLI(argv []string) {
	flags := map[string]string{}
	valid := map[string]bool{"--repository": true, "--map": true, "--expected-base": true, "--target": true, "--artifacts": true}
	bad := len(argv) != 10
	if !bad {
		for i := 0; i < len(argv); i += 2 {
			if !valid[argv[i]] || flags[argv[i]] != "" || argv[i+1] == "" {
				bad = true
				break
			}
			flags[argv[i]] = argv[i+1]
		}
	}
	r := newCandidateResult(flags["--expected-base"], flags["--target"])
	status := 2
	if bad || len(flags) != 5 {
		r.Code = "invocation"
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), verificationBudget)
		defer cancel()
		raw, e := readBounded(ctx, flags["--map"], maxJSONBytes)
		if e != nil {
			r.Code = "map-read"
		} else if e := verifyCandidate(ctx, flags["--repository"], raw, flags["--expected-base"], flags["--target"], flags["--artifacts"]); e != nil {
			r.Code = e.code
			if !e.operational {
				status = 1
			}
		} else {
			r.Integrity = "VERIFIED"
			r.Code = "verified"
			status = 0
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(r)
	os.Exit(status)
}
