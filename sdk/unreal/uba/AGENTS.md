# Agent Guide — sdk/unreal/uba

Unreal Engine 埋点 SDK。**core/ 纯 C++17 核心层（本机 g++ 可验证）+ UnrealPlugin/ 插件壳（源码交付，宿主工程编译，本仓库无 UE 环境不编译）**。

## 铁律

1. 契约单一真相源在后端 proto；camelCase 逐字段对齐 `json_name`，禁止手造字段。
2. `tenantId`/`serverTime`/`ip` 不发送；**数值标量只进 behavior oneof**（uba_core_test.cpp 金样断言整包每个数值键恰好一次）。
3. behavior oneof 必须存在；risk 事件不带 behavior。
4. 重试语义：4xx（除 429）不重试不回队；可重试失败（网络/5xx/429）回队由定时器补发（内存 cap 10000）。
5. 零依赖：核心层禁引 UE 头/第三方库；传输（IUbaTransport）与时钟（IUbaClock）注入。
6. 队列存 wire JSON 字符串（回队补发无损）；`std::optional` 表示可空契约字段。
7. 默认值对齐其他端：批量 20/5s/3 次/1s 退避/8s 超时（Validate 校验）。

## 提交前自查

```bash
cd sdk/unreal/uba
g++ -std=c++17 -Icore core/uba_core.cpp core/uba_core_test.cpp -o /tmp/uba_tests && /tmp/uba_tests
# 输出 ALL C++ CORE TESTS PASSED
```

改 JSON 构造/Client → 必跑金样；UnrealPlugin/ 只做桥接与生命周期，业务逻辑不进插件壳。
