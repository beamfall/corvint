import { once } from 'node:events';

const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

export function ownedLifecycle(e, gap, runtime = process) {
  let child, pendingBrowser, closePromise;
  const close = () => {
    if (closePromise) return closePromise;
    closePromise = (async () => {
      const browser = await pendingBrowser?.catch(() => undefined);
      if (browser) {
        try { await browser.close(); e.browserClosed = true; }
        catch { gap(e, 'browser-close-unobserved'); }
      }
      if (!child) return;
      if (child.exitCode === null && child.signalCode === null) {
        const done = once(child, 'exit').catch(() => {});
        child.kill('SIGTERM');
        await Promise.race([done, sleep(700)]);
        if (child.exitCode === null && child.signalCode === null) {
          child.kill('SIGKILL');
          await Promise.race([done, sleep(100)]);
        }
      }
      e.serverExited = child.exitCode !== null || child.signalCode !== null;
      if (!e.serverExited) gap(e, 'server-exit-unobserved');
    })();
    return closePromise;
  };
  const interrupted = () => { void close().finally(() => runtime.exit(130)); };
  runtime.on('SIGINT', interrupted);
  runtime.on('SIGTERM', interrupted);
  return {
    get stopping() { return !!closePromise; },
    ownServer(value) { child = value; },
    launch(start) {
      if (closePromise) throw new Error('observer-stopping');
      pendingBrowser = start();
      return pendingBrowser;
    },
    close,
    dispose() {
      runtime.removeListener('SIGINT', interrupted);
      runtime.removeListener('SIGTERM', interrupted);
    },
  };
}
