package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	cc "github.com/Beamfall/corvint/internal/cemcandidate"
)

func stableCommand(ctx context.Context, args []string, out, errout io.Writer) int {
	f := flag.NewFlagSet("assemble-stable", flag.ContinueOnError)
	f.SetOutput(errout)
	experimental := f.Bool("experimental", false, "admit experimental stable assembly")
	request := f.String("request", "", "closed stable assembly request")
	output := f.String("out-dir", "", "new absolute bundle directory")
	seen := map[string]bool{}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			n := strings.SplitN(strings.TrimLeft(a, "-"), "=", 2)[0]
			if seen[n] {
				fmt.Fprintln(errout, "repeated flag", n)
				return 2
			}
			seen[n] = true
		}
	}
	if f.Parse(args) != nil || f.NArg() != 0 || !*experimental || *request == "" || *output == "" {
		return 2
	}
	fail := func(e error) int { fmt.Fprintln(errout, "stable assembly refused:", e); return 1 }
	raw, e := cc.Read(*request)
	if e != nil {
		return fail(e)
	}
	// The profile only selects the closed decoder; each decoder refuses
	// unknown and missing members on its own.
	var head struct {
		Profile string `json:"profile"`
	}
	if e := json.Unmarshal(raw, &head); e != nil {
		return fail(e)
	}
	var result any
	var next [][]string
	if head.Profile == cc.PlainAssemblyProfile {
		var r cc.PlainRequest
		if e := cc.Decode(raw, &r); e != nil {
			return fail(e)
		}
		var plain cc.PlainAssemblyResult
		plain, e = cc.AssemblePlain(ctx, r, *output)
		result = plain
		if e == nil {
			next = plainNextActions(r, plain.StablePath, *output)
		}
	} else {
		var r cc.StableRequest
		if e := cc.Decode(raw, &r); e != nil {
			return fail(e)
		}
		result, e = cc.AssembleStable(ctx, r, *output)
	}
	if e != nil {
		return fail(e)
	}
	if e := encodeOK(out, result, next); e != nil {
		return fail(e)
	}
	return 0
}

// plainNextActions names the supported verify route for the bundle a plain
// assembly wrote: cem verify-stable over its stable.json, with the request's
// own repository and revisions and the bundle directory as the artifact root,
// exactly the options the assembler's own verification used (CEM-V1-350).
func plainNextActions(r cc.PlainRequest, stablePath, bundle string) [][]string {
	return [][]string{{"corvint", "cem", "verify-stable", "--repository", r.Repository, "--map", stablePath, "--expected-base", r.ExpectedBase, "--target", r.Target, "--artifacts", bundle}}
}

// encodeOK writes result as json.Encoder does, with one leading top-level
// "ok": true member; every other byte is the result's own encoding
// (CEM-V1-341), except that a non-nil next is appended as one trailing
// "nextActions" member (CEM-V1-350).
func encodeOK(out io.Writer, result any, next [][]string) error {
	encoded, e := json.Marshal(result)
	if e != nil {
		return e
	}
	if len(encoded) < 2 || encoded[0] != '{' {
		return fmt.Errorf("assembly result is not a JSON object")
	}
	envelope := []byte(`{"ok":true`)
	if string(encoded) != "{}" {
		envelope = append(envelope, ',')
	}
	envelope = append(envelope, encoded[1:len(encoded)-1]...)
	if next != nil {
		actions, e := json.Marshal(next)
		if e != nil {
			return e
		}
		envelope = append(append(envelope, `,"nextActions":`...), actions...)
	}
	envelope = append(envelope, '}', '\n')
	_, e = out.Write(envelope)
	return e
}
