"""Per-SDK feature-token probe contract (Python, sync + async).

The probe is a lazy, scoped capability HINT (not auth):
  - it fires ONLY when ``network_mode`` is truthy on the resolved config,
  - it GETs ``/api/v1/version`` exactly once and caches the result,
  - a server missing the ``network_mode`` token raises ``DenError`` BEFORE
    the create POST is sent.

httpx is mocked with respx; no real network. Both the ``config=`` and the
``**kwargs`` entrypoints are exercised, including the three "no probe" cases:
``network_mode=None`` kwarg, ``SandboxConfig(network_mode=None)``, and an
unset kwarg.
"""

from __future__ import annotations

import httpx
import pytest
import respx

from den.exceptions import DenError
from den.sandbox import SandboxManager
from den.types import SandboxConfig

API = "http://den.test/api/v1"

_SANDBOX_JSON = {
    "id": "sb-1",
    "image": "x",
    "status": "running",
    "created_at": "2026-05-18T00:00:00Z",
}


def _manager() -> SandboxManager:
    return SandboxManager(
        client=httpx.Client(),
        async_client=httpx.AsyncClient(),
        base_url=API,
    )


def _routes(router: respx.MockRouter, features) -> tuple:
    body = {"version": "test"}
    if features is not None:
        body["features"] = features
    ver = router.get(f"{API}/version").mock(return_value=httpx.Response(200, json=body))
    crt = router.post(f"{API}/sandboxes").mock(return_value=httpx.Response(201, json=_SANDBOX_JSON))
    return ver, crt


# --- sync --------------------------------------------------------------------


@respx.mock
def test_sync_supported_probes_once_then_creates():
    ver, crt = _routes(respx.mock, ["network_mode"])
    sb = _manager().create(network_mode="none")
    assert sb.id == "sb-1"
    assert ver.call_count == 1
    assert crt.call_count == 1


@respx.mock
def test_sync_unsupported_fails_fast_before_create():
    ver, crt = _routes(respx.mock, [])
    with pytest.raises(DenError, match="does not advertise"):
        _manager().create(network_mode="none")
    assert ver.call_count == 1
    assert crt.call_count == 0  # fail-fast: no sandbox created


@respx.mock
def test_sync_missing_features_key_is_unsupported():
    ver, crt = _routes(respx.mock, None)
    with pytest.raises(DenError):
        _manager().create(config=SandboxConfig(image="x", network_mode="none"))
    assert ver.call_count == 1
    assert crt.call_count == 0


@respx.mock
def test_sync_network_mode_none_kwarg_skips_probe():
    ver, crt = _routes(respx.mock, ["network_mode"])
    _manager().create(image="x", network_mode=None)
    assert ver.call_count == 0
    assert crt.call_count == 1


@respx.mock
def test_sync_config_network_mode_none_skips_probe():
    ver, crt = _routes(respx.mock, ["network_mode"])
    _manager().create(config=SandboxConfig(image="x", network_mode=None))
    assert ver.call_count == 0
    assert crt.call_count == 1


@respx.mock
def test_sync_unset_kwarg_skips_probe():
    ver, crt = _routes(respx.mock, ["network_mode"])
    _manager().create(image="x")
    assert ver.call_count == 0
    assert crt.call_count == 1


@respx.mock
def test_sync_probe_cached_across_creates():
    ver, crt = _routes(respx.mock, ["network_mode"])
    mgr = _manager()
    for _ in range(3):
        mgr.create(network_mode="none")
    assert ver.call_count == 1  # cached, not re-probed per create
    assert crt.call_count == 3


# --- async -------------------------------------------------------------------


@respx.mock
async def test_async_supported_probes_once_then_creates():
    ver, crt = _routes(respx.mock, ["network_mode"])
    sb = await _manager().acreate(network_mode="none")
    assert sb.id == "sb-1"
    assert ver.call_count == 1
    assert crt.call_count == 1


@respx.mock
async def test_async_unsupported_fails_fast_before_create():
    ver, crt = _routes(respx.mock, [])
    with pytest.raises(DenError, match="does not advertise"):
        await _manager().acreate(network_mode="none")
    assert ver.call_count == 1
    assert crt.call_count == 0


@respx.mock
async def test_async_network_mode_none_kwarg_skips_probe():
    ver, crt = _routes(respx.mock, ["network_mode"])
    await _manager().acreate(image="x", network_mode=None)
    assert ver.call_count == 0
    assert crt.call_count == 1


@respx.mock
async def test_async_config_network_mode_none_skips_probe():
    ver, crt = _routes(respx.mock, ["network_mode"])
    await _manager().acreate(config=SandboxConfig(image="x", network_mode=None))
    assert ver.call_count == 0
    assert crt.call_count == 1


@respx.mock
async def test_async_probe_cached_across_creates():
    ver, crt = _routes(respx.mock, ["network_mode"])
    mgr = _manager()
    for _ in range(3):
        await mgr.acreate(network_mode="none")
    assert ver.call_count == 1
    assert crt.call_count == 3
