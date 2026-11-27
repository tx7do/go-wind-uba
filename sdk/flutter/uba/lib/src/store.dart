import 'package:shared_preferences/shared_preferences.dart';

/// 键值存储抽象：隔离 shared_preferences，测试可注入内存实现。
/// 键名沿用 Web / Unity SDK 的 `__uba_*__` 约定，同一用户多端排查时对得上。
abstract class KeyValueStore {
  Future<String?> getString(String key);

  Future<void> setString(String key, String value);

  Future<void> remove(String key);
}

/// 设备 ID 持久化键（对齐 Web localStorage / Unity PlayerPrefs）。
const String kDeviceIdKey = '__uba_device_id__';

/// 离线队列持久化键（对齐 Web localStorage）。
const String kOfflineQueueKey = '__uba_offline_queue__';

/// 基于 shared_preferences 的实现（Android/iOS/桌面全平台可用）。
class PrefsStore implements KeyValueStore {
  final SharedPreferences _prefs;

  PrefsStore(this._prefs);

  /// 异步工厂：shared_preferences 的实例获取是异步的。
  static Future<PrefsStore> create() async =>
      PrefsStore(await SharedPreferences.getInstance());

  @override
  Future<String?> getString(String key) async => _prefs.getString(key);

  @override
  Future<void> setString(String key, String value) =>
      _prefs.setString(key, value);

  @override
  Future<void> remove(String key) => _prefs.remove(key);
}

/// 内存实现，仅测试与临时客户端使用（进程退出即丢）。
class MemoryStore implements KeyValueStore {
  final Map<String, String> _data = {};

  @override
  Future<String?> getString(String key) async => _data[key];

  @override
  Future<void> setString(String key, String value) async {
    _data[key] = value;
  }

  @override
  Future<void> remove(String key) async {
    _data.remove(key);
  }
}
