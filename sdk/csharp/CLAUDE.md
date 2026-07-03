# CLAUDE.md — go-wind-uba C# SDK

本文件供 Claude / Claude Code 在 `sdk/csharp/` 目录工作时使用。
完整指南见同目录 `AGENTS.md`,两者保持同步;面向最终用户的接入文档见 `README.md`。
下面是 Claude 最该先记住的要点,详细约定请读 `AGENTS.md`。

## 你在哪里

C# 埋点 SDK(`sdk/csharp/`),上报到后端 `collector`(`POST /uba/v1/report`,appId+appSecret 鉴权)。
两个项目:`Uba.Core`(核心,.NET Standard 2.0,零依赖)、`Uba.Unity`(Unity 适配层)。
平台:Unity(原生+WebGL)、Godot 4、通用 .NET。

## 三条铁律

1. **契约以后端 proto 为准。** `Types.cs` 的字段/类型/`oneof` payload 必须对齐
   `backend/api/protos/` 的 collector 契约。后端改了这里同步,不是反过来。字段全 `camelCase`。

2. **零依赖 + netstandard2.0 锁死。** JSON 是 `Json.cs` 手写的,**别引入 Newtonsoft.Json 等**;
   csproj 是 `netstandard2.0`(`LangVersion 9.0`),不要抬到 net6/8。

3. **网络层走抽象。** `IHttpTransport`:默认 `HttpClientTransport`,Unity WebGL **必须**
   `UnityWebRequestTransport`(HttpClient 在 WebGL 抛异常)。新平台加 Transport 实现,别动 Client。

## 关键约束

- 鉴权在 body(appId+appSecret),无 Authorization header;超时 8s(< 服务端 10s);401 不重试。
- 自动采集字段(eventId/eventTime/deviceId/sessionId/platform/userAgent)由 SDK 填,业务层别覆盖;
  tenantId 服务端权威覆盖,无需上报。
- 改字段必查 `Json.cs` 输出的实际 key 是否仍 camelCase 且对齐后端。

## 动手前默认动作

- 改字段/类型 → 先看后端 proto,再改 `Types.cs` + `Json.cs`,确认 camelCase / oneof 正确。
- 新增平台 → 加 `IHttpTransport` 实现,不动 Client/Batcher。
- 加依赖 → 先问能否用 BCL/手写替代,`.csproj` 新增 `PackageReference` 要慎重。
- 改 API → 同步 `README.md`,并注意仓库根 README 的[三语同步 checklist](../../AGENTS.md#readme-三语同步-checklist)
  (C# SDK 接入段在 zh/en/ja 三份 README 都要存在,当前 en/ja 缺失该子段)。

## 构建与自检

```bash
cd sdk/csharp/src/Uba.Core && dotnet build -c Release
# Uba.Unity 依赖 UnityEngine.dll,需在 Unity 内编译,或命令行设 UnityAssemblies 环境变量
```

401 不重试是设计;WebGL 上报失败先确认用的是 `UnityWebRequestTransport`。

详见 `AGENTS.md`。
