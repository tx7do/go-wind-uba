"""工具函数：uuid / RFC3339 / FNV 采样哈希（与各端 SDK 算法一致）。"""

from __future__ import annotations

import time
import uuid as uuid_module
from datetime import datetime, timezone
from typing import Dict, Optional


def uuid_v4() -> str:
    return str(uuid_module.uuid4())


def to_rfc3339_ms(dt: Optional[datetime] = None) -> str:
    """UTC 毫秒精度 RFC3339，与其他端 SDK 一致：2026-03-27T12:00:00.123Z。"""
    dt = dt or datetime.now(timezone.utc)
    return dt.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.") + f"{dt.microsecond // 1000:03d}Z"


def now_ms() -> int:
    return int(time.time() * 1000)


def fnv1a32(s: str) -> int:
    """FNV-1a 32 位哈希：按 sessionId 确定性采样。"""
    h = 0x811C9DC5
    for byte in s.encode("utf-8"):
        h ^= byte
        h = (h * 0x01000193) & 0xFFFFFFFF
    return h


def session_in_sample(session_id: str, sample_rate: float) -> bool:
    """会话是否落在采样区间：hash(sessionId) % 1000 / 1000 < sampleRate。"""
    if sample_rate >= 1:
        return True
    if sample_rate <= 0:
        return False
    return (fnv1a32(session_id) % 1000) / 1000.0 < sample_rate


def merge_properties(base: Optional[Dict[str, str]],
                     extra: Optional[Dict[str, str]]) -> Optional[Dict[str, str]]:
    """super 属性在前、调用属性在后（后者覆盖前者）。空则返回 None（wire 跳过）。"""
    if not base and not extra:
        return None
    merged: Dict[str, str] = {}
    if base:
        merged.update(base)
    if extra:
        merged.update(extra)
    return merged


def backoff_delay_ms(base_ms: int, attempt: int) -> int:
    """重试退避：第 attempt 次（从 0 计）= base * 2^attempt，封顶防溢出。"""
    return base_ms * (2 ** min(attempt, 30))
