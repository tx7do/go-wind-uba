# Agent Guide — sdk/android/uba

Android 原生埋点 SDK（Kotlin）。**核心层纯 Kotlin（无 android.* 依赖，JVM 可测）**；`android/` 目录是适配层源码（宿主 App 内编译，本仓库无 Android SDK 不编译它）。

## 分层

- `src/main/kotlin/com/gowind/uba/`：核心（UbaClient/UbaConfig/Types/UbaJson/HttpTransport/KeyValueStore/SessionManager）。改核心只影响 JVM 测试。
- `android/UbaAndroid.kt`：适配层（SharedPreferencesStore + ActivityLifecycleCallbacks 自动 page_view/前后台 flush）。**只做注入与生命周期桥接，业务逻辑不许下沉到这里**。

## 铁律

1. 契约单一真相源在后端 proto；camelCase 逐字段对齐 `json_name`，禁止手造字段。
2. `tenantId`/`serverTime`/`ip` 不发送；**数值标量只进 behavior oneof**（UbaClientTest 金样断言整包每个数值键恰好一次）。
3. behavior oneof 必须存在；risk 事件不带 behavior。
4. 重试语义：4xx（除 429）不重试不落盘；可重试失败落盘（SharedPreferences cap 500）补发。
5. **离线补发按 wire JSON 原样保存、恢复后直接重发（无损）**——不要改成反序列化重建 ReportEvent。
6. 零依赖：HttpURLConnection + 手写 JSON writer；传输/存储接口可注入。
7. 默认值对齐其他端：批量 20/5s/3 次/1s 退避/8s 超时。

## 提交前自查

```bash
cd sdk/android/uba
gradle test   # JVM 核心层全绿（本机验证范围 = 核心层 + 测试；android/ 适配层需宿主工程编译）
```

改 UbaJson/UbaClient 序列化 → 必跑金样；改离线补发 → 必跑 retryExhaustedRestoresFromStore。
