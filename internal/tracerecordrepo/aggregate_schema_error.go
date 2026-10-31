// SPDX-License-Identifier: Apache-2.0

package tracerecordrepo

import (
	"errors"
	"strconv"
)

// AggregateSchemaError is an aggregate-schema-invalid refusal that names the
// offending field (ASD-V0-089, V1-3248). Its Error text stays the public code,
// so every caller and wire classifier sees the same reason as before; Fix
// names the field and the recovery for the person reading the refusal.
type AggregateSchemaError struct {
	Field  string
	Detail string
}

func (e *AggregateSchemaError) Error() string { return "aggregate-schema-invalid" }

func schemaInvalid(field, detail string) error {
	return &AggregateSchemaError{Field: field, Detail: detail}
}

// aggregateSchemaFixes is the recovery for each field a refusal can name.
var aggregateSchemaFixes = map[string]string{
	"task":         "the recorded task must already be trimmed and non-empty; remove its leading and trailing whitespace and rerun",
	"verification": "each verify command must be non-empty and already trimmed; remove leading or trailing whitespace from that DOGFOOD_VERIFY_FILE line, keep at least one command, and rerun corvint dogfood change BASE",
	"candidates":   "the recorder's own candidate path list is malformed; rerun with the lane-built tree binary $(git rev-parse --absolute-git-dir)/corvint/corvint and, if it repeats, keep local-outcome.stderr and report it",
	"admitted":     "the recorder's own admitted path list is malformed; rerun with the lane-built tree binary $(git rev-parse --absolute-git-dir)/corvint/corvint and, if it repeats, keep local-outcome.stderr and report it",
	"receipt":      "the aggregate local-outcome receipt is not the recorder's canonical bytes (edited, truncated or written by another version); do not edit it: move it aside with mv $(git rev-parse --absolute-git-dir)/corvint/local-outcome.json $(git rev-parse --absolute-git-dir)/corvint/local-outcome.json.stale, since dogfood change refuses aggregate-enrollment-required while it is present, then rerun corvint dogfood change BASE with the same --corvint-bin to record it again; an enrolled aggregate change finishes with its keyed corvint dogfood finish --session-key KEY instead",
	"report":       "the aggregate dogfood report .corvint/dogfood-report.json is not canonical; do not edit it: move it aside with mv .corvint/dogfood-report.json .corvint/dogfood-report.json.stale, since dogfood change refuses aggregate-enrollment-required while it is present, then rerun corvint dogfood change BASE to rewrite it and rerun the same check",
}

// AggregateSchemaFix is the one-line field and recovery of an
// aggregate-schema-invalid error, or "" for any other error.
func AggregateSchemaFix(err error) string {
	var schema *AggregateSchemaError
	if !errors.As(err, &schema) {
		return ""
	}
	field := schema.Field
	fix := aggregateSchemaFixes[field]
	if fix == "" {
		fix = aggregateSchemaFixes["receipt"]
	}
	if schema.Detail != "" {
		field += " " + strconv.Quote(schema.Detail)
	}
	return "field: " + field + "; fix: " + fix
}

// AggregateSchemaReportError names the aggregate dogfood report member a
// report refusal found malformed.
func AggregateSchemaReportError(member string) error {
	return schemaInvalid("report", member)
}
