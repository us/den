import { afterEach, beforeEach, describe, expect, test } from "bun:test";

import { DenError, SandboxManager } from "../src/index.ts";

// The TS SDK feature-token probe is a lazy, scoped capability HINT (not auth):
//   - it fires ONLY when `network_mode` is set on the create config,
//   - it GETs /api/v1/version exactly once and caches the Set,
//   - a server missing the "network_mode" token throws DenError(0, …) BEFORE
//     the create POST is sent.
//
// No real network: we swap globalThis.fetch for a counting stub returning
// standard Web Response objects (Bun's built-in Response).

type Hits = { version: number; create: number };

const realFetch = globalThis.fetch;

function installFetch(features: string[] | null): Hits {
  const hits: Hits = { version: 0, create: 0 };
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    if (url.endsWith("/api/v1/version")) {
      hits.version++;
      const body =
        features === null
          ? { version: "test" } // server predates the feature list
          : { version: "test", features };
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }
    if (url.endsWith("/api/v1/sandboxes")) {
      hits.create++;
      return new Response(
        JSON.stringify({
          id: "sb-1",
          image: "x",
          status: "running",
          created_at: "2026-05-18T00:00:00Z",
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      );
    }
    return new Response("not found", { status: 404 });
  }) as typeof fetch;
  return hits;
}

afterEach(() => {
  globalThis.fetch = realFetch;
});

describe("SandboxManager feature-token probe", () => {
  let mgr: SandboxManager;
  beforeEach(() => {
    mgr = new SandboxManager("http://den.test", {});
  });

  test("network_mode supported ⇒ probe once, then create", async () => {
    const hits = installFetch(["network_mode"]);
    const sb = await mgr.create({ image: "x", network_mode: "none" });
    expect(sb.id).toBe("sb-1");
    expect(hits.version).toBe(1);
    expect(hits.create).toBe(1);
  });

  test("network_mode unsupported ⇒ DenError(0) BEFORE create POST", async () => {
    const hits = installFetch([]);
    let err: unknown;
    try {
      await mgr.create({ image: "x", network_mode: "none" });
    } catch (e) {
      err = e;
    }
    expect(err).toBeInstanceOf(DenError);
    expect((err as DenError).statusCode).toBe(0);
    expect((err as DenError).message).toContain("does not advertise");
    expect(hits.version).toBe(1);
    expect(hits.create).toBe(0); // fail-fast: no sandbox was created
  });

  test("server with no features array ⇒ treated as unsupported", async () => {
    const hits = installFetch(null);
    await expect(
      mgr.create({ image: "x", network_mode: "none" }),
    ).rejects.toBeInstanceOf(DenError);
    expect(hits.version).toBe(1);
    expect(hits.create).toBe(0);
  });

  test("no network_mode ⇒ probe is skipped entirely", async () => {
    const hits = installFetch(["network_mode"]);
    await mgr.create({ image: "x" });
    expect(hits.version).toBe(0);
    expect(hits.create).toBe(1);
  });

  test("probe result is cached across creates", async () => {
    const hits = installFetch(["network_mode"]);
    await mgr.create({ image: "x", network_mode: "none" });
    await mgr.create({ image: "x", network_mode: "none" });
    await mgr.create({ image: "x", network_mode: "none" });
    expect(hits.version).toBe(1); // cached, not re-probed per create
    expect(hits.create).toBe(3);
  });
});
