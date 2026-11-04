// Fixture application under test for the browsercov end-to-end test.
const http = require("http");
const fs = require("fs");
const path = require("path");
const math = require("./server/math");

const port = Number(process.env.PORT || 3123);

http
  .createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/api/add") {
      const sum = math.add(Number(url.searchParams.get("a")), Number(url.searchParams.get("b")));
      res.setHeader("content-type", "application/json");
      res.end(JSON.stringify({ result: sum }));
      return;
    }
    if (url.pathname === "/api/sign") {
      const sign = math.sign(Number(url.searchParams.get("x")));
      res.setHeader("content-type", "application/json");
      res.end(JSON.stringify({ result: sign }));
      return;
    }
    const file = url.pathname === "/" ? "index.html" : url.pathname.slice(1);
    fs.readFile(path.join(__dirname, "public", path.basename(file)), (err, body) => {
      if (err) {
        res.statusCode = 404;
        res.end("not found");
        return;
      }
      res.setHeader("content-type", file.endsWith(".js") ? "text/javascript" : "text/html");
      res.end(body);
    });
  })
  .listen(port, () => console.log(`listening on ${port}`));
