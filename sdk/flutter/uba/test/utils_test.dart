import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/utils.dart';

void main() {
  group('uuidV4', () {
    final pattern = RegExp(
      r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
    );

    test('格式为 uuid v4（版本位与变体位正确）', () {
      for (var i = 0; i < 100; i++) {
        expect(uuidV4(), matches(pattern));
      }
    });

    test('唯一性', () {
      final ids = <String>{};
      for (var i = 0; i < 1000; i++) {
        ids.add(uuidV4());
      }
      expect(ids.length, 1000);
    });
  });

  group('toRfc3339Ms', () {
    test('UTC 毫秒精度', () {
      final dt = DateTime.utc(2026, 3, 27, 12, 0, 0, 123);
      expect(toRfc3339Ms(dt), '2026-03-27T12:00:00.123Z');
    });

    test('时间前零补齐', () {
      final dt = DateTime.utc(2026, 1, 2, 3, 4, 5, 6);
      expect(toRfc3339Ms(dt), '2026-01-02T03:04:05.006Z');
    });

    test('本地时间自动转 UTC', () {
      final local = DateTime(2026, 3, 27, 12, 0, 0, 123);
      expect(toRfc3339Ms(local), toRfc3339Ms(local.toUtc()));
    });
  });

  group('mergeProperties', () {
    test('base 在前 extra 覆盖', () {
      expect(
        mergeProperties({'a': '1', 'b': '1'}, {'b': '2', 'c': '3'}),
        {'a': '1', 'b': '2', 'c': '3'},
      );
    });

    test('任一侧为 null 返回另一侧副本', () {
      final base = {'a': '1'};
      final merged = mergeProperties(base, null);
      expect(merged, {'a': '1'});
      merged['a'] = 'mutated';
      expect(base['a'], '1'); // 不污染原 map
    });
  });

  group('trimAndLimit', () {
    test('去空白、空串转 null', () {
      expect(trimAndLimit('  hello  '), 'hello');
      expect(trimAndLimit('   '), isNull);
      expect(trimAndLimit(null), isNull);
    });

    test('按 Unicode 码点截断（emoji 不截半）', () {
      final v = '🎉🎉🎉'; // 3 个码点
      expect(trimAndLimit(v, 2), '🎉🎉');
    });

    test('超限截断到 limit 个码点', () {
      expect(trimAndLimit('abcdef', 3), 'abc');
    });
  });

  group('sampling', () {
    test('fnv1a32 确定性', () {
      expect(fnv1a32('session-a'), fnv1a32('session-a'));
      expect(fnv1a32('session-a'), isNot(fnv1a32('session-b')));
    });

    test('采样率边界', () {
      expect(sessionIdInSample('anything', 0), isFalse);
      expect(sessionIdInSample('anything', 1), isTrue);
    });

    test('同会话结果恒定（0.5 采样）', () {
      final r = sessionIdInSample('fixed-session', 0.5);
      for (var i = 0; i < 10; i++) {
        expect(sessionIdInSample('fixed-session', 0.5), r);
      }
    });
  });

  group('osDisplayName', () {
    test('平台名映射', () {
      expect(osDisplayName('android'), 'Android');
      expect(osDisplayName('ios'), 'iOS');
      expect(osDisplayName('macos'), 'macOS');
      expect(osDisplayName('windows'), 'Windows');
      expect(osDisplayName('linux'), 'Linux');
    });
  });
}
