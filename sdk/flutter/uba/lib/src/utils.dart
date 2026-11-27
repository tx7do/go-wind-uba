import 'dart:math';

/// SDK 版本号，与 pubspec.yaml 保持一致，同时用于 User-Agent。
const String sdkVersion = '1.0.0';

final Random _secureRandom = Random.secure();

/// 生成 UUID v4（小写、连字符格式），用作 eventId / deviceId / sessionId。
String uuidV4() {
  final bytes = List<int>.generate(16, (_) => _secureRandom.nextInt(256));
  // version 4
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  // variant 10xx
  bytes[8] = (bytes[8] & 0x3f) | 0x80;

  final hex = bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
  return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-'
      '${hex.substring(12, 16)}-${hex.substring(16, 20)}-${hex.substring(20)}';
}

/// UTC 时间 → RFC3339 毫秒精度字符串，如 `2026-11-27T08:30:00.123Z`。
/// 与后端 protojson Timestamp 兼容，精度对齐 C# SDK。
String toRfc3339Ms(DateTime dt) {
  final u = dt.toUtc();
  final y = u.year.toString().padLeft(4, '0');
  final m = u.month.toString().padLeft(2, '0');
  final d = u.day.toString().padLeft(2, '0');
  final hh = u.hour.toString().padLeft(2, '0');
  final mm = u.minute.toString().padLeft(2, '0');
  final ss = u.second.toString().padLeft(2, '0');
  final ms = u.millisecond.toString().padLeft(3, '0');
  return '$y-$m-${d}T$hh:$mm:$ss.${ms}Z';
}

/// super 属性在前、调用属性在后（后者覆盖前者同名 key）。
Map<String, String> mergeProperties(
  Map<String, String>? base,
  Map<String, String>? extra,
) {
  final merged = <String, String>{};
  if (base != null) merged.addAll(base);
  if (extra != null) merged.addAll(extra);
  return merged;
}

/// 去除首尾空白并按 Unicode 码点截断到 [limit]，契约约定属性值有长度上限。
String? trimAndLimit(String? value, [int limit = 256]) {
  if (value == null) return null;
  final v = value.trim();
  if (v.isEmpty) return null;
  if (v.runes.length <= limit) return v;
  return String.fromCharCodes(v.runes.take(limit));
}

/// FNV-1a 32 位哈希：用于按 sessionId 确定性采样（同会话同结果，与 Web SDK 算法一致）。
int fnv1a32(String s) {
  var hash = 0x811c9dc5;
  for (final rune in s.runes) {
    hash ^= rune & 0xff;
    hash = (hash * 0x01000193) & 0xffffffff;
    hash ^= (rune >> 8) & 0xff;
    hash = (hash * 0x01000193) & 0xffffffff;
    hash ^= (rune >> 16) & 0xff;
    hash = (hash * 0x01000193) & 0xffffffff;
    hash ^= (rune >> 24) & 0xff;
    hash = (hash * 0x01000193) & 0xffffffff;
  }
  return hash;
}

/// 会话是否落在采样区间：`hash(sessionId) % 1000 / 1000 < sampleRate`。
bool sessionIdInSample(String sessionId, double sampleRate) {
  if (sampleRate >= 1) return true;
  if (sampleRate <= 0) return false;
  return (fnv1a32(sessionId) % 1000) / 1000 < sampleRate;
}

/// 平台字符串（dart:io `Platform.operatingSystem`）→ 展示用 os 字段。
String osDisplayName(String operatingSystem) {
  switch (operatingSystem) {
    case 'android':
      return 'Android';
    case 'ios':
      return 'iOS';
    case 'macos':
      return 'macOS';
    case 'windows':
      return 'Windows';
    case 'linux':
      return 'Linux';
    default:
      return trimAndLimit(operatingSystem, 32) ?? operatingSystem;
  }
}
