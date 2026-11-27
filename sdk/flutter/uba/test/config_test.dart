import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/config.dart';

void main() {
  const valid = UbaConfig(
    appId: 'app',
    appSecret: 'secret',
    endpoint: 'http://localhost:5700/',
  );

  test('必填缺失抛断言', () {
    expect(
      () => UbaConfig(appId: '', appSecret: 's', endpoint: 'e'),
      throwsA(isA<Error>()),
      reason: 'appId 为空',
    );
    expect(
      () => UbaConfig(appId: 'a', appSecret: '', endpoint: 'e'),
      throwsA(isA<Error>()),
      reason: 'appSecret 为空',
    );
    expect(
      () => UbaConfig(appId: 'a', appSecret: 's', endpoint: ''),
      throwsA(isA<Error>()),
      reason: 'endpoint 为空',
    );
  });

  test('timeoutMs 必须 < 10000（服务端 10s）', () {
    expect(
      () => UbaConfig(
        appId: 'a',
        appSecret: 's',
        endpoint: 'e',
        timeoutMs: 10000,
      ),
      throwsA(isA<Error>()),
    );
  });

  test('maxBatchBytes 必须压在 1MiB 内', () {
    expect(
      () => UbaConfig(
        appId: 'a',
        appSecret: 's',
        endpoint: 'e',
        maxBatchBytes: 1024 * 1024,
      ),
      throwsA(isA<Error>()),
    );
  });

  test('endpoint 尾斜杠规整与 URL 拼接', () {
    expect(valid.baseEndpoint, 'http://localhost:5700');
    expect(valid.reportUrl, 'http://localhost:5700/uba/v1/report');
    expect(valid.flagsUrl, 'http://localhost:5700/uba/v1/flags');

    const noSlash = UbaConfig(
      appId: 'a',
      appSecret: 's',
      endpoint: 'http://localhost:5700',
    );
    expect(noSlash.baseEndpoint, 'http://localhost:5700');
    expect(noSlash.reportUrl, 'http://localhost:5700/uba/v1/report');
  });

  test('默认值与 Web/C# SDK 对齐', () {
    expect(valid.path, '/uba/v1/report');
    expect(valid.batchSize, 20);
    expect(valid.flushIntervalMs, 5000);
    expect(valid.maxRetries, 3);
    expect(valid.timeoutMs, 8000);
    expect(valid.retryBaseDelayMs, 1000);
    expect(valid.persistQueue, isTrue);
    expect(valid.maxPersistedEvents, 500);
    expect(valid.sessionTimeoutMs, 30000);
    expect(valid.sampleRate, 1.0);
    expect(valid.requireConsent, isFalse);
    expect(valid.autoTrackLaunch, isFalse);
    expect(valid.pageViewEvent, 'page_view');
  });
}
