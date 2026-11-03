// lib
export function add(a: number, b: number): number {
  const s = a + b;
  if (s > 100) {
    return 100;
  }
  return s;
}
export function mul(a: number, b: number): number {
  return a * b;
}
