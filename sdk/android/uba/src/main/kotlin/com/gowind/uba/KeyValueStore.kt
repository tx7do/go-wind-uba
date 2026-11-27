package com.gowind.uba

/** 键值存储抽象：隔离 SharedPreferences，测试可注入内存实现。键名沿用 `__uba_*__` 约定。 */
interface KeyValueStore {
    fun getString(key: String): String?
    fun setString(key: String, value: String)
    fun remove(key: String)
}

/** 设备 ID 持久化键（对齐 Web localStorage / Unity PlayerPrefs / Flutter 约定）。 */
const val DEVICE_ID_KEY = "__uba_device_id__"

/** 离线队列持久化键（对齐 Web localStorage）。 */
const val OFFLINE_QUEUE_KEY = "__uba_offline_queue__"

/** 内存实现：测试与临时客户端用（进程退出即丢）。 */
class InMemoryStore : KeyValueStore {
    private val data = mutableMapOf<String, String>()
    override fun getString(key: String): String? = data[key]
    override fun setString(key: String, value: String) {
        data[key] = value
    }
    override fun remove(key: String) {
        data.remove(key)
    }
}
