import 'dart:async';
import 'dart:convert';
import 'dart:io';

/// HTTP 结果抽象：statusCode 0 表示网络层失败（超时/DNS/连接拒绝），可重试。
class HttpResult {
  final int statusCode;
  final String body;

  const HttpResult(this.statusCode, this.body);

  bool get isOk => statusCode >= 200 && statusCode < 300;
}

/// 传输层抽象。默认实现走 `dart:io HttpClient`（Android/iOS/桌面全支持）；
/// 换 WebView 通道 / 代理网关 / Web fetch 时实现本接口即可，无需改动 Client 与 Batcher。
abstract class HttpTransport {
  Future<HttpResult> send(String url, String body, int timeoutMs);

  /// 释放底层连接资源。
  void close();
}

/// `dart:io` 实现的长连接 transport，复用 keep-alive 连接。
class IoHttpTransport implements HttpTransport {
  final HttpClient _client = HttpClient()
    ..autoUncompress = false // collector 不支持 gzip，保持明文
    ..userAgent = null; // UA 在 body 的 clientInfo 里带，header 不需要

  @override
  Future<HttpResult> send(String url, String body, int timeoutMs) async {
    final timeout = Duration(milliseconds: timeoutMs);
    try {
      final request = await _client
          .postUrl(Uri.parse(url))
          .timeout(timeout);
      request.headers.set(HttpHeaders.contentTypeHeader, 'application/json');
      request.write(body);
      final response = await request.close().timeout(timeout);
      final text = await response
          .transform(utf8.decoder)
          .join()
          .timeout(timeout);
      return HttpResult(response.statusCode, text);
    } on TimeoutException {
      return const HttpResult(0, 'timeout');
    } on SocketException catch (e) {
      return HttpResult(0, e.message);
    } on HttpException catch (e) {
      return HttpResult(0, e.message);
    }
  }

  @override
  void close() {
    _client.close(force: true);
  }
}
