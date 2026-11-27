import 'dart:io';

import 'store.dart';
import 'utils.dart';

/// 初始化完成后不再变化的客户端环境快照。
class ContextSnapshot {
  /// 持久化设备 ID（uuid v4），未登录态的用户主键。
  final String deviceId;

  /// deviceId 是否本次新建 —— 首装判定依据（app_launch 自动埋点用）。
  final bool deviceIdIsNew;

  /// 平台标识：android / ios / windows / macos / linux。
  final String platform;

  /// 操作系统展示名：Android / iOS / Windows / macOS / Linux。
  final String os;

  /// 客户端 UA（envelope 的 clientInfo.userAgent），服务端会做 UA 解析补全。
  final String userAgent;

  const ContextSnapshot({
    required this.deviceId,
    required this.deviceIdIsNew,
    required this.platform,
    required this.os,
    required this.userAgent,
  });
}

/// 环境提供方抽象。特殊宿主（如通过原生侧取 OAID/IDFA 的场景）可实现本接口注入。
abstract class ContextProvider {
  /// 加载（或生成并持久化）设备环境。SDK 初始化时调用一次。
  Future<ContextSnapshot> load();
}

/// 默认实现：deviceId 用 uuid v4 持久化到 KeyValueStore，
/// platform/os 取 dart:io Platform，UA 手工拼装（移动端无系统 UA 可读）。
class DefaultContextProvider implements ContextProvider {
  final KeyValueStore _store;

  DefaultContextProvider(this._store);

  @override
  Future<ContextSnapshot> load() async {
    final persisted = await _store.getString(kDeviceIdKey);
    final deviceIdIsNew = persisted == null || persisted.isEmpty;
    final deviceId = deviceIdIsNew ? uuidV4() : persisted;
    if (deviceIdIsNew) {
      await _store.setString(kDeviceIdKey, deviceId);
    }

    final platform = Platform.operatingSystem; // android / ios / windows / ...
    return ContextSnapshot(
      deviceId: deviceId,
      deviceIdIsNew: deviceIdIsNew,
      platform: platform,
      os: osDisplayName(platform),
      userAgent: 'uba_flutter/$sdkVersion ($platform)',
    );
  }
}
