import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/client.dart';
import 'package:uba_flutter/src/config.dart';
import 'package:uba_flutter/src/store.dart';
import 'package:uba_flutter/src/types.dart';

import 'helpers.dart';

/// 契约金样：上报 body 的字段名/位置/类型必须与后端 proto 契约完全一致。
/// 对照 backend/api/protos/uba/service/v1/report.proto 的 json_name 与
/// backend/app/collector/service/README.md 的样例。
void main() {
  late FakeTransport transport;
  late MemoryStore store;

  Future<UbaClient> buildClient({bool persistQueue = true}) {
    return UbaClient.create(
      UbaConfig(
        appId: 'app_001',
        appSecret: 'secret_001',
        endpoint: 'http://localhost:5700',
        appVersion: '1.2.3',
        channel: 'official',
        batchSize: 100,
        persistQueue: persistQueue,
        debug: false,
      ),
      transport: transport,
      store: store,
      contextProvider: FakeContextProvider(
        deviceId: 'device-fixed-0001',
      ),
    );
  }

  setUp(() {
    transport = FakeTransport();
    store = MemoryStore();
  });

  Map<String, Object?> lastEvent() {
    expect(transport.requests, isNotEmpty, reason: 'flush 后应产生一次上报');
    final body = transport.requests.last.body;
    expect(body['appId'], 'app_001');
    expect(body['appSecret'], 'secret_001');
    final events = body['events'] as List<Object?>;
    return ((events.single as Map<Object?, Object?>).cast<String, Object?>());
  }

  group('行为事件契约', () {
    test('全维度事件：顶层 + behavior oneof 镜像，数值标量只在 oneof 内', () async {
      final client = await buildClient();
      client.identify(1001);
      client.track(
        'level_pass',
        properties: {'scene': 'game', 'diff': 'hard'},
        options: const TrackOptions(
          eventCategory: 'game',
          eventAction: 'pass',
          objectType: 'level',
          objectId: 'level_42',
          objectName: '第42关',
          durationMs: 3500,
          amount: '0.99',
          quantity: 2,
          score: 88,
          metrics: {'speed': 1.5, 'combo': 3.0},
          network: 'WiFi',
          opResult: 'success',
          serverId: 's1',
          level: 42,
          traceId: 'trace_abc',
        ),
      );
      await client.flush();
      await client.close();

      final e = lastEvent();

      // ── 必填四件套 ──
      expect(e['eventType'], 'BEHAVIOR');
      expect(e['eventId'], matches(RegExp(r'^[0-9a-f-]{36}$')));
      expect(e['eventName'], 'level_pass');
      expect(e['eventTime'], matches(RegExp(r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$')));

      // ── 身份与环境 ──
      expect(e['userId'], 1001); // int，不是字符串
      expect(e['deviceId'], 'device-fixed-0001');
      expect(e['sessionId'], client.session.id);
      expect(e['platform'], 'android');

      // ── 属性 ──
      expect(e['properties'], {'scene': 'game', 'diff': 'hard'});
      expect(e['metrics'], {'speed': 1.5, 'combo': 3.0});
      expect(e['traceId'], 'trace_abc');
      expect(e['eventCategory'], 'game');

      // ── 顶层字符串镜像维度 ──
      expect(e['eventAction'], 'pass');
      expect(e['objectType'], 'level');
      expect(e['objectId'], 'level_42');
      expect(e['objectName'], '第42关');
      expect(e['amount'], '0.99');
      expect(e['opResult'], 'success');

      // ── 数值标量绝不出现在顶层（proto3 无法区分 unset 0，服务端只信 oneof） ──
      expect(e.containsKey('durationMs'), isFalse,
          reason: 'durationMs 顶层无意义，必须只进 behavior');
      expect(e.containsKey('quantity'), isFalse);
      expect(e.containsKey('score'), isFalse);
      expect(e.containsKey('sessionSeq'), isFalse);
      expect(e.containsKey('level'), isFalse);

      // ── 服务端专属字段禁止发送 ──
      expect(e.containsKey('tenantId'), isFalse,
          reason: 'tenantId 由服务端按 appId 覆盖，客户端不应发送');
      expect(e.containsKey('serverTime'), isFalse);
      expect(e.containsKey('ip'), isFalse, reason: '服务端从连接信息取真实 IP');

      // ── behavior oneof 载荷 ──
      final behavior = e['behavior'] as Map<String, Object?>;
      expect(behavior['eventAction'], 'pass');
      expect(behavior['objectType'], 'level');
      expect(behavior['objectId'], 'level_42');
      expect(behavior['objectName'], '第42关');
      expect(behavior['durationMs'], 3500); // 数值标量在 oneof 内才被信任
      expect(behavior['quantity'], 2);
      expect(behavior['score'], 88);
      expect(behavior['os'], 'Android');
      expect(behavior['appVersion'], '1.2.3');
      expect(behavior['channel'], 'official');
      expect(behavior['network'], 'WiFi');
      expect(behavior['opResult'], 'success');
      expect(behavior['serverId'], 's1');
      expect(behavior['level'], 42);
      expect(behavior['metrics'], {'speed': 1.5, 'combo': 3.0});

      // ── 顶层键集合精确（无多余字段，camelCase 对齐 proto json_name） ──
      expect(
        e.keys.toSet(),
        {
          'eventType', 'eventId', 'eventName', 'eventTime',
          'userId', 'deviceId', 'sessionId', 'platform',
          'eventCategory', 'properties', 'metrics', 'traceId',
          'eventAction', 'objectType', 'objectId', 'objectName',
          'amount', 'opResult', 'behavior',
        },
      );
    });

    test('最小事件：只带事件名也能成立', () async {
      final client = await buildClient();
      client.track('button_click');
      await client.flush();
      await client.close();

      final e = lastEvent();
      expect(e['eventType'], 'BEHAVIOR');
      expect(e['eventName'], 'button_click');
      expect(e['userId'], isNull); // 未 identify 不带 userId
      expect(e.containsKey('userId'), isFalse);
      expect(e['properties'], isNull); // 无属性不发 properties
      expect((e['behavior'] as Map<String, Object?>)['os'], 'Android');
    });

    test('super 属性与调用属性合并：调用侧覆盖', () async {
      final client = await buildClient();
      client.setSuperProperties({'common': 's', 'a': 'super'});
      client.track('t1', properties: {'a': 'call', 'own': 'x'});
      client.setSuperProperties({'later': 'yes'});
      client.track('t2');
      await client.flush();
      await client.close();

      final body = transport.requests.last.body;
      final events =
          (body['events'] as List<Object?>).cast<Map<Object?, Object?>>();
      expect(events[0]['properties'], {'common': 's', 'a': 'call', 'own': 'x'});
      // setSuperProperties 是增量合并：t1 的 common 仍在，t2 也有 later
      expect(events[1]['properties'], {'common': 's', 'a': 'super', 'later': 'yes'});
    });

    test('TrackOptions.userId 覆盖 identify 的用户', () async {
      final client = await buildClient();
      client.identify(1);
      client.track('t', options: const TrackOptions(userId: 999));
      await client.flush();
      await client.close();
      expect(lastEvent()['userId'], 999);
    });
  });

  group('风险事件契约', () {
    test('risk oneof 载荷逐字段', () async {
      final client = await buildClient();
      client.identify(1001);
      client.trackRisk(
        'login_failed',
        const RiskEvent(
          riskType: 'login_anomaly',
          riskLevel: 'HIGH',
          riskScore: 88.5,
          ruleId: 101,
          ruleName: '1分钟内登录失败超过5次',
          ruleContext: {'threshold': 5, 'window_sec': 60},
          relatedEventIds: ['e1', 'e2'],
          description: '1分钟内登录失败8次',
          evidence: {'ip': '1.2.3.4'},
          status: 'PENDING',
        ),
        properties: {'try_count': '8'},
      );
      await client.flush();
      await client.close();

      final e = lastEvent();
      expect(e['eventType'], 'RISK');
      expect(e['eventName'], 'login_failed');
      expect(e['userId'], 1001);
      expect(e['deviceId'], 'device-fixed-0001');
      expect(e['sessionId'], client.session.id);
      expect(e['platform'], 'android');
      expect(e['properties'], {'try_count': '8'});
      expect(e.containsKey('behavior'), isFalse, reason: 'risk 事件不应带 behavior');

      final risk = e['risk'] as Map<String, Object?>;
      expect(risk['riskType'], 'login_anomaly');
      expect(risk['riskLevel'], 'HIGH');
      expect(risk['riskScore'], 88.5);
      expect(risk['ruleId'], 101);
      expect(risk['ruleName'], '1分钟内登录失败超过5次');
      expect(risk['ruleContext'], {'threshold': 5, 'window_sec': 60});
      expect(risk['relatedEventIds'], ['e1', 'e2']);
      expect(risk['description'], '1分钟内登录失败8次');
      expect(risk['evidence'], {'ip': '1.2.3.4'});
      expect(risk['status'], 'PENDING');
      // 服务端补全字段不发送
      expect(risk.containsKey('occurTime'), isFalse);
      expect(risk.containsKey('reportTime'), isFalse);
    });
  });

  group('page_view 契约', () {
    test('页面浏览带停留时长与来源', () async {
      final client = await buildClient();
      client.trackPageView('home');
      // 隔开 2ms：同毫秒内连续切页 dwell=0 会被正确过滤（不发 durationMs）
      await Future<void>.delayed(const Duration(milliseconds: 2));
      client.trackPageView('detail', title: '详情页', referer: 'home');
      await client.flush();
      await client.close();

      final body = transport.requests.last.body;
      final events = body['events'] as List<Object?>;
      final second = ((events[1] as Map<Object?, Object?>).cast<String, Object?>());

      expect(second['eventName'], 'page_view'); // 默认页面事件名
      expect(second['eventCategory'], 'navigation');
      expect(second['properties'], {
        'pageUrl': 'detail',
        'title': '详情页',
        'referer': 'home',
      });
      final behavior = second['behavior'] as Map<String, Object?>;
      expect(behavior['objectType'], 'page');
      expect(behavior['objectId'], 'detail');
      expect(behavior['durationMs'], greaterThan(0));
    });
  });
}
