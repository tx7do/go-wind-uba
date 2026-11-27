import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:uba_flutter/src/store.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('PrefsStore（shared_preferences 真插件通道，mock 值）', () {
    setUp(() {
      SharedPreferences.setMockInitialValues(<String, Object>{});
    });

    test('set/get/remove 基本读写', () async {
      final store = await PrefsStore.create();

      expect(await store.getString('k'), isNull);
      await store.setString('k', 'v1');
      expect(await store.getString('k'), 'v1');
      await store.setString('k', 'v2');
      expect(await store.getString('k'), 'v2', reason: '覆盖写');
      await store.remove('k');
      expect(await store.getString('k'), isNull);
    });

    test('契约键名的常量值沿用 Web/Unity 约定', () {
      expect(kDeviceIdKey, '__uba_device_id__');
      expect(kOfflineQueueKey, '__uba_offline_queue__');
    });
  });

  group('MemoryStore（测试替身自身的行为）', () {
    test('与 PrefsStore 相同语义', () async {
      final store = MemoryStore();
      expect(await store.getString('a'), isNull);
      await store.setString('a', '1');
      expect(await store.getString('a'), '1');
      await store.remove('a');
      expect(await store.getString('a'), isNull);
    });
  });
}
