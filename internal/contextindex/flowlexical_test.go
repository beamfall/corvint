package contextindex

import "testing"

// TestFlowWebMaskJSXOperandsAndTypeParameters pins the AFU-V1-043 web mask
// cases of V1-5495..V1-5499: markup nested in an expression container is
// blanked like any other markup text, while object-literal type arguments,
// generic arrow type parameters and a spaced comparison stay code. Each case
// gives the exact masked output, so a blanked declaration line fails it.
func TestFlowWebMaskJSXOperandsAndTypeParameters(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{
			name: "object-literal type arguments (V1-5495)",
			text: "const x = api.post<{ a: { b: string }; c: number }>(url, body)\nconst y = load(1)",
			want: "const x = api.post<{ a: { b: string }; c: number }>(url, body)\nconst y = load(1)",
		},
		{
			name: "markup in a conditional container (V1-5496)",
			text: "return <div>{u ? <a href={u}>Get data</a> : null}</div>\nconst y = load(1)",
			want: "return <div>{u ? <a href={u}>        </a> : null}</div>\nconst y = load(1)",
		},
		{
			name: "markup after && in a container (V1-5497)",
			text: "return <p>{cond && <Panel title=\"x\" />}{cond && <b>Open now</b>} tail</p>\nconst y = load(1)",
			want: "return <p>{cond && <Panel title=    />}{cond && <b>        </b>}     </p>\nconst y = load(1)",
		},
		{
			name: "markup in a mapped container",
			text: "return <ul>{items.map((i) => <li key={i}>{i} item</li>)} more</ul>\nconst y = load(1)",
			want: "return <ul>{items.map((i) => <li key={i}>{i}     </li>)}     </ul>\nconst y = load(1)",
		},
		{
			name: "generic arrow with a trailing comma (V1-5498)",
			text: "const f = <T,>(x: T) => x\nconst y = load(1)",
			want: "const f = <T,>(x: T) => x\nconst y = load(1)",
		},
		{
			name: "generic arrow with extends (V1-5498)",
			text: "const set = <K extends keyof S>(key: K, value: S[K]) =>\n  put(key)\nconst y = load(1)",
			want: "const set = <K extends keyof S>(key: K, value: S[K]) =>\n  put(key)\nconst y = load(1)",
		},
		{
			name: "plain element named like a type parameter",
			text: "const v = <T>x</T>\nconst y = load(1)",
			want: "const v = <T> </T>\nconst y = load(1)",
		},
		{
			name: "spaced comparison before an object (V1-5499)",
			text: "const r = a <b ? {k:1} : c > d\nconst y = load(1)",
			want: "const r = a <b ? {k:1} : c > d\nconst y = load(1)",
		},
		{
			name: "comparison after a call or index",
			text: "const r = f(a) <b ? 1 : c > d\nconst s = xs[0] <b ? 1 : c > d",
			want: "const r = f(a) <b ? 1 : c > d\nconst s = xs[0] <b ? 1 : c > d",
		},
		{
			name: "element after a keyword across a line break",
			text: "return (\n  <p>Get data</p>\n)\nconst y = load(1)",
			want: "return (\n  <p>        </p>\n)\nconst y = load(1)",
		},
		{
			name: "fragment in a conditional container (V1-5524)",
			text: "return <div>{c && (<>\n  <b>Hi</b>\n</>)} x</div>\nconst y = load(1)",
			want: "return <div>{c && (<>\n  <b>  </b>\n</>)}  </div>\nconst y = load(1)",
		},
		{
			name: "fragment at the root of a return (V1-5524)",
			text: "return (<>\n  <p>Get data</p>\n</>)\nconst y = load(1)",
			want: "return (<>\n  <p>        </p>\n</>)\nconst y = load(1)",
		},
	}
	for _, c := range cases {
		if got := flowWebMask(c.text); got != c.want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.name, got, c.want)
		}
	}
}
