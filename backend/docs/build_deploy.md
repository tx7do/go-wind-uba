# 编译和部署

## 生成Protobuf API

使用[buf.build](https://buf.build/)进行Protobuf API的构建。

相关命令行工具和插件的具体安装方法请参见：[Kratos微服务框架API工程化指南](https://juejin.cn/post/7191095845096259641)

在`blog-backend`根目录下执行命令：

- 更新buf.lock

    ```bash
    buf mod update
    ```

- 生成GO代码

    ```bash
    buf generate
    ```

- 生成OpenAPI v3文档

    ```bash
    buf generate --path api/admin/service/v1 --template api/admin/service/v1/buf.openapi.gen.yaml
    ```

## Make构建

请在`app/{服务名}/service`下执行：

- 初始化开发环境

   ```bash
   make init
   ```

- 生成API的go代码

   ```bash
   make api
   ```

- 生成API的OpenAPI v3 文档

   ```bash
   make openapi
   ```

- 生成服务器配置结构代码

   ```bash
   make conf
   ```

- 生成ent代码

   ```bash
   make ent
   ```

- 生成wire代码

   ```bash
   make wire
   ```

- 构建程序

   ```bash
   make build
   ```

- 构建Docker镜像

   ```bash
   make docker
   ```

## Bazel构建

使用[bazel.build](https://bazel.build/)进行服务器程序的构建。

如何安装bazel.build的文档，请参考官方文档：<https://bazel.build/install>。

在`blog-backend`根目录下执行命令：

- 更新GO依赖库引入的构建配置文件repos.bzl

   ```bash
   bazel run //:gazelle-update-repos
   ```

- 拉取依赖项，生成配置文件BUILD.bazel

   ```bash
   bazel run //:gazelle
   ```

- 构建单个服务

  ```bash
  bazel build //app/admin/service/cmd/server:server
  bazel build //app/core/service/cmd/server:server
  bazel build //app/agent/service/cmd/server:server
  ```

  或者

  ```bash
  bazel build //:admin-service
  bazel build //:core-service
  bazel build //:agent-service
  ```

- 运行单个服务

  ```bash
  bazel run //app/admin/service/cmd/server:server
  bazel run //app/core/service/cmd/server:server
  bazel run //app/agent/service/cmd/server:server
  ```

  或者

  ```bash
  bazel run //:admin-service
  bazel run //:core-service
  bazel run //:agent-service
  ```

- 单个服务生成Docker镜像tar文件

  ```bash
  bazel build //:admin-service-image
  bazel build //:core-service-image
  bazel build //:agent-service-image
  ```

- 单个服务生成本地Docker镜像

  ```bash
  bazel run //:admin-service-image
  bazel run //:core-service-image
  bazel run //:agent-service-image
  ```

- 单个服务推送Docker镜像到DockerHub

  ```bash
  bazel run //:admin-service-image-push
  bazel run //:core-service-image-push
  bazel run //:agent-service-image-push
  ```

- 构建全部服务

  ```bash
  bazel build //...
  ```

## Docker部署
