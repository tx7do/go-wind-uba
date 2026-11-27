import 'dart:async';
import 'dart:convert';

import 'config.dart';
import 'store.dart';
import 'transport.dart';
import 'types.dart';

/// 队列 + 批量上报器：
/// - 内存队列，达到 batchSize 或 flushIntervalMs 定时器到期即 flush；
/// - 重试策略对齐 C# SDK：指数退避，4xx（除 429）不重试整批丢弃；
/// - 重试耗尽且开启 persistQueue 时把事件落盘，下次启动补发（对齐 Web SDK 离线镜像）。
class Batcher {
  final UbaConfig config;
  final HttpTransport transport;
  final KeyValueStore? store;

  /// 每次 flush 时刷新 envelope 的 clientInfo（如 UA）。
  final ClientInfo? Function()? clientInfoProvider;

  /// 调试日志回调（仅 config.debug 时被调用）。
  final void Function(String message)? onLog;

  final List<ReportEvent> _queue = <ReportEvent>[];
  Timer? _timer;
  bool _flushing = false;
  bool _closed = false;

  Batcher({
    required this.config,
    required this.transport,
    this.store,
    this.clientInfoProvider,
    this.onLog,
  });

  int get size => _queue.length;

  void _log(String message) => onLog?.call(message);

  /// 启动定时 flush。flushIntervalMs 为 0 时不启动。
  void start() {
    if (config.flushIntervalMs <= 0 || _timer != null) return;
    _timer = Timer.periodic(
      Duration(milliseconds: config.flushIntervalMs),
      (_) {
        if (_queue.isNotEmpty) unawaited(flush());
      },
    );
  }

  /// 停止定时器（不影响手动 flush）。
  void stop() {
    _timer?.cancel();
    _timer = null;
  }

  /// 入队；达到批量阈值时异步触发一次 flush（不阻塞调用方）。
  void enqueue(ReportEvent event) {
    if (_closed) return;
    _queue.add(event);
    if (_queue.length >= config.batchSize) {
      unawaited(flush());
    }
  }

  /// 把落盘的离线队列恢复到内存队首（最旧优先发送）。
  Future<void> restorePersisted() async {
    final s = store;
    if (s == null || !config.persistQueue) return;
    final raw = await s.getString(kOfflineQueueKey);
    if (raw == null || raw.isEmpty) return;
    try {
      final list = jsonDecode(raw) as List<Object?>;
      final restored = list
          .whereType<Map<Object?, Object?>>()
          .map((m) => ReportEvent.fromJson(m.cast<String, Object?>()))
          .toList();
      _queue.insertAll(0, restored);
      await s.remove(kOfflineQueueKey);
      _log('离线队列恢复 ${restored.length} 条');
    } on FormatException catch (e) {
      _log('离线队列解析失败，已忽略：$e');
      await s.remove(kOfflineQueueKey);
    }
  }

  /// 手动 flush：按批量/字节上限分批发送，直到清空队列或遇到失败。
  /// 并发调用是安全的：正在 flush 时再次调用直接返回（不重复发送）。
  Future<FlushResult> flush() async {
    if (_flushing || _closed) {
      return const FlushResult(success: true);
    }
    _flushing = true;
    var sent = 0;
    var dropped = 0;
    var ok = true;
    var retryable = false;
    String? message;
    try {
      while (_queue.isNotEmpty) {
        final batch = _takeBatch();
        final result = await _sendWithRetry(batch);
        sent += result.sent;
        dropped += result.dropped;
        if (!result.success) {
          ok = false;
          message = result.message;
          retryable = result.retryable;
          // 可重试失败（网络/5xx/429）回到队首，稍后补发；确定性 4xx 直接丢弃；
          // 停止本轮避免在持续故障下空转
          if (result.retryable) _queue.insertAll(0, batch);
          break;
        }
      }
      await _persistQueue();
    } finally {
      _flushing = false;
    }
    return FlushResult(
      success: ok,
      sent: sent,
      dropped: dropped,
      retryable: retryable,
      message: message,
    );
  }

  /// 关闭：停止定时器并拒绝后续入队。已有队列交给调用方 flush 或落盘。
  void close() {
    _closed = true;
    stop();
  }

  /// 取出一批：受 batchSize 与 maxBatchBytes（UTF-8 字节数）双重限制。
  /// 单条超限的事件仍然单独成批（服务端会 413，按不可重试处理丢弃）。
  List<ReportEvent> _takeBatch() {
    final batch = <ReportEvent>[];
    var bytes = 0;
    while (_queue.isNotEmpty && batch.length < config.batchSize) {
      final size = utf8.encode(jsonEncode(_queue.first.toJson())).length;
      if (batch.isNotEmpty && bytes + size > config.maxBatchBytes) break;
      bytes += size;
      batch.add(_queue.removeAt(0));
    }
    return batch;
  }

  Future<FlushResult> _sendWithRetry(List<ReportEvent> batch) async {
    final envelope = <String, Object?>{
      'appId': config.appId,
      'appSecret': config.appSecret,
      'events': batch.map((e) => e.toJson()).toList(),
    };
    final clientInfo = clientInfoProvider?.call();
    if (clientInfo != null) {
      envelope['clientInfo'] = clientInfo.toJson();
    }
    final body = jsonEncode(envelope);

    for (var attempt = 0;; attempt++) {
      final res = await transport.send(config.reportUrl, body, config.timeoutMs);

      if (res.isOk) {
        var failed = 0;
        try {
          final parsed = PostReportResponse.fromJson(
            (jsonDecode(res.body) as Map<Object?, Object?>)
                .cast<String, Object?>(),
          );
          // HTTP 200 也可能部分失败（单条校验被拒），失败条数按丢弃计
          failed = parsed.failedCount;
          if (failed > 0) {
            _log('部分事件被服务端拒绝：${parsed.message}');
          }
        } on FormatException {
          _log('响应解析失败（HTTP ${res.statusCode}），按整批成功处理');
        }
        return FlushResult(
          success: true,
          sent: batch.length - failed,
          dropped: failed,
        );
      }

      final status = res.statusCode;
      // 4xx（除 429）是确定性失败：整批丢弃，不重试（重试也必然再失败）
      if (status >= 400 && status < 500 && status != 429) {
        _log('HTTP $status（不可重试），丢弃 ${batch.length} 条');
        return FlushResult(
          success: false,
          dropped: batch.length,
          message: 'HTTP $status',
        );
      }

      if (attempt >= config.maxRetries) {
        _log('重试耗尽（HTTP $status），${batch.length} 条待落盘补发');
        return FlushResult(
          success: false,
          dropped: batch.length,
          retryable: true,
          message: 'HTTP $status after ${attempt + 1} attempts',
        );
      }

      final delay = config.retryBaseDelayMs * (1 << attempt);
      _log('HTTP $status，${delay}ms 后第 ${attempt + 1} 次重试');
      await Future<void>.delayed(Duration(milliseconds: delay));
    }
  }

  /// 把当前内存队列镜像到 KV 存储（cap 后保留最新的事件）。
  Future<void> _persistQueue() async {
    final s = store;
    if (s == null || !config.persistQueue) return;
    if (_queue.length > config.maxPersistedEvents) {
      _queue.removeRange(0, _queue.length - config.maxPersistedEvents);
    }
    if (_queue.isEmpty) {
      await s.remove(kOfflineQueueKey);
      return;
    }
    final encoded = jsonEncode(_queue.map((e) => e.toJson()).toList());
    await s.setString(kOfflineQueueKey, encoded);
  }
}
