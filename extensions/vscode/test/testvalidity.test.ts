import * as assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import * as path from "node:path";
import { test } from "node:test";
import { project, type ClaimFacts, type ExecutionFacts, type Input, type MutationFacts, type MutationWitness, type Projection } from "../src/testvalidity.js";

const VECTORS_PATH = path.join(__dirname, "..", "..", "..", "..", "conformance", "test-validity-v0", "vectors.json");

interface VectorMutation {
  readonly verdict: string;
  readonly detail: string;
  readonly witness: MutationWitness | null;
}

interface VectorInput {
  readonly claim: ClaimFacts | null;
  readonly execution: ExecutionFacts | null;
  readonly mutation: VectorMutation | null;
}

interface VectorCase {
  readonly name: string;
  readonly input: VectorInput;
  readonly expected: Projection;
}

interface VectorFile {
  readonly cases: readonly VectorCase[];
}

function toInput(input: VectorInput): Input {
  const mutation: MutationFacts | null =
    input.mutation === null
      ? null
      : { verdict: input.mutation.verdict, detail: input.mutation.detail, witness: input.mutation.witness };
  return { claim: input.claim, execution: input.execution, mutation };
}

test("project() matches every shared test-validity vector", async () => {
  const raw = await readFile(VECTORS_PATH, "utf-8");
  const file = JSON.parse(raw) as VectorFile;
  assert.ok(file.cases.length > 0, "vectors.json has no cases");
  for (const testCase of file.cases) {
    const actual = project(toInput(testCase.input));
    assert.deepEqual(actual, testCase.expected, `case ${testCase.name}`);
  }
});
