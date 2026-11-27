# UBA Android SDK（Kotlin）

GoWind UBA 风行用户行为分析平台的 **Android 原生埋点采集 SDK**。
对接 collector `POST /uba/v1/report`，鉴权在请求 body（`appId` + `appSecret`），JSON camelCase，无 gzip，单批 < 1MiB。
**核心零依赖**（纯 Kotlin：HttpURLConnection + 手写 JSON writer），Android 适配层源码随宿主 App 编译。

## 结构

```
sdk/android/uba/
├── src/main/kotlin/com/gowind/uba/     # 核心层：纯 Kotlin，JVM 可测（无 android.* 依赖）
│   ├── UbaClient.kt      # 客户端：队列 + 定时/定量 flush + 重试退避 + AB 开关（线程安全）
│   ├── UbaConfig.kt      # 配置（data class + init 校验，默认值对齐其他端）
│   ├── Types.kt          # BehaviorPayload/RiskEvent/TrackOptions（数值标量只进 oneof）
│   ├── UbaJson.kt        # 手写零依赖 JSON writer（跳 null/空集合）
│   ├── HttpTransport.kt  # HttpURLConnection 默认实现 + 注入接口
│   ├── KeyValueStore.kt  # 存储抽象 + InMemoryStore（键名 __uba_*__ 对齐 Web/Unity/Flutter）
│   └── SessionManager.kt # 后台 30s 超时轮换（nowProvider 可注入时钟）
├── src/test/kotlin/      # kotlin.test：契约金样 + 桩服务行为测试（重试/落盘/恢复）
└── android/              # Android 适配层源码（宿主 App 内编译，不参与本仓库 JVM 构建）
    ├── UbaAndroid.kt     # init 入口 + SharedPreferencesStore
    └── （同文件）UbaActivityLifecycleCallbacks  # 前后台自动 flush + Activity 自动 page_view
```

## 特性

- ✅ 批量上报 + 指数退避重试；4xx（除 429）不重试不落盘；可重试失败落盘补发（SharedPreferences，cap 500）
- ✅ 会话轮换（后台 30s 超时，`onActivityStarted/Stopped` 计数判定前后台）+ 会话级确定性采样 + consent 门控
- ✅ 首装 `app_launch` + Activity 自动 `page_view`（含停留时长与来源页）
- ✅ AB 开关 + `$feature_flag` 曝光去重
- ✅ 离线补发按 wire JSON 原样保存（无损重发，无需反序列化）

## 快速开始

### 1. 引入核心层

把本目录作为 Gradle 子项目或 AAR 引入宿主（产物 `com.gowind.uba:uba-android:1.0.0`）。

### 2. 接入适配层

把 `android/UbaAndroid.kt` 拷入宿主 App 模块（无需额外依赖），在自定义 Application 里初始化：

```kotlin
class MyApp : Application() {
    override fun onCreate() {
        super.onCreate()
        UbaAndroid.init(this, UbaConfig(
            appId = "your_app_id",
            appSecret = "your_app_secret",
            endpoint = "http://collector.example.com:5700",
            appVersion = BuildConfig.VERSION_NAME,
            channel = "official",
            autoTrackLaunch = true,   // 首装自动埋 app_launch
            debug = true,             // 联调期打开
        ))
    }
}
```

之后任意位置：

```kotlin
UbaAndroid.client?.track(
    "button_click",
    mapOf("scene" to "home"),
    TrackOptions(objectType = "button", objectId = "btn_buy"),
)
UbaAndroid.client?.trackRisk(
    "abnormal_click",
    RiskEvent(riskType = "device_anomaly", riskLevel = "HIGH", riskScore = 85.0),
)
UbaAndroid.client?.identify(1001)
```

页面浏览由 `UbaActivityLifecycleCallbacks` 自动埋点（Activity resumed → `page_view`，含停留时长）；
前后台切换自动 flush / 会话轮换，无需手工接入。

### 纯 JVM 环境（测试/非 Android 宿主）

```kotlin
val client = UbaClient(UbaConfig(appId = "...", appSecret = "...", endpoint = "..."), InMemoryStore())
```

## 构建

```bash
cd sdk/android/uba
gradle test            # JVM 核心层全绿（契约金样 + 行为）
# android/ 适配层随宿主 App 构建（本仓库无 Android SDK 环境，不在此编译）
```

## 契约约束

- 字段 camelCase 与后端 proto `json_name` 逐一对齐，**禁止手造字段**
- `tenantId`/`serverTime`/`ip` 服务端补全不发送；数值标量（durationMs/quantity/score/level）只进 behavior oneof
- 鉴权在 body；单批压在 1MiB 内；超时必须 < 服务端 10s（默认 8s）
- 4xx（除 429）不重试不落盘；HTTP 200 仍可能有单条部分失败（`failedCount`）
