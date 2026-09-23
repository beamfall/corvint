package main

import (
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
