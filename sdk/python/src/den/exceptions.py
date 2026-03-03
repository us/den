"""Exceptions for the Den SDK."""

from __future__ import annotations


class DenError(Exception):
    """Base exception for all Den SDK errors."""

    def __init__(self, message: str, status_code: int | None = None) -> None:
        self.message = message
        self.status_code = status_code
        super().__init__(message)


class NotFoundError(DenError):
    """Raised when a resource is not found (404)."""

    def __init__(self, message: str = "Resource not found") -> None:
        super().__init__(message, status_code=404)


class AuthenticationError(DenError):
    """Raised when authentication fails (401/403)."""

    def __init__(self, message: str = "Authentication failed") -> None:
        super().__init__(message, status_code=401)


class RateLimitError(DenError):
    """Raised when rate limit is exceeded (429)."""

    def __init__(self, message: str = "Rate limit exceeded") -> None:
        super().__init__(message, status_code=429)


class ValidationError(DenError):
    """Raised when the request is invalid (400)."""

    def __init__(self, message: str = "Invalid request") -> None:
        super().__init__(message, status_code=400)
