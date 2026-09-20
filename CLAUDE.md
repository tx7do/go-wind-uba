# CLAUDE.md — go-wind-uba(根)

本文件供 Claude / Claude Code 在本仓库工作时使用。
完整指南见同目录 `AGENTS.md`,两者保持同步;子目录各有更细的 `AGENTS.md` / `CLAUDE.md`。
下面是 Claude 最该先记住的要点,详细约定请读 `AGENTS.md`。

## 这是什么

**GoWind UBA** —— 企业级用户行为分析平台。SDK 采集 → collector 转 Kafka → OLAP 引擎自拉入仓 → core 分析(Doris/ClickHouse)→ admin 前端可视化。

## 仓库结构

```
backend/      Go 1.25 + Kratos + Ent + Wire(三服务:admin/collector/core)
frontend/
  admin/      vue-vben-admin v5 monorepo(Vue3 + TS,应用 @vben/web-antd)
  sdk/web/uba/  纯 TS 埋点 SDK
sdk/csharp/   C# SDK
docs/         架构/开发/SDK 文档
```

**进子目录工作时,以该子目录的 AGENTS.md 为准。**

## 三条全局铁律

1. **契约真相源在后端 proto。** `backend/api/protos/` → `make api` 同时生成后端 Go 与前端 TS(`frontend/admin/apps/admin/src/generated/api/`)。所有生成物禁手改。

2. **包管理器各就各位:** 后端 Go modules;前端 pnpm(锁 only-allow pnpm);C# 见其 README。

3. **分析模型是双端核心。** 改分析模型要同时看 `backend/app/core/`(Doris 查询)和 `frontend/admin/apps/admin/src/views/app/data-analysis/`。

## 数据流(必须理解)

```
SDK → collector(采集,只发不写库) → Kafka(uba_events_raw / uba_risk_events)
      → OLAP 引擎侧自动拉取入库(Doris Routine Load / CK Kafka 表引擎+MV)
      → core(28+ 分析模型,读 PG + OLAP) → admin(SSE/REST 可视化)
                          PostgreSQL 存业务元数据(core 启动时 ent 自动迁移)
```

- PG 存业务元数据(事务),Doris/ClickHouse 存行为与分析聚合(OLAP),别混用。
- **入仓不写 Go 消费者**:搬运由 OLAP 引擎自己拉;装配/调度/观测走 `backend/cmd/uba-ingest`(仅 Doris)。

## 找对文件

| 任务 | 先读 |
|---|---|
| 后端 API/模型/逻辑 | `backend/AGENTS.md` |
| 管理后台页面/i18n | `frontend/AGENTS.md` |
| 分析模型(双端) | `backend/AGENTS.md` + `frontend/AGENTS.md` |
| Web SDK | `frontend/AGENTS.md`(sdk/web/uba)|
| 整体架构 | `docs/architecture.md` + 根 `AGENTS.md` 第 4 节 |

## 常用入口

```bash
# 后端
cd backend && make help      # api / ent / wire / build / lint / test
# 前端
cd frontend/admin && pnpm install && pnpm dev:antd
# Web SDK
cd frontend/sdk/web/uba && pnpm build
```

详见根 `AGENTS.md` 及各子目录指南。
