package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Beamfall/corvint/internal/sharedpath"
)

// runChangesBehind runs `changes behind` (SPC-V0-009..012): exit 0 with one
// report (or, with --format doctor-plugin, the TQD-V0-010 plugin array) and
// 2 on refusal. A too-large range also names limit and commits.
func runChangesBehind(ctx context.Context, root string, args []string, out, diagnostic io.Writer, refuse func(string) int) int {
	f := flag.NewFlagSet("behind", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	base, head, config, ticket, stale, format := "", "", "", "", "", ""
	setOnce := func(dst *string) func(string) error {
		return func(v string) error {
			if *dst != "" || v == "" {
				return errChangesArgument{}
			}
			*dst = v
			return nil
		}
	}
	f.Func("base", "base revision", setOnce(&base))
	f.Func("head", "head revision", setOnce(&head))
	f.Func("config", "shared-path configuration", setOnce(&config))
	f.Func("ticket", "ticket the head belongs to", setOnce(&ticket))
	f.Func("stale-hours", "age in hours that makes a finding", setOnce(&stale))
	f.Func("format", "json or doctor-plugin", setOnce(&format))
	if f.Parse(args) != nil || f.NArg() != 0 || base == "" {
		return refuse(sharedpath.CodeInvalidArguments)
	}
	hours := 0
	if stale != "" {
		n, err := strconv.Atoi(stale)
		if err != nil || !sharedpath.ValidStaleHours(n) {
			return refuse(sharedpath.CodeInvalidArguments)
		}
		hours = n
	}
	plugin := format == "doctor-plugin"
	if (format != "" && format != "json" && !plugin) || (ticket == "") != (hours == 0) || (ticket != "" && !sharedpath.ValidTicket(ticket)) || (plugin && ticket == "") {
		return refuse(sharedpath.CodeInvalidArguments)
	}
	if root == "" {
		var err error
		if root, err = os.Getwd(); err != nil {
			return refuse(sharedpath.CodeGitUnavailable)
		}
	}
	if config == "" {
		config = sharedpath.DefaultConfig
	}
	if !filepath.IsAbs(config) {
		config = filepath.Join(root, config)
	}
	c, err := sharedpath.LoadConfig(config)
	if err != nil {
		return refuse(err.Error())
	}
	report, err := sharedpath.Behind(ctx, sharedpath.BehindRequest{Root: root, Base: base, Head: head, Ticket: ticket, StaleHours: hours, Config: c})
	var tooLarge *sharedpath.RangeTooLargeError
	switch {
	case errors.As(err, &tooLarge):
		_ = json.NewEncoder(diagnostic).Encode(map[string]any{"code": err.Error(), "ok": false, "limit": tooLarge.Limit, "commits": tooLarge.Commits})
		return 2
	case err != nil:
		return refuse(err.Error())
	}
	var result any = report
	if plugin {
		type item struct {
			Kind   string `json:"kind"`
			Who    string `json:"who"`
			Detail string `json:"detail"`
			Remedy string `json:"remedy"`
		}
		items := []item{}
		for _, finding := range report.Findings {
			items = append(items, item{Kind: finding.Kind, Who: finding.Who, Detail: finding.Detail, Remedy: finding.Remedy})
		}
		result = items
	}
	if json.NewEncoder(out).Encode(result) != nil {
		return refuse("changes-output-unavailable")
	}
	return 0
}
