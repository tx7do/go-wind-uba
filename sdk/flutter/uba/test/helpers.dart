import 'dart:convert';

import 'package:uba_flutter/src/context.dart';
import 'package:uba_flutter/src/transport.dart';

/// 记录式假 transport：按 URL 可编程响应，默认返回成功。
class RecordedRequest {
  final String url;
  final Map<String, Object?> body;

  RecordedRequest(this.url, this.body);
}

class FakeTransport implements HttpTransport {
  /// 编程式响应：按请求 URL 分派，返回第 n 次命中的结果。
  final Map<String, List<HttpResult>> responsesByUrl = {};

  final List<RecordedRequest> requests = <RecordedRequest>[];

  /// 未命中编程响应时（或列表耗尽时）的默认结果。
  HttpResult defaultResult =
      const HttpResult(200, '{"success": true, "totalCount": 1, "successCount": 1, "failedCount": 0}');

  /// 记录每次 send 调用的状态码序列（重试断言用）。
  final List<int> statusLog = <int>[];

  @override
  Future<HttpResult> send(String url, String body, int timeoutMs) async {
    final decoded =
        (jsonDecode(body) as Map<Object?, Object?>).cast<String, Object?>();
    requests.add(RecordedRequest(url, decoded));
    final queue = responsesByUrl[url];
    final result = (queue != null && queue.isNotEmpty)
        ? queue.removeAt(0)
        : defaultResult;
    statusLog.add(result.statusCode);
    return result;
  }

  @override
  void close() {}
}

/// 固定环境快照，deviceId/时间可控。
class FakeContextProvider implements ContextProvider {
  final ContextSnapshot snapshot;

  FakeContextProvider({
    String deviceId = '11111111-2222-3333-4444-555555555555',
    bool deviceIdIsNew = false,
    String platform = 'android',
    String os = 'Android',
    String userAgent = 'uba_flutter/$sdkVersionForTest (android)',
  }) : snapshot = ContextSnapshot(
          deviceId: deviceId,
          deviceIdIsNew: deviceIdIsNew,
          platform: platform,
          os: os,
          userAgent: userAgent,
        );

  @override
  Future<ContextSnapshot> load() async => snapshot;
}

const String sdkVersionForTest = '1.0.0';

/// 构造一个指定字节数的 JSON-safe 字符串。
String paddedString(int bytes) => 'a' * bytes;
