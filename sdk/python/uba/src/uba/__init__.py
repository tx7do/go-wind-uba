"""GoWind UBA Python 服务端埋点 SDK（零第三方依赖，仅标准库）。"""

from .client import UbaClient
from .config import FlushResult, RiskEvent, SDK_VERSION, TrackOptions, UbaConfig
from .session import SessionManager
from .transport import HttpResult, HttpTransport, UrllibTransport
from .types import ReportEvent
from .utils import (
    fnv1a32,
    merge_properties,
    session_in_sample,
    to_rfc3339_ms,
    uuid_v4,
)

__all__ = [
    "UbaClient",
    "UbaConfig",
    "TrackOptions",
    "RiskEvent",
    "FlushResult",
    "SessionManager",
    "HttpTransport",
    "HttpResult",
    "UrllibTransport",
    "ReportEvent",
    "SDK_VERSION",
    "uuid_v4",
    "to_rfc3339_ms",
    "fnv1a32",
    "session_in_sample",
    "merge_properties",
]

__version__ = SDK_VERSION
