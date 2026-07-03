# Backend Agent Guide — go-wind-uba

本文件是 AI 编码助手(以及新人)在本目录 `backend/` 下工作时的权威指南。
请先通读,再动手。

## 1. 技术栈

| 维度 | 选型 |
|---|---|
| 语言 | Go 1.25.7 |
| 微服务框架 | [Kratos v2](https://go-kratos.dev/) `go-kratos/kratos/v2` |
| ORM | [Ent](https://entgo.io/) `entgo.io/ent` |
| 依赖注入 | [Google Wire](https://github.com/google/wire)(生成式,编译期) |
| 业务库 | PostgreSQL(`lib/pq`、`jackc/pgx`) |
| 分析库(OLAP) | Apache Doris(`tx7do/go-crud/doris`) |
| 分析库(可选) | ClickHouse(`tx7do/go-crud/clickhouse`) |
| 缓存/队列 | Redis(`redis/go-redis/v9`)、[Asynq](https://github.com/hibiken/asynq) 任务队列 |
| 对象存储 | MinIO(`minio/minio-go`) |
| 鉴权 | JWT(`golang-jwt/jwt/v5`、`tx7do/kratos-authn`)、Casbin/OPA(`tx7do/kratos-authz`) |
| API 协议 | Protocol Buffers(`api/protos/`)→ 生成 Go HTTP/gRPC + OpenAPI + TypeScript |
| 构建/工具链 | Makefile + [buf](https://buf.build/) + 一系列 `protoc-gen-*` 插件 |

## 2. 仓库结构

```
backend/
├── api/
│   ├── protos/          # .proto 契约(唯一真相源,按业务域分目录)
│   └── gen/             # 生成产物:gen/go/*、gen/openapi/* 等 —— 禁止手改
├── app/                 # 三个可独立部署的微服务
│   ├── admin/           # 后台管理服务
│   ├── collector/       # SDK 埋点数据采集服务(SDK 上报入口)
│   └── core/            # 核心服务(含数据分析)
├── pkg/                 # 跨服务复用的基础库(jwt/oss/middleware/crypto ...)
├── sql/                 # 各数据库 DDL + 初始/示例数据
│   ├── postgresql/      # 业务元数据
│   ├── doris/           # 分析聚合表 / Kafka 表 / 视图 / ETL
│   └── clickhouse/      # 同上(可选 OLAP 后端)
├── scripts/             # 部署、docker、env 脚本
└── Makefile             # 顶层编排(api / ent / wire / build / ...)
```

### 单服务 DDD 分层(`app/<svc>/service/internal/`)

```
internal/
├── data/          # 数据访问层
│   ├── ent/       # Ent 生成 + 业务仓储
│   ├── doris/     # Doris 查询(分析模型核心)
│   ├── clickhouse/
│   ├── client/    # 连接客户端
│   └── providers/ # wire Provider
├── server/        # 传输层:HTTP / gRPC 装配
│   └── providers/
└── service/       # 业务用例层(编排 data + server)
    └── providers/
```

每个服务入口:`app/<svc>/service/cmd/server/`(`main.go` + `wire.go` + 生成的 `wire_gen.go`)。

## 3. 核心工作流(务必按顺序)

### 改 API 契约
1. 编辑 `api/protos/<domain>/.../*.proto`(这是唯一真相源)。
2. `make api` —— 跑 buf + protoc 插件,重生成 `api/gen/`、OpenAPI,以及供前端的 TypeScript。
3. **绝对不要手改 `api/gen/` 下任何文件。** 如果生成结果不对,回去改 proto 或插件配置。

### 改数据模型(Ent)
1. 改 Ent schema(通常在 `internal/data/ent/schema/` 或由 `tx7do/go-crud/entgo` 约定的位置)。
2. `make ent` —— 重生成 ent 代码。
3. 若涉及 DDL,同步更新 `sql/postgresql/`(以及必要的 doris/clickhouse 聚合表)。

### 改依赖注入
1. 编辑对应 `wire.go` 中的 `ProviderSet`。
2. `make wire` —— 生成 `wire_gen.go`(`wire_gen.go` **需提交**,它是编译产物)。

### 构建 / 运行
- 顶层:`make build` / `make all`
- 单服务:进 `app/<svc>/service/` 用其 `Makefile`,或 `go run ./cmd/server -conf ./configs`。
- 常用:`make help` 查看全部目标。

## 4. 必须遵守的约定

1. **生成代码神圣不可侵犯。** `api/gen/`、`internal/data/ent/*.go`(非 schema)、`wire_gen.go` 全部是产物。改之前先改源头再重新生成。
2. **Doris 查询是分析核心。** 多数分析模型(漏斗/留存/路径/LTV 等)的查询逻辑在 `app/core/service/internal/data/doris/`。注意:
   - Doris 对 `time.Time` 列扫描敏感(历史上踩过 `EventTrend/ActiveUsers bucket time.Time scan` 的坑),新增扫描逻辑优先用 `*time.Time` 或在 SQL 中 `DATE_FORMAT` / 转字符串。
   - 注意时区:分析查询的时间分桶要与服务/数据库时区一致。
3. **三库职责分明。** PostgreSQL 存业务元数据(用户/租户/配置);Doris/ClickHouse 存行为事件与分析聚合。不要把分析数据写进 PG,也不要在 OLAP 库里做事务。
4. **错误用 Kratos errors。** 通过 `protoc-gen-go-errors` 从 proto 定义错误码,前端依赖这些码做国际化与提示。
5. **配置走 YAML。** 每个 `app/<svc>/service/configs/*.yaml`,本地可用 `.env` 覆盖(见根 `Makefile` 的 `.env` 加载)。不要把密钥写进 yaml。
6. **提交前自查:**
   - `go build ./...` 通过
   - 改了 proto/ent/wire 必须已重新生成并提交生成物
   - `go vet ./...` 无新增告警

## 5. 常用命令速查

```bash
make help            # 列出全部目标
make api             # proto → go/openapi/ts
make ent             # ent 代码生成
make wire            # wire 依赖注入生成(在各 app 下也有同名目标)
make build           # 构建全部服务
make lint            # golangci-lint
make test            # 单测
make vendor          # go mod vendor
```

## 6. 调试提示

- `wire_gen.go` 编译报错 → 多半是 ProviderSet 缺 Provider 或有循环依赖,先看 `wire.go`。
- proto 改了但前端没拿到 → 确认 `make api` 也生成了 TypeScript 输出(`protoc-gen-typescript-http`),前端在 `frontend/admin/apps/admin/src/generated/api/` 消费。
- Doris 查询超时/报错 → 先看 `sql/doris/` 里对应聚合表/视图是否已建,以及 Kafka 物化表是否在消费。
