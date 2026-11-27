import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/uba_flutter.dart';

import 'helpers.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late FakeTransport transport;
  late MemoryStore store;

  setUp(() {
    transport = FakeTransport();
    store = MemoryStore();
  });

  Future<UbaClient> createClient({
    bool autoTrackLaunch = false,
    bool deviceIdIsNew = false,
  }) =>
      UbaClient.create(
        UbaConfig(
          appId: 'app_001',
          appSecret: 'secret_001',
          endpoint: 'http://localhost:5700',
          batchSize: 100,
          flushIntervalMs: 0, // widget 测试里不允许悬空定时器
          autoTrackLaunch: autoTrackLaunch,
        ),
        transport: transport,
        store: store,
        contextProvider: FakeContextProvider(deviceIdIsNew: deviceIdIsNew),
      );

  group('UbaLifecycleObserver（直调生命周期回调）', () {
    test('paused/hidden/detached 触发 flush，resumed 不触发', () async {
      final client = await createClient();
      final observer = UbaLifecycleObserver(client)
        ..register()
        ..unregister(); // register/unregister 幂等且可逆（不依赖真实 WidgetsBinding）

      client.track('e1');
      expect(client.pendingCount, 1);

      observer.didChangeAppLifecycleState(AppLifecycleState.paused);
      await Future<void>.delayed(const Duration(milliseconds: 10));
      expect(client.pendingCount, 0, reason: 'paused 应立即 flush');
      expect(transport.requests.length, 1);

      observer.didChangeAppLifecycleState(AppLifecycleState.resumed);
      await Future<void>.delayed(const Duration(milliseconds: 10));
      expect(transport.requests.length, 1, reason: 'resumed 不产生上报');

      // hidden / detached 同样按后台处理
      client.track('e2');
      observer.didChangeAppLifecycleState(AppLifecycleState.hidden);
      await Future<void>.delayed(const Duration(milliseconds: 10));
      expect(transport.requests.length, 2);
      await client.close();
    });

    test('inactive 是瞬时打断，不触发 flush', () async {
      final client = await createClient();
      final observer = UbaLifecycleObserver(client);

      client.track('e1');
      observer.didChangeAppLifecycleState(AppLifecycleState.inactive);
      await Future<void>.delayed(const Duration(milliseconds: 10));
      expect(client.pendingCount, 1, reason: 'inactive 不 flush');
      expect(transport.requests, isEmpty);
      await client.close();
    });
  });

  group('UbaScreenObserver（widget 级路由埋点）', () {
    testWidgets('push/pop 上报 page_view，含停留时长与来源页', (tester) async {
      final client = await createClient();

      await tester.pumpWidget(MaterialApp(
        initialRoute: '/home',
        navigatorObservers: [UbaScreenObserver(client)],
        routes: {
          '/home': (_) => const Scaffold(body: Text('home')),
          '/detail': (_) => const Scaffold(body: Text('detail')),
        },
      ));
      await tester.pump(const Duration(milliseconds: 5)); // home 页停留

      final navigator = tester.state<NavigatorState>(find.byType(Navigator));
      navigator.pushNamed('/detail');
      await tester.pumpAndSettle();
      await tester.pump(const Duration(milliseconds: 5)); // detail 页停留

      navigator.pop();
      await tester.pumpAndSettle();

      await client.flush();
      await client.close();

      final pageViews = transport.requests
          .where((r) => r.url.endsWith('/uba/v1/report'))
          .expand((r) => r.body['events'] as List<Object?>)
          .cast<Map<Object?, Object?>>()
          .where((e) => e['eventName'] == 'page_view')
          .toList();

      expect(pageViews, hasLength(3), reason: 'home 进入 / detail push / pop 回 home');
      expect(pageViews[0]['properties'], {'pageUrl': '/home'});

      final detail = pageViews[1];
      expect(detail['properties'],
          {'pageUrl': '/detail', 'referer': '/home'});
      expect((detail['behavior'] as Map)['objectId'], '/detail');
      expect((detail['behavior'] as Map)['durationMs'], greaterThan(0),
          reason: 'detail 事件携带上一页（home）的停留时长');
      expect((pageViews[0]['behavior'] as Map)['durationMs'], isNull,
          reason: '首屏无上一页，不带 durationMs');
      expect((pageViews[2]['properties']),
          {'pageUrl': '/home', 'referer': '/detail'},
          reason: 'pop 回 home，来源页为 detail');
    });
  });

  group('app_launch 与 lifecycle 组合', () {
    testWidgets('首装：进入首页即已带 app_launch 事件', (tester) async {
      final client =
          await createClient(autoTrackLaunch: true, deviceIdIsNew: true);

      await tester.pumpWidget(MaterialApp(
        initialRoute: '/home',
        navigatorObservers: [UbaScreenObserver(client)],
        routes: {'/home': (_) => const Scaffold()},
      ));
      await tester.pumpAndSettle();

      await client.flush();
      await client.close();

      final names = transport.requests
          .where((r) => r.url.endsWith('/uba/v1/report'))
          .expand((r) => r.body['events'] as List<Object?>)
          .cast<Map<Object?, Object?>>()
          .map((e) => e['eventName'])
          .toList();
      expect(names, ['app_launch', 'page_view']);
    });
  });
}
