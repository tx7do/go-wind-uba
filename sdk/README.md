# 数据采集 SDK

GoWind UBA 各端埋点采集 SDK 的总目录。**怎么选型 / 接入前置 / 上报协议契约**见
[SDK 接入指南](../docs/sdk_integration.md)；第一次接触平台请先跟
[上手教程](../docs/tutorial.md) 走一遍。

## SDK 总览（11 端）

| 目录 | 平台 | 语言/构建 | 包管理 | 本地验证 | 文档 |
|---|---|---|---|---|---|
| [`android/`](android/uba/) | Android 原生 | Kotlin · Gradle（核心层纯 JVM） | — | `gradle test` | [README](android/uba/README.md) |
| [`csharp/`](csharp/) | Unity（原生+WebGL）/ Godot 4 / .NET | C# · netstandard2.0 零依赖 | 独立子项目 | `dotnet test` | [README](csharp/README.md) |
| [`flutter/`](flutter/uba/) | Android / iOS / 桌面 | Dart · pub | `pubspec.yaml` | `flutter test` | [README](flutter/uba/README.md) |
| [`go/`](go/uba/) | Go 服务端 | Go 1.21+ 零依赖 | `go.mod` | `go test -race` | [README](go/uba/README.md) |
| [`harmonyos/`](harmonyos/uba/) | HarmonyOS（鸿蒙） | ArkTS · hvigor library | `oh-package.json5` | DevEco `hvigorw test` | [README](harmonyos/uba/README.md) |
| [`ios/`](ios/uba/) | iOS / macOS | Swift · SPM（仅 Foundation） | `Package.swift` | macOS `swift test` | [README](ios/uba/README.md) |
| [`java/`](java/uba/) | Java 服务端 | Java 11+ 零依赖 | Maven | `mvn test` | [README](java/uba/README.md) |
| [`python/`](python/uba/) | Python 服务端 | Python 3.9+ 零依赖（标准库） | `pyproject.toml` | `unittest` | [README](python/uba/README.md) |
| [`unreal/`](unreal/uba/) | UE 游戏（PC/主机） | C++17 核心 + UE 插件壳 | 插件源码 | `g++` 核心层测试 | [README](unreal/uba/README.md) |
| [`web/`](web/uba/) | 浏览器 / Node | TypeScript · `tsc` 构建，sendBeacon 兜底 | `package.json` | `pnpm typecheck` + `vitest` | [README](web/uba/README.md) |
| [`wechat/`](wechat/uba/) | 微信小程序 / 小游戏 | TypeScript · 零运行时依赖 | `package.json` | `vitest`（wx mock） | [README](wechat/uba/README.md) |

## 共同契约（所有端一致，改动需 11 端同步）

- **协议**：`POST /uba/v1/report`，camelCase 字段逐一对齐后端 proto `json_name`，鉴权在请求 body（`appId`+`appSecret`），无 gzip，单批 < 1MiB，超时 < 服务端 10s
- **服务端专属字段不发送**：`tenantId` / `serverTime` / `ip` 由 collector 按 appId 补全
- **数值标量只进 behavior oneof**：`durationMs`/`quantity`/`score`/`sessionSeq`/`serverId`/`level` 在 proto3 顶层无法区分 unset 0，服务端只信任 oneof 内的值（各端金样测试锁定）
- **重试语义**：4xx（除 429）确定性失败不重试；网络错误/5xx/429 指数退避重试，耗尽后客户端端落盘补发、服务端端内存队列补发

> 契约单一真相源在后端 proto（`backend/api/protos/`）。**改契约先改 proto，再同步全部 SDK**
> ——每端的契约金样测试会在字段漂移时失败。

## 目录约定

每个 SDK 子目录自带三份文档：`README.md`（接入与 API）、`AGENTS.md`（AI 编码助手/新人的
权威约定，含「提交前自查」命令）、`CLAUDE.md`（速记）。进入某子目录工作以该目录 `AGENTS.md` 为准。
