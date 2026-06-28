# UBA C# SDK（Unity / Godot）

C# 埋点 SDK，对接 **go-wind-uba** collector 服务（`POST /uba/v1/report`，appId+appSecret 鉴权）。

支持 Unity（原生平台 + WebGL）与 Godot 4（.NET）。

## 结构

```
sdk/csharp/src/
├── Uba.Core/             # 核心库（.NET Standard 2.0，零依赖）
│   ├── Types.cs          # 数据类型（对齐 proto 契约）
│   ├── Client.cs         # UbaClient 核心 + 高层 API + IContextProvider
│   ├── Batcher.cs        # 缓冲 + 批量合并 + 重试
│   ├── Transport.cs      # IHttpTransport 抽象 + HttpClientTransport 默认实现
│   ├── Json.cs           # 零依赖手写 JSON 序列化（camelCase）
│   ├── Utils.cs          # uuid / RFC3339 / 合并 / trimAndLimit
│   └── Config.cs         # 配置 + TrackOptions
└── Uba.Unity/            # Unity 适配层（引用 UnityEngine）
    ├── UnityWebRequestTransport.cs   # UnityWebRequest 实现（WebGL 必需）
    ├── UnityContextProvider.cs       # SystemInfo 设备/平台采集
    └── UbaUnityBehaviour.cs          # MonoBehaviour 便捷封装
```

## 特性

- ✅ **应用级鉴权**：appId + appSecret（请求体内）
- ✅ **批量上报**：本地缓冲，定时/定量 flush
- ✅ **高层 API**：`Track` / `TrackRisk` / `Identify` / `SetSuperProperties` / `FlushAsync`
- ✅ **重试降级**：指数退避，401 不重试，超限丢弃
- ✅ **Unity WebGL 兼容**：网络层抽象（`IHttpTransport`），WebGL 用 `UnityWebRequestTransport`
- ✅ **零依赖**：核心库手写 JSON，无 NuGet 依赖，DLL 分发干净

## Unity 使用

### 方式一：便捷组件（推荐）

1. 把 `Uba.Core/bin/Release/netstandard2.0/Uba.Core.dll` 拷入 Unity 的 `Assets/Plugins/`
2. 把 `Uba.Unity/*.cs` 拷入 `Assets/Scripts/Uba/`
3. 在场景中创建空 GameObject，挂载 `UbaUnityBehaviour`，配置 endpoint/appId/appSecret
4. 调用：

```csharp
using Uba.Unity;

// 任意脚本中
UbaUnityBehaviour.Track("level_finish", new() { ["level"] = "1-1" },
    new TrackOptions { Score = 100, DurationMs = 45000 });

UbaUnityBehaviour.Identify(1001);
UbaUnityBehaviour.Track("purchase", new() { ["orderId"] = "ORD-001" },
    new TrackOptions { Amount = "99.90", Quantity = 1 });
```

### 方式二：手动初始化（适合非 MonoBehaviour 场景）

```csharp
using Uba;
using Uba.Unity;

var client = new UbaClient(new UbaConfig {
    AppId = "demo_app_001",
    AppSecret = "demo_secret_123456",
    Endpoint = "http://localhost:5700",
}, new UnityWebRequestTransport(this), new UnityContextProvider());

client.Track("click", new() { ["button"] = "buy" });
```

> **注意**：`UnityWebRequestTransport` 构造需要传一个 `MonoBehaviour`（用于启动协程）。

## Godot 4 使用

Godot 4（.NET）支持 `HttpClient`，直接用核心库即可：

```csharp
using Uba;

var client = new UbaClient(new UbaConfig {
    AppId = "demo_app_001",
    AppSecret = "demo_secret_123456",
    Endpoint = "http://localhost:5700",
});
// 默认用 HttpClientTransport + DefaultContextProvider

client.Track("scene_load", new() { ["scene"] = "Main" });
```

## 平台选择指南

| 环境 | Transport | 说明 |
|------|-----------|------|
| Unity 原生（iOS/Android/PC） | `UnityWebRequestTransport` 或 `HttpClientTransport` | 两者都可用，推荐前者 |
| **Unity WebGL** | **`UnityWebRequestTransport`** | HttpClient 在 WebGL 抛异常，**必须**用前者 |
| Godot 4 桌面/移动 | `HttpClientTransport`（默认） | 直接用 |
| .NET 控制台/服务 | `HttpClientTransport`（默认） | 直接用 |

## API 速查

```csharp
client.Track("eventName", properties, options);          // 行为事件
client.TrackBehavior("eventName", properties, options);  // 显式行为
client.TrackRisk("eventName", riskEvent, options);       // 风险事件
client.Identify(1001);                                   // 设置用户
client.SetSuperProperties(new() { ["ver"] = "1.0" });    // 公共属性
await client.FlushAsync();                               // 手动上报
client.PendingCount;                                     // 队列长度
```

## 自动采集字段

| 字段 | 来源 |
|------|------|
| `eventId` | GUID 自动生成 |
| `eventTime` | 当前 UTC，RFC3339 |
| `deviceId` | Unity: PlayerPrefs 持久化 / Godot: 进程级 |
| `sessionId` | 进程级 GUID |
| `platform` | Unity: 编译宏探测 / Godot: dotnet |
| `clientInfo.userAgent` | Unity 版本 + 操作系统 |

## 构建

```bash
cd sdk/csharp/src/Uba.Core
dotnet build -c Release
# 产物：bin/Release/netstandard2.0/Uba.Core.dll
```

> Unity 适配层（`Uba.Unity`）依赖 `UnityEngine.dll`，需在 Unity 项目内编译，或在命令行编译时通过 `UnityAssemblies` 环境变量指定 Unity 安装路径下的 `Managed/UnityEngine.dll`。

## 契约约束

- 字段名全 camelCase（核心库手写 JSON 序列化保证）
- 鉴权在 body（appId+appSecret），无 Authorization header
- 必填：eventId / eventName / eventTime + 对应 oneof payload
- tenantId 无需上报（服务端权威覆盖）
- 超时 8s（< 服务端 10s）
