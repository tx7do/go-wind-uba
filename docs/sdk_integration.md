# 数据采集 SDK 接入指南

本文档是 go-wind-uba 埋点 SDK 的**统一接入入口**（字典式参考），覆盖：接入前置准备、SDK 选型、各 SDK 快速上手、上报协议契约。
> 🎓 **第一次接触本平台？** 跟着 [上手教程](tutorial.md) 走一遍（约 15 分钟），从启动服务到在管理后台看见你的第一条事件。

> 各 SDK 的完整 API 文档见：
> - Web SDK：[`frontend/sdk/web/uba/README.md`](../frontend/sdk/web/uba/README.md)
> - C# SDK：[`sdk/csharp/README.md`](../sdk/csharp/README.md)
> - Flutter SDK：[`sdk/flutter/uba/README.md`](../sdk/flutter/uba/README.md)
> - 微信小程序/小游戏 SDK：[`frontend/sdk/wechat/uba/README.md`](../frontend/sdk/wechat/uba/README.md)
> - Go 服务端 SDK：[`sdk/go/uba/README.md`](../sdk/go/uba/README.md)
> - Java 服务端 SDK：[`sdk/java/uba/README.md`](../sdk/java/uba/README.md)
> - Android SDK：[`sdk/android/uba/README.md`](../sdk/android/uba/README.md)
> - iOS/macOS SDK：[`sdk/ios/uba/README.md`](../sdk/ios/uba/README.md)
> - Python SDK：[`sdk/python/uba/README.md`](../sdk/python/uba/README.md)
> - Unreal SDK：[`sdk/unreal/uba/README.md`](../sdk/unreal/uba/README.md)
> - HarmonyOS SDK：[`sdk/harmonyos/uba/README.md`](../sdk/harmonyos/uba/README.md)

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
| Flutter App / 游戏（Android / iOS / 桌面） | **Flutter SDK** | `sdk/flutter/uba/` |
| 微信小程序 / 小游戏 | **微信 SDK（TypeScript）** | `frontend/sdk/wechat/uba/` |
| Go 服务端（支付/订单/风控事件源） | **Go SDK** | `sdk/go/uba/` |
| Java 服务端（订单/支付/风控系统） | **Java SDK** | `sdk/java/uba/` |
| Android 原生 App | **Android SDK（Kotlin）** | `sdk/android/uba/` |
| iOS / macOS 原生 App | **iOS SDK（Swift）** | `sdk/ios/uba/` |
| Python 脚本 / 自动化任务 | **Python SDK** | `sdk/python/uba/` |
| Unreal 游戏（PC / 主机） | **Unreal SDK（C++）** | `sdk/unreal/uba/` |
| HarmonyOS（鸿蒙）App | **HarmonyOS SDK（ArkTS）** | `sdk/harmonyos/uba/` |

**共同能力**（各端一致）：

- 应用级鉴权（appId + appSecret 在请求体）
- 批量上报（本地缓冲，定时 / 定量自动 flush）
- 高层 API：`track` / `trackRisk` / `identify` / `setSuperProperties` / `flush`
- 自动补全设备 / 会话 / 时间 / 平台信息
- 重试与降级（指数退避，401 鉴权失败不重试）

---

## 三、平台采集差异

不同 SDK 自动采集字段的来源差异（接入时需知晓）：

| 字段 | Web SDK | C# SDK (Unity) | C# SDK (Godot) | Flutter SDK | 微信 SDK | Go/Java SDK（服务端） | Android SDK | iOS SDK |
|------|---------|---------------|----------------|-------------|----------|----------------------|-------------|---------|
| `deviceId` | localStorage 持久化 | PlayerPrefs 持久化 | 进程级（重启变化） | shared_preferences 持久化（uuid v4） | wx storage 持久化 | 实例 hostname（可配） | SharedPreferences 持久化 | UserDefaults 持久化 |
| `sessionId` | sessionStorage（标签关闭失效） | 进程级 GUID | 进程级 GUID | 进程内 uuid，后台超 30s 回前台轮换 | 进程内 uuid，后台超 30s 轮换 | 进程级（可选超时轮换） | 进程内 uuid，后台超 30s 轮换 | 进程内 uuid，后台超 30s 轮换 |
| `platform` | UA 探测（web/ios/...） | 编译宏探测（UNITY_IOS 等） | 固定 `dotnet` | `Platform.operatingSystem` | 自动探测 mini_program / mini_game | 固定 `server` | 固定 `android` | 固定 `ios` / `macos` |
| `clientInfo.userAgent` | navigator.userAgent | Unity 版本 + 操作系统 | .NET 运行时信息 | `uba_flutter/<版本> (<platform>)` | 系统信息拼接（model/system） | `uba-go` / `uba-java` + GOOS/os.name | `uba-android` | `uba-swift` |
| 卸载兜底 | `sendBeacon`（enableBeacon） | 无（需手动 FlushAsync） | 无（需手动 FlushAsync） | 退后台自动 flush（UbaLifecycleObserver） | App onHide 自动 flush | 常驻进程无需兜底 | 全部 Activity 退后台自动 flush | 退后台自动 flush |
| 离线补发 | 落盘补发（cap 500） | 无 | 无 | 落盘补发（cap 500） | 落盘补发（cap 500） | 内存队列补发（cap 10000） | 落盘补发（cap 500） | 落盘补发（cap 500） |

**注意事项**：
- **Godot 的 deviceId 进程级**：重启后变化，若需稳定设备标识，建议自行持久化（如存配置文件）后通过 `SetSuperProperties` 或自定义 ContextProvider 注入。
- **Unity WebGL 必须用 `UnityWebRequestTransport`**：HttpClient 在 WebGL 平台不可用，会抛异常。
- **客户端卸载兜底**：Web SDK 有 sendBeacon；C# 端需在 `OnApplicationQuit` / `OnDestroy` 调用 `FlushAsync()`；Flutter/微信/Android/iOS 端退到后台自动 flush（App 被杀前的最后一次依赖系统，关键路径建议在退后台时机完成上报）。
- **离线补发**：客户端 SDK（Web/Flutter/微信/Android/iOS）flush 失败的事件落盘（上限 500 条），下次启动自动补发；4xx（除 429）确定性失败不落盘不重试。服务端 SDK（Go/Java）为常驻进程，失败留在内存队列由定时器补发。
- **服务端 SDK 无设备/会话语义**：deviceId 取实例 hostname（可配），sessionId 为进程级（可选超时轮换），适合支付回调、风控触发、订单流转等后端事件。Python SDK 同此语义（deviceId=hostname、platform=`server`）。
- **Unreal 的 deviceId 存 `Saved/Uba/device_id.txt`**（持久化 + 首装判定）；`Flush` 为同步语义（与其他端一致），勿在游戏线程高频调用——建议在退后台/关卡切换/退出时机调用；`IUbaTransport`/`IUbaClock` 可注入替换。
- **HarmonyOS 的 deviceId 存 preferences**（`__uba_*__` 键名对齐其他端）；`module.json5` 已声明 `ohos.permission.INTERNET`；SDK 源码与 hypium 单测已交付，**需 DevEco Studio 跑 `hvigorw test` 验证**。

---

## 四、Web SDK 快速上手

### 安装

```bash
cd frontend/sdk/web/uba
pnpm install     # 安装 typescript
pnpm build       # 构建 dist/
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

## 六、Flutter SDK 快速上手（Android / iOS / 桌面）

### 引入

在宿主 App 的 `pubspec.yaml` 以 path 依赖引入（或发布到私有 pub 源）：

```yaml
dependencies:
  uba_flutter:
    path: ../go-wind-uba/sdk/flutter/uba
```

### 初始化与上报

```dart
import 'package:flutter/material.dart';
import 'package:uba_flutter/uba_flutter.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  final client = await UbaClient.init(UbaConfig(
    appId: 'your_app_id',
    appSecret: 'your_app_secret',
    endpoint: 'http://localhost:5700', // collector 服务地址
    appVersion: '1.0.0',
    debug: true,                       // 联调期打开
  ));

  // 后台自动 flush + 会话轮换（强烈建议）
  UbaLifecycleObserver(client).register();
  runApp(const MyApp());
}

// MaterialApp 挂页面浏览自动埋点：
// navigatorObservers: [UbaScreenObserver(UbaClient.instance!)]

// 行为事件（最常用）
client.track('button_click', properties: {'scene': 'home'}, options: const TrackOptions(
  objectType: 'button',
  objectId: 'btn_buy',
));

// 风险事件
client.trackRisk('abnormal_click', const RiskEvent(
  riskType: 'device_anomaly',
  riskLevel: 'HIGH',
  riskScore: 85,
  description: '短时间内频繁点击，疑似机器操作',
));

// 登录绑定用户（后续事件自动带 userId）
client.identify(1001);

// 关键转化节点，立即上报
client.track('purchase', properties: {'orderId': 'ORD-001'});
await client.flush();
```

### 常用配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `batchSize` | `20` | 缓冲达到该数量触发 flush |
| `flushIntervalMs` | `5000` | 定时 flush 间隔（毫秒） |
| `maxRetries` | `3` | 失败最大重试次数（指数退避） |
| `timeoutMs` | `8000` | 单次请求超时（须 < 服务端 10s） |
| `persistQueue` | `true` | flush 失败落盘，下次启动补发（上限 500 条） |
| `sessionTimeoutMs` | `30000` | 后台超过该时长回前台 = 新会话 |
| `autoTrackLaunch` | `false` | 首装自动埋 `app_launch` |
| `sampleRate` | `1.0` | 会话级确定性采样率 |
| `requireConsent` | `false` | 隐私合规：授权（`setConsent(true)`）前不发 |

> 完整 API、平台说明（Flutter Web 暂不支持）、自动采集字段见 [Flutter SDK 文档](../sdk/flutter/uba/README.md)。

---

## 七、微信小程序/小游戏 SDK 快速上手

```bash
cd frontend/sdk/wechat/uba
pnpm install && pnpm build   # 产物 dist/
```

> 小程序后台需把 collector 域名加入 **request 合法域名**。

```ts
// app.ts
import { UbaClient } from '@go-wind-uba/uba-wechat-sdk';

const uba = UbaClient.init({
  appId: 'your_app_id',
  appSecret: 'your_app_secret',
  endpoint: 'https://collector.example.com',
  appVersion: '1.0.0',
  autoTrackLaunch: true, // 首装自动埋 app_launch
});

App(uba.instrumentApp({ onLaunch() {} }));   // onHide 自动 flush + 会话轮换
Page(uba.instrumentPage({ onLoad() {} }));   // onLoad 自动埋 page_view（含来源页与停留时长）

uba.track('button_click', { scene: 'home' });
uba.identify(1001);
await uba.flush();
```

常用配置与其他端一致（batchSize 20 / flushInterval 5000 / maxRetries 3 / timeout 8000），
额外支持 `platform` 覆盖（默认自动探测 mini_program / mini_game）。
详见 [微信 SDK 文档](../frontend/sdk/wechat/uba/README.md)。

---

## 八、Go 服务端 SDK 快速上手

服务端事件源（支付回调、风控触发、订单流转），零第三方依赖（仅标准库）。

```go
import (
    "go-wind-uba/sdk/go/uba"
)

client, err := uba.NewClient(uba.Config{
    AppID:      "your_app_id",
    AppSecret:  "your_app_secret",
    Endpoint:   "http://localhost:5700",
    AppVersion: "1.0.0",
})
if err != nil {
    panic(err)
}
defer client.Close()

client.Identify(1001)
client.Track("order_paid", uba.P{"orderId": "ORD-001"}, &uba.TrackOptions{
    EventCategory: "order",
    ObjectType:    "order",
    ObjectID:      "ORD-001",
    Amount:        "99.90",
    OpResult:      "success",
})

client.TrackRisk("login_failed", &uba.RiskEvent{
    RiskType: "login_anomaly", RiskLevel: "HIGH", RiskScore: 88.5,
}, uba.P{"try_count": "8"}, nil)
```

服务端语义：deviceId 取实例 hostname（可配 `DeviceID`），platform 固定 `server`，
sessionId 进程级（`SessionTimeout > 0` 时按事件间隔轮换）；失败留在内存队列由定时器补发。
详见 [Go SDK 文档](../sdk/go/uba/README.md)。

---

## 九、Java 服务端 SDK 快速上手

JDK 11+，零第三方依赖（内置 HttpClient + 手写 JSON writer）。

```xml
<dependency>
  <groupId>com.gowind</groupId>
  <artifactId>uba-sdk</artifactId>
  <version>1.0.0</version>
</dependency>
```

```java
import com.gowind.uba.*;

UbaClient client = new UbaClient(new UbaConfig()
        .appId("your_app_id").appSecret("your_app_secret")
        .endpoint("http://localhost:5700").appVersion("1.0.0"));

client.identify(1001);
client.track("order_paid", TrackOptions.p("orderId", "ORD-001"),
        new TrackOptions().objectType("order").amount("99.90").opResult("success"));
client.trackRisk("login_failed",
        new RiskEvent().riskType("login_anomaly").riskLevel("HIGH").riskScore(88.5),
        TrackOptions.p("try_count", "8"), null);
client.close(); // 冲刷剩余队列
```

服务端语义与 Go SDK 一致（deviceId=hostname、platform=server、内存队列补发）。
详见 [Java SDK 文档](../sdk/java/uba/README.md)。

---

## 十、Android SDK 快速上手（Kotlin）

核心层纯 Kotlin（零依赖），Android 适配层源码随宿主 App 编译（SharedPreferences 持久化 +
ActivityLifecycleCallbacks 自动 page_view / 前后台 flush）。

```kotlin
// 自定义 Application.onCreate
UbaAndroid.init(this, UbaConfig(
    appId = "your_app_id",
    appSecret = "your_app_secret",
    endpoint = "http://localhost:5700",
    appVersion = BuildConfig.VERSION_NAME,
    autoTrackLaunch = true,
))

// 任意位置
UbaAndroid.client?.track("button_click", mapOf("scene" to "home"))
UbaAndroid.client?.trackRisk("abnormal_click",
    RiskEvent(riskType = "device_anomaly", riskLevel = "HIGH", riskScore = 85.0))
UbaAndroid.client?.identify(1001)
```

页面浏览自动埋点（Activity resumed → page_view）、退后台自动 flush、会话轮换均由适配层自动完成。
详见 [Android SDK 文档](../sdk/android/uba/README.md)。

---

## 十一、iOS/macOS SDK 快速上手（Swift）

Swift Package（SPM），核心仅 Foundation，iOS 13+ / macOS 10.15+ 通用。

```swift
let client = UbaClient(config: UbaConfig(
    appId: "your_app_id",
    appSecret: "your_app_secret",
    endpoint: "http://localhost:5700",
    appVersion: "1.0.0",
    autoTrackLaunch: true
))

// AppDelegate
applicationDidEnterBackground → client.handleBackground()   // 退后台立即 flush
applicationWillEnterForeground → client.handleForeground()  // 超时轮换会话

client.identify(1001)
client.track("button_click", properties: ["scene": "home"])
client.trackPageView("HomeViewController")
```

不采集 IDFA；构建与测试需在 macOS 执行（`swift build && swift test`）。
详见 [iOS SDK 文档](../sdk/ios/uba/README.md)。

---

## 十二、Python 服务端 SDK 快速上手

数据团队脚本、自动化任务、后端事件源。零第三方依赖（标准库 json/urllib/threading），Python 3.9+。

```bash
pip install -e sdk/python/uba
```

```python
from uba import UbaClient, UbaConfig, TrackOptions, RiskEvent

client = UbaClient(UbaConfig(
    app_id="your_app_id",
    app_secret="your_app_secret",
    endpoint="http://localhost:5700",
))
client.identify(1001)
client.track("order_paid", {"orderId": "ORD-001"},
             TrackOptions(object_type="order", amount="99.90"))
client.track_risk("login_failed",
                  RiskEvent(risk_type="login_anomaly", risk_level="HIGH", risk_score=88.5))
client.close()
```

服务端语义与 Go/Java 一致（deviceId=hostname、platform=server、内存队列补发）。
测试：`PYTHONPATH=src python -m unittest discover -s tests -v`（25 用例）。
详见 [Python SDK 文档](../sdk/python/uba/README.md)。

---

## 十三、Unreal Engine SDK 快速上手（C++）

PC/主机游戏覆盖。分层交付：`core/` 纯 C++17 核心层（零依赖，本机 g++ 测试全绿）+ `UnrealPlugin/` 插件壳（源码交付，宿主工程编译）。

```cpp
// UE：把 UnrealPlugin/ 拷入工程 Plugins/，C++ 初始化
#include "UbaSubsystem.h"
FUbaSettings Settings;
Settings.AppId = TEXT("your_app_id");
Settings.AppSecret = TEXT("your_app_secret");
Settings.Endpoint = TEXT("http://localhost:5700");
GEngine->GetEngineSubsystem<UUbaSubsystem>()->Init(Settings);
GEngine->GetEngineSubsystem<UUbaSubsystem>()->Track(TEXT("level_pass"), {}, TEXT("game"), TEXT("level"), TEXT("1-1"), 100);
```

Blueprint 启用插件后在「Uba」分类下直接调用。核心层测试：
`g++ -std=c++17 -Icore core/uba_core.cpp core/uba_core_test.cpp -o uba_tests && ./uba_tests`。
详见 [Unreal SDK 文档](../sdk/unreal/uba/README.md)。

---

## 十四、HarmonyOS SDK 快速上手（ArkTS）

鸿蒙原生 App。hvigor library 模块，`@ohos.net.http` 传输 + preferences 设备 ID 持久化。

> 需 DevEco Studio 验证（本仓库 Windows 无 hvigor 环境）：`hvigorw test`。

```ts
import { UbaClient, UbaConfig, loadDeviceId } from '@go-wind/uba-harmonyos'

const config = new UbaConfig()
config.appId = 'your_app_id'
config.appSecret = 'your_app_secret'
config.endpoint = 'https://collector.example.com'
const deviceId = await loadDeviceId(this.context)
const client = await UbaClient.create(config, deviceId)
client.identify(1001)
client.track('button_click', new Map([['scene', 'home']]))
```

详见 [HarmonyOS SDK 文档](../sdk/harmonyos/uba/README.md)。

---

## 十五、上报协议契约

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

## 十六、事件 Schema 管理（可选）

如需对上报事件做**字段校验**，可在管理后台「开发者 / 事件 Schema」中登记合法事件名及其属性 schema（属性名 / 类型 / 是否必填）。登记后可用于上报校验，降低脏数据。

> 该功能为可选增强，SDK 上报不强制依赖 Schema 登记。

---

## 十七、联调与排错

### 启动本地 collector

```bash
cd backend
go run ./app/collector/service/cmd/server/ -c ./app/collector/service/configs
# 默认监听 HTTP: 5700
```

### Web SDK 联调

1. `cd frontend/sdk/web/uba && pnpm build` 生成 `dist/`。
2. 修改 `test.html` 里的 `appId` / `appSecret` / `endpoint`。
3. 浏览器打开，点击按钮触发上报，观察 Network 面板与 Console。

### Flutter SDK 联调

1. 在 `sdk/flutter/uba/example/lib/main.dart` 顶部填入真实 `appId` / `appSecret` / `endpoint`。
2. `flutter pub get && flutter run`（选目标设备）跑示例 App。
3. 保持 SDK `debug: true`，点击按钮触发上报；collector 侧观察请求日志。

### 常见问题

| 现象 | 排查方向 |
|------|---------|
| 上报返回 401 | appId/appSecret 错误，或应用状态非 `ON`；检查管理后台「应用管理」 |
| 事件未入库但无报错 | 检查响应体 `failedCount`，可能字段校验部分失败；开启 SDK `debug` 查看日志 |
| Unity WebGL 上报失败 | 确认使用 `UnityWebRequestTransport` 而非默认 HttpClient |
| Flutter 首次 init 报 MissingPluginException | shared_preferences 插件未注册：确认宿主跑过 `flutter pub get` 且重新构建（不能热重载恢复） |
| 微信小程序上报失败（fail 回调） | 检查小程序后台是否把 collector 域名加入 **request 合法域名**（HTTPS 必需，开发工具可临时勾选"不校验域名"） |
| Python 脚本事件未发出 | 默认 `flush_interval_ms=5000` 定时补发，脚本退出前未 `close()` 会丢内存队列残留；401 检查 appId/appSecret |
| Unreal 插件编译失败 | 插件壳需宿主工程编译（本仓库不编译）；确认 `UnrealPlugin/` 在 `Plugins/` 下且 `Uba.Build.cs` 能找到 `core/` 源码；Flush 同步语义勿在游戏线程高频调用 |
| 鸿蒙 SDK 无法外呼 | `module.json5` 需含 `ohos.permission.INTERNET`（已含）；本地单元测试用 `hvigorw test`（DevEco Studio），真机联调走 ohosTest |
| 数据查不到 | 确认 collector → Kafka → Doris（Routine Load）入仓链路在跑：`uba-ingest status`；`tenantId` 由服务端补全，勿手动上报 |
| 页面跳转丢失事件 | Web 确认 `enableBeacon: true`；Flutter/微信/Android/iOS 确认已接生命周期自动 flush（退后台时机上报） |

---

## 十八、进阶场景

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

## 十九、相关文档

- [上手教程](tutorial.md)（任务导向：从零到看见数据）
- [Web SDK 完整文档](../frontend/sdk/web/uba/README.md)
- [C# SDK 完整文档](../sdk/csharp/README.md)
- [Flutter SDK 完整文档](../sdk/flutter/uba/README.md)
- [项目总览 README](../README.md)
- [部署文档](../backend/docs/build_deploy.md)
