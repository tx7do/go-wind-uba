import 'dart:convert';

import 'config.dart';
import 'transport.dart';

/// AB 实验开关管理：从 collector `/uba/v1/flags` 拉取变体映射。
class FlagsManager {
  final UbaConfig config;
  final HttpTransport transport;

  /// 当前设备 ID（flags 请求必填 userId 或 deviceId 之一）。
  final String Function() deviceIdProvider;

  /// 当前登录用户 ID（可为 null）。
  final int? Function() userIdProvider;

  /// 调试日志回调。
  final void Function(String message)? onLog;

  final Map<String, String> _flags = <String, String>{};

  FlagsManager({
    required this.config,
    required this.transport,
    required this.deviceIdProvider,
    required this.userIdProvider,
    this.onLog,
  });

  void _log(String message) => onLog?.call(message);

  /// 拉取最新开关并缓存。失败时保留上一次缓存。
  Future<Map<String, String>> loadFlags() async {
    final body = <String, Object?>{
      'appId': config.appId,
      'appSecret': config.appSecret,
      'deviceId': deviceIdProvider(),
    };
    final userId = userIdProvider();
    if (userId != null) body['userId'] = userId;

    final res = await transport.send(
      config.flagsUrl,
      jsonEncode(body),
      config.timeoutMs,
    );
    if (!res.isOk) {
      _log('拉取 flags 失败：HTTP ${res.statusCode}');
      return Map<String, String>.unmodifiable(_flags);
    }
    try {
      final decoded = jsonDecode(res.body) as Map<Object?, Object?>;
      final flags = (decoded['flags'] as Map?) ?? const {};
      _flags
        ..clear()
        ..addAll(flags.cast<String, String>());
    } on FormatException catch (e) {
      _log('flags 响应解析失败：$e');
    }
    return Map<String, String>.unmodifiable(_flags);
  }

  /// 当前缓存的全部开关。
  Map<String, String> get flags => Map<String, String>.unmodifiable(_flags);

  /// 读取某个开关的变体值；未加载或不存在返回 null。
  String? operator [](String key) => _flags[key];

  /// 变体非空且非 `control` 即视为命中实验。
  bool isEnabled(String key) {
    final v = _flags[key];
    return v != null && v.isNotEmpty && v != 'control';
  }
}
