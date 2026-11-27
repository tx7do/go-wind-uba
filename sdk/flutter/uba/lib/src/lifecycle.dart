import 'package:flutter/widgets.dart';

import 'client.dart';

/// 应用生命周期观察者：
/// - 退到后台（hidden/paused/detached）→ 立即 flush + 记录后台时刻；
/// - 回到前台（resumed）→ 超时会话超时则轮换新会话。
///
/// 在 `runApp` 后调用 [register]，或直接 `WidgetsBinding.instance.addObserver`。
class UbaLifecycleObserver with WidgetsBindingObserver {
  final UbaClient client;

  bool _registered = false;

  UbaLifecycleObserver(this.client);

  /// 注册到 WidgetsBinding（幂等）。
  void register() {
    if (_registered) return;
    WidgetsBinding.instance.addObserver(this);
    _registered = true;
  }

  /// 反注册（一般用不到，测试时用）。
  void unregister() {
    if (!_registered) return;
    WidgetsBinding.instance.removeObserver(this);
    _registered = false;
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      client.handleForeground();
    } else if (state == AppLifecycleState.hidden ||
        state == AppLifecycleState.paused ||
        state == AppLifecycleState.detached) {
      // inactive 是来电/下拉通知栏等瞬时打断，不算后台，不触发会话判定
      client.handleBackground();
    }
  }
}

/// 页面浏览自动埋点：挂在 Navigator 上，push/pop/replace 时上报 [pageViewEvent]
/// （默认 `page_view`，含来源页停留时长 durationMs）。
///
/// ```dart
/// MaterialApp(
///   navigatorObservers: [UbaScreenObserver(client)],
///   // 路由请给 settings.name 命名，埋点取 route.settings.name
/// )
/// ```
class UbaScreenObserver extends NavigatorObserver {
  final UbaClient client;

  UbaScreenObserver(this.client);

  @override
  void didPush(Route<Object?> route, Route<Object?>? previousRoute) =>
      _trackScreen(route, previousRoute);

  @override
  void didPop(Route<Object?> route, Route<Object?>? previousRoute) =>
      _trackScreen(previousRoute, route);

  @override
  void didReplace({Route<Object?>? newRoute, Route<Object?>? oldRoute}) =>
      _trackScreen(newRoute, oldRoute);

  void _trackScreen(Route<Object?>? route, Route<Object?>? previous) {
    // 页面名优先取命名路由；未命名路由回退到 Widget runtimeType
    final name = route?.settings.name ?? route?.runtimeType.toString();
    if (name == null || name.isEmpty) return;
    client.trackPageView(name, referer: previous?.settings.name);
  }
}
