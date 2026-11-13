# SPDX-License-Identifier: Apache-2.0
"""GHA-V1-040, GHA-V1-041 (V1-5236): bind the release-binaries tag workflow and run its publish steps.

Run: python3 -B .github/forward-rows/release-binaries_test.py
No hosted calls: `gh` is a local stub, so this is a workflow extract check, not evidence of a
hosted release run.
"""
import hashlib
import os
import pathlib
import re
import stat
import subprocess
import tempfile
import textwrap
import unittest

WORKFLOW = pathlib.Path(__file__).resolve().parents[1] / "workflows" / "release-binaries.yml"


def job(text, name):
    return re.search(r"(?ms)^  %s:\n(.*?)(?=^  [\w-]+:\n|\Z)" % re.escape(name), text)[1]


def step_script(job_text, step_name):
    step = re.search(r"(?ms)^      - name: %s\n(.*?)(?=^      - |\Z)" % re.escape(step_name), job_text)[1]
    return textwrap.dedent(step.split("        run: |\n", 1)[1])


class ReleaseBinaries(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.text = WORKFLOW.read_text()
        cls.publish = job(cls.text, "publish")

    def test_tag_push_is_the_only_trigger(self):
        on = re.search(r"(?ms)^on:\n(.*?)(?=^\S)", self.text)[1]
        self.assertEqual(on, '  push:\n    tags:\n      - "v*"\n      - "*/v*"\n\n')

    def test_only_publish_writes_and_only_after_the_environment(self):
        self.assertIn("\npermissions:\n  contents: read\n", self.text)
        for name in ("test", "build"):
            self.assertNotIn("permissions:", job(self.text, name), name)
        self.assertEqual(re.findall(r"(?m)^ +contents: (\w+)$", self.text), ["read", "write"])
        self.assertIn("    needs: build\n", self.publish)
        self.assertIn("    environment: release\n", self.publish)
        self.assertIn("    permissions:\n      contents: write\n", self.publish)
        # The write job runs no repository code: no checkout, no go, no make, no script path.
        for forbidden in ("actions/checkout", "go run", "go build", "make ", "script/", "tools/"):
            self.assertNotIn(forbidden, self.publish, forbidden)
        # No job holds id-token: write until the attestation of GHA-V1-040 is decided (D17).
        self.assertIsNone(re.search(r"(?m)^ +id-token:", self.text))

    def test_build_runs_release_binary_after_the_full_test(self):
        self.assertIn("    needs: test\n", job(self.text, "build"))
        self.assertIn('go run ./tools/release-binary release --tag "$TAG" --output "$out"', self.text)
        test = job(self.text, "test")
        for command in ("go test -count=1 -timeout 30m ./...", "go vet ./...",
                        "(cd interop/cem01-go && go test -count=1 -timeout 30m ./... && go vet ./...)"):
            self.assertIn(command, test)

    def test_every_action_is_pinned_by_full_sha(self):
        uses = re.findall(r"(?m)^\s*(?:- )?uses: (\S+)", self.text)
        self.assertTrue(uses)
        for ref in uses:
            self.assertRegex(ref, r"^[\w.-]+/[\w.-]+@[0-9a-f]{40}$")

    def fixture(self, tmp):
        out = pathlib.Path(tmp) / "release"
        out.mkdir()
        rows = []
        for archive in ("corvint-docs_linux_amd64.tar.gz", "corvint-docs_darwin_arm64.tar.gz"):
            (out / archive).write_bytes(archive.encode())
            rows.append("%s  %s\n" % (hashlib.sha256(archive.encode()).hexdigest(), archive))
        (out / "corvint-docs.SHA256SUMS").write_text("".join(rows))
        (out / "SHA256SUMS").write_text("".join(sorted(rows, key=lambda r: r.split()[1])))
        manifest = b'{"profile":"corvint.release-manifest/1"}\n'
        (out / "release-manifest.json").write_bytes(manifest)
        (out / "release-manifest.json.sha256").write_text(
            "%s  release-manifest.json\n" % hashlib.sha256(manifest).hexdigest())
        return out

    def run_step(self, name, out, tag="corvint-docs/v1.0.1"):
        stub = out.parent / "bin"
        stub.mkdir(exist_ok=True)
        gh = stub / "gh"
        gh.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$GH_ARGS"\n')
        gh.chmod(gh.stat().st_mode | stat.S_IXUSR)
        env = {"PATH": "%s:%s" % (stub, os.environ["PATH"]), "TAG": tag, "SHA": "a" * 40,
               "GH_ARGS": str(out.parent / "gh-args"), "GITHUB_SERVER_URL": "https://github.invalid",
               "GITHUB_REPOSITORY": "o/r", "GITHUB_RUN_ID": "1"}
        return subprocess.run(["bash", "--noprofile", "--norc", "-eo", "pipefail", "-c",
                               step_script(self.publish, name)],
                              cwd=out, env=env, capture_output=True, text=True)

    def test_recheck_passes_intact_files_and_refuses_a_changed_archive(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = self.fixture(tmp)
            result = self.run_step("Recheck the artifact's checksums", out)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            (out / "corvint-docs_darwin_arm64.tar.gz").write_bytes(b"changed")
            result = self.run_step("Recheck the artifact's checksums", out)
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_recheck_refuses_a_changed_manifest(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = self.fixture(tmp)
            (out / "release-manifest.json").write_bytes(b"{}\n")
            result = self.run_step("Recheck the artifact's checksums", out)
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_release_uploads_every_file_and_marks_only_prereleases(self):
        step = "Create the GitHub release from the stopped run's artifact (GHA-V1-041)"
        for tag, prerelease in (("v1.0.0", False), ("v1.0.0-rc.1", True),
                                ("corvint-docs/v1.0.1", False), ("corvint-docs/v1.0.1-rc.2", True),
                                ("core/v1.0.0", False)):
            with self.subTest(tag=tag), tempfile.TemporaryDirectory() as tmp:
                out = self.fixture(tmp)
                result = self.run_step(step, out, tag)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                args = (out.parent / "gh-args").read_text().splitlines()
                self.assertEqual(args[:4], ["release", "create", tag, "--verify-tag"])
                self.assertEqual("--prerelease" in args, prerelease)
                files = sorted(a[2:] for a in args if a.startswith("./"))
                self.assertEqual(files, sorted(p.name for p in out.iterdir()))


if __name__ == "__main__":
    unittest.main()
