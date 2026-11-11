// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// The schema version of `.corvint/flows.json` (FSV-V1-001 to FSV-V1-006).
// The `profile` member is `corvint-flows/N`, N a whole number without leading
// zeros. This build reads only the versions in flowsSchemas; any other
// well-formed N is refused as FLOWS_SCHEMA_UNSUPPORTED, and a profile without
// the prefix or with a malformed N as FLOWS_DECLARATION_INVALID. A refusal is
// never read as version 0. Unknown members are refused, so adding, removing or
// renaming a member, or changing a member's meaning, needs a new N with its
// own entry here, member list and golden file under testdata/flows-schema/.
const FlowsProfilePrefix = "corvint-flows/"

// flowsSchema is one supported version: N and the closed member set the
// reader decodes for it, then validates into a FlowsDeclaration.
type flowsSchema struct {
	version  int
	file     reflect.Type
	validate func(file any) (*FlowsDeclaration, error)
}

// flowsSchemas is the closed list of supported versions, strictly ascending;
// the last is the one FlowsWriteVersion names and EncodeFlowsDeclaration
// writes (FSV-V1-002, FSV-V1-006).
var flowsSchemas = [...]flowsSchema{
	{version: 0, file: reflect.TypeFor[flowsDeclarationFile](), validate: func(file any) (*FlowsDeclaration, error) {
		return validateFlowsV0(file.(*flowsDeclarationFile))
	}},
}

// FlowsSupportedVersions lists, ascending, the versions this build reads.
func FlowsSupportedVersions() []int {
	out := make([]int, len(flowsSchemas))
	for i, s := range flowsSchemas {
		out[i] = s.version
	}
	return out
}

// FlowsWriteVersion is the highest supported version, the only one a writer
// of this build emits (FSV-V1-006).
func FlowsWriteVersion() int { return flowsSchemas[len(flowsSchemas)-1].version }

// FlowsProfile is the profile of version n.
func FlowsProfile(n int) string { return FlowsProfilePrefix + strconv.Itoa(n) }

// FlowsRefusal is the typed refusal of the one reader: Reason is
// FLOWS_SCHEMA_UNSUPPORTED (a well-formed version this build does not read;
// Found names it) or FLOWS_DECLARATION_INVALID (anything else wrong with the
// file).
type FlowsRefusal struct {
	Reason string
	// Found is the version a FLOWS_SCHEMA_UNSUPPORTED profile names, as
	// written; empty for FLOWS_DECLARATION_INVALID.
	Found string
	// Supported lists the versions this build reads.
	Supported []int
	// Message says what is wrong, without the path.
	Message string
}

func (r *FlowsRefusal) Error() string {
	return FlowsDeclarationPath + ": " + r.Message
}

// Unsupported reports whether the refusal is FLOWS_SCHEMA_UNSUPPORTED.
func (r *FlowsRefusal) Unsupported() bool {
	return r.Reason == affected.UnknownFlowsSchemaUnsupported
}

// Detail is the one-line plan detail: the path and, for an unsupported
// version, the found and supported versions.
func (r *FlowsRefusal) Detail() string {
	if !r.Unsupported() {
		return FlowsDeclarationPath
	}
	return fmt.Sprintf("%s version %s; supported %s", FlowsDeclarationPath, r.Found, supportedList(r.Supported))
}

func supportedList(versions []int) string {
	parts := make([]string, len(versions))
	for i, v := range versions {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}

func flowsInvalid(format string, args ...any) *FlowsRefusal {
	return &FlowsRefusal{Reason: affected.UnknownFlowsDeclarationInvalid, Supported: FlowsSupportedVersions(), Message: fmt.Sprintf(format, args...)}
}

// Code is the refusal's command error code: the reason in lower kebab case
// (flows-schema-unsupported or flows-declaration-invalid).
func (r *FlowsRefusal) Code() string {
	return strings.ToLower(strings.ReplaceAll(r.Reason, "_", "-"))
}

// ReportRefusal is the refusal a flow report gives before writing anything
// (FSV-V1-005): nil without a declaration or with one this build reads, else
// the reader's refusal, whose message begins with its reason.
func ReportRefusal(root string) *FlowsRefusal {
	_, err := ReadFlowsDeclaration(root)
	return AsFlowsRefusal(err)
}

// AsFlowsRefusal returns err as the reader's typed refusal, or nil.
func AsFlowsRefusal(err error) *FlowsRefusal {
	var refusal *FlowsRefusal
	if errors.As(err, &refusal) {
		return refusal
	}
	return nil
}

// parseFlowsProfile reads the version a profile names. A profile without
// the prefix, or whose N is empty, signed, has a leading zero or any other
// character, is invalid; a well-formed N is returned as written and, when it
// fits, as an int (ok false for one too long to be any supported version).
func parseFlowsProfile(profile string) (written string, n int, ok bool, err error) {
	written, found := strings.CutPrefix(profile, FlowsProfilePrefix)
	if !found {
		return "", 0, false, fmt.Errorf("profile %q must be %sN", profile, FlowsProfilePrefix)
	}
	if written == "" || (len(written) > 1 && written[0] == '0') || strings.TrimLeft(written, "0123456789") != "" {
		return "", 0, false, fmt.Errorf("profile %q: N must be a whole number without leading zeros", profile)
	}
	if len(written) > 9 {
		return written, 0, false, nil
	}
	n, _ = strconv.Atoi(written)
	return written, n, true, nil
}

// ReadFlowsDeclaration is the one reader of `.corvint/flows.json`
// (FSV-V1-003): nil with no file; otherwise the declaration of a supported
// version, or a *FlowsRefusal for a file that is unreadable, not a regular
// file, oversized, of an unsupported version or invalid in any member.
func ReadFlowsDeclaration(root string) (*FlowsDeclaration, error) {
	name := filepath.Join(root, filepath.FromSlash(FlowsDeclarationPath))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, flowsInvalid("%v", err)
	}
	if !info.Mode().IsRegular() {
		return nil, flowsInvalid("not a regular file")
	}
	if info.Size() > maxFlowsDeclarationBytes {
		return nil, flowsInvalid("larger than %d bytes", maxFlowsDeclarationBytes)
	}
	body, err := os.ReadFile(name)
	if err != nil {
		return nil, flowsInvalid("%v", err)
	}
	return decodeFlowsDeclaration(body)
}

// decodeFlowsDeclaration checks the version before any other member, so a
// newer file whose members this build does not know is refused as
// unsupported, not as invalid.
func decodeFlowsDeclaration(body []byte) (*FlowsDeclaration, error) {
	var head struct {
		Profile *string `json:"profile"`
	}
	if err := json.UnmarshalRead(bytes.NewReader(body), &head); err != nil {
		return nil, flowsInvalid("%v", err)
	}
	if head.Profile == nil {
		return nil, flowsInvalid("profile is required")
	}
	written, n, fits, err := parseFlowsProfile(*head.Profile)
	if err != nil {
		return nil, flowsInvalid("%v", err)
	}
	for _, schema := range flowsSchemas {
		if !fits || schema.version != n {
			continue
		}
		file := reflect.New(schema.file)
		if err := json.UnmarshalRead(bytes.NewReader(body), file.Interface(), json.RejectUnknownMembers(true)); err != nil {
			return nil, flowsInvalid("%v", err)
		}
		declared, err := schema.validate(file.Interface())
		if err != nil {
			return nil, flowsInvalid("%v", err)
		}
		declared.Version = n
		return declared, nil
	}
	supported := FlowsSupportedVersions()
	return nil, &FlowsRefusal{Reason: affected.UnknownFlowsSchemaUnsupported, Found: written, Supported: supported,
		Message: fmt.Sprintf("profile %s is schema version %s, which this build does not read; it reads version %s (FSV-V1-004)",
			*head.Profile, written, supportedList(supported))}
}
