'use strict';
// Loopback app for the behavior-events live qualification: /items lists two items and hides the
// empty state, /done ends the walk, /old redirects to /final, /opener opens /popup in a new page.
const http = require('node:http');
const pages = {
  '/items': '<span id="count">2</span><p id="empty" hidden>No items</p><a id="done" href="/done">Done</a><iframe src="/frame"></iframe>',
  '/done': '<p>done</p>',
  '/final': '<p>final</p>',
  '/frame': '<span id="inner">2</span>',
  '/opener': '<a id="open" href="/popup" target="_blank">Open</a>',
  '/popup': '<p>popup</p>',
};
http.createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1');
  if (url.pathname === '/old') { res.writeHead(302, { location: '/final' }); res.end(); return; }
  const body = pages[url.pathname];
  res.writeHead(body ? 200 : 404, { 'content-type': 'text/html' });
  res.end(`<!doctype html><title>${url.pathname}</title>${body || 'missing'}`);
}).listen(Number(process.env.PORT), '127.0.0.1');
