export function assertEquals(a: unknown, b: unknown): void {
  if (a !== b) {
    throw new Error(`${a} !== ${b}`);
  }
}
