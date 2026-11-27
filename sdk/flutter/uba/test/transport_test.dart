import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/transport.dart';

/// IoHttpTransport 真网络路径：本地 HttpServer 收发，零外部依赖。
void main() {
  test('发送 JSON POST 并读取 200 响应', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final receivedBody = Completer<String>();
    final receivedContentType = Completer<String?>();
    server.listen((req) async {
      receivedContentType.complete(req.headers.contentType?.toString());
      receivedBody.complete(await utf8.decoder.bind(req).join());
      req.response.statusCode = 200;
      req.response.write(
        '{"success": true, "totalCount": 1, "successCount": 1, "failedCount": 0}',
      );
      await req.response.close();
    });

    final transport = IoHttpTransport();
    try {
      final res = await transport.send(
        'http://127.0.0.1:${server.port}/uba/v1/report',
        '{"appId": "a"}',
        5000,
      );
      expect(res.statusCode, 200);
      expect(res.isOk, isTrue);
      expect(await receivedContentType.future, 'application/json');
      expect(await receivedBody.future, '{"appId": "a"}');
    } finally {
      transport.close();
      await server.close(force: true);
    }
  });

  test('服务端 4xx 原样透传状态码', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    server.listen((req) async {
      req.response.statusCode = 401;
      req.response.write('{"code": 401, "reason": "INCORRECT_APP_SECRET"}');
      await req.response.close();
    });

    final transport = IoHttpTransport();
    try {
      final res = await transport.send(
        'http://127.0.0.1:${server.port}/uba/v1/report',
        '{}',
        5000,
      );
      expect(res.statusCode, 401);
      expect(res.isOk, isFalse);
    } finally {
      transport.close();
      await server.close(force: true);
    }
  });

  test('服务端超时 → statusCode 0（可重试信号）', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    server.listen((req) async {
      await Future<void>.delayed(const Duration(milliseconds: 400));
      req.response.statusCode = 200;
      await req.response.close();
    });

    final transport = IoHttpTransport();
    try {
      final res = await transport.send(
        'http://127.0.0.1:${server.port}/uba/v1/report',
        '{}',
        100, // 比服务端 400ms 短
      );
      expect(res.statusCode, 0, reason: '超时应返回 0 以便批量器按可重试处理');
    } finally {
      transport.close();
      await server.close(force: true);
    }
  });
}
