# UBA Unreal Engine SDK（C++）

GoWind UBA 风行用户行为分析平台的 **Unreal Engine 埋点 SDK**（PC/主机游戏的唯一覆盖端，服务端补齐游戏专项 6 模型的 UE 侧）。
对接 collector `POST /uba/v1/report`，鉴权在请求 body，JSON camelCase，无 gzip，单批 < 1MiB。

分层交付：**`core/` 纯 C++17 核心层（无 UE 依赖，已本机 g++ 测试全绿）+ `UnrealPlugin/` UE 插件壳（源码交付，宿主工程编译）**。

## 结构

```
sdk/unreal/uba/
├── core/                    # 纯 C++17 核心（零依赖，任何 C++ 宿主可用）
│   ├── uba_core.h           # UbaConfig/UbaClient/TrackOptions/RiskEvent + IUbaTransport 注入接口
│   ├── uba_core.cpp         # 手写 JSON writer + 队列 + 定时/定量 flush + 指数退避重试
│   └── uba_core_test.cpp    # 手写断言测试：契约金样 + 重试/回队/并发（零测试框架依赖）
└── UnrealPlugin/            # UE 插件壳（源码交付）
    ├── Uba.uplugin
    └── Source/Uba/
        ├── Uba.Build.cs                     # 编译核心层 + HTTP 模块依赖
        ├── Public/UbaSubsystem.h            # UEngineSubsystem 门面（Blueprint 可调用）
        └── Private/UbaSubsystem.cpp         # FHttpModule 传输桥接 + Saved/ 设备 ID 持久化
```

## 特性

- ✅ 核心层零依赖（纯 C++17），`IUbaTransport`/`IUbaClock` 注入接口，测试与平台替换均可
- ✅ 批量上报 + 指数退避重试；4xx（除 429）不重试；可重试失败回队由定时器补发（内存 cap 10000）
- ✅ 会话级确定性采样；AB 开关 + `$feature_flag` 曝光去重
- ✅ UE 插件壳：FHttpModule 桥接（promise/future 同步化）、Saved/Uba/device_id.txt 持久化、首装 `app_launch`

## 快速开始

### 1. 纯 C++ 宿主

```bash
g++ -std=c++17 -Icore core/uba_core.cpp your_client.cpp   # 需自行实现 IUbaTransport
```

### 2. Unreal 工程

1. 把 `UnrealPlugin/` 拷入工程的 `Plugins/` 目录（或注册到 .uproject）。
2. C++ 里初始化：

```cpp
#include "UbaSubsystem.h"

FUbaSettings Settings;
Settings.AppId = TEXT("your_app_id");
Settings.AppSecret = TEXT("your_app_secret");
Settings.Endpoint = TEXT("http://collector.example.com:5700");
Settings.AppVersion = FApp::GetProjectVersion();
Settings.Debug = true;
GEngine->GetEngineSubsystem<UUbaSubsystem>()->Init(Settings);
```

3. Blueprint：启用插件后在 **Uba** 分类下调用 `Track` / `TrackRisk` / `Identify` / `Flush` 节点。

### 3. 测试（核心层，本机可跑）

```bash
cd sdk/unreal/uba
g++ -std=c++17 -Icore core/uba_core.cpp core/uba_core_test.cpp -o uba_tests && ./uba_tests
# 输出 ALL C++ CORE TESTS PASSED
```

> UE 插件壳需宿主工程编译验证（本仓库无 UE 环境，不在此编译）；`Flush` 为同步语义（与其他端一致），勿在游戏线程高频调用——建议在退后台/关卡切换/退出时机调用。

## 契约约束

- 字段 camelCase 与后端 proto `json_name` 逐一对齐，**禁止手造字段**
- `tenantId`/`serverTime`/`ip` 服务端补全不发送；数值标量只进 behavior oneof（金样锁定整包每个数值键恰好一次）
- 鉴权在 body；单批压在 1MiB 内；超时必须 < 服务端 10s（默认 8s）
- 4xx（除 429）不重试不回队；可重试失败回队补发
