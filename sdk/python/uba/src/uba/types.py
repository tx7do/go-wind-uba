"""内部事件模型（内存队列的最小单元）。wire dict 由 json_wire.build_event 构造。"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Dict, Optional

from .config import RiskEvent


@dataclass
class ReportEvent:
    """单条上报事件（对应 proto ReportEvent 的 SDK 侧表示）。"""

    event_type: str                       # "BEHAVIOR" / "RISK"
    event_id: str
    event_name: str
    event_time: str                       # RFC3339 毫秒 UTC
    user_id: Optional[int] = None
    device_id: Optional[str] = None
    session_id: Optional[str] = None
    platform: Optional[str] = None
    event_category: Optional[str] = None
    properties: Optional[Dict[str, str]] = None
    metrics: Optional[Dict[str, float]] = None
    trace_id: Optional[str] = None
    # 顶层字符串镜像维度（数值标量不落顶层，只进 behavior）
    event_action: Optional[str] = None
    object_type: Optional[str] = None
    object_id: Optional[str] = None
    object_name: Optional[str] = None
    amount: Optional[str] = None
    op_result: Optional[str] = None
    error_code: Optional[str] = None
    behavior: Optional[Dict[str, Any]] = None   # 已构造好的 oneof 载荷（camelCase）
    risk: Optional[RiskEvent] = None
