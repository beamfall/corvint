package cli

import (
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// importMapCommand runs `corvint-tasks import-map amend` (CTS-V0-021..023):
// the only writer of intent/import-map.json after init. It records an amended
// map and the queue binding it as one OWNER receipt, refusing an amendment that
// drops or remaps a ticket identity.
func importMapCommand(env Env, args []string) *wire.Result {
	cmd := []string{"import-map", "amend"}
	if len(args) == 0 || args[0] != "amend" {
		return usage([]string{"import-map"}, "import-map needs the verb amend")
	}
	args = args[1:]
	role, request, expected, file := "OWNER", "", "", ""
	values := map[string]*string{"--role": &role, "--request-id": &request, "--expected-import-map-sha256": &expected, "--file": &file}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		key := args[i]
		destination, ok := values[key]
		if !ok || seen[key] || i+1 >= len(args) {
			return usage(cmd, "import-map amend requires --request-id ID --expected-import-map-sha256 SHA256 --file PATH and optional --role OWNER|OPERATOR; duplicate and unknown flags refuse")
		}
		seen[key] = true
		i++
		*destination = args[i]
	}
	if !seen["--request-id"] || !seen["--expected-import-map-sha256"] || !seen["--file"] {
		return usage(cmd, "import-map amend requires --request-id ID --expected-import-map-sha256 SHA256 --file PATH")
	}
	if _, err := mutation.ParseRequestID("requestId", request); err != nil {
		return errorResult(cmd, err)
	}
	digest, err := wire.ParseDigest("expectedImportMapSha256", expected)
	if err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(env.Cwd, file)
	}
	raw, err := intent.ReadFile(file, wire.MaxImportMapBytes)
	if err != nil {
		return errorResult(cmd, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	observed, _, err := writerProbe(nil, repo.StateDir)
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	report, err := store.ImportMapAmend(writerContext(), repo, actor, store.ImportMapRequest{QueueID: observed.Head.QueueID.Raw, RequestID: request, ExpectedImportMapSha256: digest, ImportMap: raw}, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	res := mutateResult(cmd, report)
	o := res.Items[0].Obj
	o.Set("oldImportMapSha256", wire.String(string(digest)))
	if report.Outcome.Outcome == mutation.OutcomeCompleted {
		o.Set("newImportMapSha256", wire.String(string(wire.Sum(raw))))
	} else {
		o.Set("newImportMapSha256", wire.Null())
	}
	return res
}
