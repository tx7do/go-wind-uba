# Agent Guide — sdk/flutter/uba

Flutter 埋点采集 SDK（`uba_flutter`）。权威契约在后端 proto：`backend/api/protos/collector/service/v1/i_report.proto` 与 `backend/api/protos/uba/service/v1/{report,behavior_event,risk_event}.proto`；权威接入文档在本目录 `README.md` 与根 `docs/sdk_integration.md`。

## 1. 是什么

面向 Android / iOS / Windows / macOS / Linux 的用户行为埋点采集 SDK，直连 collector `POST /uba/v1/report`。全 camelCase JSON、鉴权在 body、无 gzip、单批 < 1MiB。

## 2. 铁律（改代码前必读）

1. **契约单一真相源在后端 proto。** 本 SDK 的字段名必须与 proto `json_name` 逐一对齐，禁止手造字段。改契约先改 proto，再同步 Web/C#/Flutter 三端。
2. **数值标量只进 behavior oneof。** `durationMs/quantity/score/sessionSeq/serverId/level` 是 proto3 数值（无法区分 unset 0），放在 `ReportEvent` 顶层服务端不认；`types.dart` 的 `toJson` 已固化该行为（顶层无这些键）。
3. **服务端专属字段不发。** `tenantId`/`serverTime` 服务端覆盖，`ip` 服务端取连接信息。金样测试 `test/payload_golden_test.dart` 断言了这些键不存在——改序列化逻辑必须跑它。
4. **默认值与 C# SDK 对齐**：批量 20 / 定时 5000ms / 重试 3 次 / 退避基数 1000ms / 超时 8000ms（< 服务端 10s）。改动默认值需三端同步评估。
5. **重试语义**：4xx（除 429）不重试且**不落盘**（`FlushResult.retryable=false`，重试也不会成功）；网络错误 / 5xx / 429 耗尽重试后回队落盘补发。
6. **依赖最小化**：运行时依赖仅 `flutter` + `shared_preferences`。传输（`HttpTransport`）/存储（`KeyValueStore`）/环境（`ContextProvider`）全部接口化，新平台/新宿主通过实现接口接入，不碰 `client.dart`/`batcher.dart`。

## 3. 常见任务

| 要做的事 | 位置 |
|---|---|
| 改上报载荷/序列化 | `lib/src/types.dart` + `test/payload_golden_test.dart` |
| 改批量/重试/落盘策略 | `lib/src/batcher.dart` + `test/batcher_test.dart` |
| 加自动埋点 | `lib/src/client.dart`（build 路径）或 `lib/src/lifecycle.dart` |
| 加配置项 | `lib/src/config.dart`（默认值注释写清对齐来源） |
| 改协议端点 | `UbaConfig.path/flagsPath`；根路径勿动 |
| 加平台 | 实现 `HttpTransport` + `KeyValueStore`（+可选 `ContextProvider`），在 `create` 里按需装配 |

## 4. 提交前自查

```bash
cd sdk/flutter/uba
flutter pub get
flutter analyze   # 0 issue
flutter test      # 全绿
```

- 触碰了 `types.dart` 的序列化 → 必须确认 `payload_golden_test.dart` 的键集合断言仍准确（多余/缺失字段都会 fail）。
- 触碰了文档示例代码 → 确认与 `example/lib/main.dart` 一致。
- 根目录 `docs/sdk_integration.md`、三语 README 的 SDK 清单如受影响需同步（见根 `AGENTS.md` 的 README 三语同步 checklist）。
