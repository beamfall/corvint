const DEFAULT_MAX_DEPTH = 32;
const DEFAULT_MAX_NODES = 20_000;

export class JsonValidationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "JsonValidationError";
  }
}

export interface JsonLimits {
  readonly maxBytes: number;
  readonly maxDepth?: number;
  readonly maxNodes?: number;
  readonly maxStringBytes?: number;
}

export type JsonObject = { readonly [key: string]: JsonValue };
export type JsonValue = null | boolean | number | string | JsonValue[] | JsonObject;

export function parseBoundedJson(bytes: Uint8Array, limits: JsonLimits): JsonValue {
  if (!Number.isSafeInteger(limits.maxBytes) || limits.maxBytes < 1) {
    throw new JsonValidationError("invalid JSON byte bound");
  }
  if (bytes.byteLength > limits.maxBytes) {
    throw new JsonValidationError(`JSON exceeds ${limits.maxBytes}-byte bound`);
  }
  let text: string;
  try {
    text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    throw new JsonValidationError("JSON is not valid UTF-8");
  }
  const parser = new BoundedJsonParser(
    text,
    limits.maxDepth ?? DEFAULT_MAX_DEPTH,
    limits.maxNodes ?? DEFAULT_MAX_NODES,
    limits.maxStringBytes ?? 65_536,
  );
  return parser.parse();
}

class BoundedJsonParser {
  private offset = 0;
  private nodes = 0;

  constructor(
    private readonly text: string,
    private readonly maxDepth: number,
    private readonly maxNodes: number,
    private readonly maxStringBytes: number,
  ) {
    if (!Number.isSafeInteger(maxDepth) || maxDepth < 1 || maxDepth > 128) {
      throw new JsonValidationError("invalid JSON depth bound");
    }
    if (!Number.isSafeInteger(maxNodes) || maxNodes < 1) {
      throw new JsonValidationError("invalid JSON node bound");
    }
    if (!Number.isSafeInteger(maxStringBytes) || maxStringBytes < 1) {
      throw new JsonValidationError("invalid JSON string byte bound");
    }
  }

  parse(): JsonValue {
    this.skipWhitespace();
    const value = this.parseValue(0);
    this.skipWhitespace();
    if (this.offset !== this.text.length) {
      this.fail("trailing data");
    }
    return value;
  }

  private parseValue(depth: number): JsonValue {
    if (depth > this.maxDepth) {
      this.fail("depth bound exceeded");
    }
    this.nodes += 1;
    if (this.nodes > this.maxNodes) {
      this.fail("node bound exceeded");
    }
    const token = this.text[this.offset];
    if (token === "{") {
      return this.parseObject(depth + 1);
    }
    if (token === "[") {
      return this.parseArray(depth + 1);
    }
    if (token === '"') {
      return this.parseString();
    }
    if (token === "t") {
      this.expectLiteral("true");
      return true;
    }
    if (token === "f") {
      this.expectLiteral("false");
      return false;
    }
    if (token === "n") {
      this.expectLiteral("null");
      return null;
    }
    if (token === "-" || (token !== undefined && token >= "0" && token <= "9")) {
      return this.parseNumber();
    }
    this.fail("expected value");
  }

  private parseObject(depth: number): JsonObject {
    this.offset += 1;
    this.skipWhitespace();
    const result: Record<string, JsonValue> = Object.create(null) as Record<string, JsonValue>;
    const keys = new Set<string>();
    if (this.take("}")) {
      return result;
    }
    while (true) {
      if (this.text[this.offset] !== '"') {
        this.fail("expected object key");
      }
      const key = this.parseString();
      this.nodes += 1;
      if (this.nodes > this.maxNodes) {
        this.fail("node bound exceeded");
      }
      if (keys.has(key)) {
        this.fail(`duplicate object key ${JSON.stringify(key)}`);
      }
      if (key === "__proto__" || key === "prototype" || key === "constructor") {
        this.fail("unsafe object key");
      }
      keys.add(key);
      this.skipWhitespace();
      if (!this.take(":")) {
        this.fail("expected colon");
      }
      this.skipWhitespace();
      result[key] = this.parseValue(depth);
      this.skipWhitespace();
      if (this.take("}")) {
        return result;
      }
      if (!this.take(",")) {
        this.fail("expected object separator");
      }
      this.skipWhitespace();
    }
  }

  private parseArray(depth: number): JsonValue[] {
    this.offset += 1;
    this.skipWhitespace();
    const result: JsonValue[] = [];
    if (this.take("]")) {
      return result;
    }
    while (true) {
      result.push(this.parseValue(depth));
      this.skipWhitespace();
      if (this.take("]")) {
        return result;
      }
      if (!this.take(",")) {
        this.fail("expected array separator");
      }
      this.skipWhitespace();
    }
  }

  private parseString(): string {
    const start = this.offset;
    this.offset += 1;
    let escaped = false;
    while (this.offset < this.text.length) {
      const code = this.text.charCodeAt(this.offset);
      const character = this.text[this.offset];
      if (character === '"' && !escaped) {
        this.offset += 1;
        const slice = this.text.slice(start, this.offset);
        let decoded: string;
        try {
          decoded = JSON.parse(slice) as string;
        } catch {
          this.fail("invalid string escape");
        }
        validateDecodedString(decoded);
        if (Buffer.byteLength(decoded, "utf8") > this.maxStringBytes) {
          this.fail("string byte bound exceeded");
        }
        return decoded;
      }
      if (!escaped && code < 0x20) {
        this.fail("unescaped control character");
      }
      if (escaped) {
        if (character === "u") {
          const hex = this.text.slice(this.offset + 1, this.offset + 5);
          if (!/^[0-9a-fA-F]{4}$/.test(hex)) {
            this.fail("invalid unicode escape");
          }
          this.offset += 4;
        } else if (character === undefined || !'"\\/bfnrt'.includes(character)) {
          this.fail("invalid string escape");
        }
        escaped = false;
      } else {
        escaped = character === "\\";
      }
      this.offset += 1;
    }
    this.fail("unterminated string");
  }

  private parseNumber(): number {
    const remaining = this.text.slice(this.offset);
    const matched = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/.exec(remaining);
    if (matched === null) {
      this.fail("invalid number");
    }
    const token = matched[0];
    this.offset += token.length;
    const value = Number(token);
    if (!Number.isFinite(value)) {
      this.fail("non-finite number");
    }
    return value;
  }

  private expectLiteral(literal: string): void {
    if (this.text.slice(this.offset, this.offset + literal.length) !== literal) {
      this.fail(`expected ${literal}`);
    }
    this.offset += literal.length;
  }

  private skipWhitespace(): void {
    while (this.offset < this.text.length) {
      const character = this.text[this.offset];
      if (character !== " " && character !== "\n" && character !== "\r" && character !== "\t") {
        return;
      }
      this.offset += 1;
    }
  }

  private take(character: string): boolean {
    if (this.text[this.offset] !== character) {
      return false;
    }
    this.offset += 1;
    return true;
  }

  private fail(message: string): never {
    throw new JsonValidationError(`invalid JSON at offset ${this.offset}: ${message}`);
  }
}

function validateDecodedString(value: string): void {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) {
        throw new JsonValidationError("JSON contains an unpaired high surrogate");
      }
      index += 1;
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      throw new JsonValidationError("JSON contains an unpaired low surrogate");
    }
  }
}

export function isJsonObject(value: JsonValue): value is JsonObject {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
