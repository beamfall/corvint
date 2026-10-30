'use strict';
// Loopback app for the route-stubs live qualification: / fetches /api/items and shows how many
// items came back; the real /api/items returns three.
const http = require('node:http');
const page = '<!doctype html><title>items</title><span id="count">loading</span><script>fetch("/api/items").then(r => r.json()).then(d => { document.getElementById("count").textContent = String(d.items.length); });</script>';
http.createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1');
  if (url.pathname === '/api/items') { res.writeHead(200, { 'content-type': 'application/json' }); res.end('{"items":[1,2,3]}'); return; }
  if (url.pathname === '/') { res.writeHead(200, { 'content-type': 'text/html' }); res.end(page); return; }
  res.writeHead(url.pathname === '/ready' ? 200 : 404); res.end();
}).listen(Number(process.env.PORT), '127.0.0.1');
