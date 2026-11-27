import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/config.dart';
import 'package:uba_flutter/src/flags.dart';
import 'package:uba_flutter/src/transport.dart';

import 'helpers.dart';

void main() {
  late FakeTransport transport;

  FlagsManager buildManager({
    int? Function()? userIdProvider,
    String deviceId = 'dev-fixed',
  }) =>
      FlagsManager(
        config: const UbaConfig(
          appId: 'app_001',
          appSecret: 'secret_001',
          endpoint: 'http://localhost:5700',
        ),
        transport: transport,
        deviceIdProvider: () => deviceId,
        userIdProvider: userIdProvider ?? () => null,
      );

  setUp(() {
    transport = FakeTransport();
    transport.responsesByUrl['http://localhost:5700/uba/v1/flags'] = [
      const HttpResult(200, '{"flags": {"exp_color": "red", "ctl": "control", "blank": ""}}'),
    ];
  });

  test('请求体带 appId/appSecret/deviceId，可选 userId', () async {
    final manager = buildManager(userIdProvider: () => 7);
    await manager.loadFlags();

    final body = transport.requests.single.body;
    expect(body['appId'], 'app_001');
    expect(body['appSecret'], 'secret_001');
    expect(body['deviceId'], 'dev-fixed');
    expect(body['userId'], 7);
  });

  test('userId 为 null 时不上报该字段', () async {
    final manager = buildManager();
    await manager.loadFlags();
    expect(transport.requests.single.body.containsKey('userId'), isFalse);
  });

  test('flags 缓存读取与命中判定', () async {
    final manager = buildManager();
    expect(manager.flags, isEmpty);

    await manager.loadFlags();
    expect(manager.flags, {'exp_color': 'red', 'ctl': 'control', 'blank': ''});
    expect(manager['exp_color'], 'red');
    expect(manager.isEnabled('exp_color'), isTrue);
    expect(manager.isEnabled('ctl'), isFalse, reason: 'control 不命中');
    expect(manager.isEnabled('blank'), isFalse, reason: '空变体不命中');
    expect(manager.isEnabled('missing'), isFalse);
    expect(manager.flags, isNot(same(manager.flags)), reason: '返回不可变视图');
  });

  test('拉取失败保留上一次缓存', () async {
    final manager = buildManager();
    await manager.loadFlags();
    expect(manager['exp_color'], 'red');

    transport.responsesByUrl.clear();
    transport.defaultResult = const HttpResult(500, 'down');
    final again = await manager.loadFlags();

    expect(again, {'exp_color': 'red', 'ctl': 'control', 'blank': ''},
        reason: '失败时应保留旧缓存');
  });

  test('响应缺 flags 字段时清空缓存并容错', () async {
    final manager = buildManager();
    await manager.loadFlags();

    transport.responsesByUrl.clear();
    transport.defaultResult = const HttpResult(200, '{"success": true}');
    final again = await manager.loadFlags();
    expect(again, isEmpty);
  });

  test('响应解析失败保留旧缓存不崩溃', () async {
    final manager = buildManager();
    await manager.loadFlags();

    transport.responsesByUrl.clear();
    transport.defaultResult = const HttpResult(200, '{{{not json');
    final again = await manager.loadFlags();
    expect(again['exp_color'], 'red');
  });
}
