import http from "node:http";
import { gzipSync, gunzipSync } from "node:zlib";
export async function startReferenceFixture() {
  const marker = {
    request: "archscope-synthetic-request",
    response: "archscope-synthetic-response",
    korean: "합성 본문",
  };
  const server = http.createServer((request, response) => {
    if (request.url === "/redirect") {
      response.writeEarlyHints({ link: "</fixture.css>; rel=preload" });
      response.writeHead(302, { Location: "/echo", "X-Fixture": "redirect" });
      response.end(marker.response);
      return;
    }
    const chunks = [];
    let size = 0;
    request.on("data", (chunk) => {
      size += chunk.length;
      if (size > 65536) {
        request.destroy();
        return;
      }
      chunks.push(chunk);
    });
    request.on("end", () => {
      const body = JSON.stringify({
        marker: marker.response,
        korean: marker.korean,
        received: Buffer.concat(chunks).toString("utf8"),
      });
      response.setHeader("Content-Type", "application/json; charset=utf-8");
      response.setHeader("Set-Cookie", [
        "fixture=synthetic-one; Path=/a; HttpOnly; SameSite=Lax",
        "fixture=synthetic-two; Path=/b; Secure",
      ]);
      response.setHeader("X-Fixture", ["one", "two"]);
      if (request.url === "/gzip") {
        response.setHeader("Content-Encoding", "gzip");
        response.end(gzipSync(body));
        return;
      }
      setTimeout(() => {
        response.statusCode = request.method === "POST" ? 201 : 200;
        response.end(body);
      }, 50);
    });
  });
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  return {
    url: `http://127.0.0.1:${server.address().port}`,
    marker,
    close: () => new Promise((resolve) => server.close(resolve)),
  };
}
export async function exerciseReferenceFixture(fixture) {
  const rows = [];
  for (const entry of [
    { path: "/redirect", method: "GET", body: "" },
    {
      path: "/echo?query=one&query=two",
      method: "POST",
      body: `x=one&x=two&marker=${fixture.marker.request}`,
    },
    { path: "/gzip", method: "GET", body: "" },
  ]) {
    const target = new URL(entry.path, fixture.url);
    if (target.hostname !== "127.0.0.1")
      throw new Error("fixture must stay loopback");
    const row = await new Promise((resolve, reject) => {
      const informational = [];
      const request = http.request(
        target,
        {
          method: entry.method,
          headers: {
            "Content-Type": "application/x-www-form-urlencoded",
            "Content-Length": Buffer.byteLength(entry.body),
          },
        },
        (response) => {
          const chunks = [];
          response.on("data", (chunk) => chunks.push(chunk));
          response.on("error", reject);
          response.on("end", () => {
            const raw = Buffer.concat(chunks),
              decoded =
                response.headers["content-encoding"] === "gzip"
                  ? gunzipSync(raw)
                  : raw;
            resolve({
              path: entry.path,
              method: entry.method,
              status: response.statusCode,
              informational,
              requestBody: entry.body,
              responseBody: decoded.toString("utf8"),
              encodedBytes: raw.length,
              decodedBytes: decoded.length,
              cookies: response.headers["set-cookie"] ?? [],
            });
          });
        },
      );
      request.on("information", (event) =>
        informational.push(event.statusCode),
      );
      request.on("error", reject);
      request.end(entry.body);
    });
    rows.push(row);
  }
  return {
    schemaVersion: 1,
    kind: "synthetic-reference-expectations",
    fixtureOnly: true,
    pid: process.pid,
    rows,
  };
}
