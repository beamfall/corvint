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
        # Publishing is off unless the owner sets the repository variable (P1 review, round 1).
        self.assertIn("    if: vars.RELEASE_PUBLISH_ENABLED == 'true'\n", self.publish)
        self.assertEqual(self.text.count("RELEASE_PUBLISH_ENABLED == 'true'"), 1)
        self.assertEqual(re.findall(r"(?m)^    if: .*$", self.text), ["    if: vars.RELEASE_PUBLISH_ENABLED == 'true'"])
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

    def run_step(self, name, out, tag="corvint-docs/v1.0.1", ref=None, tags=None):
        # The gh stub answers `gh api` for the tag ref and annotated tag objects (already in the
        # --jq form "TYPE SHA"; an empty answer is a failed lookup), logs each api path, and
        # records any other call's arguments.
        stub = out.parent / "bin"
        stub.mkdir(exist_ok=True)
        gh = stub / "gh"
        gh.write_text(textwrap.dedent('''\
            #!/bin/sh
            if [ "$1" = api ]; then
              echo "$2" >> "$GH_API_LOG"
              case "$2" in
                repos/o/r/git/ref/tags/"$TAG") answer=$STUB_REF ;;
                repos/o/r/git/tags/*) eval "answer=\\${STUB_TAG_${2##*/}:-}" ;;
                *) answer= ;;
              esac
              [ -n "$answer" ] || exit 1
              echo "$answer"
              exit 0
            fi
            printf "%s\\n" "$@" > "$GH_ARGS"
            '''))
        gh.chmod(gh.stat().st_mode | stat.S_IXUSR)
        env = {"PATH": "%s:%s" % (stub, os.environ["PATH"]), "TAG": tag, "SHA": "a" * 40,
               "GH_ARGS": str(out.parent / "gh-args"), "GH_API_LOG": str(out.parent / "gh-api"),
               "GH_REPO": "o/r", "STUB_REF": "commit " + "a" * 40 if ref is None else ref,
               "GITHUB_SERVER_URL": "https://github.invalid",
               "GITHUB_REPOSITORY": "o/r", "GITHUB_RUN_ID": "1"}
        for obj, answer in (tags or {}).items():
            env["STUB_TAG_" + obj] = answer
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

    def test_release_refuses_unless_the_remote_tag_names_the_tested_commit(self):
        # P2 review, round 1: the remote tag, peeled through annotated tags, must equal $SHA.
        step = "Create the GitHub release from the stopped run's artifact (GHA-V1-041)"
        tested, other = "a" * 40, "b" * 40
        t1, t2 = "1" * 40, "2" * 40
        nine = {("%d" % i) * 40: "tag " + ("%d" % (i + 1)) * 40 for i in range(1, 9)}
        nine["9" * 40] = "commit " + tested
        for label, ref, tags, ok in (
                ("lightweight at the tested commit", "commit " + tested, {}, True),
                ("annotated at the tested commit", "tag " + t1, {t1: "commit " + tested}, True),
                ("annotated twice at the tested commit", "tag " + t1, {t1: "tag " + t2, t2: "commit " + tested}, True),
                ("moved to another commit", "commit " + other, {}, False),
                ("annotated at another commit", "tag " + t1, {t1: "commit " + other}, False),
                ("names a tree", "tree " + tested, {}, False),
                ("lookup fails", "", {}, False),
                ("annotated object lookup fails", "tag " + t1, {}, False),
                ("deeper than eight tag objects", "tag " + "1" * 40, nine, False)):
            with self.subTest(label), tempfile.TemporaryDirectory() as tmp:
                out = self.fixture(tmp)
                result = self.run_step(step, out, "corvint-docs/v1.0.1", ref, tags)
                created = (out.parent / "gh-args").exists()
                self.assertEqual(result.returncode == 0, ok, result.stdout + result.stderr)
                self.assertEqual(created, ok, "gh release create ran: %s" % created)
                api = (out.parent / "gh-api").read_text().splitlines()
                self.assertEqual(api[0], "repos/o/r/git/ref/tags/corvint-docs/v1.0.1")
                if not ok:
                    self.assertIn("nothing is published", result.stdout)


if __name__ == "__main__":
    unittest.main()
