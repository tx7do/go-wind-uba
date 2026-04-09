# 部署Superset

## Docker run

```shell
# 拉取并运行Superset的Docker镜像
docker run -d \
  --name superset \
  --restart always \
  -p 8088:8088 \
  -e TZ=Asia/Shanghai \
  -e SUPERSET_SECRET_KEY=*Abcd123456 \
  --user root \
  apache/superset:latest
```
```shell
# 进入Superset容器
docker exec -it superset bash

# 替换apt源为阿里云镜像源
sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources
sed -i 's/security.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources
apt-get update
# 安装编译pydoris和pymysql所需的依赖
apt-get install -y gcc python3-dev default-libmysqlclient-dev pkg-config

# 给官方阉割的虚拟环境 安装 pip
/app/.venv/bin/python -m ensurepip

# 升级 pip
/app/.venv/bin/python -m pip install --upgrade pip -i https://mirrors.aliyun.com/pypi/simple/

# 安装 pymysql 和 pydoris 到 Superset 真正使用的环境里
/app/.venv/bin/python -m pip install pymysql pydoris -i https://mirrors.aliyun.com/pypi/simple/

# 验证安装
/app/.venv/bin/pip list | grep pydoris
```

```shell
# 将本地数据库迁移到最新版本
docker exec -it superset superset db upgrade

# 设置本地superset系统管理员账户
docker exec -it superset superset fab create-admin \
              --username admin \
              --password admin \
              --firstname Superset \
              --lastname Admin \
              --email admin@superset.com

# 初始化设置角色
docker exec -it superset superset init

# 重启Superset容器
docker restart superset
```

> 访问地址：<http://localhost:8080/login/>
>
> 登录账号：[admin/admin]

## Docker compose

```yaml
networks:
  app-tier:
    driver: bridge

services:
  superset:
    image: apache/superset:latest
    container_name: superset
    hostname: superset
    restart: always
    user: root
    ports:
      - "8088:8088"
    networks:
      - app-tier
    environment:
      TZ: Asia/Shanghai
      SUPERSET_SECRET_KEY: "*Abcd123456"
      SUPERSET_ENV: production
    volumes:
      - ./superset_data:/app/data
    command: >
      bash -c "
      apt-get update &&
      apt-get install -y gcc python3-dev default-libmysqlclient-dev pkg-config &&
      /app/.venv/bin/python -m ensurepip &&
      /app/.venv/bin/python -m pip install --upgrade pip -i https://mirrors.aliyun.com/pypi/simple/ &&
      /app/.venv/bin/python -m pip install pymysql pydoris -i https://mirrors.aliyun.com/pypi/simple/ && 
      superset db upgrade &&
      superset fab create-admin
          --username admin
          --password admin
          --firstname Admin
          --lastname Admin
          --email admin@admin.com || true &&
      superset init &&
      /usr/bin/run-server.sh
      "
```

## 配置数据库

完成登录后，点击右上角 `Settings` -> `Database Connectors`

点击 添加 Database，在 Connect a database 弹窗上，选择 Apache Doris：

```shell
# 使用Doris驱动
pydoris://root:@host.docker.internal:9030/internal.gw_uba

# 使用MySQL驱动
mysql+pymysql://root:@host.docker.internal:9030/gw_uba
```

## 参考资料

- [Doris 官方文档](https://doris.apache.org/zh-CN/docs/3.x/ecosystem/bi/apache-superset)
- [Superset Connecting to Databases](https://superset.apache.org/user-docs/databases/)
