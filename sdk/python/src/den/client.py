"""Main client for the Den API."""

from __future__ import annotations

import httpx

from den.sandbox import SandboxManager


class Den:
    """Client for the Den sandbox API.

    Provides access to sandbox management and operations via both
    synchronous and asynchronous interfaces.

    Example (sync)::

        client = Den("http://localhost:8080", api_key="sk-...")
        sandbox = client.sandbox.create(image="python:3.12")
        result = sandbox.exec(["python", "-c", "print('hello')"])
        print(result.stdout)
        sandbox.destroy()
        client.close()

    Example (async)::

        client = Den("http://localhost:8080", api_key="sk-...")
        sandbox = await client.sandbox.acreate(image="python:3.12")
        result = await sandbox.aexec(["python", "-c", "print('hello')"])
        print(result.stdout)
        await sandbox.adestroy()
        await client.aclose()
    """

    def __init__(
        self,
        url: str = "http://localhost:8080",
        *,
        api_key: str | None = None,
        timeout: float = 120.0,
    ) -> None:
        """Initialize the Den client.

        Args:
            url: Base URL of the Den server (e.g. ``http://localhost:8080``).
            api_key: Optional API key for authentication.
            timeout: Request timeout in seconds. Defaults to 120.
        """
        self._url = url.rstrip("/")
        self._api_key = api_key
        self._base_url = f"{self._url}/api/v1"

        headers: dict[str, str] = {}
        if api_key:
            headers["X-API-Key"] = api_key

        self._client = httpx.Client(
            base_url="",
            headers=headers,
            timeout=timeout,
        )
        self._async_client = httpx.AsyncClient(
            base_url="",
            headers=headers,
            timeout=timeout,
        )

        self._sandbox_manager = SandboxManager(
            client=self._client,
            async_client=self._async_client,
            base_url=self._base_url,
        )

    @property
    def sandbox(self) -> SandboxManager:
        """Access the sandbox manager for creating and managing sandboxes."""
        return self._sandbox_manager

    def health(self) -> dict:
        """Check server health (sync).

        Returns:
            Dictionary with server health status.
        """
        resp = self._client.get(f"{self._base_url}/health")
        resp.raise_for_status()
        return resp.json()

    async def ahealth(self) -> dict:
        """Check server health (async).

        Returns:
            Dictionary with server health status.
        """
        resp = await self._async_client.get(f"{self._base_url}/health")
        resp.raise_for_status()
        return resp.json()

    def version(self) -> dict:
        """Get server version info (sync).

        Returns:
            Dictionary with version, commit, and build_date.
        """
        resp = self._client.get(f"{self._base_url}/version")
        resp.raise_for_status()
        return resp.json()

    async def aversion(self) -> dict:
        """Get server version info (async).

        Returns:
            Dictionary with version, commit, and build_date.
        """
        resp = await self._async_client.get(f"{self._base_url}/version")
        resp.raise_for_status()
        return resp.json()

    def close(self) -> None:
        """Close the underlying HTTP clients (sync)."""
        self._client.close()

    async def aclose(self) -> None:
        """Close the underlying HTTP clients (async)."""
        await self._async_client.aclose()

    def __enter__(self) -> Den:
        return self

    def __exit__(self, *args: object) -> None:
        self.close()

    async def __aenter__(self) -> Den:
        return self

    async def __aexit__(self, *args: object) -> None:
        await self.aclose()

    def __repr__(self) -> str:
        return f"Den(url={self._url!r})"
