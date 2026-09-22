export class AsyncTaskTracker {
  private readonly pending = new Set<Promise<unknown>>();
  private closing = false;

  track<T>(task: Promise<T>): Promise<T> {
    if (this.closing) {
      return Promise.reject(new Error("LIFECYCLE_CLOSED"));
    }
    this.pending.add(task);
    void task.finally(() => this.pending.delete(task)).catch(() => undefined);
    return task;
  }

  async close(): Promise<void> {
    this.closing = true;
    while (this.pending.size > 0) {
      await Promise.allSettled([...this.pending]);
    }
  }
}
