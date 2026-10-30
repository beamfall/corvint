// AHI-016 (docs/specs/agent-harness-integration-v0.md, decision 0097): a user
// prompt past the query bound is served by a derived query made only of the
// prompt's explicit anchors, copied verbatim, with a trusted disclosure of that
// derivation. Nothing is stored and no anchor subset is ever chosen. This file is
// the JavaScript twin of cmd/corvint/prompt_bound.go; the anchor-bearing boundary
// cases in conformance/harness-event-v0/common-logical-interaction.json pin both
// to the same task and disclosure bytes, and integrations/host-adapters.test.mjs
// keeps integrations/gemini-cli/hooks/prompt-bound.mjs and
// integrations/opencode/src/prompt-bound.js byte-identical.
const MAX_QUERY_CHARACTERS = 2000;
const MAX_TASK_BYTES = 16384;
const BACKTICK_SPAN = /`[ -_a-~]{1,128}`/g;
const TOKEN = /[A-Za-z0-9_][A-Za-z0-9_./-]*[A-Za-z0-9_]/g;
const PATH_ANCHOR = /^(?:[A-Za-z0-9_.-]+\/)+[A-Za-z0-9_.-]+$|^[A-Za-z0-9_-]{2,}\.[A-Za-z][A-Za-z0-9]{0,7}$/;
const IDENTIFIER = /^[A-Za-z_][A-Za-z0-9_]*$/;
const IDENTIFIER_HOW = /[A-Za-z0-9]_[A-Za-z0-9]|[a-z][A-Z]/;
const REQUIREMENT_ID = /^[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-[0-9]+$/;

// Go strings.TrimSpace trims unicode.IsSpace (the Unicode White_Space set);
// String.prototype.trim differs on U+0085 (Go trims) and U+FEFF (Go keeps).
const GO_SPACE = /[\t\n\v\f\r \u0085\u00A0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000]/;

const characters = (text) => Array.from(text).length;
const utf8Bytes = (text) => new TextEncoder().encode(text).length;

export function overQueryBound(text) {
  return characters(text) > MAX_QUERY_CHARACTERS || utf8Bytes(text) > MAX_TASK_BYTES;
}

/** The prompt with leading and trailing Go strings.TrimSpace whitespace removed. */
export function trimSpace(text) {
  let start = 0;
  let end = text.length;
  while (start < end && GO_SPACE.test(text[start])) start++;
  while (end > start && GO_SPACE.test(text[end - 1])) end--;
  return text.slice(start, end);
}

function lexicalAnchor(token) {
  const identifier = IDENTIFIER.test(token) && IDENTIFIER_HOW.test(token);
  return identifier || PATH_ANCHOR.test(token) || REQUIREMENT_ID.test(token);
}

/**
 * For a trimmed, non-empty prompt: `{task, disclosure: ""}` within the bound,
 * `{task: derivedQuery, disclosure}` over it, or undefined when the prompt must
 * keep the `prompt-over-query-bound` refusal.
 */
export function promptQuery(prompt) {
  if (!overQueryBound(prompt)) {
    return { task: prompt, disclosure: "" };
  }
  const spans = Array.from(prompt.matchAll(BACKTICK_SPAN), (match) => ({
    offset: match.index,
    text: match[0].slice(1, -1),
  }));
  const masked = prompt.replace(BACKTICK_SPAN, (span) => " ".repeat(span.length));
  const tokens = Array.from(masked.matchAll(TOKEN), (match) => ({ offset: match.index, text: match[0] }));
  const anchors = [...spans, ...tokens.filter((token) => lexicalAnchor(token.text))];
  const ordered = anchors.sort((left, right) => left.offset - right.offset);
  const texts = [...new Set(ordered.map((anchor) => anchor.text))];
  const task = texts.join(" ");
  if (task === "" || overQueryBound(task)) {
    return undefined;
  }
  const disclosure =
    `Corvint prompt bound (trusted adapter disclosure): the prompt has ${characters(prompt)} characters and ${utf8Bytes(prompt)} bytes, ` +
    `over the ${MAX_QUERY_CHARACTERS}-character/${MAX_TASK_BYTES}-byte query bound. ` +
    `Retrieval used only a derived query of ${texts.length} distinct explicit anchors (backtick spans, paths, identifiers, requirement IDs) ` +
    `copied verbatim from the prompt, ${task.length} characters long. ` +
    "The rest of the prompt was not queried; this context makes no claim about it and may omit evidence the full prompt would select.\n";
  return { task, disclosure };
}
