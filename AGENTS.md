# Agent Guide — go-wind-uba(根)

本文件是 AI 编码助手(以及新人)在整个仓库工作时的**总入口**。
先读这里,再进入对应子目录的详细指南。

## 1. 项目是什么

**GoWind UBA · 风行用户行为分析平台** —— 开箱即用的企业级用户行为分析与商业智能平台。
核心能力:多 SDK 埋点采集 → 数据入库(PostgreSQL + Apache Doris)→ 28+ 种分析模型 → 管理后台可视化。

## 2. 仓库总览

```
go-wind-uba/
├── backend/        ★ Go 1.25 + Kratos + Ent + Wire 多服务后端 → 见 backend/AGENTS.md
├── frontend/       ★ 前端(双产物)→ 见 frontend/AGENTS.md
│   ├── admin/        vue-vben-admin v5 monorepo(Vue3 + TS + AntD Vue)
│   └── sdk/web/uba/  浏览器/Node 埋点 TS SDK
├── sdk/
│   └── csharp/       C# SDK(独立子项目)
├── docs/             架构 / 开发指南 / SDK 集成文档
├── README.md         中文(另附 en / ja 版本)
└── LICENSE
```

> 子目录各有自己的 `AGENTS.md`(权威完整)和 `CLAUDE.md`(精简速记)。
> **进入某子目录工作时,以该子目录的指南为准。**

## 3. 各模块速记

| 模块 | 技术栈 | 详细指南 |
|---|---|---|
| 后端 | Go 1.25 · Kratos v2 · Ent · Wire · PostgreSQL · Doris · ClickHouse · Redis · Asynq · MinIO · Kratos authn(JWT)/ authz(Casbin/OPA) | `backend/AGENTS.md` |
| 管理后台 | Vue 3 · TypeScript · Vite · Ant Design Vue · vue-vben-admin v5 · pnpm monorepo + turbo + changeset | `frontend/AGENTS.md` |
| Web SDK | 纯 TypeScript,`tsc` 构建,上报到 collector | `frontend/AGENTS.md` |
| C# SDK | C#(`sdk/csharp/`) | `sdk/csharp/README.md` |

## 4. 数据流(理解全局的关键)

```
用户终端 (浏览器/小程序/App …)
  │ SDK 上报(Web TS / C# / …)
  ▼
backend/app/collector        鉴权·校验·补全,只转发不落库
  │ Publish
  ▼
Kafka                        uba_events_raw / uba_risk_events
  │ 引擎侧自动拉取入库 —— 不在 Go 侧写消费者是设计选择
  ▼
OLAP(二选一)                 Doris: Routine Load / ClickHouse: Kafka 表引擎 + 物化视图
  ▲
backend/cmd/uba-ingest       装配·调度·观测(仅 Doris)

PostgreSQL                   业务/配置元数据,core 启动时由 ent 自动迁移
  │
  └─► backend/app/core       读 PG + OLAP,跑 28+ 分析模型
        ▲ gRPC
      backend/app/admin      管理后台 BFF:权限·菜单·转发
        ▲ HTTP / SSE
      frontend/admin         管理后台可视化
```

- **采集**:`collector` 服务接收 SDK 上报,鉴权/校验/补全后只发 Kafka。
- **入仓**:Kafka → OLAP 由**引擎自己拉**(Doris Routine Load / ClickHouse Kafka 表引擎),
  不写 Go 消费者是设计选择;装配与观测走 `backend/cmd/uba-ingest`,详见 `backend/AGENTS.md` 第 6 节。
- **存储分流**:业务配置/元数据进 PG;行为事件与分析聚合进 OLAP(二选一)。
- **分析**:`core` 服务跑各类分析模型查询 OLAP。
- **呈现**:`admin` 服务转 `core` 的 gRPC,前端通过 REST + SSE 实时展示。

## 5. 全仓库铁律

1. **契约单一真相源在后端 proto。** `backend/api/protos/` 是 API 的唯一来源;`make api` 会同时生成后端 Go 代码与前端的 TypeScript 调用代码(`frontend/admin/apps/admin/src/generated/api/`)。**所有生成物禁止手改。**
2. **包管理器严格匹配:**
   - backend:Go modules(`go.mod`)
   - frontend:pnpm(admin 锁 `only-allow pnpm`,禁 npm/yarn)
   - sdk/csharp:按其 README
3. **多语言文档同步。** `README.md`(zh)/ `README_en.md` / `README_ja.md` 三份结构必须严格对齐,改任一产品级说明就按下方 [README 三语同步 checklist](#readme-三语同步-checklist) 同步三语。
4. **提交前分模块自查。** 改了哪个目录,就跑那个目录的检查(详见各 `AGENTS.md` 的"提交前自查")。

### README 三语同步 checklist

三份 README 的标题结构已知存在漂移(例:中文版有 `### C# SDK(Unity / Godot)` 段,英文/日文版当前缺失)。改任一 README 时,把下列 checklist 跑一遍,杜绝"只改中文忘了翻译"。

**结构对齐(最高优先级)**
- [ ] 三个文件标题层级(`#` 数量、顺序、对应关系)一致。用 `grep -n '^#' README.md README_en.md README_ja.md` 逐行对照,任何一行缺失都要补。
- [ ] 同一章节在 zh/en/ja 行号偏差控制在 ≤3 行(排除翻译本身长度差)。偏差大说明有一版漏改或多了过时内容。

**内容对齐**
- [ ] **分析模型清单**(三大类 25 个):中文的"通用行为分析 10 / 用户深度洞察 9 / 游戏专项 6"——三语模型名一一对应,数量一致。这是最易漏译的硬清单。
- [ ] **技术栈表**(后端 / 管理后台前端 / 数据采集 SDK):库版本、工具名三语一致,只翻译描述文字。
- [ ] **快速开始命令块**:命令本身(`make ...`、`pnpm ...`、`docker ...`)三语**逐字相同**,只注释翻译。新增/修改命令必须三处同步。
- [ ] **项目结构树**:目录注释三语对应,新增目录(如新增 SDK/服务)三处都加。
- [ ] **SDK 接入段**:Web SDK / C# SDK 的代码示例与步骤三语一致。**特别检查:zh 有的子段,en/ja 也要有。**

**翻译质量**
- [ ] 术语统一:同一技术名词在单语内不混译(例:ja 不要一会儿"分析エンジン"一会儿"アナリティクス")。
- [ ] 中→英/日时,保留所有代码、命令、路径、版本号、表名原样不译。
- [ ] 不要机器直译中文标题里的中文符号(`、`、`（）`),按目标语言排版习惯替换。

**自检命令**
```bash
# 快速比对三语标题(一眼看出结构漂移)
cd /d/GoProject/go-wind-uba  # 或项目根
paste <(grep -n '^#' README.md) <(grep -n '^#' README_en.md) <(grep -n '^#' README_ja.md) | less
```
> 任一行出现空白或明显错位,就是漏译/结构漂移点,按上面内容对齐项补齐。

## 6. 典型任务 → 先读哪个文件

| 我要做的事 | 先读 |
|---|---|
| 改后端 API / 数据模型 / 业务逻辑 | `backend/AGENTS.md` |
| 改后端契约,影响前端调用 | `backend/AGENTS.md`(注意 `make api` 生成链)|
| 改管理后台页面 / 组件 / i18n | `frontend/AGENTS.md` |
| 改分析模型(前后端) | `backend/AGENTS.md` + `frontend/AGENTS.md`(分析模型是双端核心)|
| 改 Web 埋点 SDK | `frontend/AGENTS.md`(sdk/web/uba 段)|
| 改 C# SDK | `sdk/csharp/README.md` |
| 理解整体架构 | `docs/architecture.md` + 本文件第 4 节 |

## 7. 常用命令(顶层)

仓库顶层没有统一构建脚本(各模块独立)。典型入口:

```bash
# 后端(在 backend/ 下)
make help && make api && make build

# 前端(在 frontend/admin/ 下)
pnpm install && pnpm dev:antd

# Web SDK(在 frontend/sdk/web/uba/ 下)
pnpm build && pnpm typecheck
```

---

子目录指南才是干活时的细则;本文件只负责让你"找对地方"。
