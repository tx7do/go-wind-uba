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
├── cmd/
│   └── uba-ingest/      # 入仓运维 CLI(Doris schema / Routine Load / ETL 调度 / 健康度)
├── pkg/                 # 跨服务复用的基础库(jwt/oss/middleware/crypto/dorisinit ...)
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
- 入仓运维 CLI:`make ingest` → `bin/uba-ingest`(详见第 6 节)。
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

## 6. 数据入仓:`cmd/uba-ingest` + `pkg/dorisinit`

SDK 事件先由 `collector` 发到 kafka(`pkg/topic`:`uba_events_raw`、`uba_risk_events`),
**从 kafka 搬进 Doris 由 Doris 自己的 Routine Load 完成** —— 不写 Go 消费者是有意的设计:
FE 负责调度、并发、重试与位点,Go 侧只做"装配 → 调度 → 观测"这三段,入口是 `uba-ingest`。

| 子命令 | 干什么 |
|---|---|
| `apply` | 按 `sql/doris/` 的嵌入脚本建库表,并确保声明的 Routine Load 任务在跑(幂等;`--wait 5m` 等 Doris 起来) |
| `status` | 列出任务状态 / 位点延迟 / 错误计数;脚本声明了但没在消费 → 退出码 1(可直接当 healthcheck) |
| `etl` | 跑 `06_etl.sql` 的编号小节;`--loop --at 02:00` 让它常驻代管每日调度 |
| `render` | 只做模板渲染、不连库,产物可粘进 Doris 客户端核对 |

纯逻辑(语句切分、分类、决策)在 `pkg/dorisinit`,`sql/doris` 用 `go:embed` 编进二进制,
所以 slim 运行镜像里没有 `sql/` 目录也能跑。

**DSN 与 broker 只从 configs 或环境变量来,绝不上命令行**(命令行值会进 `ps` 与 shell 历史),
打印时 DSN 一律脱敏:

```bash
export UBA_DORIS_DSN='root:***@tcp(doris-fe:9030)/gw_uba'   # 覆盖 data.doris.dsn
export UBA_KAFKA_BROKERS=kafka:9092                          # 覆盖 data.kafka.endpoints
./bin/uba-ingest -c app/core/service/configs apply --wait 5m
./bin/uba-ingest -c app/core/service/configs status --json
```

退出码:`0` 完成 / `1` 失败 / `2` 需要人工(任务处于 `STOPPED|CANCELLED|FINISHED` 终态,自动
重建会重放整个 topic)/ `3` 用法或配置错误。stdout 只有机器可读产物(`render`、`status --json`),
给人看的一律走 stderr。

compose 已接好:`ingest`(一次性 `apply`)与 `ingest-etl`(`etl --loop`,healthcheck 就是 `status`)。

改管道时的四条硬规矩:
1. **`sql/doris/*.sql` 不许带 BOM、统一 LF**(BOM 紧跟 `--` 会让 Doris 解析失败)。守门测试在
   `sql/doris/assets_test.go`,规约在根 `.gitattributes`。
2. 模板占位符只有 `{{.KafkaBrokerList}}` 与 `{{.RunDate}}` 两个,别再加一个 —— 参数越多,
   脚本渲染出错误 SQL 的面越大。
3. `DROP/STOP ROUTINE LOAD` 会丢位点,默认被 `apply` 拒绝;真要重放才加
   `--allow-routine-load-mutations`。
4. `06_etl.sql` 的 §3~§5 写 AGGREGATE 表,同一天的第二次 INSERT 是**累加**不是覆盖;所以
   `etl` 默认只跑幂等的 §1/§2,要跑聚合小节就得保证一天只算一次。

## 7. 调试提示

- `wire_gen.go` 编译报错 → 多半是 ProviderSet 缺 Provider 或有循环依赖,先看 `wire.go`。
- proto 改了但前端没拿到 → 确认 `make api` 也生成了 TypeScript 输出(`protoc-gen-typescript-http`),前端在 `frontend/admin/apps/admin/src/generated/api/` 消费。
- Doris 查询超时/报错 → 先看 `sql/doris/` 里对应聚合表/视图是否已建,以及 Kafka 物化表是否在
  消费 —— 后者一句 `uba-ingest status` 就有状态、位点延迟和错误计数,不用手写 SHOW。
