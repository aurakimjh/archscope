import assert from "node:assert/strict";
import {
  startReferenceFixture,
  exerciseReferenceFixture,
} from "./http-reference-fixture.mjs";
const fixture = await startReferenceFixture();
try {
  const expected = await exerciseReferenceFixture(fixture);
  assert.equal(expected.fixtureOnly, true);
  assert.equal(expected.rows.length, 3);
  assert.deepEqual(
    expected.rows.map((row) => row.status),
    [302, 201, 200],
  );
  assert.deepEqual(expected.rows[0].informational, [103]);
  assert.match(expected.rows[1].requestBody, /x=one&x=two/);
  assert.match(expected.rows[1].responseBody, /합성 본문/);
  assert.equal(expected.rows[1].cookies.length, 2);
  assert.ok(expected.rows[2].encodedBytes > 0);
  assert.equal(
    expected.rows[2].decodedBytes,
    Buffer.byteLength(expected.rows[2].responseBody),
  );
  assert.equal(
    JSON.parse(expected.rows[2].responseBody).korean,
    fixture.marker.korean,
  );
  console.log("Reference synthetic fixture tests passed");
} finally {
  await fixture.close();
}
