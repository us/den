import type { ClientConfig } from "./types.js";
import { SandboxManager } from "./sandbox.js";

/**
 * Main entry point for the Den SDK.
 *
 * @example
 * ```ts
 * const client = new Den({ url: "http://localhost:8080", apiKey: "my-key" });
 * const sandbox = await client.sandbox.create({ image: "ubuntu:22.04" });
 * const result = await sandbox.exec(["echo", "hello"]);
 * console.log(result.stdout); // "hello\n"
 * await sandbox.destroy();
 * ```
 */
export class Den {
  private readonly _sandbox: SandboxManager;
  private readonly baseUrl: string;
  private readonly headers: Record<string, string>;

  constructor(config: ClientConfig) {
    // Strip trailing slash from URL
    this.baseUrl = config.url.replace(/\/+$/, "");
    this.headers = {};

    if (config.apiKey) {
      this.headers["X-API-Key"] = config.apiKey;
    }

    this._sandbox = new SandboxManager(this.baseUrl, this.headers);
  }

  /** Access sandbox management operations. */
  get sandbox(): SandboxManager {
    return this._sandbox;
  }

  /**
   * Check if the Den server is healthy.
   * @returns True if the server is reachable and healthy.
   */
  async health(): Promise<boolean> {
    try {
      const response = await fetch(`${this.baseUrl}/api/v1/health`, {
        headers: this.headers,
      });
      if (!response.ok) return false;
      const body = (await response.json()) as { status: string };
      return body.status === "ok";
    } catch {
      return false;
    }
  }

  /**
   * Get the server version information.
   * @returns Version, commit hash, and build date.
   */
  async version(): Promise<{
    version: string;
    commit: string;
    build_date: string;
  }> {
    const response = await fetch(`${this.baseUrl}/api/v1/version`, {
      headers: this.headers,
    });
    if (!response.ok) {
      throw new Error(`Failed to fetch version: ${response.status}`);
    }
    return response.json() as Promise<{
      version: string;
      commit: string;
      build_date: string;
    }>;
  }
}
