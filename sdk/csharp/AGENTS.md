# Agent Guide — go-wind-uba C# SDK(Unity / Godot)

本文件是 AI 编码助手(以及新人)在 `sdk/csharp/` 下工作时的权威指南。
先通读再动手。完整产品说明见同目录 `README.md`(面向最终用户的使用文档)。

## 1. 这是什么

C# 埋点 SDK,对接 **go-wind-uba** 后端的 `collector` 服务(上报端点 `POST /uba/v1/report`,
appId + appSecret 鉴权)。采集终端用户行为,批量上报。目标平台:**Unity**(原生平台 + WebGL)
与 **Godot 4(.NET)**,也可用于通用 .NET 控制台/服务。

## 2. 结构

```
sdk/csharp/
├── README.md                面向用户的接入文档(Unity/Godot 用法、API 速查、平台选择)
└── src/
    ├── Uba.Core/            核心库(.NET Standard 2.0,零依赖)
    │   ├── Types.cs         数据类型(对齐后端 proto 契约)
    │   ├── Client.cs        UbaClient 核心 + 高层 API + IContextProvider
    │   ├── Batcher.cs       缓冲 + 批量合并 + 重试
    │   ├── Transport.cs     IHttpTransport 抽象 + HttpClientTransport 默认实现
    │   ├── Json.cs          零依赖手写 JSON 序列化(camelCase)
    │   ├── Utils.cs         uuid / RFC3339 / 合并 / trimAndLimit
    │   └── Config.cs        配置 + TrackOptions
    └── Uba.Unity/           Unity 适配层(引用 UnityEngine)
        ├── UnityWebRequestTransport.cs   UnityWebRequest 实现(WebGL 必需)
        ├── UnityContextProvider.cs       SystemInfo 设备/平台采集
        └── UbaUnityBehaviour.cs          MonoBehaviour 便捷封装
```

## 3. 设计要点(改代码前必读)

1. **零依赖是硬约束。** `Uba.Core` 不引入任何 NuGet 包——JSON 序列化是 `Json.cs` 手写的(`camelCase`),
   目的是让 Unity 里 DLL 分发干净。**不要为了"方便"引入 Newtonsoft.Json 等。**
2. **.NET Standard 2.0 锁定。** 两个 csproj 都是 `netstandard2.0`(`LangVersion 9.0`,`Nullable enable`),
   这是 Unity/Godot 共同支持面。不要抬到 net6/8。
3. **契约严格对齐后端 proto。** `Types.cs` 的字段名、类型、`oneof` payload 结构必须与
   `backend/api/protos/` 里 collector 上报契约一致;字段全 `camelCase`(由手写 JSON 保证)。
   后端契约改了,这里要同步——而不是反过来。
4. **网络层抽象。** `IHttpTransport` 是抽象点:默认 `HttpClientTransport`(.NET/Godot),
   Unity WebGL **必须**用 `UnityWebRequestTransport`(HttpClient 在 WebGL 抛异常)。
   新增平台支持就加一个 Transport 实现,别动 Client。
5. **鉴权在 body,不在 header。** appId + appSecret 放请求体内,无 `Authorization` header。
6. **超时 8s**(< 服务端 10s),401 不重试,超限丢弃,指数退避——见 `Batcher.cs`。

## 4. 必须遵守的约定

1. **改契约先查后端。** 字段/类型改动以 `backend/api/protos/` collector 契约为准,本 SDK 同步,
   并同步 `Types.cs` 与手写 JSON 的字段输出。
2. **保持零依赖。** 任何新依赖先问"能否用 BCL / 手写实现"。`.csproj` 里新增 `PackageReference` 前必须慎重。
3. **camelCase 不破。** 序列化字段名、JSON key 全 camelCase;改 `Json.cs` 时别破坏既有字段命名。
4. **自动采集字段不要重复上报。** `eventId / eventTime / deviceId / sessionId / platform / clientInfo.userAgent`
   由 SDK 自动填充(见 README"自动采集字段"表),业务层别覆盖,`tenantId` 由服务端权威覆盖(无需上报)。
5. **README 与代码同步。** API 速查、平台选择表、自动采集字段表改动时,`README.md` 要同步更新;
   本 SDK 也受仓库根 README 的[三语同步 checklist](../../AGENTS.md#readme-三语同步-checklist)约束——
   C# SDK 接入段在 zh/en/ja 三份 README 都要存在(当前已知 en/ja 缺失该子段)。
6. **提交前自查:**
   - `dotnet build src/Uba.Core/Uba.Core.csproj -c Release` 通过
   - `Uba.Unity` 因依赖 `UnityEngine.dll`,需在 Unity 项目内编译,或命令行设 `UnityAssemblies` 环境变量;
     CI 外通常只验证 `Uba.Core`。
   - 改了字段/类型,确认 `Json.cs` 的输出与后端契约一致(camelCase、oneof payload 正确)。

## 5. 构建

```bash
cd sdk/csharp/src/Uba.Core
dotnet build -c Release
# 产物:bin/Release/netstandard2.0/Uba.Core.dll
```

Unity 适配层(`Uba.Unity`)依赖 `UnityEngine.dll`:在 Unity 项目内编译,或命令行编译时通过
`UnityAssemblies` 环境变量指向 Unity 安装目录下的 `Managed/UnityEngine.dll`。

## 6. 调试提示

- 上报 401 → appId/appSecret 错;401 不重试是设计,检查配置而非重试逻辑。
- Unity WebGL 上报失败/异常 → 确认用的是 `UnityWebRequestTransport` 而非 `HttpClientTransport`。
- 字段名对不上后端 → 先看 `Json.cs` 输出的实际 key(camelCase),再对后端 proto。
- `deviceId` 不持久(Godot) → 设计如此(Godot 进程级),Unity 走 PlayerPrefs 持久化。

## 7. 与其他 SDK 的关系

- 同一上报契约还有 `frontend/sdk/web/uba/`(浏览器/Node,TS)。改契约时两处都要同步,
   以 `backend/api/protos/` 为唯一真相源。
