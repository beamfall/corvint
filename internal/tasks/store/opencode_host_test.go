package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0077_OpenCodeStageArgv(t *testing.T) {
	c := ProgramConfig{Model: "local/probe", Effort: "low", StageEfforts: map[string]string{"review": "high"}}
	for _, tc := range []struct{ stage, session, want string }{
		{"implement", "", "run --standalone --format json --model local/probe#low"},
		{"implement", "ses_1", "run --standalone --format json --model local/probe#low --session ses_1 --fork"},
		{"review", "", "run --standalone --format json --model local/probe#high"},
		{"integrate", "", "run --standalone --format json --model local/probe#low"},
	} {
		if got := strings.Join(opencodeStageArgv(c, tc.stage, tc.session), " "); got != tc.want {
			t.Errorf("%s %q argv %q, want %q", tc.stage, tc.session, got, tc.want)
		}
	}
	for _, stage := range []string{"implement", "review", "integrate"} {
		env := opencodeStageEnv([]string{"HOME=/h"}, stage)
		if len(env) != 6 || env[0] != "HOME=/h" || env[1] != "OPENCODE_DISABLE_AUTOUPDATE=1" || env[2] != "OPENCODE_DISABLE_PROJECT_CONFIG=1" || env[3] != "OPENCODE_PRINT_LOGS=1" || env[4] != "OPENCODE_LOG_LEVEL=ERROR" {
			t.Fatalf("%s env %q", stage, env)
		}
		content, ok := strings.CutPrefix(env[5], "OPENCODE_CONFIG_CONTENT=")
		var config struct {
			Permission map[string]string `json:"permission"`
		}
		if !ok || json.Unmarshal([]byte(content), &config) != nil {
			t.Fatalf("%s config %q", stage, env[5])
		}
		want := map[string]string{"external_directory": "deny", "task": "deny"}
		if stage != "implement" {
			want["edit"] = "deny"
		}
		if len(config.Permission) != len(want) {
			t.Fatalf("%s permission %v", stage, config.Permission)
		}
		for k, v := range want {
			if config.Permission[k] != v {
				t.Fatalf("%s permission %v", stage, config.Permission)
			}
		}
	}
}

func TestCALV0076_CheckOpenCodeConfig(t *testing.T) {
	if e := checkOpenCodeConfig(ProgramConfig{Model: "local/probe"}); e != nil {
		t.Fatal(e)
	}
	for _, c := range []ProgramConfig{
		{Model: "probe"},
		{Model: "/probe"},
		{Model: "local/"},
		{Model: "local/probe#high"},
		{Model: "local/probe", Repositories: []ProgramRepository{{Name: "site", Checkout: "/w/site"}}},
	} {
		if e := checkOpenCodeConfig(c); e == nil || wire.CodeOf(e) != wire.CodeUnsupported {
			t.Errorf("%+v admitted: %v", c, e)
		}
	}
}

func TestCALV0076_CheckProgramConfigOpenCodeHost(t *testing.T) {
	opencode := &intent.SupervisionPolicy{Host: intent.SupervisedHostOpenCode}
	claude := &intent.SupervisionPolicy{Host: intent.SupervisedHostClaudeCode}
	base := ProgramConfig{Effort: "low", WallSeconds: 60, Model: "local/probe", Host: supervisor.HostOpenCode}
	if e := CheckProgramConfig(base, opencode); e != nil {
		t.Fatal(e)
	}
	variant := base
	variant.Model = "local/probe#high"
	codexConfig := base
	codexConfig.Host = ""
	for name, tc := range map[string]struct {
		c      ProgramConfig
		policy *intent.SupervisionPolicy
	}{
		"opencode config under codex policy":       {base, nil},
		"opencode config under claude-code policy": {base, claude},
		"codex config under opencode policy":       {codexConfig, opencode},
		"model carries a variant":                  {variant, opencode},
	} {
		if e := CheckProgramConfig(tc.c, tc.policy); e == nil || wire.CodeOf(e) != wire.CodeUnsupported {
			t.Errorf("%s: %s %v", name, wire.CodeOf(e), e)
		}
	}
}
