# 二次开发导引

本文档指导开发者如何在 go-wind-uba 上进行二次开发：环境准备、代码生成管线、以及四类最常见的扩展场景（新增对外服务、新增业务实体、新增分析聚合、新增前端页面）。

> 本文档的每一步均来自真实操作验证，命令可直接复制执行。

---

## 一、代码生成管线

项目采用**契约优先**：先写 `.proto` / ent schema，再生成 Go / TypeScript / OpenAPI 代码。理解这条管线是二开的前提。

### 1.1 工具链

```bash
cd backend
make init    # 安装 protoc 插件 + CLI 工具（buf / wire / ent 等）
```

| 工具 | 用途 |
|------|------|
| `buf` | proto 编译（替代 protoc） |
| `protoc-gen-go` / `protoc-gen-go-grpc` | Go 消息 + gRPC stub |
| `protoc-gen-go-http` | kratos REST handler |
| `protoc-gen-typescript-http` | admin 前端 TS 客户端 |
| `protoc-gen-openapi` | Swagger 文档 |
| `wire` | 编译时依赖注入 |
| `ent` | ORM 实体代码生成 |

### 1.2 核心命令（在 `backend/` 下执行）

```bash
make api       # 生成 Go（proto → gen/go/）+ struct tag
make ts        # 生成前端 TS 客户端（仅 admin/service/v1 作为输入）
make openapi   # 生成 OpenAPI / Swagger
make ent       # 生成 ent 实体（在各 service 目录下）
make wire      # 重新生成依赖注入（wire_gen.go）
make gen       # = ent + wire + api + openapi（不含 ts）
make build     # 编译所有服务
```

### 1.3 关键配置文件

| 文件 | 作用 |
|------|------|
| `backend/api/buf.yaml` | buf 模块定义、依赖 |
| `backend/api/buf.gen.yaml` | Go 生成配置（managed mode 注入 go_package） |
| `backend/api/buf.admin.typescript.gen.yaml` | **TS 生成配置**，注意 `inputs.paths` 只含 `protos/admin/service/v1` —— **只有 admin proto 才会生成 TS 客户端** |
| `backend/app/core/service/app.mk` | ent 生成命令（含 feature flags：privacy/entql/sql/modifier 等） |

### 1.4 ⚠️ ent 生成的坑（重要）

**不要直接跑 `ent generate ./schema`**！会丢失项目的扩展（privacy / sql modifier），导致生成的查询方法缺少 `Modify`/`Filter`，编译报错。

正确方式（见 `backend/app/core/service/app.mk`）：

```bash
cd backend/app/core/service
ent generate \
  --feature privacy --feature entql \
  --feature sql/modifier --feature sql/upsert --feature sql/lock \
  ./internal/data/ent/schema
```

或直接 `make ent`（在 core/service 目录）。

### 1.5 TS 生成产物的同步

`make ts` 输出到 `frontend/admin/apps/admin/src/api/generated/admin/service/v1/index.ts`，但 composables 实际导入的是 `src/generated/api/admin/service/v1/index.ts`。**生成后需手动同步**：

```bash
cp frontend/admin/apps/admin/src/api/generated/admin/service/v1/index.ts \
   frontend/admin/apps/admin/src/generated/api/admin/service/v1/index.ts
```

---

## 二、场景 A：新增一个对外服务（admin HTTP 能力）

以「数据分析 AnalyticsService」为例（项目内已实现，可参照）。目标：暴露一组 admin REST 接口，业务逻辑在 core。

### 步骤

**1. 定义领域 proto**（`backend/api/protos/uba/service/v1/xxx.proto`）
- `package uba.service.v1;`
- 定义 `service XxxService { rpc ... }`（gRPC 契约，无 http 注解）
- 定义所有 `message XxxRequest/Response`

**2. 定义 admin 网关 proto**（`backend/api/protos/admin/service/v1/i_xxx.proto`）
- `package admin.service.v1;`
- `import "uba/service/v1/xxx.proto";`
- 定义 `service XxxService`，每个 rpc 加 `option (google.api.http)`：
  - GET 查询用 query 参数：`get: "/admin/v1/xxx"`
  - POST 聚合用 body：`post: "/admin/v1/xxx" body: "*"`

**3. 生成代码**
```bash
cd backend && make api && make ts
# 同步 TS（见 1.5）
```

**4. core 层实现业务**（`backend/app/core/service/`）
- `internal/service/xxx_service.go`：实现 `ubaV1.UnimplementedXxxServiceServer`
- `internal/data/` 下加 repo（ent 实体走 `go-crud` Repository；OLAP 聚合走原生 SQL）
- `internal/server/grpc_server.go`：`NewGrpcServer` 形参加 service，调 `ubaV1.RegisterXxxServiceServer`
- `internal/service/providers/wire_set.go` + `internal/data/providers/wire_set.go`：加 provider
- `make wire`（在 core/service 目录）

**5. admin 层转发**（`backend/app/admin/service/`）
- `internal/service/xxx_service.go`：实现 `adminV1.XxxServiceHTTPServer`，方法体 `return s.client.Method(ctx, req)`
- `internal/data/data.go`：加 `NewXxxServiceClient`（仿已有 client 工厂，经 etcd 发现 core）
- `internal/data/providers/wire_set.go` + `internal/service/providers/wire_set.go`：加 provider
- `internal/server/rest_server.go`：`NewRESTServer` 形参加 service，调 `adminV1.RegisterXxxServiceHTTPServer`
- `make wire`（在 admin/service 目录）

**6. 验证**
```bash
cd backend && go build ./...
```

---

## 三、场景 B：新增业务实体（PostgreSQL + ent）

以「事件 Schema EventSchema」为例（项目内已实现）。

### 步骤

**1. 定义 ent schema**（`backend/app/core/service/internal/data/ent/schema/uba_xxx.go`）
- 定义 `type Xxx struct{ ent.Schema }`
- `Fields()`：字段（注意 Nillable / Optional / Default）
- `Mixin()`：复用 `mixin.AutoIncrementId{}` / `mixin.TimeAt{}` / `mixin.OperatorID{}` / `mixin.TenantID[T]{}`
- `Indexes()`：唯一索引 + 普通索引
- `Annotations()`：表名、字符集

**2. 生成 ent 代码**（用正确命令，见 1.4）
```bash
cd backend/app/core/service
ent generate --feature privacy --feature entql --feature sql/modifier --feature sql/upsert --feature sql/lock ./internal/data/ent/schema
```

**3. 定义 proto**（领域 `uba/.../xxx.proto` + admin 网关 `admin/.../i_xxx.proto`），含完整 CRUD（List/Count/Get/Create/Update/Delete）。

**4. 生成 proto 代码**：`make api && make ts`，同步 TS。

**5. 实现 repo**（`backend/app/core/service/internal/data/uba_xxx_repo.go`）
- 仿 `uba_tag_definition_repo.go` / `uba_event_schema_repo.go`
- 用 `entCrud.Repository[...]` 泛型 + `mapper.CopierMapper`
- `Count` / `List` 用 `BuildListSelectorWithPaging`
- 注意 ID 类型：ent 默认 `uint32`，proto 用 `uint64`，需 `uint32(req.GetId())` 转换

**6. 实现 core service + admin 转发**（同场景 A 的步骤 4-5）。

---

## 四、场景 C：新增分析聚合（OLAP）

以「漏斗/留存/趋势」为例（项目内已实现）。

### 关键点

- **聚合查询走原生 SQL**，不用 ent。Doris 用 `r.db.SelectContext(ctx, &rows, sql, args...)`，ClickHouse 用 `r.db.Select(ctx, &rows, sql, args...)`（注意两套 API 略有差异）。
- **双引擎镜像**：在 `internal/data/doris/` 和 `internal/data/clickhouse/` 各实现一份，SQL 函数按方言调整（如 Doris 用 `DATE_FORMAT`，ClickHouse 用 `toStartOfHour`/`toDate`）。
- **防 SQL 注入**：维度字段走**白名单 map**，metric 走 switch，数值用 `%d` 强转后拼接。
- **service 层按 `data.UseClickHouse` 分支**选 repo。
- 聚合请求/响应消息参考已有的 `RiskEventSummary`、`AnalyticsService` 设计。

---

## 五、场景 D：新增前端页面（admin）

以「漏斗分析页」为例（项目内已实现）。

### 步骤

**1. 加 API composable**（`frontend/admin/apps/admin/src/api/composables/xxx.ts`）
- 三种导出范式：
  - `useListXxx(query, options)` —— 响应式（vue-query useQuery）
  - `fetchListXxx(params)` —— 命令式（VxeGrid proxyConfig.ajax.query 用）
  - `useCreateXxx` / `useUpdateXxx` / `useDeleteXxx` —— mutation
- 调用生成的 `apiClient.xxxService.Method(...)`
- 在 `composables/index.ts` 加 `export * from './xxx';`

**2. 加页面视图**（`frontend/admin/apps/admin/src/views/app/<module>/<page>/index.vue`）
- 列表页：`useVbenVxeGrid` + `<Page>` + `<Grid>`
- BI 页：`@vben/plugins/echarts` 的 `EchartsUI` + `useEcharts` + `AnalysisChartCard`（来自 `@vben/common-ui`）
- 表单抽屉：`useVbenDrawer` + `useVbenForm`

**3. 加路由**（`frontend/admin/apps/admin/src/router/routes/modules/app/<module>.ts`）
- 父级 `component: BasicLayout`，`meta.order` 控制菜单位置
- 子路由懒加载，`meta.icon` 用 `lucide:*` 字符串，`meta.authority` 控制权限
- **`modules/app/*.ts` 下任何新 .ts 文件自动被 `import.meta.glob` 收录**，无需手动注册

**4. 加 i18n**（`frontend/admin/apps/admin/src/locales/langs/{zh-CN,en-US}/`）
- `menu.json`：菜单标题（`menu.<module>.<key>`）
- `page.json`：页面文案（字段标签、按钮、表格列名）

**5. ECharts 图表类型**

`@vben/plugins/echarts` 默认注册了 line/bar/pie/radar。**漏斗/热力图需在 `packages/effects/plugins/src/echarts/echarts.ts` 的 `echarts.use([])` 追加注册**：
```ts
import { FunnelChart, HeatmapChart } from 'echarts/charts';
import { VisualMapComponent } from 'echarts/components';
echarts.use([FunnelChart, HeatmapChart, VisualMapComponent]);
```

**6. 验证**
```bash
cd frontend/admin/apps/admin
npx vue-tsc --noEmit --skipLibCheck    # 类型检查
cd frontend/admin
npx eslint <你改的文件> --fix            # lint
```

---

## 六、目录结构速查

```
backend/
├── api/
│   ├── protos/
│   │   ├── admin/service/v1/      # admin HTTP 网关 proto（生成 TS 客户端的唯一输入）
│   │   ├── uba/service/v1/         # UBA 领域消息 + gRPC 服务契约
│   │   └── <其他领域>/             # identity/permission/dict/...
│   └── gen/go/                     # buf 生成的 Go 代码
├── app/
│   ├── admin/service/              # Admin 服务（HTTP BFF，薄转发）
│   ├── collector/service/          # Collector 服务（采集 BFF）
│   └── core/service/               # Core 服务（核心业务）
│       └── internal/
│           ├── data/
│           │   ├── ent/schema/     # ent 实体定义（改这里 → make ent）
│           │   ├── doris/          # Doris repo（含 schema/ 事实表定义）
│           │   └── clickhouse/     # ClickHouse repo
│           ├── service/            # 业务 service 实现
│           └── server/             # grpc/rest server 注册
├── pkg/                            # 公共包
└── Makefile                        # 代码生成 / 构建命令

frontend/admin/
├── apps/admin/src/
│   ├── api/composables/            # API 组合式函数（加 composable）
│   ├── generated/api/admin/service/v1/index.ts  # 生成的 TS 客户端（实际导入）
│   ├── router/routes/modules/app/  # 路由模块（加 .ts 自动收录）
│   ├── locales/langs/              # i18n
│   └── views/app/                  # 页面视图（加页面）
└── packages/effects/plugins/src/echarts/  # ECharts 注册
```

---

## 七、调试技巧

- **联调 admin 前端**：`cd frontend/admin && pnpm dev`，后端 admin 跑 `5600`。
- **联调 SDK 上报**：`go run ./app/collector/service/cmd/server/`（5700），用 SDK 的 `test.html`。
- **gRPC 调试**：core 端口动态，用 etcd 查注册；或临时把 `server.yaml` 的 grpc addr 改成固定端口。
- **proto 改完没生效？** 多数是忘了 `make ts` + 同步 TS（1.5），或忘了 `make wire`。
- **ent 编译报 Modify 缺失？** 用了错误的 ent 生成命令（见 1.4）。

---

## 八、相关文档

- [系统架构](architecture.md)
- [SDK 接入指南](sdk_integration.md)
- [部署文档](../backend/docs/build_deploy.md)
