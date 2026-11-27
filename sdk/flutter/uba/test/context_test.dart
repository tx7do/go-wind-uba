import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/context.dart';
import 'package:uba_flutter/src/store.dart';

void main() {
  group('DefaultContextProvider', () {
    test('新设备：生成 uuid 并持久化，deviceIdIsNew=true', () async {
      final store = MemoryStore();
      final snapshot = await DefaultContextProvider(store).load();

      expect(snapshot.deviceIdIsNew, isTrue);
      expect(
        snapshot.deviceId,
        matches(RegExp(r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$')),
      );
      expect(await store.getString(kDeviceIdKey), snapshot.deviceId);
    });

    test('已有设备：复用持久化值，deviceIdIsNew=false', () async {
      final store = MemoryStore();
      await store.setString(kDeviceIdKey, 'existing-device-id');

      final snapshot = await DefaultContextProvider(store).load();

      expect(snapshot.deviceId, 'existing-device-id');
      expect(snapshot.deviceIdIsNew, isFalse);
    });

    test('平台与 UA 来自宿主环境', () async {
      final snapshot =
          await DefaultContextProvider(MemoryStore()).load();

      expect(snapshot.platform, Platform.operatingSystem);
      expect(snapshot.os, isNotEmpty);
      expect(snapshot.userAgent, startsWith('uba_flutter/'));
      expect(snapshot.userAgent, contains('(${snapshot.platform})'));
    });
  });
}
