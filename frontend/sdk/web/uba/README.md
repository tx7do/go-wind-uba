# UBA Web SDK

浏览器 / Node 端用户行为埋点 SDK，对接 **go-wind-uba** 项目的 collector 服务。

## 特性

- ✅ **应用级鉴权**：`appId` + `appSecret`（请求体内，无需 token）
- ✅ **批量上报**：本地缓冲事件，定时 / 定量自动 flush
- ✅ **高层 API**：`track()` / `trackBehavior()` / `trackRisk()`，自动补全设备/会话/时间/平台
- ✅ **重试与降级**：指数退避重试，鉴权失败不重试，超限丢弃防内存膨胀
- ✅ **页面卸载兜底**：`pagehide` / `beforeunload` 时用 `sendBeacon` 投递
- ✅ **环境采集**：自动填充 `deviceId`（持久化）/ `sessionId` / `platform` / `clientInfo`
- ✅ **TypeScript**：完整类型定义，契约与后端 proto 对齐

## 安装

本项目内使用，直接引用源码或构建产物：

```bash
cd frontend/sdk/web/uba
npm install      # 安装 typescript
npm run build    # 构建 dist/
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
| `batchSize` | number | `20` | 缓冲达到该数量触发 flush |
| `flushInterval` | number | `5000` | 定时 flush 间隔（毫秒） |
| `maxRetries` | number | `3` | 失败最大重试次数 |
| `timeout` | number | `8000` | 单次请求超时（毫秒，须 < 服务端 10s） |
| `retryBaseDelay` | number | `1000` | 重试基础退避（毫秒，指数递增） |
| `debug` | boolean | `false` | 调试日志 |
| `enableBeacon` | boolean | `true` | 页面卸载时用 sendBeacon 兜底 |

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

## 上报协议

SDK 严格对接 collector 服务的 `POST /uba/v1/report`：

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
1. `npm run build` 生成 `dist/`
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
│   ├── types.ts      # 类型定义
│   └── utils.ts      # 工具函数
├── test.html         # 联调页
├── package.json
└── tsconfig.json
```
