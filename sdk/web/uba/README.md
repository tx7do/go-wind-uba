# UBA Web SDK

浏览器 / Node 端用户行为埋点 SDK，对接 **go-wind-uba** 项目的 collector 服务。

## 特性

- ✅ **应用级鉴权**：`appId` + `appSecret`（请求体内，无需 token）
- ✅ **批量上报**：本地缓冲事件，定时 / 定量自动 flush
- ✅ **高层 API**：`track()` / `trackBehavior()` / `trackRisk()`，自动补全设备/会话/时间/平台
- ✅ **重试与降级**：指数退避重试，鉴权失败不重试，超限丢弃防内存膨胀
- ✅ **页面卸载兜底**：`pagehide` / `beforeunload` 时用 `sendBeacon` 投递
- ✅ **环境采集**：自动填充 `deviceId`（持久化）/ `sessionId` / `platform` / `clientInfo`
- ✅ **全埋点**：自动 `page_view`（SPA 路由监听）+ 页面停留（`page_dwell`）
- ✅ **离线补发**：队列镜像 localStorage，崩溃 / 断网 / beacon 失败后自动补报
- ✅ **采样与合规**：会话级确定性采样（`sampleRate`）、隐私授权门控（`setConsent`）、尊重 DNT
- ✅ **可视化埋点**：管理端配置"页面 + XPath → 事件名"规则，SDK 自动匹配上报
- ✅ **会话回放**：基于 rrweb 录制用户操作，分块上传，管理端播放
- ✅ **功能开关 / A/B 实验**：`loadFlags` / `getFlag` / `isFeatureEnabled`，首次取值自动上报曝光事件
- ✅ **服务端补全**：collector 侧解析 UA（os/browser/设备类型）与 IP 地理，客户端不可信字段不依赖
- ✅ **TypeScript**：完整类型定义，契约与后端 proto 对齐

## 安装

本项目内使用，直接引用源码或构建产物：

```bash
cd sdk/web/uba
pnpm install     # 安装 typescript
pnpm build       # 构建 dist/
```

## 快速开始

### ES Module

```ts
import { UbaClient } from '@go-wind-uba/uba-sdk';

const uba = UbaClient.init({
  appId: 'demo_app_001',
  appSecret: 'demo_secret_123456',
  endpoint: 'http://localhost:5700',
});

// 上报行为事件
uba.track('page_view', { page: '/home' });
uba.track('click', { button: 'buy' }, { objectType: 'button', objectId: 'btn_buy' });

// 登录后绑定用户
uba.identify(1001);
uba.track('purchase', { orderId: 'ORD-001' }, { amount: '99.90', quantity: 1 });
```

### 浏览器 script 标签

```html
<script type="module">
  import { UbaClient } from './dist/index.js';
  UbaClient.init({
    appId: 'demo_app_001',
    appSecret: 'demo_secret_123456',
    endpoint: 'http://localhost:5700',
  }).track('page_view', { page: location.pathname });
</script>
```

## 配置项

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `appId` | string | **必填** | 应用 ID（鉴权） |
| `appSecret` | string | **必填** | 应用密钥（鉴权） |
| `endpoint` | string | **必填** | collector 地址，如 `http://localhost:5700` |
| `path` | string | `/uba/v1/report` | 上报路径 |
| `flagsPath` | string | `/uba/v1/flags` | 功能开关评估路径 |
| `batchSize` | number | `20` | 缓冲达到该数量触发 flush |
| `flushInterval` | number | `5000` | 定时 flush 间隔（毫秒） |
| `maxRetries` | number | `3` | 失败最大重试次数 |
| `timeout` | number | `8000` | 单次请求超时（毫秒，须 < 服务端 10s） |
| `retryBaseDelay` | number | `1000` | 重试基础退避（毫秒，指数递增） |
| `debug` | boolean | `false` | 调试日志 |
| `enableBeacon` | boolean | `true` | 页面卸载时用 sendBeacon 兜底 |
| `autoTrack` | boolean | `true` | 自动埋点（监听 click 上报点击事件，含坐标与 XPath） |
| `autoTrackPv` | boolean | `true` | 自动 PV：首屏 + SPA 路由变化上报 `page_view` |
| `pageViewEvent` | string | `'page_view'` | 自动 PV 的事件名 |
| `trackDwell` | boolean | `true` | 页面卸载时补发最后一页停留（`page_dwell`，≥1 秒才报） |
| `sampleRate` | number | `1` | 采样率 0~1。会话级确定性采样（按 sessionId），同一会话全采或全不采 |
| `requireConsent` | boolean | `false` | 是否强制隐私授权后才采集（配合 `setConsent`） |
| `respectDoNotTrack` | boolean | `false` | 尊重浏览器 DNT（DNT=1 时整体禁用采集） |
| `persistQueue` | boolean | `true` | 未上报队列镜像 localStorage，崩溃/断网后补发 |
| `maxPersistedEvents` | number | `500` | 持久化队列上限（条），超出丢最旧 |

## API

### `UbaClient.init(config): UbaClient`
初始化 SDK（单例）。多次调用会销毁旧实例并重建。

### `uba.track(eventName, properties?, options?)`
上报行为事件（`trackBehavior` 的别名）。最常用的方法。

```ts
uba.track('click', { button: 'buy' }, {
  objectType: 'button',
  objectId: 'btn_buy',
  amount: '99.90',
});
```

### `uba.trackBehavior(eventName, properties?, options?)`
显式上报行为事件，语义同 `track`。

### `uba.trackRisk(eventName, risk, options?)`
上报风险事件。

```ts
uba.trackRisk('abnormal_click', {
  riskType: 'device_anomaly',
  riskLevel: 'HIGH',
  riskScore: 85,
  description: '短时间内频繁点击，疑似机器操作',
});
```

### `uba.identify(userId)`
设置当前登录用户 ID，后续事件自动带上。登出用 `uba.resetUser()`。

### `uba.setSuperProperties(props)`
设置公共属性，注入后续每条事件的 `properties`。清除用 `uba.clearSuperProperties()`。

```ts
uba.setSuperProperties({ appVersion: '1.0.0', channel: 'official' });
// 之后每条事件都会带上这两个属性
```

### `uba.identify(userId)` 后的 flag 缓存
`identify` / `resetUser` 会清空功能开关取值缓存——身份变了取值要重拉，
记得重新调用 `loadFlags()`。

### `uba.loadFlags(): Promise<Record<string, string>>`
拉取当前身份的功能开关取值（服务端一致性哈希评估，同一身份结果稳定）。
建议应用启动时调用一次，登录态变化后重调。

```ts
await uba.loadFlags();
```

### `uba.getFlag(key): string | undefined`
取某开关的变体值。首次访问自动上报 `$feature_flag` 曝光事件（会话内每 flag 一次），
实验分析以此圈人。未命中放量 / 开关停用 / 未 loadFlags 时返回 `undefined`。

```ts
const variant = uba.getFlag('new-checkout-flow');
if (variant === 'treatment') {
  renderNewCheckout();
}
```

### `uba.loadVisualEvents(): Promise<VisualEventRule[]>`
拉取可视化埋点规则（管理端配置的"页面 + 元素 XPath → 事件名"映射）。
与自动埋点共享 click 监听——命中规则的以配置的事件名 + 静态属性上报。

```ts
await uba.loadVisualEvents();
// 之后点击命中 XPath 的元素时自动上报配置的事件名
```

### `uba.startVisualPicker()` / `uba.stopVisualPicker()`
可视化埋点选点模式（调试用）：开启后点任何元素 Console 打印 XPath，
复制到管理端「可视化埋点」页建规则。连点自动退出。

```ts
uba.startVisualPicker();  // 在目标页面执行，点元素看 Console
uba.stopVisualPicker();   // 选完关闭
```

### `uba.startReplayRecording(): Promise<boolean>` / `uba.stopReplayRecording()`
启动/停止会话回放录制。基于 rrweb（动态导入，未安装优雅降级），
录制事件分块定时上传。管理端「会话回放」页可播放。

```ts
await uba.startReplayRecording();
// ... 用户在页面上操作 ...
uba.stopReplayRecording();  // 停止并上传剩余
```

### `uba.isFeatureEnabled(key): boolean`
命中放量且变体不是对照组（`control`）时为 `true`。

```ts
if (uba.isFeatureEnabled('new-checkout-flow')) {
  renderNewCheckout();
}
```

### `uba.setConsent(granted)`
隐私授权开关。`requireConsent: true` 时未授权不采集不发送；授予后自动恢复
（含离线队列补发）。

```ts
// 用户在隐私弹窗点同意后
uba.setConsent(true);
```

### `uba.isCollecting(): boolean`
当前是否处于采集状态（DNT / 授权 / 采样综合判定），调试用。

### `uba.flush()`
手动触发批量上报（通常无需调用，SDK 自动 flush）。

```ts
// 关键转化节点，立即上报不等缓冲
uba.track('purchase', { orderId: 'ORD-001' });
await uba.flush();
```

## 自动采集的字段

SDK 会自动填充以下字段，无需手动设置：

| 字段 | 来源 | 说明 |
|------|------|------|
| `eventId` | uuid | 每条事件唯一 |
| `eventTime` | 当前时间 RFC3339 | 事件发生时间 |
| `deviceId` | localStorage 持久化 | 同一设备稳定 |
| `sessionId` | sessionStorage | 会话级，标签关闭失效 |
| `platform` | UA 探测 | web/ios/android/mini_program/node |
| `clientInfo` | navigator | userAgent / referer |
| `properties.pageUrl` | location | 当前页面 URL |
| `properties.title` | document.title | 自动 PV 事件附带页面标题 |

## 上报协议

SDK 对接 collector 服务的两个端点：

**`POST /uba/v1/report`（事件上报）**
- **鉴权**：`appId` + `appSecret` 放在请求体（非 header）
- **字段命名**：全 camelCase（protojson 编码）
- **批量**：`{ appId, appSecret, clientInfo, events: [...] }`
- **响应**：HTTP 200 也可能含部分失败（`failedCount > 0` / `errorsByType`），SDK 会记录 warn
- **错误码**：`400` 校验失败、`401` 鉴权失败（不重试）、`500` 服务端错误

**`POST /uba/v1/flags`（功能开关评估）**
- 请求：`{ appId, appSecret, deviceId, userId? }`
- 响应：`{ flags: { flagKey: variantKey } }`
- 服务不可用返回空结果（SDK 回退默认行为，不中断业务）

- **鉴权**：`appId` + `appSecret` 放在请求体（非 header）
- **字段命名**：全 camelCase（protojson 编码）
- **批量**：`{ appId, appSecret, clientInfo, events: [...] }`
- **响应**：HTTP 200 也可能含部分失败（`failedCount > 0` / `errorsByType`），SDK 会记录 warn
- **错误码**：`400` 校验失败、`401` 鉴权失败（不重试）、`500` 服务端错误

## 构建产物

```
dist/
├── index.js          # ESM 入口
├── index.d.ts        # 类型声明
└── ...               # 各模块 .js + .d.ts + .map
```

## 联调

启动 collector 服务后，浏览器打开 `test.html`（需同目录或正确路径）：
1. `pnpm build` 生成 `dist/`
2. 修改 `test.html` 里的 `appId` / `appSecret` / `endpoint`
3. 浏览器打开，点击按钮触发上报，观察 Network 和 Console

## 目录结构

```
uba/
├── src/
│   ├── index.ts      # 对外入口
│   ├── client.ts     # UbaClient 核心
│   ├── batcher.ts    # 缓冲 + 批量合并
│   ├── retry.ts      # 重试与降级
│   ├── context.ts    # 环境采集
│   ├── spa.ts        # SPA 路由监听
│   ├── storage.ts    # 离线队列持久化
│   ├── types.ts      # 类型定义
│   └── utils.ts      # 工具函数
├── test.html         # 联调页
├── package.json
└── tsconfig.json
```
