package testsource

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// actorAuthFixture is the synthetic Rails + Playwright repository from GitHub issue #1115: fixture
// alpha seeds an assistant, the rules grant `:update, Widget` only to manager, and manager
// inherits assistant (not the reverse).
func actorAuthFixture() map[string]string {
	return map[string]string{
		"app/models/ability.rb": `class Ability
  include CanCan::Ability

  def initialize(user)
    send(user.role)
  end

  def assistant
    can :read, Widget
  end

  def manager
    assistant
    can :update, Widget # managers edit widgets
  end

  def admin
    manager
    can :manage, :all
  end
end
`,
		"config/routes.rb": `Rails.application.routes.draw do
  namespace :api do
    resources :widgets, only: [:show, :update]
  end
end
`,
		"app/controllers/api/widgets_controller.rb": `module Api
  class WidgetsController < ApplicationController
    def show
      authorize! :read, Widget
      render json: Widget.find(params[:id])
    end

    def update
      authorize! :update, Widget
      widget = Widget.find(params[:id])
      widget.update!(params.require(:widget).permit(:name))
      render json: widget
    end
  end
end
`,
		"e2e/fixtures/users.ts": `export async function createUser({ email, role = 'assistant' }: { email: string; role?: string }) {
  return db.insert('users', { email, role });
}
`,
		"e2e/fixtures/alpha.ts": `import { createUser } from './users';

export async function alpha() {
  const email = 'assistant@example.test';
  await createUser({ email, role: 'assistant' });
  return { assistant: { email } };
}
`,
		"e2e/fixtures/beta.ts": `import { createUser } from './users';

export const beta = async () => {
  const manager = await createUser({ email: 'manager@example.test', role: 'manager' });
  const helper = await createUser({ email: 'helper@example.test' });
  return { manager, helper: { email: helper.email } };
};
`,
		"e2e/widgets.spec.ts": `import { test, expect } from '@playwright/test';
import { alpha } from './fixtures/alpha';
import { beta } from './fixtures/beta';
import { login } from './helpers';

test('assistant renames a widget', async ({ request }) => {
  const fixture = await alpha();
  await login(request, fixture.assistant);
  const response = await request.put(` + "`/api/widgets/${widgetId}`" + `, { data: { name: 'new' } });
  expect(response.status()).toBe(200);
});

test('manager renames a widget', async ({ request }) => {
  const team = await beta();
  await login(request, team.manager);
  const response = await request.put('/api/widgets/7', { data: { name: 'new' } });
  expect(response.ok()).toBeTruthy();
});

test('assistant cannot rename a widget', async ({ request }) => {
  const fixture = await alpha();
  await login(request, fixture.assistant);
  const response = await request.put('/api/widgets/7', { data: { name: 'new' } });
  expect(response.status()).toBe(403);
});
`,
	}
}

// TestActorAuthorizationIssueRepro reproduces the issue's assistant/manager case as an
// ACTOR-UNAUTHORIZED finding naming the granting rule, the lowest sufficient role and the
// alternative fixture, reports the manager pair clean and the 403-asserting test exempt, reading
// committed objects only (TSA-V0-033..037, TSA-V0-039).
func TestActorAuthorizationIssueRepro(t *testing.T) {
	root := t.TempDir()
	head := commitFiles(t, root, actorAuthFixture())
	report, err := ActorAuthorization(OpenRepo(context.Background(), root), ActorAuthOptions{Revision: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	assertValidStatuses(t, report)
	if report.Schema != ActorAuthSchema || report.Revision != head || report.Verdict != "ACTOR_UNAUTHORIZED" {
		t.Fatalf("header = %s %s %s", report.Schema, report.Revision, report.Verdict)
	}
	if len(report.Requests) != 3 {
		t.Fatalf("requests = %+v", report.Requests)
	}
	finding := report.Requests[0]
	wantActor := FixtureActor{Fixture: "alpha", Key: "assistant", Role: "assistant", RoleFrom: "literal", Path: "e2e/fixtures/alpha.ts", Line: 5}
	if finding.Status != ActorUnauthorized || finding.Code != ActorUnauthorizedCode || finding.Line != 9 ||
		finding.Method != "PUT" || finding.URL != "/api/widgets/${widgetId}" || finding.Test != "assistant renames a widget" ||
		finding.Actor == nil || *finding.Actor != wantActor {
		t.Fatalf("finding = %+v actor %+v", finding, finding.Actor)
	}
	if *finding.Route != (RouteRef{Controller: "api/widgets", Action: "update", Path: "config/routes.rb", Line: 3}) ||
		*finding.Check != (AbilityCheck{Action: "update", Subject: "Widget", Path: "app/controllers/api/widgets_controller.rb", Line: 9}) {
		t.Fatalf("route %+v check %+v", finding.Route, finding.Check)
	}
	if !reflect.DeepEqual(finding.GrantedBy, []GrantRef{{Role: "manager", Path: "app/models/ability.rb", Line: 14}}) ||
		!reflect.DeepEqual(finding.LowestRoles, []string{"manager"}) {
		t.Fatalf("grantedBy %+v lowest %v", finding.GrantedBy, finding.LowestRoles)
	}
	if len(finding.Alternatives) != 1 || finding.Alternatives[0].Fixture != "beta" || finding.Alternatives[0].Key != "manager" ||
		finding.Alternatives[0].Path != "e2e/fixtures/beta.ts" || finding.Alternatives[0].Line != 4 {
		t.Fatalf("alternatives = %+v", finding.Alternatives)
	}
	if clean := report.Requests[1]; clean.Status != ActorAuthorized || clean.Code != "" || clean.Actor.Role != "manager" || len(clean.LowestRoles) != 0 {
		t.Fatalf("authorized pair = %+v", clean)
	}
	if exempt := report.Requests[2]; exempt.Status != ActorExempt || exempt.Reason != "asserts-rejection" ||
		exempt.Detail != "e2e/widgets.spec.ts:24 asserts status 403" || exempt.Code != "" {
		t.Fatalf("exempt = %+v", exempt)
	}
	helper := FixtureActor{Fixture: "beta", Key: "helper", Role: "assistant", RoleFrom: "factory-default", Path: "e2e/fixtures/beta.ts", Line: 5, Detail: "e2e/fixtures/users.ts:1"}
	if len(report.Actors) != 3 || report.Actors[1] != helper {
		t.Fatalf("actors = %+v", report.Actors)
	}
}

// TestActorAuthorizationDeterministic proves the pure analysis is stable and that only CLEAN,
// UNKNOWN and ACTOR_UNAUTHORIZED verdicts arise (TSA-V0-037).
func TestActorAuthorizationDeterministic(t *testing.T) {
	files := actorAuthFixture()
	first, _ := json.Marshal(AnalyzeActorAuthorization(files, DefaultRoutesPath, DefaultAbilityPath))
	second, _ := json.Marshal(AnalyzeActorAuthorization(files, DefaultRoutesPath, DefaultAbilityPath))
	if string(first) != string(second) {
		t.Fatal("report is not deterministic")
	}
	delete(files, "e2e/fixtures/alpha.ts")
	files["e2e/widgets.spec.ts"] = strings.ReplaceAll(files["e2e/widgets.spec.ts"], "toBe(403)", "toBe(200)")
	report := AnalyzeActorAuthorization(files, DefaultRoutesPath, DefaultAbilityPath)
	if report.Verdict != ActorUnknown || report.Requests[1].Status != ActorAuthorized {
		t.Fatalf("verdict %s requests %+v", report.Verdict, report.Requests)
	}
	lines := strings.Split(files["e2e/widgets.spec.ts"], "\n")
	files["e2e/widgets.spec.ts"] = strings.Join(append(lines[:5:5], lines[12:18]...), "\n")
	if report := AnalyzeActorAuthorization(files, DefaultRoutesPath, DefaultAbilityPath); report.Verdict != "CLEAN" {
		t.Fatalf("verdict = %s %+v", report.Verdict, report.Requests)
	}
}

// TestActorAuthorizationInheritance evaluates inherited rule sets: an inherited grant authorizes,
// `:manage, :all` covers any action, CanCanCan's default aliases apply, and the lowest roles exclude
// a role that inherits another granting role (TSA-V0-035, TSA-V0-036).
func TestActorAuthorizationInheritance(t *testing.T) {
	model := parseAbility(DefaultAbilityPath, actorAuthFixture()[DefaultAbilityPath])
	cases := []struct {
		role, action, subject, state string
		line                         int
	}{
		{"assistant", "update", "Widget", "denied", 0},
		{"manager", "update", "Widget", "granted", 14},
		{"manager", "show", "Widget", "granted", 9},
		{"manager", "edit", "Widget", "granted", 14},
		{"admin", "destroy", "Gadget", "granted", 19},
		{"assistant", "destroy", "Widget", "denied", 0},
	}
	for _, c := range cases {
		g := model.evaluate(c.role, c.action, c.subject)
		if g.state != c.state || g.rule.Line != c.line {
			t.Errorf("%s %s %s = %+v", c.role, c.action, c.subject, g)
		}
	}
}

// TestActorAuthorizationUnknownLinks proves every unresolved link yields UNKNOWN with a typed
// reason and never AUTHORIZED or UNAUTHORIZED (TSA-V0-038).
func TestActorAuthorizationUnknownLinks(t *testing.T) {
	const spec, ability, routes = "e2e/widgets.spec.ts", DefaultAbilityPath, DefaultRoutesPath
	const controller = "app/controllers/api/widgets_controller.rb"
	replace := func(name, old, new string) func(map[string]string) {
		return func(files map[string]string) {
			if !strings.Contains(files[name], old) {
				t.Fatalf("%s lacks %q", name, old)
			}
			files[name] = strings.Replace(files[name], old, new, 1)
		}
	}
	cases := []struct {
		reason string
		edit   func(map[string]string)
	}{
		{"role-not-literal", replace("e2e/fixtures/alpha.ts", "role: 'assistant'", "role: roles.assistant")},
		{"role-default-unresolved", replace("e2e/fixtures/alpha.ts", "createUser({ email, role: 'assistant' })", "makeUser({ email })")},
		{"fixture-return-not-literal", replace("e2e/fixtures/alpha.ts", "return { assistant: { email } };", "const out = { assistant: { email } };\n  return out;")},
		{"role-ambiguous", replace("e2e/fixtures/alpha.ts", "role: 'assistant' }", "role: 'assistant', profile: { role: 'manager' } }")},
		{"actor-key-ambiguous", replace("e2e/fixtures/alpha.ts", "  return {", "  await createUser({ email, role: 'manager' });\n  return {")},
		{"fixture-ambiguous", func(files map[string]string) { files["e2e/fixtures/alpha2.ts"] = files["e2e/fixtures/alpha.ts"] }},
		{"actor-key-not-user", replace(spec, "fixture.assistant);\n  const response = await request.put(`", "fixture.widget);\n  const response = await request.put(`")},
		{"actor-not-literal", replace(spec, "login(request, fixture.assistant);\n  const response = await request.put(`", "login(request, users[0]);\n  const response = await request.put(`")},
		{"actor-unbound", replace(spec, "await login(request, fixture.assistant);\n  const response = await request.put(`", "const response = await request.put(`")},
		{"path-not-literal", replace(spec, "`/api/widgets/${widgetId}`", "`/api/widgets/id-${widgetId}`")},
		{"route-not-found", replace(spec, "`/api/widgets/${widgetId}`", "`/api/gadgets/${widgetId}`")},
		{"route-unparsed", func(files map[string]string) {
			replace(spec, "`/api/widgets/${widgetId}`", "`/api/v1/widgets/${widgetId}`")(files)
			replace(routes, "  namespace :api do", "  scope module: 'v1' do\n    resources :widgets\n  end\n  namespace :api do")(files)
		}},
		{"routes-unavailable", func(files map[string]string) { delete(files, routes) }},
		{"controller-unavailable", func(files map[string]string) { delete(files, controller) }},
		{"action-not-found", replace(controller, "def update", "def modify")},
		{"check-not-literal", replace(controller, "authorize! :update, Widget", "authorize! :update, @widget")},
		{"check-not-found", replace(controller, "      authorize! :update, Widget\n", "")},
		{"check-implicit", func(files map[string]string) {
			replace(controller, "      authorize! :update, Widget\n", "")(files)
			replace(controller, "class WidgetsController < ApplicationController\n", "class WidgetsController < ApplicationController\n    load_and_authorize_resource\n")(files)
		}},
		{"check-multiple", replace(controller, "authorize! :update, Widget", "authorize! :update, Widget\n      authorize! :read, Widget")},
		{"ability-unavailable", func(files map[string]string) { delete(files, ability) }},
		{"role-dispatch-not-literal", replace(ability, "    send(user.role)\n", "    case user.role\n    when 'manager' then manager\n    end\n")},
		{"role-undefined", replace("e2e/fixtures/alpha.ts", "role: 'assistant'", "role: 'guest'")},
		{"inheritance-cycle", replace(ability, "    can :read, Widget\n", "    can :read, Widget\n    admin\n")},
		{"rule-conditional", replace(ability, "    can :read, Widget\n", "    can :read, Widget\n    can :update, Widget, owner_id: user.id\n")},
		{"rule-cannot", replace(ability, "    can :read, Widget\n", "    can :read, Widget\n    cannot :update, Widget\n")},
		{"statement-unparsed", replace(ability, "    can :read, Widget\n", "    can :read, Widget\n    if user.vip?\n      can :update, Widget\n    end\n")},
	}
	for _, c := range cases {
		t.Run(c.reason, func(t *testing.T) {
			files := actorAuthFixture()
			c.edit(files)
			report := AnalyzeActorAuthorization(files, routes, ability)
			assertValidStatuses(t, report)
			first := report.Requests[0]
			if first.Status != ActorUnknown || first.Reason != c.reason || first.Detail == "" || first.Code != "" {
				t.Fatalf("request = %+v", first)
			}
			if report.Verdict != ActorUnknown {
				t.Fatalf("verdict = %s", report.Verdict)
			}
		})
	}
}

// assertValidStatuses fails when any request lacks one of the four request statuses.
func assertValidStatuses(t *testing.T, report ActorAuthReport) {
	t.Helper()
	for _, r := range report.Requests {
		switch r.Status {
		case ActorUnauthorized, ActorAuthorized, ActorExempt, ActorUnknown:
		default:
			t.Fatalf("request without a valid status: %+v", r)
		}
	}
}

// TestActorAuthorizationReviewRegressions covers inputs that must not read as resolved: a role a
// later spread can override, a non-literal factory default, a guarded controller check, a rule
// hidden in another statement, an unread statement that could revoke a grant, a negated or
// non-status rejection assertion, a non-literal request path and an empty role
// (TSA-V0-033..036, TSA-V0-038, TSA-V0-039).
func TestActorAuthorizationReviewRegressions(t *testing.T) {
	const spec, ability = "e2e/widgets.spec.ts", DefaultAbilityPath
	const controller, alpha = "app/controllers/api/widgets_controller.rb", "e2e/fixtures/alpha.ts"
	replace := func(name, old, new string) func(map[string]string) {
		return func(files map[string]string) {
			if !strings.Contains(files[name], old) {
				t.Fatalf("%s lacks %q", name, old)
			}
			files[name] = strings.Replace(files[name], old, new, 1)
		}
	}
	cases := []struct {
		name           string
		edit           func(map[string]string)
		index          int
		status, reason string
	}{
		{"spread-overrides-role", replace(alpha, "role: 'assistant' })", "role: 'assistant', ...overrides })"), 0, ActorUnknown, "role-not-literal"},
		{"spread-sets-default-role", replace(alpha, "createUser({ email, role: 'assistant' })", "createUser({ email, ...overrides })"), 0, ActorUnknown, "role-not-literal"},
		{"computed-key-overrides-role", replace(alpha, "role: 'assistant' })", "role: 'assistant', [key]: 'manager' })"), 0, ActorUnknown, "role-not-literal"},
		{"default-not-single-literal", func(files map[string]string) {
			replace("e2e/fixtures/users.ts", "role = 'assistant' }", "role = 'assistant'.replace('a', 'b') }")(files)
			replace(alpha, "createUser({ email, role: 'assistant' })", "createUser({ email })")(files)
		}, 0, ActorUnknown, "role-default-unresolved"},
		{"check-after-early-return", replace(controller, "      authorize! :update, Widget\n", "      return head(:forbidden) unless current_user\n      authorize! :update, Widget\n"), 0, ActorUnknown, "check-conditional"},
		{"check-inside-if", replace(controller, "      authorize! :update, Widget\n", "      if current_user.active?\n        authorize! :update, Widget\n      end\n"), 0, ActorUnknown, "check-conditional"},
		{"check-in-rescue", replace(controller, "      authorize! :update, Widget\n      widget = Widget.find(params[:id])\n", "      widget = Widget.find(params[:id])\n    rescue ActiveRecord::RecordNotFound\n      authorize! :update, Widget\n"), 0, ActorUnknown, "check-conditional"},
		{"return-can-unparsed", replace(ability, "    can :update, Widget # managers edit widgets\n", "    return can :update, Widget\n"), 1, ActorUnknown, "statement-unparsed"},
		{"assigned-can-unparsed", replace(ability, "    can :update, Widget # managers edit widgets\n", "    granted ||= can :update, Widget\n"), 1, ActorUnknown, "statement-unparsed"},
		{"unparsed-cannot-outranks-grant", replace(ability, "    can :update, Widget # managers edit widgets\n", "    can :update, Widget\n    if user.suspended?\n      cannot :update, Widget\n    end\n"), 1, ActorUnknown, "statement-unparsed"},
		{"negated-rejection-not-exempt", replace(spec, "expect(response.status()).toBe(403)", "expect(response.status()).not.toBe(403)"), 2, ActorUnauthorized, ""},
		{"non-status-rejection-not-exempt", replace(spec, "expect(response.status()).toBe(403)", "expect(widget.id).toBe(403)"), 2, ActorUnauthorized, ""},
		{"status-property-exempt", replace(spec, "expect(response.status()).toBe(403)", "expect(response.statusCode).toBe(403)"), 2, ActorExempt, "asserts-rejection"},
		{"variable-path", replace(spec, "request.put(`/api/widgets/${widgetId}`", "request.put(url"), 0, ActorUnknown, "path-not-literal"},
		{"concatenated-path", replace(spec, "request.put(`/api/widgets/${widgetId}`", "request.put('/api/widgets/' + widgetId"), 0, ActorUnknown, "path-not-literal"},
		{"empty-role", replace(alpha, "role: 'assistant'", "role: ''"), 0, ActorUnknown, "role-undefined"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := actorAuthFixture()
			c.edit(files)
			report := AnalyzeActorAuthorization(files, DefaultRoutesPath, ability)
			assertValidStatuses(t, report)
			if len(report.Requests) != 3 {
				t.Fatalf("requests = %+v", report.Requests)
			}
			r := report.Requests[c.index]
			if r.Status != c.status || r.Reason != c.reason || (c.status == ActorUnknown && (r.Detail == "" || r.Code != "")) {
				t.Fatalf("request = %+v", r)
			}
			if c.status == ActorUnknown && report.Verdict == "CLEAN" {
				t.Fatalf("verdict = %s", report.Verdict)
			}
		})
	}
}
