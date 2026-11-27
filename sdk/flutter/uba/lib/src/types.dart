/// 上报契约数据模型，字段名与后端 proto（`uba/service/v1/report.proto` 等的
/// `json_name`）逐一对齐，序列化输出 camelCase 并跳过 null 值。
///
/// 铁律：
/// - `tenantId` / `serverTime` 由服务端覆盖，SDK 一律不发送；
/// - 数值标量（durationMs/quantity/score/sessionSeq/serverId/level）proto3 无法
///   区分 unset 0，**必须放在 behavior oneof 内才被服务端信任**；
/// - 顶层业务扩展字段与 behavior 载荷做镜像冗余，对齐 Web / C# SDK 的成熟写法。
library;

/// 往 map 里写非 null 字段（JSON 输出跳过 null）。
void putIfPresent(Map<String, Object?> m, String key, Object? value) {
  if (value == null) return;
  if (value is Map && value.isEmpty) return;
  if (value is List && value.isEmpty) return;
  m[key] = value;
}

/// behavior / risk 事件类型（线上以 `"BEHAVIOR"` / `"RISK"` 字符串传输）。
enum UbaEventType {
  behavior('BEHAVIOR'),
  risk('RISK');

  final String wire;
  const UbaEventType(this.wire);
}

/// 客户端信息（envelope 顶层），服务端可解析用于补全 os/browser/device/geo。
class ClientInfo {
  final String? userAgent;
  final String? referer;
  final String? country;
  final String? city;

  const ClientInfo({this.userAgent, this.referer, this.country, this.city});

  Map<String, Object?> toJson() {
    final m = <String, Object?>{};
    putIfPresent(m, 'userAgent', userAgent);
    putIfPresent(m, 'referer', referer);
    putIfPresent(m, 'country', country);
    putIfPresent(m, 'city', city);
    return m;
  }
}

/// 行为事件载荷（ReportEvent 的 `behavior` oneof）。
/// 数值标量只在此处上报才有意义（proto3 顶层无法区分 unset 0）。
class BehaviorEvent {
  final String? eventAction;
  final String? objectType;
  final String? objectId;
  final String? objectName;
  final int? sessionSeq;
  final String? os;
  final String? appVersion;
  final String? channel;
  final String? network;
  final int? durationMs;
  final String? amount;
  final int? quantity;
  final int? score;
  final Map<String, double>? metrics;
  final Map<String, String>? properties;
  final String? opResult;
  final String? errorCode;

  /// 点击热力图字段（移动端可选填充）。
  final int? clickX;
  final int? clickY;
  final String? elementXpath;
  final String? pageUrl;
  final int? viewportWidth;

  /// 游戏专属维度。
  final String? serverId;
  final int? level;

  const BehaviorEvent({
    this.eventAction,
    this.objectType,
    this.objectId,
    this.objectName,
    this.sessionSeq,
    this.os,
    this.appVersion,
    this.channel,
    this.network,
    this.durationMs,
    this.amount,
    this.quantity,
    this.score,
    this.metrics,
    this.properties,
    this.opResult,
    this.errorCode,
    this.clickX,
    this.clickY,
    this.elementXpath,
    this.pageUrl,
    this.viewportWidth,
    this.serverId,
    this.level,
  });

  Map<String, Object?> toJson() {
    final m = <String, Object?>{};
    putIfPresent(m, 'eventAction', eventAction);
    putIfPresent(m, 'objectType', objectType);
    putIfPresent(m, 'objectId', objectId);
    putIfPresent(m, 'objectName', objectName);
    putIfPresent(m, 'sessionSeq', sessionSeq);
    putIfPresent(m, 'os', os);
    putIfPresent(m, 'appVersion', appVersion);
    putIfPresent(m, 'channel', channel);
    putIfPresent(m, 'network', network);
    putIfPresent(m, 'durationMs', durationMs);
    putIfPresent(m, 'amount', amount);
    putIfPresent(m, 'quantity', quantity);
    putIfPresent(m, 'score', score);
    putIfPresent(m, 'metrics', metrics);
    putIfPresent(m, 'properties', properties);
    putIfPresent(m, 'opResult', opResult);
    putIfPresent(m, 'errorCode', errorCode);
    putIfPresent(m, 'clickX', clickX);
    putIfPresent(m, 'clickY', clickY);
    putIfPresent(m, 'elementXpath', elementXpath);
    putIfPresent(m, 'pageUrl', pageUrl);
    putIfPresent(m, 'viewportWidth', viewportWidth);
    putIfPresent(m, 'serverId', serverId);
    putIfPresent(m, 'level', level);
    return m;
  }

  static BehaviorEvent fromJson(Map<String, Object?> j) => BehaviorEvent(
        eventAction: j['eventAction'] as String?,
        objectType: j['objectType'] as String?,
        objectId: j['objectId'] as String?,
        objectName: j['objectName'] as String?,
        sessionSeq: j['sessionSeq'] as int?,
        os: j['os'] as String?,
        appVersion: j['appVersion'] as String?,
        channel: j['channel'] as String?,
        network: j['network'] as String?,
        durationMs: j['durationMs'] as int?,
        amount: j['amount'] as String?,
        quantity: j['quantity'] as int?,
        score: j['score'] as int?,
        metrics: (j['metrics'] as Map?)?.cast<String, double>(),
        properties: (j['properties'] as Map?)?.cast<String, String>(),
        opResult: j['opResult'] as String?,
        errorCode: j['errorCode'] as String?,
        clickX: j['clickX'] as int?,
        clickY: j['clickY'] as int?,
        elementXpath: j['elementXpath'] as String?,
        pageUrl: j['pageUrl'] as String?,
        viewportWidth: j['viewportWidth'] as int?,
        serverId: j['serverId'] as String?,
        level: j['level'] as int?,
      );
}

/// 风险事件载荷（ReportEvent 的 `risk` oneof）。
/// riskEventId / occurTime / reportTime 服务端会补全，可省略。
class RiskEvent {
  final String? riskEventId;
  final String? riskType;
  final String? riskLevel;
  final double? riskScore;
  final int? ruleId;
  final String? ruleName;

  /// 规则上下文（任意 JSON 对象，对应 google.protobuf.Struct）。
  final Map<String, Object?>? ruleContext;
  final List<String>? relatedEventIds;
  final String? description;
  final Map<String, String>? evidence;

  /// 处置状态：PENDING / INVESTIGATING / CONFIRMED / FALSE_POSITIVE / IGNORED / AUTO_BLOCKED。
  final String? status;

  const RiskEvent({
    this.riskEventId,
    this.riskType,
    this.riskLevel,
    this.riskScore,
    this.ruleId,
    this.ruleName,
    this.ruleContext,
    this.relatedEventIds,
    this.description,
    this.evidence,
    this.status,
  });

  Map<String, Object?> toJson() {
    final m = <String, Object?>{};
    putIfPresent(m, 'riskEventId', riskEventId);
    putIfPresent(m, 'riskType', riskType);
    putIfPresent(m, 'riskLevel', riskLevel);
    putIfPresent(m, 'riskScore', riskScore);
    putIfPresent(m, 'ruleId', ruleId);
    putIfPresent(m, 'ruleName', ruleName);
    putIfPresent(m, 'ruleContext', ruleContext);
    putIfPresent(m, 'relatedEventIds', relatedEventIds);
    putIfPresent(m, 'description', description);
    putIfPresent(m, 'evidence', evidence);
    putIfPresent(m, 'status', status);
    return m;
  }

  static RiskEvent fromJson(Map<String, Object?> j) => RiskEvent(
        riskEventId: j['riskEventId'] as String?,
        riskType: j['riskType'] as String?,
        riskLevel: j['riskLevel'] as String?,
        riskScore: (j['riskScore'] as num?)?.toDouble(),
        ruleId: j['ruleId'] as int?,
        ruleName: j['ruleName'] as String?,
        ruleContext: (j['ruleContext'] as Map?)?.cast<String, Object?>(),
        relatedEventIds: (j['relatedEventIds'] as List?)?.cast<String>(),
        description: j['description'] as String?,
        evidence: (j['evidence'] as Map?)?.cast<String, String>(),
        status: j['status'] as String?,
      );
}

/// 单条上报事件（对应 proto `ReportEvent`）。
class ReportEvent {
  /// [UbaEventType.behavior] 或 [UbaEventType.risk]。
  final UbaEventType eventType;

  /// UUID v4，全局唯一标识一条事件。
  final String eventId;

  /// 事件名，如 `page_view` / `app_launch` / `login_failed`。
  final String eventName;

  /// RFC3339 毫秒 UTC（`toRfc3339Ms` 产出）。
  final String eventTime;

  final int? userId;
  final String? deviceId;
  final String? sessionId;
  final String? platform;
  final String? eventCategory;
  final Map<String, String>? properties;
  final Map<String, double>? metrics;
  final String? traceId;

  /// 顶层业务扩展字段（字符串维度镜像进 behavior，数值标量只进 behavior）。
  final String? eventAction;
  final String? objectType;
  final String? objectId;
  final String? objectName;
  final int? durationMs;
  final String? amount;
  final int? quantity;
  final int? score;
  final String? opResult;
  final String? errorCode;
  final String? serverId;
  final int? level;

  final BehaviorEvent? behavior;
  final RiskEvent? risk;

  const ReportEvent({
    required this.eventType,
    required this.eventId,
    required this.eventName,
    required this.eventTime,
    this.userId,
    this.deviceId,
    this.sessionId,
    this.platform,
    this.eventCategory,
    this.properties,
    this.metrics,
    this.traceId,
    this.eventAction,
    this.objectType,
    this.objectId,
    this.objectName,
    this.durationMs,
    this.amount,
    this.quantity,
    this.score,
    this.opResult,
    this.errorCode,
    this.serverId,
    this.level,
    this.behavior,
    this.risk,
  });

  Map<String, Object?> toJson() {
    final m = <String, Object?>{
      'eventType': eventType.wire,
      'eventId': eventId,
      'eventName': eventName,
      'eventTime': eventTime,
    };
    putIfPresent(m, 'userId', userId);
    putIfPresent(m, 'deviceId', deviceId);
    putIfPresent(m, 'sessionId', sessionId);
    putIfPresent(m, 'platform', platform);
    putIfPresent(m, 'eventCategory', eventCategory);
    putIfPresent(m, 'properties', properties);
    putIfPresent(m, 'metrics', metrics);
    putIfPresent(m, 'traceId', traceId);
    putIfPresent(m, 'eventAction', eventAction);
    putIfPresent(m, 'objectType', objectType);
    putIfPresent(m, 'objectId', objectId);
    putIfPresent(m, 'objectName', objectName);
    putIfPresent(m, 'amount', amount);
    putIfPresent(m, 'opResult', opResult);
    putIfPresent(m, 'errorCode', errorCode);
    // durationMs/quantity/score/sessionSeq/serverId/level 为数值标量：
    // proto3 无法区分 unset 0，服务端只信任 oneof 内的值，故不落顶层。
    putIfPresent(m, 'behavior', behavior?.toJson());
    putIfPresent(m, 'risk', risk?.toJson());
    return m;
  }

  static ReportEvent fromJson(Map<String, Object?> j) {
    final type = UbaEventType.values.firstWhere(
      (t) => t.wire == j['eventType'],
      orElse: () => UbaEventType.behavior,
    );
    return ReportEvent(
      eventType: type,
      eventId: j['eventId'] as String? ?? '',
      eventName: j['eventName'] as String? ?? '',
      eventTime: j['eventTime'] as String? ?? '',
      userId: (j['userId'] as num?)?.toInt(),
      deviceId: j['deviceId'] as String?,
      sessionId: j['sessionId'] as String?,
      platform: j['platform'] as String?,
      eventCategory: j['eventCategory'] as String?,
      properties: (j['properties'] as Map?)?.cast<String, String>(),
      metrics: (j['metrics'] as Map?)?.cast<String, double>(),
      traceId: j['traceId'] as String?,
      eventAction: j['eventAction'] as String?,
      objectType: j['objectType'] as String?,
      objectId: j['objectId'] as String?,
      objectName: j['objectName'] as String?,
      amount: j['amount'] as String?,
      opResult: j['opResult'] as String?,
      errorCode: j['errorCode'] as String?,
      behavior: j['behavior'] is Map
          ? BehaviorEvent.fromJson(
              (j['behavior'] as Map).cast<String, Object?>())
          : null,
      risk: j['risk'] is Map
          ? RiskEvent.fromJson((j['risk'] as Map).cast<String, Object?>())
          : null,
    );
  }
}

/// 一次 flush 的结果。
class FlushResult {
  /// 本次 flush 是否成功（服务端 2xx 且队列清空）。
  final bool success;

  /// 本次成功发送的事件数。
  final int sent;

  /// 本次未送达的事件数（不可重试 4xx / 重试耗尽 / 部分失败）。
  final int dropped;

  /// 附加说明（如失败原因）。
  final String? message;

  /// 失败是否值得补发：网络错误 / 5xx / 429 为 true；
  /// 401、400 等确定性失败为 false（补发也不会成功，直接丢弃）。
  final bool retryable;

  const FlushResult({
    required this.success,
    this.sent = 0,
    this.dropped = 0,
    this.message,
    this.retryable = false,
  });

  @override
  String toString() =>
      'FlushResult(success: $success, sent: $sent, dropped: $dropped, '
      'retryable: $retryable, message: $message)';
}

/// collector `PostReportResponse`（仅解析 SDK 关心的字段）。
class PostReportResponse {
  final bool success;
  final String? message;
  final int totalCount;
  final int successCount;
  final int failedCount;
  final String? requestId;

  const PostReportResponse({
    required this.success,
    this.message,
    this.totalCount = 0,
    this.successCount = 0,
    this.failedCount = 0,
    this.requestId,
  });

  static PostReportResponse fromJson(Map<String, Object?> j) =>
      PostReportResponse(
        success: j['success'] == true,
        message: j['message'] as String?,
        totalCount: (j['totalCount'] as num?)?.toInt() ?? 0,
        successCount: (j['successCount'] as num?)?.toInt() ?? 0,
        failedCount: (j['failedCount'] as num?)?.toInt() ?? 0,
        requestId: j['requestId'] as String?,
      );
}
