/// SDK 配置。默认值与 C# / Web SDK 对齐：
/// 批量 20 条 / 5s 定时 / 3 次重试 / 1s 退避基数 / 8s 超时（必须小于服务端 10s）。
class UbaConfig {
  /// 应用 ID（管理后台「数据采集/应用管理」生成），必填。
  final String appId;

  /// 应用密钥，鉴权在请求 body 中完成，必填。
  final String appSecret;

  /// collector 服务地址，如 `http://localhost:5700`，必填（不带尾斜杠）。
  final String endpoint;

  /// 事件上报路径。
  final String path;

  /// AB 实验开关拉取路径。
  final String flagsPath;

  /// 批量触发阈值：队列达到该条数立即 flush。
  final int batchSize;

  /// 定时 flush 间隔（毫秒）。
  final int flushIntervalMs;

  /// 失败重试次数（指数退避，总尝试 = maxRetries + 1）。
  final int maxRetries;

  /// 单次 HTTP 请求超时（毫秒），必须 < 服务端 10s。
  final int timeoutMs;

  /// 重试退避基数（毫秒），第 n 次重试延迟 = retryBaseDelayMs × 2^n。
  final int retryBaseDelayMs;

  /// 调试日志开关。
  final bool debug;

  /// 离线队列持久化开关：flush 失败时把队列落盘，下次启动补发。
  final bool persistQueue;

  /// 持久化队列最大条数，超出丢弃最旧事件。
  final int maxPersistedEvents;

  /// 单批请求体字节上限；服务端对未鉴权大包直接 401，必须压在 1MiB 以内。
  final int maxBatchBytes;

  /// 应用版本号（如 `1.2.3`），填了会随每个事件上报 `appVersion`。
  final String? appVersion;

  /// 投放/下载渠道。
  final String? channel;

  /// 首次启动自动埋 `app_launch` 事件（首装判定 = deviceId 新建）。
  final bool autoTrackLaunch;

  /// 页面浏览事件名（配合 [UbaScreenObserver] / trackPageView）。
  final String pageViewEvent;

  /// 会话轮换超时（毫秒）：后台超过该时长后回到前台会开启新会话。
  final int sessionTimeoutMs;

  /// 采样率 0~1，按 sessionId 确定性采样（同会话要么全采要么全丢）。
  final double sampleRate;

  /// 隐私合规开关：为 true 时，调用 [UbaClient.setConsent] 之前所有事件静默丢弃。
  final bool requireConsent;

  const UbaConfig({
    required this.appId,
    required this.appSecret,
    required this.endpoint,
    this.path = '/uba/v1/report',
    this.flagsPath = '/uba/v1/flags',
    this.batchSize = 20,
    this.flushIntervalMs = 5000,
    this.maxRetries = 3,
    this.timeoutMs = 8000,
    this.retryBaseDelayMs = 1000,
    this.debug = false,
    this.persistQueue = true,
    this.maxPersistedEvents = 500,
    this.maxBatchBytes = 900 * 1024,
    this.appVersion,
    this.channel,
    this.autoTrackLaunch = false,
    this.pageViewEvent = 'page_view',
    this.sessionTimeoutMs = 30000,
    this.sampleRate = 1.0,
    this.requireConsent = false,
  })  : assert(appId != '', 'appId 不能为空'),
        assert(appSecret != '', 'appSecret 不能为空'),
        assert(endpoint != '', 'endpoint 不能为空'),
        assert(batchSize > 0),
        assert(flushIntervalMs >= 0),
        assert(maxRetries >= 0),
        assert(timeoutMs > 0 && timeoutMs < 10000, 'timeoutMs 必须 < 10000（服务端超时 10s）'),
        assert(retryBaseDelayMs >= 0),
        assert(maxPersistedEvents > 0),
        assert(maxBatchBytes > 0 && maxBatchBytes < 1024 * 1024, 'maxBatchBytes 必须压在 1MiB 以内'),
        assert(sessionTimeoutMs >= 0),
        assert(sampleRate >= 0 && sampleRate <= 1);

  /// 规整 endpoint：去掉尾斜杠。
  String get baseEndpoint => endpoint.endsWith('/')
      ? endpoint.substring(0, endpoint.length - 1)
      : endpoint;

  /// 事件上报完整 URL。
  String get reportUrl => '$baseEndpoint$path';

  /// AB 开关拉取完整 URL。
  String get flagsUrl => '$baseEndpoint$flagsPath';
}

/// 单次上报的可选维度，字段与 C# SDK `TrackOptions` 一一对应。
/// 传入的字段优先级高于客户端状态（identify 设置的用户、自动采集的设备/会话）。
class TrackOptions {
  /// 事件大类，如 `navigation` / `experiment` / `app`。
  final String? eventCategory;

  /// 覆盖本次事件的用户 ID。
  final int? userId;

  /// 覆盖本次事件的设备 ID。
  final String? deviceId;

  /// 覆盖本次事件的会话 ID。
  final String? sessionId;

  /// 覆盖本次事件的平台标识。
  final String? platform;

  /// 事件动作，如 click / view / purchase。
  final String? eventAction;

  /// 对象类型，如 product / level / page。
  final String? objectType;

  /// 对象 ID。
  final String? objectId;

  /// 对象名称。
  final String? objectName;

  /// 持续时长（毫秒），如页面停留 / 视频播放。
  final int? durationMs;

  /// 金额（字符串形式以兼容不同货币格式）。
  final String? amount;

  /// 数量。
  final int? quantity;

  /// 评分/分数。
  final int? score;

  /// 数值指标。
  final Map<String, double>? metrics;

  /// 网络类型，如 WiFi / 4G / 5G。
  final String? network;

  /// 操作结果，如 success / fail。
  final String? opResult;

  /// 错误码（事件失败时）。
  final String? errorCode;

  /// 额外属性，与调用方 properties 合并后上报。
  final Map<String, String>? properties;

  /// 游戏区服 ID。
  final String? serverId;

  /// 玩家等级（事件发生时快照）。
  final int? level;

  /// 链路追踪 ID。
  final String? traceId;

  const TrackOptions({
    this.eventCategory,
    this.userId,
    this.deviceId,
    this.sessionId,
    this.platform,
    this.eventAction,
    this.objectType,
    this.objectId,
    this.objectName,
    this.durationMs,
    this.amount,
    this.quantity,
    this.score,
    this.metrics,
    this.network,
    this.opResult,
    this.errorCode,
    this.properties,
    this.serverId,
    this.level,
    this.traceId,
  });
}
