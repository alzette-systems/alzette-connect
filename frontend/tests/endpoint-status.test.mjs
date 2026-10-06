import test from "node:test";
import assert from "node:assert/strict";
import { endpointFreshUntil, endpointStatus } from "../endpoint-status.js";

test("native Date bindings and JSON timestamps expire identically", () => {
  const stamp = "2026-09-06T12:00:00.000Z";
  assert.equal(endpointFreshUntil(new Date(stamp)), stamp);
  assert.equal(endpointFreshUntil(stamp), stamp);
  assert.equal(endpointFreshUntil(new Date("invalid")), "");
  assert.equal(endpointFreshUntil(undefined), "");
  assert.equal(endpointStatus({status: "operational", fresh_until: endpointFreshUntil(new Date(stamp))}, Date.parse(stamp) + 1).status, "unknown");
});

test("fresh endpoint evidence distinguishes health and preserves policy denial", () => {
  const now = Date.parse("2026-09-06T12:00:00Z");
  const endpoint = { status:"operational",callable:true,freshness:"fresh",fresh_until:"2026-09-06T12:01:00Z" };
  assert.equal(endpointStatus(endpoint,now).label,"Available");
  assert.equal(endpointStatus({...endpoint,status:"degraded"},now).label,"Degraded");
  assert.equal(endpointStatus({...endpoint,status:"unavailable",callable:false,freshness:"stale"},now).label,"Unavailable");
  assert.equal(endpointStatus(endpoint,now+61000).label,"Status unknown");
});

test("missing or failed refresh evidence never invents availability", () => {
  assert.equal(endpointStatus(undefined).label,"Status unknown");
  assert.equal(endpointStatus({status:"operational"}).label,"Status unknown");
  assert.equal(endpointStatus({status:"unexpected"}).label,"Status unknown");
  assert.equal(endpointStatus({status:"unknown",freshness:"stale",callable:true}).label,"Status unknown");
});
