'use strict';
// Loopback app for the route-stubs residual live qualification (LPCV-V0-079): / fetches /api/items
// and shows how many items came back (the real /api/items returns three); /ws opens a WebSocket to
// /live and shows the first message. No WebSocket server exists: only a routeWebSocket stub answers.
const http = require('node:http');
const items = '<!doctype html><title>items</title><span id="count">loading</span><script>fetch("/api/items").then(r => r.json()).then(d => { document.getElementById("count").textContent = String(d.items.length); });</script>';
const socket = '<!doctype html><title>socket</title><span id="message">waiting</span><script>const s = new WebSocket("ws://127.0.0.1:4880/live"); s.onmessage = e => { document.getElementById("message").textContent = String(e.data); };</script>';
http.createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1');
  if (url.pathname === '/api/items') { res.writeHead(200, { 'content-type': 'application/json' }); res.end('{"items":[1,2,3]}'); return; }
  if (url.pathname === '/') { res.writeHead(200, { 'content-type': 'text/html' }); res.end(items); return; }
  if (url.pathname === '/ws') { res.writeHead(200, { 'content-type': 'text/html' }); res.end(socket); return; }
  res.writeHead(url.pathname === '/ready' ? 200 : 404); res.end();
}).listen(Number(process.env.PORT), '127.0.0.1');
