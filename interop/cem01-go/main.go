package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type response struct {
	Accept bool        `json:"accept"`
	Spec   string      `json:"spec"`
	Drift  []driftItem `json:"drift"`
	Code   string      `json:"code,omitempty"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "verify-candidate" {
		runCandidateCLI(os.Args[2:])
	}
	if len(os.Args) > 1 && os.Args[1] == "ci" {
		writeCIReport(runCI(os.Args[2:]))
	}
	r := response{Spec: specVersion, Drift: []driftItem{}}
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		r.Code = "invocation"
		writeResponse(r, 2)
	}
	drift, err := verify(args)
	if err != nil {
		r.Code = err.code
		if err.operational {
			writeResponse(r, 2)
		}
		writeResponse(r, 1)
	}
	r.Accept = true
	r.Drift = drift
	for _, item := range drift {
		if item.Status != "stable" && item.Status != "relocated" {
			r.Accept = false
			break
		}
	}
	if !r.Accept {
		writeResponse(r, 1)
	}
	writeResponse(r, 0)
}

func writeResponse(r response, status int) {
	b, err := json.Marshal(r)
	if err != nil {
		b = []byte(`{"accept":false,"spec":"cem/0.1","drift":[],"code":"internal"}`)
		status = 2
	}
	_, _ = fmt.Fprintln(os.Stdout, string(b))
	os.Exit(status)
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
