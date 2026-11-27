import 'dart:convert';

import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/batcher.dart';
import 'package:uba_flutter/src/config.dart';
import 'package:uba_flutter/src/store.dart';
import 'package:uba_flutter/src/transport.dart';
import 'package:uba_flutter/src/types.dart';

import 'helpers.dart';

void main() {
  late FakeTransport transport;
  late MemoryStore store;

  UbaConfig config({
    int batchSize = 100,
    int maxRetries = 0,
    int retryBaseDelayMs = 0,
    bool persistQueue = true,
    int maxBatchBytes = 900 * 1024,
    int maxPersistedEvents = 500,
    int flushIntervalMs = 0,
  }) =>
      UbaConfig(
        appId: 'app_001',
        appSecret: 'secret_001',
        endpoint: 'http://localhost:5700',
        batchSize: batchSize,
        maxRetries: maxRetries,
        retryBaseDelayMs: retryBaseDelayMs,
        persistQueue: persistQueue,
        maxBatchBytes: maxBatchBytes,
        maxPersistedEvents: maxPersistedEvents,
        flushIntervalMs: flushIntervalMs,
      );

  ReportEvent makeEvent(String name) => ReportEvent(
        eventType: UbaEventType.behavior,
        eventId: 'id-$name',
        eventName: name,
        eventTime: '2026-03-27T12:00:00.000Z',
        deviceId: 'dev',
      );

  setUp(() {
    transport = FakeTransport();
    store = MemoryStore();
  });

  group('批量触发', () {
    test('达到 batchSize 自动 flush', () async {
      final b = Batcher(
        config: config(batchSize: 3),
        transport: transport,
      );
      b.enqueue(makeEvent('e1'));
      b.enqueue(makeEvent('e2'));
      expect(transport.requests, isEmpty);
      b.enqueue(makeEvent('e3'));
      await Future<void>.delayed(const Duration(milliseconds: 10));
      expect(transport.requests.length, 1);
      expect((transport.requests.first.body['events'] as List).length, 3);
      expect(b.size, 0);
    });

    test('定时器到期 flush 非空队列', () async {
      final b = Batcher(
        config: config(flushIntervalMs: 50),
        transport: transport,
      )..start();
      b.enqueue(makeEvent('e1'));
      await Future<void>.delayed(const Duration(milliseconds: 300));
      b.stop();
      expect(transport.requests.length, 1);
    });

    test('队列空时定时器不发请求', () async {
      final b = Batcher(
        config: config(flushIntervalMs: 50),
        transport: transport,
      )..start();
      await Future<void>.delayed(const Duration(milliseconds: 300));
      b.stop();
      expect(transport.requests, isEmpty);
    });

    test('envelope 结构：appId/appSecret/clientInfo/events', () async {
      final b = Batcher(
        config: config(),
        transport: transport,
        clientInfoProvider: () => const ClientInfo(userAgent: 'test-ua'),
      );
      b.enqueue(makeEvent('e1'));
      await b.flush();

      final body = transport.requests.first.body;
      expect(body['appId'], 'app_001');
      expect(body['appSecret'], 'secret_001');
      expect((body['clientInfo'] as Map)['userAgent'], 'test-ua');
      expect(body['events'], isA<List>());
    });

    test('正在 flush 时的并发调用直接跳过', () async {
      final b = Batcher(config: config(), transport: transport);
      b.enqueue(makeEvent('e1'));
      final first = b.flush();
      final second = await b.flush(); // _flushing=true → 立即返回
      expect(second.sent, 0);
      expect(second.success, isTrue);
      await first;
      expect(transport.requests.length, 1);
    });
  });

  group('重试策略（对齐 C# SDK）', () {
    test('5xx 重试直至成功', () async {
      transport.responsesByUrl['http://localhost:5700/uba/v1/report'] = [
        const HttpResult(500, 'boom'),
        const HttpResult(503, 'boom'),
      ];
      final b = Batcher(
        config: config(maxRetries: 3),
        transport: transport,
      );
      b.enqueue(makeEvent('e1'));
      final r = await b.flush();
      expect(r.success, isTrue);
      expect(r.sent, 1);
      expect(transport.requests.length, 3); // 1 + 2 次重试
    });

    test('401 不重试，整批丢弃且不落盘', () async {
      transport.defaultResult = const HttpResult(401, '{"reason": "UNAUTHORIZED"}');
      final b = Batcher(
        config: config(maxRetries: 3),
        transport: transport,
        store: store,
      );
      b.enqueue(makeEvent('e1'));
      b.enqueue(makeEvent('e2'));
      final r = await b.flush();

      expect(r.success, isFalse);
      expect(r.dropped, 2);
      expect(r.retryable, isFalse);
      expect(transport.requests.length, 1); // 只打了一次
      expect(await store.getString(kOfflineQueueKey), isNull);
    });

    test('429 视为可重试', () async {
      transport.defaultResult = const HttpResult(429, 'rate limited');
      final b = Batcher(
        config: config(maxRetries: 2),
        transport: transport,
        store: store,
      );
      b.enqueue(makeEvent('e1'));
      final r = await b.flush();
      expect(transport.requests.length, 3); // 1 + 2 重试
      expect(r.retryable, isTrue);
    });

    test('网络失败（statusCode 0）视为可重试', () async {
      transport.defaultResult = const HttpResult(0, 'timeout');
      final b = Batcher(
        config: config(maxRetries: 1),
        transport: transport,
        store: store,
      );
      b.enqueue(makeEvent('e1'));
      final r = await b.flush();
      expect(transport.requests.length, 2);
      expect(r.retryable, isTrue);
    });

    test('指数退避节奏：1s → 2s → 4s', () {
      fakeAsync((async) {
        transport.defaultResult = const HttpResult(500, 'boom');
        final b = Batcher(
          config: UbaConfig(
            appId: 'a',
            appSecret: 's',
            endpoint: 'http://x',
            maxRetries: 3,
            retryBaseDelayMs: 1000,
            flushIntervalMs: 0,
          ),
          transport: transport,
        );
        b.enqueue(makeEvent('e1'));
        var completed = false;
        final done = b.flush();
        done.then((_) => completed = true);
        async.elapse(const Duration(milliseconds: 1000)); // 第 1 次重试
        expect(transport.requests.length, 2);
        async.elapse(const Duration(milliseconds: 2000)); // 第 2 次
        expect(transport.requests.length, 3);
        async.elapse(const Duration(milliseconds: 4000)); // 第 3 次 → 耗尽
        expect(transport.requests.length, 4);
        expect(completed, isTrue);
      });
    });

    test('HTTP 200 但 failedCount>0：部分失败按丢弃计', () async {
      transport.defaultResult = const HttpResult(
        200,
        '{"success": true, "totalCount": 3, "successCount": 1, "failedCount": 2, '
        '"errorsByType": [{"type": "behavior", "errors": [{"code": "INVALID_EVENT", "message": "no name", "eventId": "id-e1"}]}]}',
      );
      final b = Batcher(
        config: config(),
        transport: transport,
        store: store,
      );
      b.enqueue(makeEvent('e1'));
      b.enqueue(makeEvent('e2'));
      b.enqueue(makeEvent('e3'));
      final r = await b.flush();
      expect(r.success, isTrue);
      expect(r.sent, 1);
      expect(r.dropped, 2);
      expect(b.size, 0); // 失败条目服务端已明确拒绝，不回队
    });
  });

  group('离线持久化', () {
    test('可重试耗尽 → 队列落盘 → 新客户端恢复补发', () async {
      transport.defaultResult = const HttpResult(500, 'down');
      final b = Batcher(
        config: config(maxRetries: 1),
        transport: transport,
        store: store,
      );
      b.enqueue(makeEvent('e1'));
      b.enqueue(makeEvent('e2'));
      final r = await b.flush();
      expect(r.dropped, 2);

      final persisted = await store.getString(kOfflineQueueKey);
      expect(persisted, isNotNull);
      expect((jsonDecode(persisted!) as List).length, 2);

      // 网络恢复：新 batcher 恢复队列并成功补发
      transport.defaultResult = FakeTransport().defaultResult;
      final b2 = Batcher(config: config(), transport: transport, store: store);
      await b2.restorePersisted();
      expect(b2.size, 2);
      final r2 = await b2.flush();
      expect(r2.sent, 2);
      expect(await store.getString(kOfflineQueueKey), isNull,
          reason: '清空后应移除持久化键');
    });

    test('persistQueue=false 时失败直接丢', () async {
      transport.defaultResult = const HttpResult(500, 'down');
      final b = Batcher(
        config: config(maxRetries: 0, persistQueue: false),
        transport: transport,
        store: store,
      );
      b.enqueue(makeEvent('e1'));
      await b.flush();
      expect(await store.getString(kOfflineQueueKey), isNull);
    });

    test('恢复的损坏 JSON 被忽略并清除', () async {
      await store.setString(kOfflineQueueKey, '{{{not json');
      final b = Batcher(config: config(), transport: transport, store: store);
      await b.restorePersisted();
      expect(b.size, 0);
      expect(await store.getString(kOfflineQueueKey), isNull);
    });

    test('落盘上限：保留最新 maxPersistedEvents 条', () async {
      transport.defaultResult = const HttpResult(500, 'down');
      final b = Batcher(
        config: config(
          maxRetries: 0,
          persistQueue: true,
          maxPersistedEvents: 3,
        ),
        transport: transport,
        store: store,
      );
      for (var i = 0; i < 7; i++) {
        b.enqueue(makeEvent('e$i'));
      }
      await b.flush();
      final persisted =
          (jsonDecode((await store.getString(kOfflineQueueKey))!) as List)
              .cast<Map<Object?, Object?>>();
      expect(persisted.length, 3);
      // 保留的是最新的 e4/e5/e6
      final names = persisted.map((m) => m['eventName']).toList();
      expect(names, containsAll(['e4', 'e5', 'e6']));
      expect(names, isNot(contains('e0')));
    });
  });

  group('字节上限', () {
    test('按 maxBatchBytes 拆分多批', () async {
      final b = Batcher(
        config: config(batchSize: 100, maxBatchBytes: 400),
        transport: transport,
      );
      // 每条约 ~200 字节 → 两批
      for (var i = 0; i < 4; i++) {
        b.enqueue(ReportEvent(
          eventType: UbaEventType.behavior,
          eventId: 'id-$i',
          eventName: 'evt',
          eventTime: '2026-03-27T12:00:00.000Z',
          deviceId: 'dev',
          properties: {'payload': paddedString(150)},
        ));
      }
      final r = await b.flush();
      expect(r.sent, 4);
      expect(transport.requests.length, greaterThanOrEqualTo(2));
      // 每个请求都在上限内
      for (final req in transport.requests) {
        expect(req.body['events'] as List<Object?>, hasLength(lessThan(4)));
      }
    });
  });

  group('关闭语义', () {
    test('close 后不再入队、flush 直接返回', () async {
      final b = Batcher(config: config(), transport: transport);
      b.enqueue(makeEvent('e1'));
      await b.flush();
      b.close();
      b.enqueue(makeEvent('e2'));
      expect(b.size, 0);
      final r = await b.flush();
      expect(r.sent, 0);
      expect(transport.requests.length, 1);
    });
  });
}
