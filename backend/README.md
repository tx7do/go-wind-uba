# GoWind 用户行为分析系统 - 后端

## 项目简介

GoWind 是一套面向多租户、可扩展的用户行为分析系统，支持实时埋点采集、数据仓库分析、风险识别、标签管理等功能。后端采用 Go
语言开发，支持高性能数据写入和多维分析。

## 后端组成

1. **Core Service**：核心服务，负责数据采集、存储、分析、风险识别等核心业务逻辑。
2. **Admin Service**：系统后台的 BFF，提供管理、配置、监控、报表等功能。
3. **Collector Service**：埋点上报的 BFF，负责接收客户端埋点数据、校验、转发。

## 主要功能

- 用户行为事件采集与存储（ClickHouse/Doris）
- 用户、对象、会话、标签等多维度建模
- 风险事件识别与规则引擎
- 标签管理与画像分析
- 实时与离线数据分析
- 多租户隔离与权限管理
- 支持 Kafka、Redis、MinIO 等组件集成

## 目录结构

```
backend/
  app/           # 各服务代码（core/admin/collector）
  api/           # proto、接口定义、代码生成
  pkg/           # 公共库、工具包
  sql/           # 数据库建表、迁移脚本
  docs/          # 文档、部署说明
  scripts/       # 部署、运维脚本
```

## 环境要求

- Go 1.20+
- ClickHouse 25.0+ 或 Apache Doris 4.0+
- PostgreSQL 12+
- Redis 7+
- Kafka 2.8+
- MinIO 2022+
- Consul/Etcd/Jaeger（可选）

## 部署流程

1. 安装依赖组件（数据库、消息队列、对象存储等）
2. 执行 `sql/clickhouse/` 和 `sql/doris/` 下的建表脚本
3. 配置环境变量和配置文件（参考 `app/core/service/configs/`）
4. 编译并启动各服务（可用 Makefile 或脚本自动化）
5. 通过 Admin Service 管理租户、标签、规则等

## 后端依赖的第三方组件

- [PostgreSQL](https://www.postgresql.org/)
- [ClickHouse](https://clickhouse.com/) / [Doris](https://doris.apache.org/)
- [Redis](https://redis.io/)
- [Kafka](https://kafka.apache.org/)
- [Consul](https://www.consul.io/) / [Etcd](https://etcd.io/)
- [Jaeger](https://www.jaegertracing.io/)
- [MinIO](https://min.io/)

## 常见问题

- 数据库建表脚本需与 entity/schema 保持一致
- DTO/entity 字段类型需严格匹配，建议用 copier/mapper 自动转换
- 多租户环境下需配置租户隔离参数
- Kafka/MinIO 等组件需提前初始化并配置权限

## 参考资料

- [Business Intelligence in Microservices: Improving Performance](https://dzone.com/articles/business-intelligence-in-microservices-improving-p)
- [基于 ClickHouse 高性能引擎集群构建数据湖](https://toutiao.io/posts/pklw5vz/preview)
- [ClickHouse整合Kafka](https://learn-bigdata.incubator.edurt.io/docs/ClickHouse/Action/engine-kafka/)
- [Apply CDC from MySQL to ClickHouse](https://medium.com/@hoptical/apply-cdc-from-mysql-to-clickhouse-d660873311c7)
- [ClickHouse 在实时场景的应用和优化](https://mp.weixin.qq.com/s/hqUCFSr8cu3x3u8HCA6WYg)
- [ClickHouse基础&实践&调优全视角解析](https://xie.infoq.cn/article/37886f3baca09057580bdd5aa)
- [从维护几百张表到只需维护一张表，一个UEI模型就够了](https://zhuanlan.zhihu.com/p/623182999)
- [BI花5天完成的分析，UBA只需30秒](https://zhuanlan.zhihu.com/p/629574865)
