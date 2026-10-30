// Package cli is the falsify companion's command entry, shared by the `corvint-behavior-falsify`
// shim and the `corvint-test falsify` group (CTF-V0-004). It is an explicitly approved experimental
// entry for browser-criterion falsification controls.
package cli

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
)

// Run runs the falsify companion under the invocation name prog: `corvint-behavior-falsify` for the
// old companion shim, `corvint-test falsify` for the group (CTF-V0-004). Only the usage diagnostic
// names prog. The receipt tool identity keeps the name `corvint-behavior-falsify`, which receipt
// verification requires; its revision and executable digest name the binary that ran (CTF-V0-002).
func Run(ctx context.Context, prog string, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s <plan|execute|execute-receipt|verify-receipt> --experimental [--approve-plan sha256:...] < input.json", prog)
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	experimental := flags.Bool("experimental", false, "admit the experimental falsification profile")
	approval := flags.String("approve-plan", "", "authorize execution of exactly this plan digest")
	expectedPlan := flags.String("expected-plan", "", "pin the reviewed plan digest for offline receipt verification")
	expectedTool := flags.String("expected-tool", "", "pin the executing tool SHA-256 for offline receipt verification")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if !*experimental || flags.NArg() != 0 {
		return fmt.Errorf("explicit --experimental required; positional arguments forbidden")
	}
	if args[0] != "verify-receipt" && (*expectedPlan != "" || *expectedTool != "") {
		return fmt.Errorf("verification pins require verify-receipt")
	}
	data, err := io.ReadAll(io.LimitReader(input, (32<<20)+1))
	if err != nil {
		return err
	}
	var result any
	switch args[0] {
	case "plan":
		if *approval != "" {
			return fmt.Errorf("planning does not accept execution approval")
		}
		var request behaviorfalsify.Request
		if err := behaviorfalsify.Decode(data, &request); err != nil {
			return err
		}
		result, err = behaviorfalsify.BuildPlan(request)
	case "execute", "execute-receipt":
		if *approval == "" {
			return fmt.Errorf("explicit --approve-plan required")
		}
		var plan behaviorfalsify.Plan
		if err := behaviorfalsify.Decode(data, &plan); err != nil {
			if args[0] == "execute-receipt" {
				return fmt.Errorf("receipt-plan-invalid")
			}
			return err
		}
		if args[0] == "execute-receipt" {
			tool, identityErr := executingToolIdentity()
			if identityErr != nil {
				return identityErr
			}
			result, err = behaviorfalsify.ExecuteReceipt(ctx, plan, *approval, tool)
			if err != nil {
				return fmt.Errorf("receipt-execution-refused")
			}
		} else {
			result, err = behaviorfalsify.Execute(ctx, plan, *approval)
		}
	case "verify-receipt":
		if *approval != "" || *expectedPlan == "" || *expectedTool == "" {
			return fmt.Errorf("verify-receipt requires --expected-plan and --expected-tool; execution approval is forbidden")
		}
		var receipt behaviorfalsify.EvidenceReceipt
		if behaviorfalsify.Decode(data, &receipt) != nil || behaviorfalsify.VerifyReceipt(receipt, *expectedPlan, *expectedTool) != nil {
			return fmt.Errorf("receipt-invalid-control")
		}
		result = struct {
			Schema string                 `json:"schema"`
			Status string                 `json:"status"`
			Report behaviorfalsify.Report `json:"report"`
		}{"corvint-browser-behavior-evidence-verification/1", "verified-observation", receipt.Report}
	default:
		return fmt.Errorf("unknown operation %q", args[0])
	}
	if err != nil {
		return err
	}
	if receipt, ok := result.(behaviorfalsify.EvidenceReceipt); ok {
		data, err = behaviorfalsify.EncodeEvidence(receipt)
	} else {
		data, err = behaviorfalsify.Encode(result)
	}
	if err != nil {
		return err
	}
	written, err := output.Write(data)
	if err == nil && written != len(data) {
		return io.ErrShortWrite
	}
	return err
}

func executingToolIdentity() (behaviorfalsify.ToolIdentity, error) {
	tool := behaviorfalsify.ToolIdentity{Name: "corvint-behavior-falsify", Version: "1.0.0-rc.1"}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				tool.Revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				tool.SourceDirty = setting.Value == "true"
			}
		}
	}
	path, err := os.Executable()
	if err != nil || tool.Revision == "" {
		return tool, fmt.Errorf("receipt-tool-identity-unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return tool, fmt.Errorf("receipt-tool-identity-unavailable")
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return tool, fmt.Errorf("receipt-tool-identity-unavailable")
	}
	tool.Executable = fmt.Sprintf("sha256:%x", hash.Sum(nil))
	return tool, nil
}
