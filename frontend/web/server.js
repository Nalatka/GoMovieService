const http = require("http");
const fs = require("fs");
const path = require("path");

const port = process.env.PORT || 3000;
const roots = {
  "/api/users": process.env.USER_GATEWAY_URL || "http://user-gateway:8081",
  "/api/content": process.env.CONTENT_GATEWAY_URL || "http://content-gateway:8082",
  "/api/streams": process.env.STREAM_GATEWAY_URL || "http://stream-gateway:8083"
};

const types = {
  ".html": "text/html",
  ".css": "text/css",
  ".js": "application/javascript"
};

const server = http.createServer((req, res) => {
  const prefix = Object.keys(roots).find((key) => req.url === key || req.url.startsWith(key + "/") || req.url.startsWith(key + "?"));
  if (prefix) {
    proxy(req, res, prefix);
    return;
  }
  const file = req.url === "/" ? "index.html" : req.url.replace(/^\/+/, "");
  const full = path.join(__dirname, "public", file);
  if (!full.startsWith(path.join(__dirname, "public"))) {
    res.writeHead(403);
    res.end();
    return;
  }
  fs.readFile(full, (err, data) => {
    if (err) {
      res.writeHead(404);
      res.end("not found");
      return;
    }
    res.writeHead(200, {"Content-Type": types[path.extname(full)] || "text/plain"});
    res.end(data);
  });
});

function proxy(req, res, prefix) {
  const target = new URL(roots[prefix]);
  const upstreamPath = req.url.replace(prefix, "");
  const options = {
    hostname: target.hostname,
    port: target.port,
    path: upstreamPath || "/",
    method: req.method,
    headers: {...req.headers, host: target.host}
  };
  const upstream = http.request(options, (upstreamRes) => {
    res.writeHead(upstreamRes.statusCode || 500, upstreamRes.headers);
    upstreamRes.pipe(res);
  });
  upstream.on("error", (err) => {
    res.writeHead(502, {"Content-Type": "application/json"});
    res.end(JSON.stringify({error: err.message}));
  });
  req.pipe(upstream);
}

server.listen(port, () => console.log(`frontend listening on ${port}`));
