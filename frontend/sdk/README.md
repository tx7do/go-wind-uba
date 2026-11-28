# Web 端数据采集 SDK

| 目录 | 平台 | 文档 |
|---|---|---|
| [`web/`](web/uba/) | 浏览器 / Node（TypeScript，`tsc` 构建，sendBeacon 卸载兜底） | [README](web/uba/README.md) |
| [`wechat/`](wechat/uba/) | 微信小程序 / 小游戏（TypeScript，零运行时依赖） | [README](wechat/uba/README.md) |

非 Web 端（移动/桌面/游戏/服务端）的 9 个 SDK 见 [`../../sdk/`](../../sdk/README.md)。

选型、接入前置与上报协议契约见 [SDK 接入指南](../../docs/sdk_integration.md)。
所有端共享同一套上报契约（camelCase / body 鉴权 / 数值标量只进 behavior oneof / 4xx 除 429 不重试）。
