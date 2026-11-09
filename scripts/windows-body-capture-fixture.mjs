// Synthetic loopback-only request/response bodies for native Windows T-603.
// Start with: node scripts/windows-body-capture-fixture.mjs <ready.json>
import http from "node:http";
import { fork } from "node:child_process";
import fs from "node:fs";

if (process.argv[2] === "client") {
  const client = process.argv[3];
  process.on("message", ({ id, proxy, target, tag }) => {
    const address = new URL(proxy);
    const requestBody = JSON.stringify({ client, marker: `T603-REQUEST-${client}`, tag, message: "본문 캡처 검증" });
    const request = http.request({
      hostname: address.hostname, port: address.port, path: target, method: "POST", agent: false,
      headers: { Host: new URL(target).host, "Content-Type": "application/json", "Content-Length": Buffer.byteLength(requestBody), "X-ArchScope-Client": client },
    }, response => {
      let responseBody = "";
      response.setEncoding("utf8");
      response.on("data", chunk => responseBody += chunk);
      response.on("end", () => process.send({ id, client, pid: process.pid, status: response.statusCode, requestBody, responseBody }));
    });
    request.on("error", error => process.send({ id, error: String(error) }));
    request.setTimeout(5000, () => request.destroy(new Error("fixture timeout")));
    request.end(requestBody);
  });
  process.on("disconnect", () => process.exit());
} else {
  const origin = http.createServer((request, response) => {
    let body = "";
    request.setEncoding("utf8");
    request.on("data", chunk => body += chunk);
    request.on("end", () => {
      const received = JSON.parse(body);
      response.setHeader("Content-Type", "application/json; charset=utf-8");
      response.setHeader("X-ArchScope-Fixture", "T603-body");
      response.setHeader("X-ArchScope-Client", received.client);
      response.end(JSON.stringify({ client: received.client, marker: `T603-RESPONSE-${received.client}`, received }));
    });
  });
  await new Promise(resolve => origin.listen(0, "127.0.0.1", resolve));
  const children = Object.fromEntries(["A", "B"].map(label => [label, fork(process.argv[1], ["client", label], {
    windowsHide: true, stdio: ["ignore", "ignore", "inherit", "ipc"],
  })]));
  const pending = new Map();
  let sequence = 0;
  for (const child of Object.values(children)) child.on("message", result => {
    pending.get(result.id)?.(result);
    pending.delete(result.id);
  });
  const control = http.createServer(async (request, response) => {
    const url = new URL(request.url, "http://127.0.0.1");
    response.setHeader("Content-Type", "application/json");
    if (url.pathname === "/shutdown") {
      response.end('{"stopped":true}');
      for (const child of Object.values(children)) child.disconnect();
      control.close(); origin.close(); return;
    }
    const client = url.searchParams.get("client");
    const tag = url.searchParams.get("tag") ?? "first";
    const proxy = url.searchParams.get("proxy");
    let validProxy = false;
    try { const parsed = new URL(proxy); validProxy = parsed.protocol === "http:" && parsed.hostname === "127.0.0.1"; } catch {}
    if (!children[client] || !validProxy || !/^[a-z0-9-]+$/.test(tag)) {
      response.statusCode = 400; response.end('{"error":"invalid fixture input"}'); return;
    }
    const id = ++sequence;
    const result = await new Promise(resolve => {
      pending.set(id, resolve);
      children[client].send({ id, proxy, tag, target: `http://127.0.0.1:${origin.address().port}/t603/body/${client.toLowerCase()}-${tag}` });
    });
    response.end(JSON.stringify(result));
  });
  await new Promise(resolve => control.listen(0, "127.0.0.1", resolve));
  const ready = { originPort: origin.address().port, controlPort: control.address().port,
    clients: Object.fromEntries(Object.entries(children).map(([label, child]) => [label, child.pid])) };
  fs.writeFileSync(process.argv[2], JSON.stringify(ready));
  console.log(JSON.stringify(ready));
}
