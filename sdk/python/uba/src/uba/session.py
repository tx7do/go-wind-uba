"""会话管理：进程内 uuid；前后台超时轮换（客户端场景）+ 事件间隔超时轮换（服务端场景）。"""

from __future__ import annotations

from typing import Callable, Optional

from .utils import session_in_sample, uuid_v4


class SessionManager:
    """与 Go/Java/Kotlin SDK 语义一致：
    - onBackground/onForeground：后台超过 timeout 后回前台轮换；
    - check_timeout：距上次事件超过 timeout 则轮换（服务端常驻场景）。
    """

    def __init__(self, timeout_ms: int = 30000,
                 now_ms_fn: Optional[Callable[[], int]] = None) -> None:
        self._timeout_ms = timeout_ms
        self._now = now_ms_fn or _default_now
        self._id = uuid_v4()
        self._paused_at: Optional[int] = None
        self._last_event_at: Optional[int] = None

    @property
    def current(self) -> str:
        return self._id

    def in_sample(self, sample_rate: float) -> bool:
        return session_in_sample(self._id, sample_rate)

    def on_background(self) -> None:
        if self._paused_at is None:
            self._paused_at = self._now()

    def on_foreground(self) -> None:
        paused = self._paused_at
        self._paused_at = None
        if paused is not None and self._timeout_ms > 0 and self._now() - paused > self._timeout_ms:
            self.rotate()

    def check_timeout(self) -> str:
        """事件间隔超时轮换：返回当前（可能已轮换的）会话 ID。"""
        now = self._now()
        last = self._last_event_at
        if last is not None and self._timeout_ms > 0 and now - last > self._timeout_ms:
            self.rotate()
        self._last_event_at = now
        return self._id

    def rotate(self) -> None:
        self._id = uuid_v4()
        self._paused_at = None


def _default_now() -> int:
    import time
    return int(time.time() * 1000)
