const http = require('node:http');
const fs = require('node:fs');
const crypto = require('node:crypto');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const port = Number(process.argv[2]);
const mode = process.argv[3] || 'good';
const fixture = process.argv[4] || 'fixture-good';
let items = [];
const binding = () => {
  const paths = ['index.html', 'server.cjs'].sort();
  const digests = Object.fromEntries(paths.map(p => [p, hash(fs.readFileSync(p))]));
  return { runId:process.env.CORVINT_FLOW_RUN_ID, fixture,
    frontendDigest:hash(paths.map(p => `${p}\0${digests[p]}\n`).join('')),
    backendDigest:digests['server.cjs'] };
};
const json = (res, value, status = 200) => {
  res.writeHead(status, { 'content-type':'application/json' });
  res.end(JSON.stringify(value));
};
const server = http.createServer(async (req, res) => {
  if (req.url === '/__identity') {
    const value = binding();
    if (mode === 'wrong-identity') value.runId = 'another-run';
    return json(res, value);
  }
  if (req.url === '/__reset' && req.method === 'POST') { items = []; return json(res, { reset:true }); }
  if (req.url === '/__probe') return json(res, { count:items.length, first:items[0]?.name ?? '' });
  if (req.url === '/api/items' && req.method === 'GET') return json(res, items);
  if (req.url === '/api/items' && req.method === 'POST') {
    let body = '';
    for await (const c of req) { body += c; if (body.length > 8192) return json(res, {}, 413); }
    const value = JSON.parse(body);
    if (value.role === 'viewer' && mode !== 'auth-broken') return json(res, { status:'denied' }, 403);
    if (!value.name) return json(res, { status:'invalid' }, 400);
    if (mode !== 'persist-broken') items.push({ name:value.name });
    return json(res, { status:'saved' });
  }
  if (req.url === '/redirect') { res.writeHead(302, { location:'http://localhost:9/blocked' }); return res.end(); }
  res.writeHead(200, { 'content-type':'text/html', 'x-flow-build':binding().frontendDigest });
  res.end(fs.readFileSync('index.html'));
});
server.listen(port, '127.0.0.1');
const stop = () => { server.closeAllConnections(); server.close(() => process.exit(0)); };
process.once('SIGTERM', stop);
process.once('SIGINT', stop);
