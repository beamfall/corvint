package main

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/scopelease"
)

var leaseActions = []string{"acquire", "list", "release", "renew", "status"}

type leaseOptions struct {
	holder string
	id     string
	ticket string
	note   string
	paths  []string
	ttl    time.Duration
	ttlSet bool
}

// isLeaseInvocation reports whether arguments select the lease verb, after the
// same leading --root forms the sibling verbs accept.
func isLeaseInvocation(arguments []string) bool {
	_, _, isLease, _ := parseLeaseInvocation(arguments)
	return isLease
}

func parseLeaseInvocation(arguments []string) (string, []string, bool, error) {
	index := 0
	root := ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false, nil
			}
			root = arguments[index+1]
			index += 2
		} else {
			root = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
	}
	if index >= len(arguments) || arguments[index] != "lease" {
		return "", nil, false, nil
	}
	if root == "" {
		resolved, err := normalizeRoot(".")
		if err != nil {
			return "", nil, true, argumentError("cannot resolve current directory")
		}
		root = resolved
	} else {
		resolved, err := resolveExplicitRoot(root)
		if err != nil {
			return "", nil, true, err
		}
		root = resolved
	}
	return root, arguments[index+1:], true, nil
}

// runLease executes one scope-lease action. acquire, release, and renew mutate
// the private local lease directory; status and list never write.
func runLease(arguments []string, stdout, stderr io.Writer) int {
	root, rest, _, err := parseLeaseInvocation(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if len(rest) == 0 {
		emitError(stderr, argumentError("argument action: expected one of "+strings.Join(leaseActions, ", ")))
		return 2
	}
	action := rest[0]
	if !slices.Contains(leaseActions, action) {
		emitError(stderr, invalidLeaseAction(action))
		return 2
	}
	options, err := parseLeaseFlags(action, rest[1:])
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	payload, status, err := leaseResult(root, action, options)
	if errors.Is(err, scopelease.ErrInvalidRequest) {
		err = argumentError(err.Error())
	}
	if err != nil {
		emitError(stderr, err)
		return status
	}
	return writeLeaseOutput(stdout, stderr, payload)
}

func leaseResult(root, action string, options leaseOptions) (map[string]any, int, error) {
	switch action {
	case "acquire":
		lease, conflicts, err := scopelease.Acquire(root, scopelease.Request{
			Holder: options.holder, Paths: options.paths, Ticket: options.ticket,
			Note: options.note, TTL: options.ttl,
		})
		if errors.Is(err, scopelease.ErrConflict) {
			return conflictPayload(conflicts), 1, nil
		}
		if err != nil {
			return nil, 2, err
		}
		return leasePayload("acquire", lease, "live"), 0, nil
	case "release":
		lease, err := scopelease.Release(root, options.id, options.holder)
		if err != nil {
			return nil, leaseStatus(err), err
		}
		return leasePayload("release", lease, "released"), 0, nil
	case "renew":
		lease, err := scopelease.Renew(root, options.id, options.holder, options.ttl)
		if err != nil {
			return nil, leaseStatus(err), err
		}
		return leasePayload("renew", lease, "live"), 0, nil
	case "status":
		report, err := scopelease.Status(root, options.id)
		if err != nil {
			return nil, leaseStatus(err), err
		}
		return leasePayload("status", report.Lease, report.State), 0, nil
	case "list":
		reports, err := scopelease.List(root)
		if err != nil {
			return nil, 2, err
		}
		return listPayload(reports), 0, nil
	default:
		return nil, 2, invalidLeaseAction(action)
	}
}

// invalidLeaseAction is argparse's refusal of the action positional, which it
// reports before any unrecognized option.
func invalidLeaseAction(action string) error {
	return argumentError("argument action: invalid choice: " + pythonRepr(action) +
		" (choose from " + strings.Join(quotedLeaseActions(), ", ") + ")")
}

func leaseStatus(err error) int {
	if errors.Is(err, scopelease.ErrNotFound) || errors.Is(err, scopelease.ErrHolderMismatch) {
		return 1
	}
	return 2
}

func parseLeaseFlags(action string, arguments []string) (leaseOptions, error) {
	var options leaseOptions
	for index := 0; index < len(arguments); {
		argument := arguments[index]
		name, value, inline := strings.Cut(argument, "=")
		if name != "--holder" && name != "--ttl" && name != "--path" && name != "--ticket" &&
			name != "--note" && name != "--id" {
			return options, argumentError("unrecognized arguments: " + argument)
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, argumentError("argument " + name + ": expected one argument")
			}
			value = arguments[index+1]
			index += 2
		} else {
			index++
		}
		switch name {
		case "--holder":
			options.holder = value
		case "--id":
			options.id = value
		case "--ticket":
			options.ticket = value
		case "--note":
			options.note = value
		case "--path":
			options.paths = append(options.paths, value)
		case "--ttl":
			duration, err := time.ParseDuration(value)
			if err != nil {
				return options, argumentError("argument --ttl: invalid duration: " + pythonRepr(value))
			}
			options.ttl, options.ttlSet = duration, true
		}
	}
	return options, checkLeaseFlags(action, options)
}

func checkLeaseFlags(action string, options leaseOptions) error {
	missing := make([]string, 0, 3)
	if options.holder == "" && (action == "acquire" || action == "release" || action == "renew") {
		missing = append(missing, "--holder")
	}
	if options.id == "" && (action == "release" || action == "renew" || action == "status") {
		missing = append(missing, "--id")
	}
	if !options.ttlSet && (action == "acquire" || action == "renew") {
		missing = append(missing, "--ttl")
	}
	if len(missing) != 0 {
		return argumentError("the following arguments are required: " + strings.Join(missing, ", "))
	}
	return nil
}

func quotedLeaseActions() []string {
	quoted := make([]string, 0, len(leaseActions))
	for _, action := range leaseActions {
		quoted = append(quoted, pythonRepr(action))
	}
	return quoted
}

func leasePayload(action string, lease scopelease.Lease, state string) map[string]any {
	mutates := action != "status"
	return map[string]any{
		"ok": true, "mutates": mutates, "tool": "lease", "action": action,
		"lease": leaseMap(lease, state),
	}
}

func listPayload(reports []scopelease.Report) map[string]any {
	leases := make([]any, 0, len(reports))
	for _, report := range reports {
		leases = append(leases, leaseMap(report.Lease, report.State))
	}
	return map[string]any{
		"ok": true, "mutates": false, "tool": "lease", "action": "list", "leases": leases,
	}
}

func conflictPayload(conflicts []scopelease.Conflict) map[string]any {
	rows := make([]any, 0, len(conflicts))
	for _, conflict := range conflicts {
		rows = append(rows, map[string]any{
			"lease_id": conflict.LeaseID, "holder": conflict.Holder,
			"reason": conflict.Reason, "scope": conflict.Scope,
		})
	}
	return map[string]any{
		"ok": false, "mutates": false, "tool": "lease", "action": "acquire",
		"error": scopelease.ErrConflict.Error(), "conflicts": rows,
	}
}

func leaseMap(lease scopelease.Lease, state string) map[string]any {
	paths := make([]any, 0, len(lease.Paths))
	for _, path := range lease.Paths {
		paths = append(paths, path)
	}
	return map[string]any{
		"schema_version": lease.SchemaVersion, "lease_id": lease.ID, "holder": lease.Holder,
		"paths": paths, "ticket": lease.Ticket, "note": lease.Note,
		"acquired_at": lease.AcquiredAt, "expires_at": lease.ExpiresAt,
		"revision": lease.Revision, "state": state,
	}
}

func writeLeaseOutput(stdout, stderr io.Writer, payload map[string]any) int {
	encoded, err := contextindex.CanonicalJSON(payload)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot encode lease output"})
		return 2
	}
	written, err := fmt.Fprintf(stdout, "%s\n", encoded)
	if err != nil || written != len(encoded)+1 {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write lease output"})
		return 2
	}
	if ok, _ := payload["ok"].(bool); !ok {
		return 1
	}
	return 0
}

const leaseHelp = `
Scope lease (experimental, SCL-V0 proposed), JSON:

  corvint [--root PATH] lease acquire --holder NAME --ttl DURATION [--path GLOB]...
    [--ticket ID] [--note TEXT]
  corvint [--root PATH] lease renew --holder NAME --id ID --ttl DURATION
  corvint [--root PATH] lease release --holder NAME --id ID
  corvint [--root PATH] lease status --id ID
  corvint [--root PATH] lease list

acquire, renew, and release write the private local lease directory under
.corvint/leases/; status and list never write, and every payload declares which
it was in its "mutates" field. An acquisition overlapping a live lease is
refused with exit status 1, which is a result, not a usage failure. DURATION is
a Go duration such as 90s or 2h; acquire needs at least one --path or --ticket.
`
