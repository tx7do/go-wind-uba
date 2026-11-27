# UBA Flutter SDK（uba_flutter）

GoWind UBA 风行用户行为分析平台的 Flutter 埋点采集 SDK，面向 **Android / iOS / Windows / macOS / Linux**（Flutter Web 暂不支持，传输层留有接口可扩展）。
对接 collector 服务 `POST /uba/v1/report`，鉴权在请求 body（`appId` + `appSecret`），JSON camelCase，无 gzip。

## 结构

```
sdk/flutter/uba/
├── lib/
│   ├── uba_flutter.dart      # 导出入口
│   └── src/
│       ├── client.dart       # UbaClient 门面：单例 init + 可注入构造（transport/store/context 可替换）
│       ├── config.dart       # UbaConfig + TrackOptions（默认值对齐 C#：批量 20/5s/3 次重试/1s 退避/8s 超时）
│       ├── types.dart        # ReportEvent/BehaviorEvent/RiskEvent + toJson/fromJson（对齐 proto json_name）
│       ├── batcher.dart      # 队列 + 定时/定量 flush + 指数退避重试 + 失败落盘
│       ├── transport.dart    # HttpTransport 抽象 + dart:io HttpClient 实现（超时可配）
│       ├── context.dart      # ContextProvider 抽象 + 默认实现（deviceId 持久化 uuid / platform / UA / os）
│       ├── session.dart      # SessionManager：后台超时轮换会话（默认 30s）
│       ├── store.dart        # KeyValueStore 抽象 + shared_preferences 实现（离线队列 / deviceId）
│       ├── flags.dart        # AB 实验开关：/uba/v1/flags 拉取 + 缓存
│       ├── lifecycle.dart    # UbaLifecycleObserver（后台 flush）+ UbaScreenObserver（page_view）
│       └── utils.dart        # uuid v4 / RFC3339 毫秒 / 属性合并 / rune 截断 / 采样哈希
├── example/                  # 最小接入示例（Flutter app）
├── test/                     # 单元测试（契约金样 + 批量/重试/落盘/会话/consent/flags）
├── pubspec.yaml
└── analysis_options.yaml
```

## 特性

- ✅ 鉴权在请求 body（appId + appSecret），camelCase 契约逐字段对齐后端 proto
- ✅ 批量上报（20 条或 5s 定时触发）+ 指数退避重试（1s×2^n，最多 3 次；4xx 除 429 不重试）
- ✅ 离线持久化队列（shared_preferences，上限 500 条，断网/崩溃补发）
- ✅ 会话管理（uuid，后台 30s 超时轮换，会话级确定性采样）
- ✅ 生命周期自动埋点：后台自动 flush、首装 `app_launch`、路由 `page_view`（含停留时长）
- ✅ AB 实验开关（`/uba/v1/flags`）+ `$feature_flag` 曝光事件（每会话每开关一次）
- ✅ 隐私合规：`requireConsent` 授权门控、`setConsent` 运行时开关
- ✅ 零原生依赖：纯 Dart 实现，仅依赖 `shared_preferences`；传输/存储/环境均留接口可替换

## 接入

前置：在管理后台「数据采集/应用管理」创建应用，拿到 `appId` / `appSecret`（应用需为 ON 状态）。

```bash
# 在宿主 App 的 pubspec.yaml 里以 path 依赖引入（或发布到私有 pub 源）
dependencies:
  uba_flutter:
    path: ../go-wind-uba/sdk/flutter/uba
```

### 推荐接入（完整能力）

```dart
import 'package:flutter/material.dart';
import 'package:uba_flutter/uba_flutter.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  final client = await UbaClient.init(UbaConfig(
    appId: 'your_app_id',
    appSecret: 'your_app_secret',
    endpoint: 'http://collector.your-domain.com:5700',
    appVersion: '1.0.0',          // 随事件上报 appVersion
    channel: 'official',          // 渠道
    debug: true,                  // 联调期打开
    autoTrackLaunch: true,        // 首装自动埋 app_launch
  ));

  // 后台自动 flush + 会话轮换
  UbaLifecycleObserver(client).register();

  runApp(const MyApp());
}

// MaterialApp 上挂页面浏览自动埋点：
MaterialApp(
  initialRoute: '/home',
  navigatorObservers: [UbaScreenObserver(UbaClient.instance!)],
  routes: {'/home': (_) => const HomePage()},
);
```

### 精简接入（手动控制一切）

```dart
final client = await UbaClient.init(UbaConfig(
  appId: 'your_app_id',
  appSecret: 'your_app_secret',
  endpoint: 'http://localhost:5700',
));

client.track('button_click', properties: {'scene': 'home'});
client.trackBehavior('level_pass', options: const TrackOptions(
  eventCategory: 'game',
  objectType: 'level',
  objectId: 'level_42',
  score: 88,
  durationMs: 3500,
  serverId: 's1',
  level: 42,
));
client.trackRisk('login_failed', const RiskEvent(
  riskType: 'login_anomaly',
  riskLevel: 'HIGH',
  riskScore: 88.5,
));
client.identify(10001);           // 登录绑定
client.setSuperProperties({'vip': 'true'});  // 全局公共属性
await client.flush();             // 手动冲刷（App 退出前调用）
```

### 隐私合规（GDPR / 个保法）

```dart
final client = await UbaClient.init(UbaConfig(
  // ...
  requireConsent: true, // 未授权前所有事件静默丢弃
));
// 首次启动弹隐私弹窗，用户同意后：
client.setConsent(true);
```

## 平台说明

| 平台 | 状态 | 说明 |
|---|---|---|
| Android | ✅ | deviceId 存 SharedPreferences；平台标识 `android` |
| iOS | ✅ | deviceId 存 NSUserDefaults；平台标识 `ios`（不采集 IDFA，如需请自实现 ContextProvider） |
| Windows / macOS / Linux | ✅ | 桌面端直接可用；平台标识同 `Platform.operatingSystem` |
| Flutter Web | ❌ | 未内置；实现 `HttpTransport` + `KeyValueStore` 即可接入 |

## API 速查

```dart
await UbaClient.init(config);                  // 全局单例（重复 init 会先关闭旧实例）
UbaClient.create(config, transport:, store:, contextProvider:);  // 多实例/测试
UbaClient.instance;                            // 当前全局实例

client.track(name, properties:, options:);     // 自定义事件（= trackBehavior）
client.trackRisk(name, RiskEvent(...));        // 风险事件
client.trackPageView('/home', title:, referer:); // 页面浏览（手动版）
client.identify(userId);                       // 登录绑定
client.resetUser();                            // 登出解绑
client.setSuperProperties({...});              // 全局公共属性（增量合并）
client.clearSuperProperties();
client.setConsent(true);                       // 隐私授权
await client.loadFlags();                      // 拉取实验开关
client.getFlag('key');                         // 读开关（自动曝光 $feature_flag）
client.isFeatureEnabled('key');                // 变体非空非 control
client.handleBackground();                     // 退后台（UbaLifecycleObserver 自动调）
client.handleForeground();                     // 回前台（超时轮换会话）
await client.flush();                          // 手动冲刷，返回 FlushResult
client.pendingCount;                           // 队列长度
client.isCollecting;                           // consent/采样/关闭状态
await client.close();                          // flush → 停定时器 → 关连接
```

## 自动采集字段

| 字段 | 来源 |
|---|---|
| `eventId` | 每事件 uuid v4 |
| `eventTime` | 事件构造时刻，RFC3339 毫秒 UTC |
| `deviceId` | uuid v4，持久化（`__uba_device_id__`），首装生成 |
| `sessionId` | uuid v4，进程内；后台超过 sessionTimeoutMs（默认 30s）回前台轮换 |
| `platform` | `Platform.operatingSystem`（android/ios/windows/macos/linux） |
| `behavior.os` | Android / iOS / macOS / Windows / Linux |
| `behavior.appVersion` / `behavior.channel` | UbaConfig.appVersion / channel |
| `clientInfo.userAgent` | `uba_flutter/<版本> (<platform>)` |
| `properties` | super 属性 + 调用属性合并（后者覆盖） |

服务端补全（SDK 不发）：`tenantId`（按 appId 覆盖）、`serverTime`、真实 IP、UA 解析（os/browser/device_type）、GeoLite 国家/城市。

## 构建

```bash
cd sdk/flutter/uba
flutter pub get
flutter analyze   # 0 issue
flutter test      # 单元测试全绿（契约金样 + 行为）
```

## 契约约束

- 字段名 camelCase 与后端 proto `json_name` 逐一对齐；**禁止手造字段**
- `tenantId` / `serverTime` 服务端覆盖，客户端不发；真实 IP 服务端取连接信息
- 数值标量（durationMs/quantity/score/sessionSeq/serverId/level）proto3 无法区分 unset 0，**只进 behavior oneof** 才被服务端信任
- 鉴权在 body，无 Authorization header；单批请求体压在 1MiB 内（服务端对未鉴权大包直接 401）
- 超时必须 < 服务端 10s（默认 8s）；4xx 除 429 不重试；HTTP 200 仍可能有单条部分失败（`failedCount`/`errorsByType`）
- `eventTime` RFC3339 毫秒 UTC；必填四件套：`eventType`/`eventId`/`eventName`/`eventTime` + 匹配的 oneof 载荷
