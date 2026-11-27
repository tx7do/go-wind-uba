import 'utils.dart';

/// 会话管理：进程内 uuid，后台超过 [timeoutMs] 后回到前台时轮换新会话。
/// 对齐移动端 UBA 常见的「后台 30s 切回 = 新会话」语义。
class SessionManager {
  final int timeoutMs;

  /// 时钟注入点（测试用），默认系统时间。
  final DateTime Function() now;

  String _id = uuidV4();
  DateTime? _pausedAt;

  SessionManager({this.timeoutMs = 30000, DateTime Function()? now})
      : now = now ?? DateTime.now;

  /// 当前会话 ID。
  String get id => _id;

  /// 会话是否处于采样区间（按 sessionId 确定性判定）。
  bool inSample(double sampleRate) => sessionIdInSample(_id, sampleRate);

  /// 进入后台（paused/hidden/detached）。
  void onBackground() {
    _pausedAt ??= now();
  }

  /// 回到前台：后台时长超过 timeoutMs 则轮换新会话。
  void onForeground() {
    final paused = _pausedAt;
    _pausedAt = null;
    if (paused != null) {
      final elapsed = now().difference(paused).inMilliseconds;
      if (timeoutMs > 0 && elapsed > timeoutMs) {
        rotate();
      }
    }
  }

  /// 手动开启新会话。
  void rotate() {
    _id = uuidV4();
    _pausedAt = null;
  }
}
