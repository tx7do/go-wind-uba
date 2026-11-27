"""GoWind UBA Python 埋点 SDK —— 契约模型与配置。

铁律（与 Web/C#/Flutter/Go/Java/Android/iOS SDK 一致）：
- 字段 camelCase 对齐后端 proto json_name（由 json_wire.py 的键名保证）；
- tenantId / serverTime / ip 服务端补全，SDK 不发送；
- 数值标量（durationMs/quantity/score/sessionSeq/serverId/level）只进 behavior oneof。
"""

from __future__ import annotations

import socket
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

SDK_VERSION = "1.0.0"

# 单批请求体字节上限；服务端对未鉴权大包直接 401，必须压在 1MiB 内
MAX_BATCH_BYTES = 900 * 1024
# 内存队列上限（防持续故障下无界增长）
MAX_QUEUE_SIZE = 10000


@dataclass
class UbaConfig:
    """SDK 配置。默认值与其他端 SDK 对齐：批量 20 条 / 5s 定时 / 3 次重试 / 1s 退避 / 8s 超时。"""

    app_id: str
    app_secret: str
    endpoint: str
    path: str = "/uba/v1/report"
    flags_path: str = "/uba/v1/flags"
    batch_size: int = 20
    flush_interval_ms: int = 5000      # 0 或负数禁用定时器
    max_retries: int = 3               # 总尝试 = max_retries + 1
    timeout_ms: int = 8000             # 必须 < 服务端 10s
    retry_base_delay_ms: int = 1000
    debug: bool = False
    # 服务端实例标识（deviceId 语义），空则自动取 hostname
    device_id: str = ""
    platform: str = "server"
    app_version: str = ""
    channel: str = ""
    session_timeout_ms: int = 0        # 0 = 进程级会话不轮换
    sample_rate: float = 1.0
    max_batch_bytes: int = MAX_BATCH_BYTES
    max_queue_size: int = MAX_QUEUE_SIZE

    def __post_init__(self) -> None:
        if not self.app_id:
            raise ValueError("app_id 不能为空")
        if not self.app_secret:
            raise ValueError("app_secret 不能为空")
        if not self.endpoint:
            raise ValueError("endpoint 不能为空")
        if self.timeout_ms <= 0 or self.timeout_ms >= 10000:
            raise ValueError("timeout_ms 必须 > 0 且 < 10000（服务端超时 10s）")
        if self.max_batch_bytes <= 0 or self.max_batch_bytes >= 1024 * 1024:
            raise ValueError("max_batch_bytes 必须压在 1MiB 以内")
        if self.batch_size <= 0:
            raise ValueError("batch_size 必须 > 0")
        if not 0.0 <= self.sample_rate <= 1.0:
            raise ValueError("sample_rate 必须在 0~1")

    @property
    def base_endpoint(self) -> str:
        return self.endpoint.rstrip("/")

    @property
    def report_url(self) -> str:
        return self.base_endpoint + self.path

    @property
    def flags_url(self) -> str:
        return self.base_endpoint + self.flags_path

    @property
    def resolved_device_id(self) -> str:
        return self.device_id or default_hostname()


@dataclass
class TrackOptions:
    """单次上报维度，字段与其他端 SDK 的 TrackOptions 一一对应（蛇形命名 → wire 转驼峰）。"""

    event_category: Optional[str] = None
    user_id: Optional[int] = None
    device_id: Optional[str] = None
    session_id: Optional[str] = None
    platform: Optional[str] = None
    event_action: Optional[str] = None
    object_type: Optional[str] = None
    object_id: Optional[str] = None
    object_name: Optional[str] = None
    duration_ms: Optional[int] = None
    amount: Optional[str] = None
    quantity: Optional[int] = None
    score: Optional[int] = None
    metrics: Optional[Dict[str, float]] = None
    network: Optional[str] = None
    op_result: Optional[str] = None
    error_code: Optional[str] = None
    properties: Optional[Dict[str, str]] = None
    server_id: Optional[str] = None
    level: Optional[int] = None
    trace_id: Optional[str] = None


@dataclass
class RiskEvent:
    """风险事件载荷（risk oneof）。risk_event_id/occur_time/report_time 服务端补全，无需填写。"""

    risk_type: Optional[str] = None
    risk_level: Optional[str] = None          # HIGH / MEDIUM / LOW
    risk_score: Optional[float] = None        # 0-100
    rule_id: Optional[int] = None
    rule_name: Optional[str] = None
    rule_context: Optional[Dict[str, Any]] = None   # 任意 JSON 对象（proto Struct）
    related_event_ids: Optional[List[str]] = None
    description: Optional[str] = None
    evidence: Optional[Dict[str, str]] = None
    status: Optional[str] = None


@dataclass
class FlushResult:
    """一次 flush 的结果。"""

    success: bool
    sent: int = 0
    dropped: int = 0
    # 失败是否值得补发：网络错误/5xx/429 为 True；4xx 确定性失败为 False
    retryable: bool = False
    message: Optional[str] = None

    def __str__(self) -> str:  # pragma: no cover
        return (
            f"FlushResult(success={self.success}, sent={self.sent}, "
            f"dropped={self.dropped}, retryable={self.retryable}, message={self.message})"
        )


def default_hostname() -> str:
    try:
        host = socket.gethostname()
        return host or "unknown-host"
    except OSError:
        return "unknown-host"
