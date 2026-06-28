# 数据采集 SDK 接入指南

本文档是 go-wind-uba 埋点 SDK 的**统一接入入口**，覆盖：接入前置准备、SDK 选型、各 SDK 快速上手、上报协议契约。

> 各 SDK 的完整 API 文档见：
> - Web SDK：[`frontend/sdk/web/uba/README.md`](../frontend/sdk/web/uba/README.md)
> - C# SDK：[`sdk/csharp/README.md`](../sdk/csharp/README.md)

---

## 一、接入前置：获取 appId 与 appSecret

所有 SDK 都采用**应用级鉴权**：上报时在请求体携带 `appId` + `appSecret`，无需 token。这两个凭据通过在管理后台创建「UBA 应用」获得。

### 步骤

1. **登录管理后台**（Admin 前端，默认 `http://localhost:5600`）。
2. 进入 **应用管理**（菜单：「数据采集 / 应用管理」），点击新建应用。
3. 填写应用名称、类型、支持平台，保存。
4. 系统为该应用生成三组凭据：
   - `appId`：应用唯一标识（业务用，如 `game_001`，上报时使用）
   - `appKey`：应用 Key
   - `appSecret`：应用密钥（**上报鉴权用，妥善保管，勿提交到公开仓库**）
5. 将应用状态置为 `ON`（启用）。
6. 在 SDK 初始化时填入 `appId` + `appSecret`，即可开始上报。

> `tenantId`（租户 ID）**无需**客户端上报，由服务端根据 appId 权威识别并补全，保证多租户数据隔离的正确性。

---

## 二、SDK 选型

| 你的场景 | 选择 | 包路径 |
|---------|------|--------|
| 浏览器网页 / Node 服务 | **Web SDK（TypeScript）** | `frontend/sdk/web/uba/` |
| Unity 游戏（iOS / Android / PC 原生） | **C# SDK** + `UnityWebRequestTransport` | `sdk/csharp/` |
| Unity WebGL（H5 游戏） | **C# SDK** + `UnityWebRequestTransport`（**必须**，HttpClient 在 WebGL 不可用） | `sdk/csharp/` |
| Godot 4（.NET）桌面 / 移动 | **C# SDK** + 默认 `HttpClientTransport` | `sdk/csharp/` |
| .NET 控制台 / 后台服务 | **C# SDK** + 默认 `HttpClientTransport` | `sdk/csharp/` |

**共同能力**（两端一致）：

- 应用级鉴权（appId + appSecret 在请求体）
- 批量上报（本地缓冲，定时 / 定量自动 flush）
- 高层 API：`track` / `trackRisk` / `identify` / `setSuperProperties` / `flush`
- 自动补全设备 / 会话 / 时间 / 平台信息
- 重试与降级（指数退避，401 鉴权失败不重试）

---

## 三、平台采集差异

不同 SDK 自动采集字段的来源差异（接入时需知晓）：

| 字段 | Web SDK | C# SDK (Unity) | C# SDK (Godot) |
|------|---------|---------------|----------------|
| `deviceId` | localStorage 持久化 | PlayerPrefs 持久化 | 进程级（重启变化） |
| `sessionId` | sessionStorage（标签关闭失效） | 进程级 GUID | 进程级 GUID |
| `platform` | UA 探测（web/ios/...） | 编译宏探测（UNITY_IOS 等） | 固定 `dotnet` |
| `clientInfo.userAgent` | navigator.userAgent | Unity 版本 + 操作系统 | .NET 运行时信息 |
| 卸载兜底 | `sendBeacon`（enableBeacon） | 无（需手动 FlushAsync） | 无（需手动 FlushAsync） |

**注意事项**：
- **Godot 的 deviceId 进程级**：重启后变化，若需稳定设备标识，建议自行持久化（如存配置文件）后通过 `SetSuperProperties` 或自定义 ContextProvider 注入。
- **Unity WebGL 必须用 `UnityWebRequestTransport`**：HttpClient 在 WebGL 平台不可用，会抛异常。
- **客户端卸载兜底**：仅 Web SDK 有 sendBeacon；C# 端需在 `OnApplicationQuit` / `OnDestroy` 调用 `FlushAsync()` 保证最后一批不丢。

---

## 四、Web SDK 快速上手

### 安装

```bash
cd frontend/sdk/web/uba
npm install      # 安装 typescript
npm run build    # 构建 dist/
```

### 初始化与上报

```ts
import { UbaClient } from '@go-wind-uba/uba-sdk';

const uba = UbaClient.init({
  appId: 'your_app_id',
  appSecret: 'your_app_secret',
  endpoint: 'http://localhost:5700', // collector 服务地址
});

// 行为事件（最常用）
uba.track('click', { button: 'buy' }, {
  objectType: 'button',
  objectId: 'btn_buy',
});

// 风险事件
uba.trackRisk('abnormal_click', {
  riskType: 'device_anomaly',
  riskLevel: 'HIGH',
  riskScore: 85,
  description: '短时间内频繁点击，疑似机器操作',
});

// 登录绑定用户（后续事件自动带 userId）
uba.identify(1001);

// 关键转化节点，立即上报
uba.track('purchase', { orderId: 'ORD-001' });
await uba.flush();
```

### 常用配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `batchSize` | `20` | 缓冲达到该数量触发 flush |
| `flushInterval` | `5000` | 定时 flush 间隔（毫秒） |
| `maxRetries` | `3` | 失败最大重试次数 |
| `timeout` | `8000` | 单次请求超时（须 < 服务端 10s） |
| `enableBeacon` | `true` | 页面卸载时用 sendBeacon 兜底 |

> 完整 API 与自动采集字段见 [Web SDK 文档](../frontend/sdk/web/uba/README.md)。

---

## 五、C# SDK 快速上手（Unity / Godot）

### 构建

```bash
cd sdk/csharp/src/Uba.Core
dotnet build -c Release
# 产物：bin/Release/netstandard2.0/Uba.Core.dll
```

### Unity 使用（推荐：便捷组件）

1. 把 `Uba.Core.dll` 拷入 Unity 的 `Assets/Plugins/`，把 `Uba.Unity/*.cs` 拷入 `Assets/Scripts/Uba/`。
2. 场景中创建空 GameObject，挂载 `UbaUnityBehaviour`，配置 endpoint/appId/appSecret。
3. 调用：

```csharp
using Uba.Unity;

UbaUnityBehaviour.Track("level_finish", new() { ["level"] = "1-1" },
    new TrackOptions { Score = 100, DurationMs = 45000 });

UbaUnityBehaviour.Identify(1001);
UbaUnityBehaviour.Track("purchase", new() { ["orderId"] = "ORD-001" },
    new TrackOptions { Amount = "99.90", Quantity = 1 });
```

### Godot 4 使用

```csharp
using Uba;

var client = new UbaClient(new UbaConfig {
    AppId = "your_app_id",
    AppSecret = "your_app_secret",
    Endpoint = "http://localhost:5700",
});
// 默认用 HttpClientTransport + DefaultContextProvider

client.Track("scene_load", new() { ["scene"] = "Main" });
```

> ⚠️ **Unity WebGL 必须用 `UnityWebRequestTransport`**：HttpClient 在 WebGL 平台会抛异常。
> 完整 API、平台选择表、自动采集字段见 [C# SDK 文档](../sdk/csharp/README.md)。

---

## 六、上报协议契约

所有 SDK 对接 collector 服务的统一接口 `POST /uba/v1/report`，两端契约完全一致。

### 鉴权

- `appId` + `appSecret` 放在**请求体**（非 Header），无需 Authorization token。
- 鉴权失败返回 `401`，SDK **不重试**（避免无限刷错误请求）。

### 请求体结构

```jsonc
{
  "appId": "your_app_id",
  "appSecret": "your_app_secret",
  "clientInfo": { "userAgent": "...", "referer": "..." },
  "events": [
    {
      "eventId": "uuid",            // SDK 自动生成，唯一
      "eventName": "click",         // 必填
      "eventTime": "RFC3339",       // SDK 自动补全
      "deviceId": "...",            // SDK 持久化，同设备稳定
      "sessionId": "...",           // 会话级
      "platform": "web",            // SDK 探测
      "userId": 1001,               // identify 后自动带
      "properties": { "button": "buy" },
      "behavior": { "objectType": "button", "objectId": "btn_buy" }
    }
  ]
}
```

### 字段命名规则

- **全 camelCase**（protojson 编码），与后端 proto 契约对齐。
- `tenantId` **不上报**，服务端根据 appId 权威覆盖。

### 响应约定

- HTTP `200` 也可能含**部分失败**：响应体 `failedCount > 0` 或 `errorsByType` 非空时，SDK 记录 warn。
- 错误码：`400` 校验失败、`401` 鉴权失败（不重试）、`500` 服务端错误（重试）。
- 错误体遵循 Kratos error envelope 格式。

### 事件类型（events 元素的 oneof payload）

| 类型 | 字段 | 触发 API |
|------|------|---------|
| 行为事件 | `behavior` + `properties` | `track` / `trackBehavior` |
| 风险事件 | `risk`（riskType / riskLevel / riskScore / description） | `trackRisk` |

### 事件字段全集

`events[]` 元素支持的字段（SDK 自动补全的标注 ✅，业务可主动设置 ✏️）：

| 字段 | 类型 | 来源 | 说明 |
|------|------|------|------|
| `eventId` | string(uuid) | ✅ | 事件唯一 ID，去重键 |
| `eventName` | string | ✏️ **必填** | 事件名，如 `click` / `purchase` |
| `eventTime` | RFC3339 | ✅ | 事件发生时间 |
| `userId` | uint32 | ✏️ | `identify(userId)` 后自动带 |
| `deviceId` | string | ✅ | 设备唯一标识（持久化） |
| `sessionId` | string | ✅ | 会话 ID（会话级） |
| `platform` | string | ✅ | web/ios/android/mini_program/... |
| `tenantId` | uint32 | ❌ 不上报 | 服务端按 appId 权威覆盖 |
| `properties` | map\<string,string\> | ✏️ | 自定义业务属性 |
| `context` | map\<string,string\> | ✏️ | 上下文属性 |
| `metrics` | map\<string,double\> | ✏️ | 数值型指标 |
| `objectType` / `objectId` / `objectName` | string | ✏️ | 行为对象（见 options） |
| `amount` | string | ✏️ | 金额（金额类事件） |
| `quantity` | uint32 | ✏️ | 数量 |
| `score` | int32 | ✏️ | 评分 |
| `durationMs` | uint32 | ✏️ | 时长（毫秒，见 options） |
| `channel` / `appVersion` / `os` / `network` / `country` / `ipCity` | string | ✅/✏️ | 终端/环境信息，部分由 SDK 探测 |
| `clientInfo` | object | ✅ | userAgent / referer 等 |

> 字段命名一律 **camelCase**，与后端 proto 契约对齐。

### 风险事件字段（risk oneof）

| 字段 | 类型 | 说明 |
|------|------|------|
| `riskType` | string | 风险类型，如 `device_anomaly` / `abnormal_click` |
| `riskLevel` | string | 风险等级：`HIGH` / `MEDIUM` / `LOW` |
| `riskScore` | int32 | 风险分（0-100） |
| `description` | string | 风险描述 |

---

## 七、事件 Schema 管理（可选）

如需对上报事件做**字段校验**，可在管理后台「开发者 / 事件 Schema」中登记合法事件名及其属性 schema（属性名 / 类型 / 是否必填）。登记后可用于上报校验，降低脏数据。

> 该功能为可选增强，SDK 上报不强制依赖 Schema 登记。

---

## 八、联调与排错

### 启动本地 collector

```bash
cd backend
go run ./app/collector/service/cmd/server/ -c ./app/collector/service/configs
# 默认监听 HTTP: 5700
```

### Web SDK 联调

1. `cd frontend/sdk/web/uba && npm run build` 生成 `dist/`。
2. 修改 `test.html` 里的 `appId` / `appSecret` / `endpoint`。
3. 浏览器打开，点击按钮触发上报，观察 Network 面板与 Console。

### 常见问题

| 现象 | 排查方向 |
|------|---------|
| 上报返回 401 | appId/appSecret 错误，或应用状态非 `ON`；检查管理后台「应用管理」 |
| 事件未入库但无报错 | 检查响应体 `failedCount`，可能字段校验部分失败；开启 SDK `debug` 查看日志 |
| Unity WebGL 上报失败 | 确认使用 `UnityWebRequestTransport` 而非默认 HttpClient |
| 数据查不到 | 确认 collector → Kafka → core 链路通畅；`tenantId` 由服务端补全，勿手动上报 |
| 页面跳转丢失事件 | 确认 `enableBeacon: true`（默认开启），卸载时用 sendBeacon 兜底 |

---

## 九、进阶场景

### 1. 自定义 Transport / ContextProvider（C#）

核心库通过 `IHttpTransport` 与 `IContextProvider` 抽象，可注入自定义实现：

```csharp
// 自定义 transport（如走游戏网关、加签名、走本地中转）
public class MyTransport : IHttpTransport {
    public Task<HttpResponse> SendAsync(string url, string body, CancellationToken ct) {
        // 加签名头、走自建网关等
    }
}

// 自定义 context（注入业务侧 deviceId/渠道）
public class MyContext : IContextProvider {
    public DeviceContext Get() => new() {
        DeviceId = MyGame.GetPersistentDeviceId(),
        Platform = "android",
    };
}

var client = new UbaClient(config, new MyTransport(), new MyContext());
```

### 2. 公共属性（super properties）

设置后**后续每条事件自动携带**，适合放 appVersion / channel / 渠道号等：

```ts
uba.setSuperProperties({ appVersion: '1.2.0', channel: 'appstore' });
// 清除
uba.clearSuperProperties();
```

### 3. 计时事件（手动测时长）

```ts
const t0 = Date.now();
// ... 用户操作 ...
uba.track('level_play', { level: '1-1' }, { durationMs: Date.now() - t0 });
```

### 4. 与「事件 Schema」配合做上报校验

在管理后台「开发者 / 事件 Schema」登记事件名 + 属性 schema（类型/必填）后，可用于：
- 上报前对齐字段约定（团队协作）
- 服务端校验（降低脏数据，配合后端扩展）

Schema 登记是**可选增强**，不登记也能上报；但登记后能显著提升数据质量。

### 5. 多应用 / 多租户

- 每个应用一套 `appId` + `appSecret`，在管理后台「应用管理」分别创建。
- `tenantId` 由服务端按 appId 识别，**客户端无需关心**，天然隔离。

### 6. 批量与节流调优

| 场景 | 建议配置 |
|------|---------|
| 高频事件（游戏内点击） | `batchSize` 调大（如 50），`flushInterval` 调长，减少请求频次 |
| 低频关键事件（支付） | 上报后立即 `flush()`，不等缓冲 |
| 弱网环境 | 调大 `maxRetries` 与 `retryBaseDelay`，但注意 `timeout < 10000` |

---

## 十、相关文档

- [Web SDK 完整文档](../frontend/sdk/web/uba/README.md)
- [C# SDK 完整文档](../sdk/csharp/README.md)
- [项目总览 README](../README.md)
- [部署文档](../backend/docs/build_deploy.md)
