package testsource

import (
	"context"
	"strings"
	"testing"
)

const roleTest = `import { test, expect } from '@playwright/experimental-ct-react';
import ErrorBanner from '../src/ErrorBanner';

test('shows the error', async ({ mount }) => {
  const component = await mount(<ErrorBanner message="boom" />);
  await expect(component.getByRole('alert')).toHaveText('boom');
});
`

const plainBanner = `export default function ErrorBanner({ message }: { message: string }) {
  return <div className="error">{message}</div>;
}
`

// TestRoleLocatorUnrendered: TSA-V0-207 — role=alert against a component that renders a plain
// element with no role is reported with the component path; one rendering role="alert" is not.
func TestRoleLocatorUnrendered(t *testing.T) {
	report := CheckRoleLocators(map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": plainBanner})
	if report.Checked != 1 || len(report.Findings) != 1 || len(report.Abstentions) != 0 {
		t.Fatalf("report = %+v", report)
	}
	f := report.Findings[0]
	if f.Rule != RuleRoleLocatorUnrendered || f.Path != "tests/banner.spec.tsx" || f.Line != 6 || f.Role != "alert" ||
		len(f.Components) != 1 || f.Components[0] != "src/ErrorBanner.tsx" {
		t.Fatalf("finding = %+v", f)
	}
	for _, markup := range []string{
		`export default function ErrorBanner({ message }) { return <div role="alert">{message}</div>; }`,
		`export default function ErrorBanner({ message }) { return <div role='alert'>{message}</div>; }`,
		`export default function ErrorBanner({ message }) { return <div role={"alert"}>{message}</div>; }`,
	} {
		if r := CheckRoleLocators(map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": markup}); len(r.Findings) != 0 || len(r.Abstentions) != 0 {
			t.Fatalf("%s: %+v", markup, r)
		}
	}
	// Other locator spellings and an implicit-role element.
	statusTest := `import { Toast } from './Toast';
test('t', async ({ mount }) => { const c = await mount(<Toast />); await c.locator('role=status').click(); await c.locator('[role="alert"]').count(); });
`
	r := CheckRoleLocators(map[string]string{"src/toast.test.tsx": statusTest, "src/Toast.tsx": `export const Toast = () => <output>saved</output>;`})
	if r.Checked != 2 || len(r.Findings) != 1 || r.Findings[0].Role != "alert" {
		t.Fatalf("toast = %+v", r)
	}
	// A commented-out locator is not checked; <output/> and <output\n> carry the implicit role,
	// <outputs> does not.
	commented := strings.Replace(roleTest, "  await expect", "  // await expect", 1) + "/*\n * getByRole('alert')\n */\n"
	if r := CheckRoleLocators(map[string]string{"tests/banner.spec.tsx": commented, "src/ErrorBanner.tsx": plainBanner}); r.Checked != 0 {
		t.Fatalf("commented = %+v", r)
	}
	statusOnly := "import { Toast } from './Toast';\ntest('t', async ({ mount }) => { const c = await mount(<Toast />); await c.locator('role=status').click(); });\n"
	for markup, findings := range map[string]int{`export const Toast = () => <output/>;`: 0, "export const Toast = () => <output\n>s</output>;": 0, `export const Toast = () => <outputs>s</outputs>;`: 1} {
		if r := CheckRoleLocators(map[string]string{"src/toast.test.tsx": statusOnly, "src/Toast.tsx": markup}); len(r.Findings) != findings {
			t.Fatalf("%q: %+v", markup, r)
		}
	}
	// A non-notification role is out of scope.
	if r := CheckRoleLocators(map[string]string{"tests/banner.spec.tsx": roleTest + "getByRole('button')\n", "src/ErrorBanner.tsx": plainBanner}); r.Checked != 1 {
		t.Fatalf("button counted: %+v", r)
	}
}

// TestRoleLocatorAbstains: TSA-V0-209 — an unresolvable component, markup that could render a
// role the reader cannot see, and a test that mounts nothing abstain without a finding.
func TestRoleLocatorAbstains(t *testing.T) {
	cases := map[string]struct {
		files  map[string]string
		reason string
		detail string
	}{
		"package import": {map[string]string{"tests/banner.spec.tsx": strings.Replace(roleTest, "'../src/ErrorBanner'", "'@ui/banner'", 1)}, roleComponentUnresolved, ""},
		"missing file":   {map[string]string{"tests/banner.spec.tsx": roleTest}, roleComponentUnresolved, ""},
		"child":          {map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": `export default () => <Notice tone="error" />;`}, roleComponentMarkupOpen, roleMarkupChildComponent},
		"dynamic role":   {map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": `export default ({ r }) => <div role={r}>x</div>;`}, roleComponentMarkupOpen, roleMarkupDynamicRole},
		"spread":         {map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": `export default (p) => <div {...p}>x</div>;`}, roleComponentMarkupOpen, roleMarkupSpreadAttributes},
		"raw html":       {map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": `export default (p) => <div dangerouslySetInnerHTML={p.html} />;`}, roleComponentMarkupOpen, roleMarkupRawHTML},
		"re-export":      {map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": `export { default } from './Banner';`}, roleComponentMarkupOpen, roleMarkupReexport},
		"children":       {map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": `export default ({ children }) => <div>{children}</div>;`}, roleComponentMarkupOpen, roleMarkupChildren},
		"props children": {map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": `export default (props) => <div>{ props.children }</div>;`}, roleComponentMarkupOpen, roleMarkupChildren},
		"not mounted":    {map[string]string{"tests/page.spec.ts": "test('x', async ({ page }) => { await page.getByRole('alert').click(); });\n"}, roleComponentNotMounted, ""},
	}
	for name, c := range cases {
		r := CheckRoleLocators(c.files)
		if len(r.Findings) != 0 || len(r.Abstentions) != 1 || r.Abstentions[0].Reason != c.reason || r.Abstentions[0].Detail != c.detail {
			t.Fatalf("%s: %+v", name, r)
		}
	}
}

// TestRoleLocatorsCommitted: TSA-V0-206 — the command reads the committed revision only.
func TestRoleLocatorsCommitted(t *testing.T) {
	root := t.TempDir()
	commit := commitFiles(t, root, map[string]string{"tests/banner.spec.tsx": roleTest, "src/ErrorBanner.tsx": plainBanner})
	report, err := RoleLocators(OpenRepo(context.Background(), root), RoleLocatorOptions{Revision: "HEAD"})
	if err != nil || report.Revision != commit || len(report.Findings) != 1 {
		t.Fatalf("report = %+v err = %v", report, err)
	}
}
