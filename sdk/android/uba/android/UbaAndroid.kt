package com.gowind.uba.android

import android.app.Activity
import android.app.Application
import android.content.Context
import android.content.SharedPreferences
import android.os.Bundle
import com.gowind.uba.KeyValueStore
import com.gowind.uba.UbaClient
import com.gowind.uba.UbaConfig

/**
 * Android 适配层（本文件随宿主 App 编译，不参与本仓库 JVM 构建）。
 *
 * 用法（自定义 Application 的 onCreate）：
 * ```
 * class MyApp : Application() {
 *     override fun onCreate() {
 *         super.onCreate()
 *         UbaAndroid.init(this, UbaConfig(
 *             appId = "your_app_id",
 *             appSecret = "your_app_secret",
 *             endpoint = "http://collector:5700",
 *             appVersion = BuildConfig.VERSION_NAME,
 *             autoTrackLaunch = true,
 *         ))
 *     }
 * }
 * ```
 * 之后任意位置 `UbaAndroid.client.track(...)`；
 * 页面埋点在 Activity onResume 后 `UbaAndroid.client.trackPageView("MainActivity")`
 * （或用 [UbaActivityLifecycleCallbacks] 全自动）。
 */
object UbaAndroid {

    @Volatile
    private var clientRef: UbaClient? = null

    /** 初始化全局客户端（幂等；重复调用返回已有实例）。 */
    @Synchronized
    fun init(context: Context, config: UbaConfig): UbaClient {
        clientRef?.let { return it }
        val appContext = context.applicationContext
        val store = SharedPreferencesStore(
            appContext.getSharedPreferences("uba_android", Context.MODE_PRIVATE),
        )
        val client = UbaClient(config, store)
        val lifecycle = UbaActivityLifecycleCallbacks(client)
        (appContext as? Application)?.registerActivityLifecycleCallbacks(lifecycle)
        clientRef = client
        return client
    }

    /** 当前全局实例（init 之后）。 */
    val client: UbaClient?
        get() = clientRef

    /** 关闭并清理全局实例（一般只在测试用）。 */
    @Synchronized
    fun shutdown(): UbaClient? {
        val c = clientRef ?: return null
        clientRef = null
        c.close()
        return c
    }
}

/** SharedPreferences 落地的 KeyValueStore（deviceId + 离线队列共用）。 */
class SharedPreferencesStore(private val prefs: SharedPreferences) : KeyValueStore {

    override fun getString(key: String): String? = prefs.getString(key, null)

    override fun setString(key: String, value: String) {
        prefs.edit().putString(key, value).apply()
    }

    override fun remove(key: String) {
        prefs.edit().remove(key).apply()
    }
}

/**
 * 自动页面埋点 + 前后台生命周期：
 * - Activity resumed → trackPageView(局部类名)（带来源页与停留时长）；
 * - 首个 Activity 恢复视为回到前台；全部 Activity 退到后台 → handleBackground 自动 flush。
 *
 * 由 [UbaAndroid.init] 注册，无需手工接入。
 */
class UbaActivityLifecycleCallbacks(private val client: UbaClient) :
    Application.ActivityLifecycleCallbacks {

    private var startedActivities = 0

    override fun onActivityStarted(activity: Activity) {
        if (startedActivities == 0) client.handleForeground()
        startedActivities++
    }

    override fun onActivityStopped(activity: Activity) {
        startedActivities = (startedActivities - 1).coerceAtLeast(0)
        if (startedActivities == 0) client.handleBackground()
    }

    override fun onActivityResumed(activity: Activity) {
        // 停留时长（durationMs）由 client.trackPageView 内部按上一页面计时自动填充
        client.trackPageView(activity.localClassName)
    }

    override fun onActivityCreated(activity: Activity, savedInstanceState: Bundle?) = Unit
    override fun onActivitySaveInstanceState(activity: Activity, outState: Bundle) = Unit
    override fun onActivityDestroyed(activity: Activity) = Unit
    override fun onActivityPaused(activity: Activity) = Unit
}
