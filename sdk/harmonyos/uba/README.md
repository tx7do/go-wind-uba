# UBA HarmonyOS SDK（ArkTS）

GoWind UBA 风行用户行为分析平台的 **HarmonyOS（鸿蒙）原生埋点 SDK**（ArkTS，hvigor library 模块）。
对接 collector `POST /uba/v1/report`，鉴权在请求 body，JSON camelCase，无 gzip，单批 < 1MiB。

> ⚠️ **验证环境**：本仓库开发机为 Windows（无 DevEco Studio / hvigor）。ArkTS 源码与 hypium 单测已交付，需在 DevEco Studio 执行 `hvigorw test`（LocalUnit）与真机集成验证。

## 结构

```
sdk/harmonyos/uba/
├── oh-package.json5 / hvigorfile.ts / build-profile.json5 / src/main/module.json5
├── src/main/ets/
│   ├── index.ets      # 导出入口
│   ├── client.ets     # UbaClient：队列 + 定时/定量 flush + 重试退避 + AB 开关
│   ├── config.ets     # UbaConfig/TrackOptions/RiskEvent（class + validate，默认值对齐其他端）
│   ├── jsonwire.ets   # （构造内联于 client.ets：数值标量只进 behavior oneof）
│   ├── transport.ets  # @ohos.net.http 封装（statusCode 0 = 网络失败可重试）
│   ├── context.ets    # @ohos.data.preferences 设备 ID 持久化（__uba_*__ 键名）
│   ├── session.ets    # 后台 30s 超时轮换 + 事件间隔超时轮换（时钟可注入）
│   └── utils.ets      # uuid/RFC3339/FNV 采样哈希
└── src/test/UbaClient.test.ets   # hypium LocalUnit 测试（DevEco 验证）
```

## 快速开始

1. 把本目录作为 har 依赖引入宿主工程（oh-package.json5 `dependencies` 指向本地路径或私有仓库）。
2. EntryAbility onCreate 初始化：

```ts
import { UbaClient, UbaConfig, loadDeviceId } from '@go-wind/uba-harmonyos'

const config = new UbaConfig()
config.appId = 'your_app_id'
config.appSecret = 'your_app_secret'
config.endpoint = 'https://collector.example.com'
config.appVersion = '1.0.0'
config.autoTrackLaunch = true

const deviceId = await loadDeviceId(this.context)
const client = await UbaClient.create(config, deviceId)
client.identify(1001)
client.track('button_click', new Map([['scene', 'home']]))
```

3. 前后台接入（Ability 回调）：`onForeground → client.handleForeground()`、
   `onBackground → client.handleBackground()`（立即 flush + 会话轮换）。

## 契约约束

- 字段 camelCase 与后端 proto `json_name` 逐一对齐，**禁止手造字段**
- `tenantId`/`serverTime`/`ip` 服务端补全不发送；数值标量只进 behavior oneof
- 鉴权在 body；单批压在 1MiB 内；超时必须 < 服务端 10s（默认 8s）
- 4xx（除 429）不重试；可重试失败回队补发；HTTP 200 仍可能有单条部分失败（failedCount）
- 需在 module.json5 申请 `ohos.permission.INTERNET`（已含）
