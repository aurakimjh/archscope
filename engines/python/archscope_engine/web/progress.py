"""Task-id keyed progress + cancellation registry for the FastAPI server.

T-206 commits the engine to a WebSocket-backed progress channel
(``/ws/progress``) and a request-id keyed cancellation API
(``POST /api/analyzer/cancel``). The actual analyzer pipeline in Phase 1
is synchronous in-process so the registry exists primarily as a
contract: the renderer can wire its cancel button before any single
analyzer learns to opt into the cancel pulse, and the same hook works
for longer-running analyzers as soon as they grow a check-pulse step.

The registry is intentionally tiny — no asyncio state machines, just a
threading lock around two maps:

* ``connections``: connection-id → ``WebSocket`` so progress events can
  be fanned out to whichever renderer instance subscribed to a given
  task. Multiple tabs/windows can subscribe to the same task without
  trampling each other.
* ``cancellations``: task-id → ``threading.Event`` so analyzer code can
  poll ``progress_registry.is_cancelled(task_id)`` (or just rely on the
  return value of ``cancel(task_id)`` for renderer feedback today).
"""
from __future__ import annotations

import asyncio
import threading
from typing import Any, Iterable, Optional


class ProgressRegistry:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._connections: dict[str, Any] = {}
        self._subscriptions: dict[str, set[str]] = {}
        self._cancellations: dict[str, threading.Event] = {}

    def attach(self, connection_id: str, websocket: Any) -> None:
        with self._lock:
            self._connections[connection_id] = websocket

    def detach(self, connection_id: str) -> None:
        with self._lock:
            self._connections.pop(connection_id, None)
            for subscribers in self._subscriptions.values():
                subscribers.discard(connection_id)

    def subscribe(self, connection_id: str, task_id: str) -> None:
        with self._lock:
            self._subscriptions.setdefault(task_id, set()).add(connection_id)

    def register_task(self, task_id: str) -> threading.Event:
        with self._lock:
            event = self._cancellations.setdefault(task_id, threading.Event())
            return event

    def cancel(self, task_id: str) -> bool:
        with self._lock:
            event = self._cancellations.get(task_id)
            if event is None:
                event = threading.Event()
                self._cancellations[task_id] = event
            event.set()
            return True

    def is_cancelled(self, task_id: str) -> bool:
        with self._lock:
            event = self._cancellations.get(task_id)
            return event is not None and event.is_set()

    def clear_task(self, task_id: str) -> None:
        with self._lock:
            self._cancellations.pop(task_id, None)
            self._subscriptions.pop(task_id, None)

    def subscribers(self, task_id: str) -> Iterable[Any]:
        with self._lock:
            connection_ids = list(self._subscriptions.get(task_id, set()))
            return [self._connections[cid] for cid in connection_ids if cid in self._connections]

    async def emit(self, task_id: str, payload: dict[str, Any]) -> None:
        """Fan out a progress payload to every renderer subscribed to ``task_id``."""
        sockets = list(self.subscribers(task_id))
        if not sockets:
            return
        await asyncio.gather(*(self._safe_send(ws, payload) for ws in sockets), return_exceptions=True)

    @staticmethod
    async def _safe_send(websocket: Any, payload: dict[str, Any]) -> None:
        try:
            await websocket.send_json(payload)
        except Exception:  # noqa: BLE001 — best-effort fan-out
            return


progress_registry = ProgressRegistry()


__all__ = ["ProgressRegistry", "progress_registry"]
