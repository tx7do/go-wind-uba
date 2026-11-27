import 'dart:async';
import 'dart:developer' as developer;

import 'batcher.dart';
import 'config.dart';
import 'context.dart';
import 'flags.dart';
import 'session.dart';
import 'store.dart';
import 'transport.dart';
import 'types.dart';
import 'utils.dart';

/// GoWind UBA Flutter 埋点客户端。
///
/// 推荐用法（在 `main()` 里 `runApp` 之前初始化）：
/// ```dart
/// void main() async {
///   WidgetsFlutterBinding.ensureInitialized();
///   final client = await UbaClient.init(UbaConfig(
///     appId: 'your_app_id',
///     appSecret: 'your_app_secret',
///     endpoint: 'http://collector:5700',
///     appVersion: '1.2.3',
///   ));
///   UbaLifecycleObserver(client).register();
///   runApp(const App());
/// }
/// ```
///
/// 方法名与 Web / C# SDK 对齐：track / trackRisk / identify / setSuperProperties /
/// flush / setConsent / loadFlags / getFlag。
class UbaClient {
  static UbaClient? _instance;

  /// 当前全局实例（[init] 之后才有值）。
  static UbaClient? get instance => _instance;

  final UbaConfig config;
  final HttpTransport transport;
  final KeyValueStore? store;
  final SessionManager session;
  late final Batcher batcher;
  late final FlagsManager _flagsManager;

  final ContextSnapshot _context;
  int? _userId;
  final Map<String, String> _superProps = <String, String>{};
  bool _consentGranted = false;
  bool _closed = false;
  DateTime? _lastScreenAt;

  /// `$feature_flag` 曝光去重：按会话记一次。
  final Set<String> _exposedFlags = <String>{};
  String? _exposedSessionId;

  UbaClient._({
    required this.config,
    required this.transport,
    this.store,
    required this.session,
    required ContextSnapshot context,
  }) : _context = context {
    batcher = Batcher(
      config: config,
      transport: transport,
      store: store,
      clientInfoProvider: () => ClientInfo(userAgent: _context.userAgent),
      onLog: _log,
    );
    _flagsManager = FlagsManager(
      config: config,
      transport: transport,
      deviceIdProvider: () => _context.deviceId,
      userIdProvider: () => _userId,
      onLog: _log,
    );
  }

  /// 创建独立实例（不走全局单例），测试或多实例场景用。
  static Future<UbaClient> create(
    UbaConfig config, {
    HttpTransport? transport,
    KeyValueStore? store,
    ContextProvider? contextProvider,
  }) async {
    final resolvedStore = store ?? await PrefsStore.create();
    final provider =
        contextProvider ?? DefaultContextProvider(resolvedStore);
    final context = await provider.load();
    final t = transport ?? IoHttpTransport();

    final client = UbaClient._(
      config: config,
      transport: t,
      store: resolvedStore,
      session: SessionManager(timeoutMs: config.sessionTimeoutMs),
      context: context,
    );

    // 恢复上次未发完的离线事件并启动定时 flush
    await client.batcher.restorePersisted();
    client.batcher.start();

    // 首装自动埋点：deviceId 本次新建即视为新用户安装
    if (config.autoTrackLaunch && context.deviceIdIsNew) {
      client.track(
        'app_launch',
        properties: const {'first_launch': 'true'},
        options: const TrackOptions(eventCategory: 'app'),
      );
    }
    return client;
  }

  /// 初始化全局单例。重复调用会先 flush 并关闭旧实例再替换。
  static Future<UbaClient> init(
    UbaConfig config, {
    HttpTransport? transport,
    KeyValueStore? store,
    ContextProvider? contextProvider,
  }) async {
    final old = _instance;
    if (old != null) await old.close();
    final client = await create(
      config,
      transport: transport,
      store: store,
      contextProvider: contextProvider,
    );
    _instance = client;
    return client;
  }

  // ─── 上报 API ───────────────────────────────────────────────────────────

  /// 上报自定义行为事件（[trackBehavior] 的别名，与 C# SDK 一致）。
  void track(
    String eventName, {
    Map<String, String>? properties,
    TrackOptions? options,
  }) =>
      trackBehavior(eventName, properties: properties, options: options);

  /// 上报自定义行为事件。
  void trackBehavior(
    String eventName, {
    Map<String, String>? properties,
    TrackOptions? options,
  }) {
    final event = _buildBehaviorEvent(eventName, properties, options);
    if (event == null) return;
    batcher.enqueue(event);
  }

  /// 上报风险事件（进入 `uba_risk_events` 通道）。
  /// riskEventId / occurTime / reportTime 由服务端补全，无需填写。
  void trackRisk(
    String eventName,
    RiskEvent risk, {
    Map<String, String>? properties,
    TrackOptions? options,
  }) {
    if (!_shouldCollect(eventName)) return;
    final ctx = _context;
    batcher.enqueue(ReportEvent(
      eventType: UbaEventType.risk,
      eventId: uuidV4(),
      eventName: eventName,
      eventTime: toRfc3339Ms(DateTime.now()),
      userId: options?.userId ?? _userId,
      deviceId: options?.deviceId ?? ctx.deviceId,
      sessionId: options?.sessionId ?? session.id,
      platform: options?.platform ?? ctx.platform,
      eventCategory: options?.eventCategory,
      properties: _mergedProperties(properties, options),
      traceId: options?.traceId,
      risk: risk,
    ));
  }

  /// 页面浏览事件（mobile 的 page_view）。[referer] 传来源页面名。
  /// 自动携带上一页面停留时长 durationMs。
  void trackPageView(String screenName, {String? title, String? referer}) {
    final now = DateTime.now();
    final last = _lastScreenAt;
    final dwell = last == null ? null : now.difference(last).inMilliseconds;
    _lastScreenAt = now;

    final props = <String, String>{
      'pageUrl': screenName,
      if (title != null && title.isNotEmpty) 'title': title,
      if (referer != null && referer.isNotEmpty) 'referer': referer,
    };
    trackBehavior(
      config.pageViewEvent,
      properties: props,
      options: TrackOptions(
        eventCategory: 'navigation',
        objectType: 'page',
        objectId: screenName,
        durationMs: dwell != null && dwell > 0 ? dwell : null,
      ),
    );
  }

  /// 登录绑定用户 ID，后续事件自动携带。
  void identify(int userId) {
    _userId = userId;
  }

  /// 登出清空用户绑定（deviceId 保持不变，匿名行为继续归因到设备）。
  void resetUser() {
    _userId = null;
  }

  /// 设置全局公共属性：合并进之后每个事件的 properties（同名 key 后写覆盖）。
  void setSuperProperties(Map<String, String> props) {
    _superProps.addAll(props);
  }

  /// 清空全局公共属性。
  void clearSuperProperties() {
    _superProps.clear();
  }

  /// 隐私合规授权开关；config.requireConsent = true 时，
  /// 未授权前所有上报静默丢弃。
  void setConsent(bool granted) {
    _consentGranted = granted;
  }

  /// 当前是否处于采集状态（未关闭、consent 放行、会话命中采样）。
  bool get isCollecting => !_closed && _consentOk && _inSample;

  // ─── AB 实验开关 ─────────────────────────────────────────────────────────

  /// 拉取最新实验开关并缓存。
  Future<Map<String, String>> loadFlags() => _flagsManager.loadFlags();

  /// 读取开关变体；命中时自动上报一次 `$feature_flag` 曝光事件（每会话每开关一次）。
  String? getFlag(String key) {
    final variant = _flagsManager[key];
    if (variant != null) {
      _trackFlagExposure(key, variant);
    }
    return variant;
  }

  /// 变体非空且非 `control` 即视为命中（不触发曝光事件）。
  bool isFeatureEnabled(String key) => _flagsManager.isEnabled(key);

  // ─── 生命周期 & 维护 ──────────────────────────────────────────────────────

  /// 退到后台（hidden/paused/detached）：记录时刻 + 立即 flush。
  /// 由 [UbaLifecycleObserver] 调用，也可手工接入原生生命周期。
  void handleBackground() {
    session.onBackground();
    unawaited(batcher.flush());
  }

  /// 回到前台：后台超过 sessionTimeoutMs 则轮换新会话。
  void handleForeground() {
    session.onForeground();
  }

  /// 手动 flush 全部队列。
  Future<FlushResult> flush() => batcher.flush();

  /// 队列中待上报条数。
  int get pendingCount => batcher.size;

  /// 关闭客户端：flush 剩余队列 → 停止定时器 → 关闭传输连接。
  /// 全局实例关闭后 [instance] 置空。
  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    await batcher.flush();
    batcher.close();
    transport.close();
    if (identical(_instance, this)) _instance = null;
  }

  // ─── 内部实现 ────────────────────────────────────────────────────────────

  bool get _consentOk => !config.requireConsent || _consentGranted;

  bool get _inSample => session.inSample(config.sampleRate);

  bool _shouldCollect(String eventName) {
    if (_closed) return false;
    if (!_consentOk) {
      _log('consent 未授权，丢弃事件 $eventName');
      return false;
    }
    if (!_inSample) {
      // 采样在会话维度确定性判定：整会话要么全采要么全丢
      return false;
    }
    return true;
  }

  ReportEvent? _buildBehaviorEvent(
    String eventName,
    Map<String, String>? properties,
    TrackOptions? options,
  ) {
    if (!_shouldCollect(eventName)) return null;
    final ctx = _context;
    final opts = options ?? const TrackOptions();
    return ReportEvent(
      eventType: UbaEventType.behavior,
      eventId: uuidV4(),
      eventName: eventName,
      eventTime: toRfc3339Ms(DateTime.now()),
      userId: opts.userId ?? _userId,
      deviceId: opts.deviceId ?? ctx.deviceId,
      sessionId: opts.sessionId ?? session.id,
      platform: opts.platform ?? ctx.platform,
      eventCategory: opts.eventCategory,
      properties: _mergedProperties(properties, opts),
      metrics: opts.metrics,
      traceId: opts.traceId,
      // 顶层字符串维度镜像（服务端 oneof 优先，顶层兜底），数值标量只进 behavior
      eventAction: opts.eventAction,
      objectType: opts.objectType,
      objectId: opts.objectId,
      objectName: opts.objectName,
      amount: opts.amount,
      opResult: opts.opResult,
      errorCode: opts.errorCode,
      behavior: BehaviorEvent(
        eventAction: opts.eventAction,
        objectType: opts.objectType,
        objectId: opts.objectId,
        objectName: opts.objectName,
        os: ctx.os,
        appVersion: config.appVersion,
        channel: config.channel,
        network: opts.network,
        durationMs: opts.durationMs,
        amount: opts.amount,
        quantity: opts.quantity,
        score: opts.score,
        metrics: opts.metrics,
        opResult: opts.opResult,
        errorCode: opts.errorCode,
        serverId: opts.serverId,
        level: opts.level,
      ),
    );
  }

  /// super 属性 → 调用 properties → options.properties，同名 key 后者覆盖。
  Map<String, String> _mergedProperties(
    Map<String, String>? properties,
    TrackOptions? options,
  ) {
    final merged = mergeProperties(_superProps, properties);
    if (options?.properties != null) {
      merged.addAll(options!.properties!);
    }
    return merged;
  }

  void _trackFlagExposure(String key, String variant) {
    if (_exposedSessionId != session.id) {
      _exposedFlags.clear();
      _exposedSessionId = session.id;
    }
    if (!_exposedFlags.add(key)) return;
    track(
      r'$feature_flag',
      properties: {'flag_key': key, 'variant': variant},
      options: const TrackOptions(eventCategory: 'experiment'),
    );
  }

  void _log(String message) {
    if (!config.debug) return;
    developer.log(message, name: 'uba_flutter');
  }
}
