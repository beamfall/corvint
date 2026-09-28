const http=require('node:http'),fs=require('node:fs'),crypto=require('node:crypto');
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const paths=['index.html','server.cjs'].sort(),sources=Object.fromEntries(paths.map(p=>[p,hash(fs.readFileSync(p))]));
const identity={runId:process.env.CORVINT_FLOW_RUN_ID,fixture:'navigation',frontendDigest:hash(paths.map(p=>`${p}\0${sources[p]}\n`).join('')),backendDigest:sources['server.cjs']};
const mode=process.argv[3]||'good';
const server=http.createServer(async(req,res)=>{
 const json=v=>{res.writeHead(200,{'content-type':'application/json'});res.end(JSON.stringify(v));};
 if(req.url==='/identity')return json(identity);
 if(req.url==='/reset'||req.url==='/write'){fs.appendFileSync('writes.log',req.url+'\n');return json({saved:true});}
 if(req.url==='/redirect'){res.writeHead(302,{location:'http://127.0.0.1:9/outside'});return res.end();}
 if(mode==='slow'){setTimeout(()=>{res.writeHead(200);res.end('late');},10000);return;}
 res.writeHead(200,{'content-type':'text/html'});res.end(fs.readFileSync('index.html','utf8').replace('FIXTURE_MODE',mode));
});
server.listen(Number(process.argv[2]),'127.0.0.1');
const stop=()=>{server.closeAllConnections();server.close(()=>process.exit(0));};process.once('SIGINT',stop);process.once('SIGTERM',stop);
