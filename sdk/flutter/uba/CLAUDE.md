# CLAUDE.md — sdk/flutter/uba 速记

- SDK 名 `uba_flutter`，`UbaClient`/`UbaConfig`/`TrackOptions` 与 Web/C# SDK 同名对齐。
- 契约真相源：`backend/api/protos/`（collector i_report + uba report/behavior_event/risk_event）。
- camelCase、鉴权在 body、无 gzip、单批 <1MiB、超时 8s（<服务端 10s）。
- `tenantId`/`serverTime`/`ip` 不发（服务端补）；数值标量只进 behavior oneof。
- 重试：指数退避 1s×2^n×3 次；4xx(除 429) 不重试**不落盘**；可重试耗尽回队落盘补发（cap 500）。
- 会话：uuid，后台 >30s 回前台轮换；采样按 sessionId hash 确定性。
- 依赖仅 `shared_preferences`；transport/store/context 三接口可注入替换。
- 提交前：`flutter pub get && flutter analyze && flutter test`（金样测试锁契约）。
