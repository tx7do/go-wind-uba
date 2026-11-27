import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/client.dart';
import 'package:uba_flutter/src/config.dart';
import 'package:uba_flutter/src/store.dart';
import 'package:uba_flutter/src/transport.dart';
import 'package:uba_flutter/src/types.dart';

import 'helpers.dart';

void main() {
  late FakeTransport transport;
  late MemoryStore store;

  UbaConfig config({
    bool requireConsent = false,
    double sampleRate = 1.0,
    bool autoTrackLaunch = false,
    int batchSize = 100,
  }) =>
      UbaConfig(
        appId: 'app_001',
        appSecret: 'secret_001',
        endpoint: 'http://localhost:5700',
        requireConsent: requireConsent,
        sampleRate: sampleRate,
        autoTrackLaunch: autoTrackLaunch,
        batchSize: batchSize,
      );

  setUp(() {
    transport = FakeTransport();
    store = MemoryStore();
  });

  group('consent 合规门控', () {
    test('requireConsent=true 时未授权不发', () async {
      final client = await UbaClient.create(
        config(requireConsent: true),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(),
      );
      expect(client.isCollecting, isFalse);
      client.track('t');
      expect(client.pendingCount, 0);
      final r = await client.flush();
      expect(r.sent, 0);
      expect(transport.requests, isEmpty);
      await client.close();
    });

    test('授权后恢复采集', () async {
      final client = await UbaClient.create(
        config(requireConsent: true),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(),
      );
      client.setConsent(true);
      expect(client.isCollecting, isTrue);
      client.track('t');
      await client.flush();
      expect(transport.requests.length, 1);
      await client.close();
    });

    test('默认不要求授权', () async {
      final client = await UbaClient.create(
        config(),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(),
      );
      expect(client.isCollecting, isTrue);
      await client.close();
    });
  });

  group('采样', () {
    test('sampleRate=0 全部丢弃', () async {
      final client = await UbaClient.create(
        config(sampleRate: 0),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(),
      );
      expect(client.isCollecting, isFalse);
      client.track('t');
      client.trackRisk('risk', const RiskEvent());
      expect(client.pendingCount, 0);
      await client.flush();
      expect(transport.requests, isEmpty);
      await client.close();
    });
  });

  group('用户绑定', () {
    test('identify / resetUser', () async {
      final client = await UbaClient.create(
        config(),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(),
      );
      client.identify(42);
      client.track('login');
      client.resetUser();
      client.track('logout');
      await client.flush();

      final events =
          (transport.requests.first.body['events'] as List<Object?>)
              .cast<Map<Object?, Object?>>();
      expect(events[0]['userId'], 42);
      expect(events[1].containsKey('userId'), isFalse);
      await client.close();
    });
  });

  group('首装自动埋点', () {
    test('deviceId 新建时上报 app_launch', () async {
      final client = await UbaClient.create(
        config(autoTrackLaunch: true),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(deviceIdIsNew: true),
      );
      expect(client.pendingCount, 1);
      await client.flush();

      final e = (transport.requests.first.body['events'] as List<Object?>)
          .single as Map<Object?, Object?>;
      expect(e['eventName'], 'app_launch');
      expect(e['eventCategory'], 'app');
      expect(e['properties'], {'first_launch': 'true'});
      await client.close();
    });

    test('老设备不触发', () async {
      final client = await UbaClient.create(
        config(autoTrackLaunch: true),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(deviceIdIsNew: false),
      );
      expect(client.pendingCount, 0);
      await client.close();
    });
  });

  group('后台 flush', () {
    test('handleBackground 立即发出队列事件', () async {
      final client = await UbaClient.create(
        config(),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(),
      );
      client.track('paused_event');
      expect(client.pendingCount, 1);
      client.handleBackground();
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(transport.requests.length, 1);
      expect(client.pendingCount, 0);
      client.handleForeground(); // 短时间后台不轮换会话
      client.track('resumed_event');
      await client.flush();
      final events = transport.requests
          .expand((r) => r.body['events'] as List<Object?>)
          .cast<Map<Object?, Object?>>()
          .toList();
      final sessionIds = events.map((e) => e['sessionId']).toSet();
      expect(sessionIds.length, 1, reason: '短后台不应轮换 sessionId');
      await client.close();
    });
  });

  group('AB 实验开关', () {
    test('loadFlags / getFlag / 曝光事件每会话一次', () async {
      transport.responsesByUrl['http://localhost:5700/uba/v1/flags'] = [
        const HttpResult(
          200,
          '{"flags": {"exp_color": "red", "ctl": "control"}}',
        ),
      ];
      final client = await UbaClient.create(
        config(),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(),
      );
      client.identify(7);
      final flags = await client.loadFlags();

      expect(flags, {'exp_color': 'red', 'ctl': 'control'});
      expect(client.getFlag('exp_color'), 'red');
      expect(client.isFeatureEnabled('exp_color'), isTrue);
      expect(client.isFeatureEnabled('ctl'), isFalse, reason: 'control 组不命中');
      expect(client.getFlag('missing'), isNull);

      // flags 请求体带身份
      final flagsReq =
          transport.requests.firstWhere((r) => r.url.contains('/flags'));
      expect(flagsReq.body['deviceId'], '11111111-2222-3333-4444-555555555555');
      expect(flagsReq.body['userId'], 7);

      // 曝光事件入队（getFlag('missing') 不曝光）
      await client.flush();
      final events = transport.requests
          .where((r) => r.url.endsWith('/uba/v1/report'))
          .expand((r) => r.body['events'] as List<Object?>)
          .cast<Map<Object?, Object?>>()
          .where((e) => e['eventName'] == r'$feature_flag')
          .toList();
      expect(events, hasLength(1));
      expect(events.single['properties'],
          {'flag_key': 'exp_color', 'variant': 'red'});
      expect(events.single['eventCategory'], 'experiment');

      // 同会话第二次 getFlag 不重复曝光；ctl 变体也曝光
      client.getFlag('exp_color');
      client.getFlag('ctl');
      expect(client.pendingCount, 1, reason: '仅 ctl 新增一次曝光，exp_color 已去重');
      await client.close();
    });
  });

  group('全局单例', () {
    test('init 后 instance 可用，重复 init 关闭旧实例', () async {
      final c1 = await UbaClient.init(
        config(),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(),
      );
      expect(UbaClient.instance, same(c1));
      expect(c1.isCollecting, isTrue);

      final transport2 = FakeTransport();
      final c2 = await UbaClient.init(
        config(),
        transport: transport2,
        store: MemoryStore(),
        contextProvider: FakeContextProvider(),
      );
      expect(UbaClient.instance, same(c2));
      expect(c1.isCollecting, isFalse, reason: '旧实例应已被 close');

      await c2.close();
      expect(UbaClient.instance, isNull);
    });
  });

  group('close 语义', () {
    test('close 前先 flush 残留队列', () async {
      final client = await UbaClient.create(
        config(),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(),
      );
      client.track('last_words');
      await client.close();
      expect(transport.requests.length, 1);
      expect(client.pendingCount, 0);
    });
  });
}
