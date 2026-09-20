##################################
# 第一阶段：构建GO可执行文件
##################################

# 使用官方的 Go 基础镜像作为构建环境
# 1.25.3-alpine 自带的 GOTOOLCHAIN=local，而 go.mod 要求 go >= 1.25.7，本地工具链版本不够会
# 直接编译失败。这里显式放宽：若下面这个标签的 Go 低于 go.mod 的版本，构建期经 GOPROXY 自动拉取
# 匹配的工具链。
# 前提是 sumdb 开着：工具链模块强制走校验数据库，GOSUMDB=off 会让这一步直接报
# "verifying module: checksum database disabled by GOSUMDB=off" 而失败（已实测复现）。
# goproxy.cn 透传 sumdb，所以国内网络照样取得到。把 GO_VERSION 提到 >= go.mod 的版本可以不拉工具链。
ARG GO_VERSION=1.25.3
FROM golang:${GO_VERSION}-alpine AS builder

ENV GOTOOLCHAIN=auto

ARG SERVICE_NAME=app
ARG APP_VERSION=1.0.0

# 设置工作目录
WORKDIR /src

# 复制项目源代码到工作目录
COPY . /src

# 下载依赖，国内代理链：首选 goproxy.cn，备选 direct（直接下载）
# sumdb 保持开启：既校验依赖哈希，也是上面 GOTOOLCHAIN=auto 能拉工具链的前提
RUN GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.org go mod download

# 编译可执行文件（使用WORKDIR和相对路径，而不是cd）
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build -ldflags "-X main.version=$APP_VERSION" \
    -o /src/bin/${SERVICE_NAME}-server ./app/${SERVICE_NAME}/service/cmd/server/

# 数据入仓运维工具：每个服务镜像都带上它，一次性任务容器（apply）与 ETL 调度容器（etl --loop）
# 复用 core-service 的镜像即可，不必为它单独维护一份构建。
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build -ldflags "-X main.version=$APP_VERSION" \
    -o /src/bin/uba-ingest ./cmd/uba-ingest/

# 复制配置文件到统一目录
RUN mkdir -p /src/bin/configs && \
    if [ -d "/src/app/${SERVICE_NAME}/service/configs" ]; then \
      cp -r /src/app/${SERVICE_NAME}/service/configs/* /src/bin/configs/ 2>/dev/null || true; \
    fi

##################################
# 第二阶段：创建最终的运行时镜像
##################################

# 使用 Alpine 作为基础镜像，因为它非常轻量级
FROM docker.io/alpine:latest

ARG SERVICE_NAME=app

# 安装必要的证书（如果应用程序需要进行 HTTPS 请求）
RUN apk --no-cache add ca-certificates

# 设置工作目录
WORKDIR /app

# 从第一阶段的构建结果中复制可执行文件到当前工作目录
COPY --from=builder /src/bin/${SERVICE_NAME}-server /app/bin/server

# 入仓运维工具（./bin/uba-ingest，供 docker compose run 调用）
COPY --from=builder /src/bin/uba-ingest /app/bin/uba-ingest

# 拷贝配置文件
COPY --from=builder /src/bin/configs/ /app/configs/

# 创建一个名为 appuser 的非 root 用户
RUN adduser -D appuser

# 切换到非特权用户
USER appuser:appuser

# 暴露服务端口，根据你的实际服务端口进行修改

# 设置容器启动时执行的命令
CMD ["/app/bin/server", "-c", "/app/configs"]
