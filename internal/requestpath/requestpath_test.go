package requestpath

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type memSource map[string]string

func (m memSource) Read(candidates []string) (string, string, bool, error) {
	for _, c := range candidates {
		if t, ok := m[c]; ok {
			return c, t, true, nil
		}
	}
	return "", "", false, nil
}

// tokenExchange is the acceptance fixture: a guard that exchanges a token with an identity
// provider, a cheap validation guard, and a handler, registered in both orders.
var tokenExchange = memSource{
	"src/app.ts": `import express from 'express';
import { exchangeToken } from './guards/exchange';
import { validateBody } from './guards/validate.js';
import * as orders from './handlers';

const app = express();
app.use(express.json());

app.post('/orders', exchangeToken, validateBody, orders.createOrder);
app.post('/orders/reordered', validateBody, exchangeToken, orders.createOrder);
export default app;
`,
	"src/guards/exchange.ts": `import axios from 'axios';
const ISSUER = process.env.OIDC_ISSUER;

export async function exchangeToken(req, res, next) {
  const subject = req.header('authorization');
  const r = await axios.post(` + "`${process.env.IDP_TOKEN_ENDPOINT}/token`" + `, { issuer: ISSUER, subject_token: subject });
  req.user = r.data;
  next();
}
`,
	"src/guards/validate.ts": `import { z } from 'zod';
const Order = z.object({ sku: z.string(), qty: z.number() });

export function validateBody(req, res, next) {
  const parsed = Order.safeParse(req.body);
  if (!parsed.success) return res.status(400).end();
  next();
}
`,
	"src/handlers/index.ts": `import config from 'config';
export async function createOrder(req, res) {
  const limit = config.get('orders.limit');
  res.json({ ok: true, limit, region: process.env.REGION || 'us' });
}
`,
}

func analyze(t *testing.T, src Source, q Query) *Report {
	t.Helper()
	r, err := Analyze(src, "HEAD", q)
	if err != nil {
		t.Fatalf("Analyze(%+v): %v", q, err)
	}
	return r
}

func envNames(r *Report) []string {
	out := []string{}
	for _, k := range r.EnvKeys {
		out = append(out, k.Name)
	}
	return out
}

func stepByName(t *testing.T, r *Report, name string) Step {
	t.Helper()
	for _, s := range r.Steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no step %s in %+v", name, r.Steps)
	return Step{}
}

// TestAMAPV0131TokenExchangeKeysAndOutbound is acceptance criterion (a) (AMAP-V0-131, AMAP-V0-132).
func TestAMAPV0131TokenExchangeKeysAndOutbound(t *testing.T) {
	r := analyze(t, tokenExchange, Query{File: "src/app.ts", Method: "POST", Path: "/orders"})
	if got, want := envNames(r), []string{"IDP_TOKEN_ENDPOINT", "OIDC_ISSUER", "REGION"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("env keys = %v, want %v", got, want)
	}
	names := []string{}
	for _, s := range r.Steps {
		names = append(names, s.Role+":"+s.Name)
	}
	if want := []string{"GUARD:express.json()", "GUARD:exchangeToken", "GUARD:validateBody", "HANDLER:orders.createOrder"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("steps = %v, want %v", names, want)
	}
	ex := stepByName(t, r, "exchangeToken")
	if ex.Outbound != OutboundYes || !reflect.DeepEqual(ex.EnvKeys, []string{"IDP_TOKEN_ENDPOINT", "OIDC_ISSUER"}) {
		t.Fatalf("exchange step = %+v", ex)
	}
	if len(r.OutboundCalls) != 1 || r.OutboundCalls[0].Callee != "axios.post" || r.OutboundCalls[0].Step != ex.Order ||
		r.OutboundCalls[0].File != "src/guards/exchange.ts" || r.OutboundCalls[0].Line != 6 {
		t.Fatalf("outbound calls = %+v", r.OutboundCalls)
	}
	for _, name := range []string{"validateBody", "orders.createOrder", "express.json()"} {
		if s := stepByName(t, r, name); s.Outbound != OutboundNoneFound || s.Unknowns != 0 {
			t.Fatalf("step %s = %+v", name, s)
		}
	}
	if len(r.ConfigKeys) != 1 || r.ConfigKeys[0].Name != "orders.limit" {
		t.Fatalf("config keys = %+v", r.ConfigKeys)
	}
	for _, k := range r.EnvKeys {
		if k.Defaulted != (k.Name == "REGION") {
			t.Fatalf("key %s defaulted = %v", k.Name, k.Defaulted)
		}
	}
	if len(r.Unknowns) != 0 {
		t.Fatalf("unknowns = %+v", r.Unknowns)
	}
}

// TestAMAPV0135PreRunBlocksByNameOnly is acceptance criterion (b) (AMAP-V0-135, AMAP-V0-136).
func TestAMAPV0135PreRunBlocksByNameOnly(t *testing.T) {
	r := analyze(t, tokenExchange, Query{File: "src/app.ts", Method: "POST", Path: "/orders"})
	p, err := ParseProfile("staging", "# staging\nexport OIDC_ISSUER=\nIDP_TOKEN_ENDPOINT=\"\"\nDATABASE_PASSWORD=hunter2-secret\nSESSION='s3cr3t-value'\n")
	if err != nil {
		t.Fatal(err)
	}
	pre := Check(r, p, false)
	if pre.Verdict != VerdictBlocked || pre.Batch != BatchDoNotSchedule || pre.Mode != ModeFailClosed {
		t.Fatalf("pre-run = %+v", pre)
	}
	if !reflect.DeepEqual(pre.Empty, []string{"IDP_TOKEN_ENDPOINT", "OIDC_ISSUER"}) || len(pre.Unset) != 0 ||
		!reflect.DeepEqual(pre.DefaultedUnsetOrEmpty, []string{"REGION"}) || !reflect.DeepEqual(pre.ConfigKeysNotChecked, []string{"orders.limit"}) {
		t.Fatalf("pre-run keys = %+v", pre)
	}
	raw, _ := json.Marshal(pre)
	for _, secret := range []string{"hunter2", "s3cr3t", "DATABASE_PASSWORD", "SESSION"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("pre-run output carries %q: %s", secret, raw)
		}
	}
	if w := Check(r, p, true); w.Verdict != VerdictBlocked || w.Batch != BatchScheduleWithWarnings || w.Mode != ModeWarn {
		t.Fatalf("warn pre-run = %+v", w)
	}
	unset, _ := ParseProfile("bare", "OIDC_ISSUER=https://issuer.example\n")
	if u := Check(r, unset, false); !reflect.DeepEqual(u.Unset, []string{"IDP_TOKEN_ENDPOINT"}) || u.Verdict != VerdictBlocked {
		t.Fatalf("unset pre-run = %+v", u)
	}
	full, _ := ParseProfile("prod", "OIDC_ISSUER=https://issuer.example\nIDP_TOKEN_ENDPOINT=https://idp.example\n")
	if ok := Check(r, full, false); ok.Verdict != VerdictReady || ok.Batch != BatchSchedule || len(ok.DefaultedUnsetOrEmpty) != 1 {
		t.Fatalf("ready pre-run = %+v", ok)
	}
}

func TestAMAPV0135ProfileRefusesByLineOnly(t *testing.T) {
	_, err := ParseProfile("bad", "A=1\nthis-is-secret-text\n")
	var e *Error
	if !errors.As(err, &e) || e.Code != "request-path-profile-invalid" || !strings.Contains(e.Message, "line 2") || strings.Contains(e.Message, "secret") {
		t.Fatalf("err = %v", err)
	}
	p, err := ParseProfile("ok", "A=x # comment\nB= # only a comment\nC='a # b'\n")
	if err != nil || !p.nonEmpty["A"] || p.nonEmpty["B"] || !p.nonEmpty["C"] {
		t.Fatalf("profile = %+v, %v", p, err)
	}
}

// TestAMAPV0133GuardOrderFlagged is acceptance criterion (c) (AMAP-V0-133).
func TestAMAPV0133GuardOrderFlagged(t *testing.T) {
	r := analyze(t, tokenExchange, Query{File: "src/app.ts", Method: "POST", Path: "/orders"})
	if len(r.GuardOrder) != 1 {
		t.Fatalf("guard order = %+v", r.GuardOrder)
	}
	g := r.GuardOrder[0]
	if g.Finding != FindingOutboundGuardFirst || g.OutboundGuard != 2 || !reflect.DeepEqual(g.CheapGuards, []int{3}) || !reflect.DeepEqual(g.StandInFor, []string{"axios.post"}) {
		t.Fatalf("finding = %+v", g)
	}
	reordered := analyze(t, tokenExchange, Query{File: "src/app.ts", Method: "POST", Path: "/orders/reordered"})
	if len(reordered.GuardOrder) != 0 {
		t.Fatalf("reordered guard order = %+v", reordered.GuardOrder)
	}
	if s := stepByName(t, reordered, "exchangeToken"); s.Order != 3 || s.Outbound != OutboundYes {
		t.Fatalf("reordered exchange step = %+v", s)
	}
}

// TestAMAPV0133GuardOrderNeedsProvenCheapGuard: a later guard with unknowns is not a cheap guard.
func TestAMAPV0133GuardOrderNeedsProvenCheapGuard(t *testing.T) {
	r := &Report{Steps: []Step{{Order: 1, Role: RoleGuard, Outbound: OutboundYes}, {Order: 2, Role: RoleGuard, Outbound: OutboundNoneFound, Unknowns: 1},
		{Order: 3, Role: RoleGuard, Outbound: OutboundUnknown}, {Order: 4, Role: RoleHandler, Outbound: OutboundNoneFound}}}
	if got := guardOrder(r); len(got) != 0 {
		t.Fatalf("guard order = %+v", got)
	}
}

var unresolvable = memSource{
	"api/dynamic.ts": `import { audit } from './missing-module';
import jwks from 'jwks-rsa';
const AUDIENCE_KEY = 'TOKEN_AUDIENCE';

export function dynamicHandler(req, res) {
  const name = req.query.tenant + '_URL';
  const v = process.env[name];
  const aud = process.env[AUDIENCE_KEY];
  const env = process.env;
  const limit = config.get(req.query.k);
  audit(req);
  jwks.getSigningKey('kid');
  res.json({ v, aud, env, limit });
}
`,
}

// TestAMAPV0134UnresolvableIsUnknown is acceptance criterion (d) (AMAP-V0-134).
func TestAMAPV0134UnresolvableIsUnknown(t *testing.T) {
	r := analyze(t, unresolvable, Query{File: "api/dynamic.ts", Handler: "dynamicHandler"})
	got := []string{}
	for _, u := range r.Unknowns {
		got = append(got, u.Kind+":"+u.Reason+":"+u.Text)
	}
	want := []string{"ENV_KEY:dynamic-key:process.env[name]", "ENV_KEY:env-object-escapes:process.env",
		"CALL:module-not-found:audit", "CALL:package-not-followed:jwks.getSigningKey (jwks-rsa)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unknowns = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(envNames(r), []string{"TOKEN_AUDIENCE"}) {
		t.Fatalf("env keys = %v", envNames(r))
	}
	if s := r.Steps[0]; s.Outbound != OutboundUnknown || s.Unknowns != 4 || s.Role != RoleHandler {
		t.Fatalf("step = %+v", s)
	}
	p, _ := ParseProfile("ci", "TOKEN_AUDIENCE=api\n")
	if pre := Check(r, p, false); pre.Verdict != VerdictIncomplete || pre.Batch != BatchDoNotSchedule ||
		!reflect.DeepEqual(pre.Unresolved, map[string]int{UnknownEnvKey: 2, UnknownCall: 2}) {
		t.Fatalf("pre-run = %+v", pre)
	}
}

func TestAMAPV0134DynamicConfigKey(t *testing.T) {
	src := memSource{"h.js": "const config = require('config');\nfunction h(req, res) { res.send(config.get(req.params.k) + config.get('a.b')); }\nmodule.exports = { h };\n"}
	r := analyze(t, src, Query{File: "h.js", Handler: "h"})
	if len(r.Unknowns) != 1 || r.Unknowns[0].Kind != UnknownConfigKey || r.Unknowns[0].Reason != "dynamic-key" {
		t.Fatalf("unknowns = %+v", r.Unknowns)
	}
	if len(r.ConfigKeys) != 1 || r.ConfigKeys[0].Name != "a.b" || r.Steps[0].Outbound != OutboundNoneFound {
		t.Fatalf("report = %+v", r)
	}
}

var commonJS = memSource{
	"server.js": `const express = require('express');
const rateLimit = require('express-rate-limit');
const { verify } = require('./lib/verify');
const idp = require('./lib/idp');
const router = express.Router();

router.use('/admin', adminOnly);
router.use('/users', (req, res, next) => { if (!req.headers.authorization) return res.sendStatus(401); next(); });
router.get('/users/:id', rateLimit({ max: Number(process.env.RATE_MAX) }), [verify, idp.lookup], function show(req, res) {
  const { DB_URL, PAGE_SIZE = '20' } = process.env;
  res.json({ db: DB_URL, size: PAGE_SIZE });
});
module.exports = router;
`,
	"lib/verify.js": `const https = require('https');
const KEY = 'JWKS_URI';
exports.verify = function (req, res, next) {
  https.get(process.env[KEY], () => next());
};
`,
	"lib/idp.js": `class Idp {
  lookup(req, res, next) { return this.fetchUser(req.params.id).then(next); }
  fetchUser(id) { return fetch(process.env.IDP_BASE + '/users/' + id); }
}
module.exports = new Idp();
`,
}

func TestAMAPV0132CommonJSRouterChain(t *testing.T) {
	r := analyze(t, commonJS, Query{File: "server.js", Method: "GET", Path: "/users/:id"})
	names := []string{}
	for _, s := range r.Steps {
		names = append(names, s.Registered+":"+s.Name+":"+s.Outbound)
	}
	want := []string{"use:<inline@8>:NONE_FOUND", "route:rateLimit({max:Number(process.env.RATE_MAX)}):UNKNOWN",
		"route:verify:YES", "route:idp.lookup:YES", "route:<inline@9>:NONE_FOUND"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("steps = %q, want %q", names, want)
	}
	if got := envNames(r); !reflect.DeepEqual(got, []string{"DB_URL", "IDP_BASE", "JWKS_URI", "PAGE_SIZE", "RATE_MAX"}) {
		t.Fatalf("env keys = %v", got)
	}
	for _, k := range r.EnvKeys {
		if k.Defaulted != (k.Name == "PAGE_SIZE") {
			t.Fatalf("key %+v", k)
		}
	}
	callees := []string{}
	for _, c := range r.OutboundCalls {
		callees = append(callees, c.Callee+"@"+strings.Join(c.Via, ">"))
	}
	if !reflect.DeepEqual(callees, []string{"https.get@verify", "fetch@idp.lookup>fetchUser"}) {
		t.Fatalf("calls = %q", callees)
	}
	kinds := []string{}
	for _, u := range r.Unknowns {
		kinds = append(kinds, u.Kind+":"+u.Reason)
	}
	if !reflect.DeepEqual(kinds, []string{"MOUNT:parent-router-not-read", "MIDDLEWARE:package-not-followed"}) {
		t.Fatalf("unknowns = %q", kinds)
	}
	// The outbound guards precede no proven-cheap guard, so nothing is flagged.
	if len(r.GuardOrder) != 0 {
		t.Fatalf("guard order = %+v", r.GuardOrder)
	}
}

func TestAMAPV0138Refusals(t *testing.T) {
	for _, tc := range []struct {
		q    Query
		code string
	}{
		{Query{File: "src/missing.ts", Method: "GET", Path: "/"}, "request-path-file-not-found"},
		{Query{File: "src/app.ts", Method: "GET", Path: "/orders"}, "request-path-route-not-found"},
		{Query{File: "src/app.ts", Handler: "nope"}, "request-path-handler-not-found"},
	} {
		_, err := Analyze(tokenExchange, "HEAD", tc.q)
		var e *Error
		if !errors.As(err, &e) || e.Code != tc.code {
			t.Fatalf("%+v: err = %v, want %s", tc.q, err, tc.code)
		}
	}
	dup := memSource{"a.js": "const app = require('express')();\napp.get('/x', a);\napp.get('/x', b);\n"}
	_, err := Analyze(dup, "HEAD", Query{File: "a.js", Method: "GET", Path: "/x"})
	var e *Error
	if !errors.As(err, &e) || e.Code != "request-path-route-ambiguous" {
		t.Fatalf("err = %v", err)
	}
}

func TestAMAPV0134UnresolvedMiddlewareAndReceiver(t *testing.T) {
	src := memSource{"r.ts": "import { router } from './shared';\nrouter.get('/x', makeGuard, cfg.handlers[0]);\n"}
	r := analyze(t, src, Query{File: "r.ts", Method: "GET", Path: "/x"})
	got := []string{}
	for _, u := range r.Unknowns {
		got = append(got, u.Kind+":"+u.Reason)
	}
	if !reflect.DeepEqual(got, []string{"MOUNT:receiver-origin-not-read", "MIDDLEWARE:middleware-not-resolved", "MIDDLEWARE:middleware-expression-not-resolved"}) {
		t.Fatalf("unknowns = %q", got)
	}
	for _, s := range r.Steps {
		if s.Outbound != OutboundUnknown {
			t.Fatalf("step = %+v", s)
		}
	}
}

// TestAMAPV0139Deterministic: the same committed inputs give the same report bytes.
func TestAMAPV0139Deterministic(t *testing.T) {
	for _, q := range []Query{{File: "server.js", Method: "GET", Path: "/users/:id"}} {
		first, _ := json.Marshal(analyze(t, commonJS, q))
		for i := 0; i < 20; i++ {
			again, _ := json.Marshal(analyze(t, commonJS, q))
			if string(again) != string(first) {
				t.Fatalf("run %d differs:\n%s\n%s", i, first, again)
			}
		}
	}
}

// TestAMAPV0137Bounds: a call chain deeper than the bound reads BOUND, not a silent stop.
func TestAMAPV0137Bounds(t *testing.T) {
	var b strings.Builder
	b.WriteString("function f0(req, res) { f1(); }\n")
	for i := 1; i <= maxDepth+2; i++ {
		fmt.Fprintf(&b, "function f%d() { f%d(); }\n", i, i+1)
	}
	r := analyze(t, memSource{"deep.js": b.String()}, Query{File: "deep.js", Handler: "f0"})
	if len(r.Unknowns) == 0 || r.Unknowns[len(r.Unknowns)-1].Kind != UnknownBound || r.Steps[0].Outbound != OutboundUnknown {
		t.Fatalf("unknowns = %+v steps = %+v", r.Unknowns, r.Steps)
	}
}
