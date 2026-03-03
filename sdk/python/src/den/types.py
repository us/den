"""Pydantic models for the Den API."""

from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, Field


class PortMapping(BaseModel):
    """Defines a port forwarding between host and sandbox."""

    sandbox_port: int
    host_port: int
    protocol: str = "tcp"


class SandboxConfig(BaseModel):
    """Configuration for creating a new sandbox."""

    image: str = ""
    env: dict[str, str] | None = None
    workdir: str | None = None
    timeout: int | None = None  # seconds
    cpu: int | None = None  # NanoCPUs (1e9 = 1 core)
    memory: int | None = None  # bytes
    ports: list[PortMapping] | None = None


class SandboxInfo(BaseModel):
    """Information about a sandbox instance."""

    id: str
    image: str = ""
    status: str = ""
    created_at: datetime | None = None
    expires_at: datetime | None = None
    ports: list[PortMapping] | None = None


class ExecResult(BaseModel):
    """Result of a command execution inside a sandbox."""

    exit_code: int
    stdout: str = ""
    stderr: str = ""


class FileInfo(BaseModel):
    """Metadata about a file inside a sandbox."""

    name: str
    path: str = ""
    size: int = 0
    mode: str = ""
    mod_time: datetime | None = None
    is_dir: bool = False


class SnapshotInfo(BaseModel):
    """Metadata about a sandbox snapshot."""

    id: str
    sandbox_id: str = ""
    name: str = ""
    image_id: str = ""
    created_at: datetime | None = None
    size: int = 0


class SandboxStats(BaseModel):
    """Resource usage statistics for a sandbox."""

    cpu_percent: float = 0.0
    memory_usage: int = 0  # bytes
    memory_limit: int = 0  # bytes
    memory_percent: float = 0.0
    network_rx: int = 0  # bytes
    network_tx: int = 0  # bytes
    disk_read: int = 0  # bytes
    disk_write: int = 0  # bytes
    pid_count: int = 0
    timestamp: datetime | None = None


class ExecOpts(BaseModel):
    """Options for executing a command in a sandbox."""

    cmd: list[str]
    env: dict[str, str] | None = None
    workdir: str | None = None
    timeout: int | None = None  # seconds


class ErrorResponse(BaseModel):
    """API error response."""

    error: str = Field(default="")
