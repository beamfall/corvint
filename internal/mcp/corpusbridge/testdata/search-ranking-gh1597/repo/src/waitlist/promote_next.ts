export function promoteNext(slot: string): void {
  moveFirstWaiting(slot);
}
