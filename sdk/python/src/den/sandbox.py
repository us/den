"""Sandbox manager and sandbox instance for the Den API."""

from __future__ import annotations

from typing import Any

import httpx

from den.exceptions import (
    DenError,
    AuthenticationError,
    NotFoundError,
    RateLimitError,
    ValidationError,
)
from den.types import (
    ExecOpts,
    ExecResult,
    FileInfo,
    SandboxConfig,
    SandboxInfo,
    SandboxStats,
    SnapshotInfo,
)


def _raise_for_status(response: httpx.Response) -> None:
    """Raise an appropriate exception for HTTP error responses."""
    if response.status_code < 400:
        return

    # Try to extract error message from JSON response body
    message = f"API error ({response.status_code})"
    try:
        body = response.json()
        if "error" in body:
            message = body["error"]
    except Exception:
        message = response.text or message

    if response.status_code == 400:
        raise ValidationError(message)
    if response.status_code in (401, 403):
        raise AuthenticationError(message)
    if response.status_code == 404:
        raise NotFoundError(message)
    if response.status_code == 429:
        raise RateLimitError(message)
    raise DenError(message, status_code=response.status_code)


class Sandbox:
    """Represents a single sandbox instance and provides operations on it.

    Use this class to execute commands, manage files, take snapshots,
    and monitor resource usage within a sandbox.
    """

    def __init__(
        self,
        info: SandboxInfo,
        client: httpx.Client,
        async_client: httpx.AsyncClient,
        base_url: str,
    ) -> None:
        self._info = info
        self._client = client
        self._async_client = async_client
        self._base_url = f"{base_url}/sandboxes/{info.id}"

    @property
    def id(self) -> str:
        """Return the sandbox ID."""
        return self._info.id

    @property
    def info(self) -> SandboxInfo:
        """Return the sandbox info."""
        return self._info

    def refresh(self) -> SandboxInfo:
        """Refresh and return updated sandbox info (sync)."""
        resp = self._client.get(self._base_url)
        _raise_for_status(resp)
        self._info = SandboxInfo.model_validate(resp.json())
        return self._info

    async def arefresh(self) -> SandboxInfo:
        """Refresh and return updated sandbox info (async)."""
        resp = await self._async_client.get(self._base_url)
        _raise_for_status(resp)
        self._info = SandboxInfo.model_validate(resp.json())
        return self._info

    # -- Exec --

    def exec(
        self,
        cmd: list[str],
        *,
        env: dict[str, str] | None = None,
        workdir: str | None = None,
        timeout: int | None = None,
    ) -> ExecResult:
        """Execute a command inside the sandbox (sync).

        Args:
            cmd: Command and arguments to execute.
            env: Optional environment variables for the command.
            workdir: Optional working directory for the command.
            timeout: Optional timeout in seconds for the command.

        Returns:
            ExecResult with exit_code, stdout, and stderr.
        """
        opts = ExecOpts(cmd=cmd, env=env, workdir=workdir, timeout=timeout)
        payload = opts.model_dump(exclude_none=True)
        resp = self._client.post(f"{self._base_url}/exec", json=payload)
        _raise_for_status(resp)
        return ExecResult.model_validate(resp.json())

    async def aexec(
        self,
        cmd: list[str],
        *,
        env: dict[str, str] | None = None,
        workdir: str | None = None,
        timeout: int | None = None,
    ) -> ExecResult:
        """Execute a command inside the sandbox (async).

        Args:
            cmd: Command and arguments to execute.
            env: Optional environment variables for the command.
            workdir: Optional working directory for the command.
            timeout: Optional timeout in seconds for the command.

        Returns:
            ExecResult with exit_code, stdout, and stderr.
        """
        opts = ExecOpts(cmd=cmd, env=env, workdir=workdir, timeout=timeout)
        payload = opts.model_dump(exclude_none=True)
        resp = await self._async_client.post(f"{self._base_url}/exec", json=payload)
        _raise_for_status(resp)
        return ExecResult.model_validate(resp.json())

    # -- File operations --

    def read_file(self, path: str) -> bytes:
        """Read a file from the sandbox (sync).

        Args:
            path: Absolute path to the file inside the sandbox.

        Returns:
            File contents as bytes.
        """
        resp = self._client.get(
            f"{self._base_url}/files",
            params={"path": path},
        )
        _raise_for_status(resp)
        return resp.content

    async def aread_file(self, path: str) -> bytes:
        """Read a file from the sandbox (async).

        Args:
            path: Absolute path to the file inside the sandbox.

        Returns:
            File contents as bytes.
        """
        resp = await self._async_client.get(
            f"{self._base_url}/files",
            params={"path": path},
        )
        _raise_for_status(resp)
        return resp.content

    def write_file(self, path: str, content: str | bytes) -> None:
        """Write a file to the sandbox (sync).

        Args:
            path: Absolute path to the file inside the sandbox.
            content: File content as string or bytes.
        """
        if isinstance(content, str):
            content = content.encode("utf-8")
        resp = self._client.put(
            f"{self._base_url}/files",
            params={"path": path},
            content=content,
        )
        _raise_for_status(resp)

    async def awrite_file(self, path: str, content: str | bytes) -> None:
        """Write a file to the sandbox (async).

        Args:
            path: Absolute path to the file inside the sandbox.
            content: File content as string or bytes.
        """
        if isinstance(content, str):
            content = content.encode("utf-8")
        resp = await self._async_client.put(
            f"{self._base_url}/files",
            params={"path": path},
            content=content,
        )
        _raise_for_status(resp)

    def list_files(self, path: str = "/") -> list[FileInfo]:
        """List files in a directory inside the sandbox (sync).

        Args:
            path: Absolute path to the directory. Defaults to root.

        Returns:
            List of FileInfo objects.
        """
        resp = self._client.get(
            f"{self._base_url}/files/list",
            params={"path": path},
        )
        _raise_for_status(resp)
        return [FileInfo.model_validate(f) for f in resp.json()]

    async def alist_files(self, path: str = "/") -> list[FileInfo]:
        """List files in a directory inside the sandbox (async).

        Args:
            path: Absolute path to the directory. Defaults to root.

        Returns:
            List of FileInfo objects.
        """
        resp = await self._async_client.get(
            f"{self._base_url}/files/list",
            params={"path": path},
        )
        _raise_for_status(resp)
        return [FileInfo.model_validate(f) for f in resp.json()]

    def mkdir(self, path: str) -> None:
        """Create a directory inside the sandbox (sync).

        Args:
            path: Absolute path to the directory to create.
        """
        resp = self._client.post(
            f"{self._base_url}/files/mkdir",
            params={"path": path},
        )
        _raise_for_status(resp)

    async def amkdir(self, path: str) -> None:
        """Create a directory inside the sandbox (async).

        Args:
            path: Absolute path to the directory to create.
        """
        resp = await self._async_client.post(
            f"{self._base_url}/files/mkdir",
            params={"path": path},
        )
        _raise_for_status(resp)

    def remove_file(self, path: str) -> None:
        """Remove a file or directory from the sandbox (sync).

        Args:
            path: Absolute path to the file or directory to remove.
        """
        resp = self._client.delete(
            f"{self._base_url}/files",
            params={"path": path},
        )
        _raise_for_status(resp)

    async def aremove_file(self, path: str) -> None:
        """Remove a file or directory from the sandbox (async).

        Args:
            path: Absolute path to the file or directory to remove.
        """
        resp = await self._async_client.delete(
            f"{self._base_url}/files",
            params={"path": path},
        )
        _raise_for_status(resp)

    # -- Snapshots --

    def snapshot(self, name: str) -> SnapshotInfo:
        """Create a snapshot of the sandbox (sync).

        Args:
            name: Human-readable name for the snapshot.

        Returns:
            SnapshotInfo with snapshot metadata.
        """
        resp = self._client.post(
            f"{self._base_url}/snapshots",
            json={"name": name},
        )
        _raise_for_status(resp)
        return SnapshotInfo.model_validate(resp.json())

    async def asnapshot(self, name: str) -> SnapshotInfo:
        """Create a snapshot of the sandbox (async).

        Args:
            name: Human-readable name for the snapshot.

        Returns:
            SnapshotInfo with snapshot metadata.
        """
        resp = await self._async_client.post(
            f"{self._base_url}/snapshots",
            json={"name": name},
        )
        _raise_for_status(resp)
        return SnapshotInfo.model_validate(resp.json())

    def list_snapshots(self) -> list[SnapshotInfo]:
        """List all snapshots of the sandbox (sync).

        Returns:
            List of SnapshotInfo objects.
        """
        resp = self._client.get(f"{self._base_url}/snapshots")
        _raise_for_status(resp)
        return [SnapshotInfo.model_validate(s) for s in resp.json()]

    async def alist_snapshots(self) -> list[SnapshotInfo]:
        """List all snapshots of the sandbox (async).

        Returns:
            List of SnapshotInfo objects.
        """
        resp = await self._async_client.get(f"{self._base_url}/snapshots")
        _raise_for_status(resp)
        return [SnapshotInfo.model_validate(s) for s in resp.json()]

    # -- Lifecycle --

    def stop(self) -> None:
        """Stop the sandbox (sync)."""
        resp = self._client.post(f"{self._base_url}/stop")
        _raise_for_status(resp)

    async def astop(self) -> None:
        """Stop the sandbox (async)."""
        resp = await self._async_client.post(f"{self._base_url}/stop")
        _raise_for_status(resp)

    def destroy(self) -> None:
        """Destroy the sandbox permanently (sync)."""
        resp = self._client.delete(self._base_url)
        _raise_for_status(resp)

    async def adestroy(self) -> None:
        """Destroy the sandbox permanently (async)."""
        resp = await self._async_client.delete(self._base_url)
        _raise_for_status(resp)

    # -- Stats --

    def stats(self) -> SandboxStats:
        """Get resource usage statistics for the sandbox (sync).

        Returns:
            SandboxStats with CPU, memory, network, and disk metrics.
        """
        resp = self._client.get(f"{self._base_url}/stats")
        _raise_for_status(resp)
        return SandboxStats.model_validate(resp.json())

    async def astats(self) -> SandboxStats:
        """Get resource usage statistics for the sandbox (async).

        Returns:
            SandboxStats with CPU, memory, network, and disk metrics.
        """
        resp = await self._async_client.get(f"{self._base_url}/stats")
        _raise_for_status(resp)
        return SandboxStats.model_validate(resp.json())

    def __repr__(self) -> str:
        return f"Sandbox(id={self.id!r}, status={self._info.status!r})"


class SandboxManager:
    """Manages sandbox lifecycle: create, list, get, and destroy.

    Accessed via ``Den.sandbox``.
    """

    def __init__(
        self,
        client: httpx.Client,
        async_client: httpx.AsyncClient,
        base_url: str,
    ) -> None:
        self._client = client
        self._async_client = async_client
        self._base_url = f"{base_url}/sandboxes"
        self._api_base_url = base_url

    def _wrap(self, info: SandboxInfo) -> Sandbox:
        """Wrap a SandboxInfo into a Sandbox instance."""
        return Sandbox(
            info=info,
            client=self._client,
            async_client=self._async_client,
            base_url=self._api_base_url,
        )

    def create(self, config: SandboxConfig | None = None, **kwargs: Any) -> Sandbox:
        """Create a new sandbox (sync).

        Args:
            config: Optional SandboxConfig instance.
            **kwargs: Alternatively, pass SandboxConfig fields as keyword arguments.

        Returns:
            A Sandbox instance ready for use.
        """
        if config is None:
            config = SandboxConfig(**kwargs)
        payload = config.model_dump(exclude_none=True)
        resp = self._client.post(self._base_url, json=payload)
        _raise_for_status(resp)
        info = SandboxInfo.model_validate(resp.json())
        return self._wrap(info)

    async def acreate(self, config: SandboxConfig | None = None, **kwargs: Any) -> Sandbox:
        """Create a new sandbox (async).

        Args:
            config: Optional SandboxConfig instance.
            **kwargs: Alternatively, pass SandboxConfig fields as keyword arguments.

        Returns:
            A Sandbox instance ready for use.
        """
        if config is None:
            config = SandboxConfig(**kwargs)
        payload = config.model_dump(exclude_none=True)
        resp = await self._async_client.post(self._base_url, json=payload)
        _raise_for_status(resp)
        info = SandboxInfo.model_validate(resp.json())
        return self._wrap(info)

    def list(self) -> list[Sandbox]:
        """List all sandboxes (sync).

        Returns:
            List of Sandbox instances.
        """
        resp = self._client.get(self._base_url)
        _raise_for_status(resp)
        return [self._wrap(SandboxInfo.model_validate(s)) for s in resp.json()]

    async def alist(self) -> list[Sandbox]:
        """List all sandboxes (async).

        Returns:
            List of Sandbox instances.
        """
        resp = await self._async_client.get(self._base_url)
        _raise_for_status(resp)
        return [self._wrap(SandboxInfo.model_validate(s)) for s in resp.json()]

    def get(self, sandbox_id: str) -> Sandbox:
        """Get a sandbox by ID (sync).

        Args:
            sandbox_id: The sandbox identifier.

        Returns:
            A Sandbox instance.

        Raises:
            NotFoundError: If the sandbox does not exist.
        """
        resp = self._client.get(f"{self._base_url}/{sandbox_id}")
        _raise_for_status(resp)
        info = SandboxInfo.model_validate(resp.json())
        return self._wrap(info)

    async def aget(self, sandbox_id: str) -> Sandbox:
        """Get a sandbox by ID (async).

        Args:
            sandbox_id: The sandbox identifier.

        Returns:
            A Sandbox instance.

        Raises:
            NotFoundError: If the sandbox does not exist.
        """
        resp = await self._async_client.get(f"{self._base_url}/{sandbox_id}")
        _raise_for_status(resp)
        info = SandboxInfo.model_validate(resp.json())
        return self._wrap(info)

    def destroy(self, sandbox_id: str) -> None:
        """Destroy a sandbox by ID (sync).

        Args:
            sandbox_id: The sandbox identifier.

        Raises:
            NotFoundError: If the sandbox does not exist.
        """
        resp = self._client.delete(f"{self._base_url}/{sandbox_id}")
        _raise_for_status(resp)

    async def adestroy(self, sandbox_id: str) -> None:
        """Destroy a sandbox by ID (async).

        Args:
            sandbox_id: The sandbox identifier.

        Raises:
            NotFoundError: If the sandbox does not exist.
        """
        resp = await self._async_client.delete(f"{self._base_url}/{sandbox_id}")
        _raise_for_status(resp)
