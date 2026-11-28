# UBA 微信小程序/小游戏 SDK（TypeScript）

GoWind UBA 风行用户行为分析平台的**微信小程序 / 微信小游戏**埋点采集 SDK。
对接 collector `POST /uba/v1/report`，鉴权在请求 body（`appId` + `appSecret`），JSON camelCase，无 gzip，单批 < 1MiB。
构建仅 `tsc`，运行零依赖（wx API 手写最小声明，不引 miniprogram-api-typings）。

## 结构

```
sdk/wechat/uba/
├── src/
│   ├── client.ts     # UbaClient 单例：init/track/trackRisk/identify/flags + App/Page 包装器
│   ├── config.ts     # UbaConfig 解析与校验（默认值对齐其他端：批量 20/5s/3 次重试/1s 退避/8s 超时）
│   ├── types.ts      # 上报契约类型（对齐 proto json_name，数值标量只进 behavior oneof）
│   ├── batcher.ts    # 队列 + 定时/定量 flush + 指数退避重试 + 失败落盘（wx storage，cap 500）
│   ├── transport.ts  # wx.request Promise 化封装（statusCode 0 = 网络失败可重试）
│   ├── context.ts    # deviceId 持久化 + getSystemInfoSync 平台/UA 探测
│   ├── session.ts    # 会话管理：后台超时轮换（默认 30s）
│   ├── storage.ts    # wx 存储 + __uba_*__ 键名约定（对齐 Web/Unity/Flutter）
│   ├── wx-env.ts     # wx API 最小声明 + 环境注入点（测试用）
│   └── utils.ts      # uuid / RFC3339 / FNV 采样哈希
└── test/             # vitest + wx mock（契约金样 / 重试 / 落盘 / App/Page 包装器）
```

## 特性

- ✅ 自动探测小程序/小游戏宿主（有 `Page` 全局 → `mini_program`，否则 `mini_game`），可配置覆盖
- ✅ 批量上报 + 指数退避重试；4xx（除 429）不重试不落盘；可重试失败落盘补发（cap 500 条）
- ✅ 会话轮换（后台 30s 超时）+ 会话级确定性采样 + `requireConsent` 隐私门控
- ✅ `instrumentApp` / `instrumentPage` 自动埋点：onHide 自动 flush、onLoad 自动 `page_view`（含来源页与停留时长）
- ✅ 首装 `app_launch`（deviceId 新建判定）
- ✅ AB 开关 + `$feature_flag` 曝光去重

## 快速开始

```bash
cd sdk/wechat/uba
pnpm install
pnpm build   # 产物 dist/，拷入小程序项目或以 npm 包方式引入
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
  autoTrackLaunch: true,  // 首装自动埋 app_launch
  debug: true,            // 联调期打开
});

App(uba.instrumentApp({
  onLaunch() { /* ... */ },
}));

// pages/index/index.ts
Page(uba.instrumentPage({
  onLoad() { /* 自动已埋 page_view */ },
}));

// 自定义事件
uba.track('button_click', { scene: 'home' }, { objectType: 'button', objectId: 'btn_buy' });
uba.trackRisk('abnormal_click', { riskType: 'device_anomaly', riskLevel: 'HIGH', riskScore: 85 });
uba.identify(1001); // 登录绑定
```

## 平台差异说明

| 维度 | 小程序 | 小游戏 |
|---|---|---|
| platform 字段 | `mini_program` | `mini_game` |
| page_view | `instrumentPage` 按 Page onLoad 自动埋 | 无页面概念，手动 `trackPageView` |
| 可视化埋点/点击热图 | 不支持（无 DOM） | 不支持 |
| 其余能力 | 一致 | 一致 |

## 构建

```bash
pnpm typecheck   # tsc --noEmit，0 error
pnpm test        # vitest，wx mock 全量行为测试
pnpm build       # 产物 dist/（ESM + d.ts）
```

> 本机 pnpm 需要 Node ≥ 22（全局 node 20 时用 `scoop node 26 直跑 pnpm.mjs`，见根目录记忆文档）。

## 契约约束

- 字段 camelCase 与后端 proto `json_name` 逐一对齐，**禁止手造字段**
- `tenantId`/`serverTime`/`ip` 服务端补全不发送；数值标量（durationMs/quantity/score/level）只进 behavior oneof
- 鉴权在 body；单批压在 1MiB 内；超时必须 < 服务端 10s（默认 8s）
- 4xx（除 429）不重试不落盘；HTTP 200 仍可能有单条部分失败（`failedCount`）
