import { runTests } from "@vscode/test-electron";

try {
  const code = await runTests({
    vscodeExecutablePath: process.env.CORVINT_VSCODE_EXECUTABLE,
    extensionDevelopmentPath: process.env.CORVINT_EXTENSION_DEVELOPMENT_PATH,
    extensionTestsPath: process.env.CORVINT_EXTENSION_TESTS_PATH,
    launchArgs: [
      process.env.CORVINT_FIXTURE_ROOT,
      "--user-data-dir", process.env.CORVINT_VSCODE_USER_DATA,
      "--extensions-dir", process.env.CORVINT_VSCODE_EXTENSIONS,
      "--disable-workspace-trust",
      "--disable-updates",
      "--disable-telemetry",
      "--use-inmemory-secretstorage",
      "--sync", "off",
    ],
  });
  process.exitCode = code;
} catch (error) {
  console.error(error instanceof Error ? error.stack : String(error));
  process.exitCode = 1;
}
