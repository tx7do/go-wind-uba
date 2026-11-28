# GoWind UBA 上手教程：从零到看见你的第一条行为数据

这是一份**任务导向**的端到端教程：跟着做完（约 15~20 分钟），你会跑通一条完整数据链路——
本地起服务 → 创建应用拿凭据 → 用 Web SDK 上报事件 → 在管理后台看见数据。
字典式的字段/协议/配置细节不在本文展开，查阅 [SDK 接入指南](sdk_integration.md)（本文多处引用它）。

> 主线用 **Web SDK**（浏览器打开即测、无需 IDE）。其他 10 个端的上报代码只差初始化与传输层，
> 数据链路完全一致——文末有[各端切换对照](#六其他端切换对照表)。

## 0. 你将完成什么

```
浏览器(test.html) ──上报──► collector(:5700) ──► Kafka ──► Doris ──► 管理后台(:5600) 查到事件
```

前提：已安装 Docker、Go 1.25+、pnpm（部署细节见 [README 快速开始](../README.md#快速开始)，本教程用开发模式）。

## 一、启动最小后端

```bash
# 1) 起中间件（Kafka/Redis/PG/Doris…，首次会拉镜像）
cd backend
./scripts/docker/libs_only.sh          # Windows 用 .\scripts\docker\libs_only.ps1

# 2) 生成代码并构建（首次需要；后续跳过）
make init && make gen && make build

# 3) 起三个服务（各开一个终端）
go run ./app/core/service/cmd/server/ -c ./app/core/service/configs       # core：分析建模
go run ./app/admin/service/cmd/server/ -c ./app/admin/service/configs     # admin：管理后台 BFF
go run ./app/collector/service/cmd/server/ -c ./app/collector/service/configs  # collector：埋点接收
```

- 管理后台：`http://localhost:5600`（默认账号见 `backend/sql/postgresql/default-data.sql` 种子数据）
- collector：`http://localhost:5700`（健康检查 `GET /uba/v1/health`）

> Doris 起来后还要**打通入仓**（Kafka → Doris 的 Routine Load），见[第四步](#四验证数据链路)。

## 二、创建应用，拿到 appId / appSecret

1. 登录管理后台 → 菜单「数据采集 / 应用管理」→ 新建应用。
2. 保存后得到 `appId` / `appKey` / `appSecret` 三组凭据（上报只用前述 `appId`+`appSecret`）。
3. **把应用状态置为 `ON`**——关闭状态的上报一律 401。

> 原理：collector 收到上报后按 `appId` 反查应用并校验 `appSecret`（SHA-256 恒时比较），
> `tenantId` 由服务端按应用自动补全——**客户端永远不要手动上报 tenantId**。

## 三、上报第一条事件（Web SDK，10 行代码）

最省事：仓库自带调试页，改两个值、浏览器打开。

```bash
cd frontend/sdk/web/uba && pnpm install && pnpm build   # 生成 dist/
# 编辑 test.html：把 init 里的 appId/appSecret/endpoint 换成你的，然后浏览器直接打开
```

手写等效 HTML（同样 10 行）：

```html
<!DOCTYPE html><meta charset="utf-8">
<button id="buy">下单</button>
<script type="module">
  import { UbaClient } from './frontend/sdk/web/uba/dist/index.js';

  const uba = UbaClient.init({
    appId: 'your_app_id',
    appSecret: 'your_app_secret',
    endpoint: 'http://localhost:5700',
    debug: true,                       // 联调期打开：控制台打印每次上报
  });

  uba.setSuperProperties({ page: 'tutorial' });   // 公共属性：之后每条事件都带
  uba.track('buy_click', { item: 'sku_001' });    // 自定义事件
  uba.identify(1001);                             // 登录绑定：之后事件自动带 userId
  document.getElementById('buy').onclick = () => uba.track('buy_click', { item: 'sku_002' });
</script>
```

**怎么确认发出去了**（三层验证，逐层排查）：

1. `debug: true` 控制台无报错，页面 Network 里有 `POST /uba/v1/report`；
2. 响应 HTTP 200 且 `successCount === 1`——注意 **200 不代表全部成功**，`failedCount > 0`
   说明有单条被校验拒绝（看 `errorsByType` 里的 eventId，常见原因是缺 `eventName`）；
3. collector 终端日志有 produce 记录。

> 点击事件没自动埋？Web SDK 的点击自动埋点（`elementXpath`/坐标热力）与 SPA 路由 `page_view`
> 需要开自动埋点（`autoTrack` / `autoTrackPv` 配置，默认开）——详见 [Web SDK 文档](../frontend/sdk/web/uba/README.md)。

## 四、验证数据链路（Kafka → Doris → 看板）

事件到 Kafka 还没完——**Doris 的 Routine Load 负责把数据拉进仓**（引擎自己拉，不写 Go 消费者）：

```bash
cd backend
make ingest                                            # 构建入仓运维 CLI
./bin/uba-ingest -c app/core/service/configs apply --wait 5m   # 建库表 + 确保 Routine Load 在跑
./bin/uba-ingest -c app/core/service/configs status --json     # 观测：位点/延迟/错误计数
```

然后到**管理后台**：菜单「数据采集」相关页面（事件查询 / 分析模型）里按 `eventId`
或时间范围查 `buy_click`。能查到 = 全链路打通 🎉

- 查不到先跑 `status`：Routine Load 停了/位点不动是最常见原因（脚本声明了但没消费会退出码 1）；
- 当天数据可配合日内刷新：`./bin/uba-ingest -c app/core/service/configs etl --loop --every 1h`；
- 用 ClickHouse 的等价命令是 `ch-apply` / `ch-etl`（引擎二选一，详见 [架构文档](architecture.md)）。

## 五、让数据真正有用（建议下一步）

| 要做的事 | 用什么 | 去 |
|---|---|---|
| 统一事件命名（`模块_动作` 如 `order_paid`） | 团队约定；可选在管理后台登记事件 Schema 做校验 | [接入指南 §八](sdk_integration.md) |
| 全局公共属性（版本/渠道/AB 分组） | `setSuperProperties`（各端同名 API） | [接入指南 §十八](sdk_integration.md) |
| 用户行为串联 | `identify(userId)` 登录绑定，deviceId 匿名行为自动归因 | 各端 README「自动采集字段」 |
| 第一次 AB 实验 | `loadFlags()` / `getFlag('exp')`（命中自动上报 `$feature_flag` 曝光） | 各端 README |
| 告警/分群/看板/定时报告 | 管理后台增长闭环模块 | [README 功能清单](../README.md#增长闭环) |

## 六、其他端切换对照表

数据链路与上报契约**完全一致**，只有初始化和传输层不同。最小用法 3 行，完整 API 看各端文档：

| 端 | 引入 | 最小上报 | 文档 |
|---|---|---|---|
| Web / Node | `pnpm build` 产物或 npm 包 | `UbaClient.init({...}).track('evt')` | [Web](../frontend/sdk/web/uba/README.md) |
| 微信小程序/小游戏 | npm 或 dist 拷入 | `uba.instrumentPage({})` 自动埋 page_view | [微信](../frontend/sdk/wechat/uba/README.md) |
| Flutter | pubspec path 依赖 | `await UbaClient.init(config)` | [Flutter](../sdk/flutter/uba/README.md) |
| Android | gradle 模块 + 适配层源码 | `UbaAndroid.init(this, config)` | [Android](../sdk/android/uba/README.md) |
| iOS/macOS | SPM 包 | `UbaClient(config:)` | [iOS](../sdk/ios/uba/README.md) |
| Unity / Godot / .NET | Uba.Core.dll 或源码 | `new UbaClient(new UbaConfig{...})` | [C#](../sdk/csharp/README.md) |
| Unreal | Plugins/ 拷入 | `UUbaSubsystem::Init(Settings)` | [Unreal](../sdk/unreal/uba/README.md) |
| Go 服务端 | `sdk/go/uba` | `uba.NewClient(cfg)` | [Go](../sdk/go/uba/README.md) |
| Java 服务端 | Maven 坐标 | `new UbaClient(new UbaConfig()...)` | [Java](../sdk/java/uba/README.md) |
| Python 服务端 | `pip install -e sdk/python/uba` | `UbaClient(UbaConfig(...))` | [Python](../sdk/python/uba/README.md) |
| HarmonyOS | har 依赖 | `await UbaClient.create(config, deviceId)` | [HarmonyOS](../sdk/harmonyos/uba/README.md) |

> **服务端三端（Go/Java/Python）语义差异**：deviceId=实例 hostname、platform=`server`、
> 无卸载兜底（失败由定时器补发）——适合支付回调、风控触发等后端事件。

## 七、常见坑速查（30 秒版）

| 症状 | 多半是 | 详查 |
|---|---|---|
| 401 | appId 错 / 应用未置 ON / appSecret 错 | [排错表](sdk_integration.md) |
| 200 但没入库 | `failedCount>0` 单条被拒，或 Routine Load 没在跑 | [排错表](sdk_integration.md) |
| 小程序发不出 | collector 域名没加 request 合法域名 | [排错表](sdk_integration.md) |
| WebGL 报 HttpClient 异常 | 没换 `UnityWebRequestTransport` | [排错表](sdk_integration.md) |
| 手动上报了 tenantId | 不需要——服务端按 appId 权威覆盖 | [协议契约](sdk_integration.md) |

---

## 相关文档

- [数据分析教程](analytics-tutorial.md)（使用侧：25 个分析模型怎么用）
- [SDK 接入指南](sdk_integration.md)（字典：协议/字段/配置全集）
- [系统架构](architecture.md)（数据流转与 OLAP 引擎二选一）
- [项目 README](../README.md)（部署与功能全景）
