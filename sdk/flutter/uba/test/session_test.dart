import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/session.dart';

void main() {
  group('SessionManager 会话轮换', () {
    test('短时间后台不轮换', () {
      var t = DateTime(2026, 3, 27, 12, 0, 0);
      final s = SessionManager(timeoutMs: 30000, now: () => t);
      final id1 = s.id;

      s.onBackground();
      t = t.add(const Duration(seconds: 10));
      s.onForeground();
      expect(s.id, id1, reason: '后台 10s 未超时，会话保持');
    });

    test('超时后台轮换新会话', () {
      var t = DateTime(2026, 3, 27, 12, 0, 0);
      final s = SessionManager(timeoutMs: 30000, now: () => t);
      final id1 = s.id;

      s.onBackground();
      t = t.add(const Duration(seconds: 31));
      s.onForeground();
      expect(s.id, isNot(id1), reason: '后台 31s 超时，应开启新会话');
    });

    test('无后台经历不轮换', () {
      var t = DateTime(2026, 3, 27, 12, 0, 0);
      final s = SessionManager(timeoutMs: 30000, now: () => t);
      final id1 = s.id;
      t = t.add(const Duration(hours: 8));
      s.onForeground(); // 没有 onBackground，前台直接调用是无操作
      expect(s.id, id1);
    });

    test('timeoutMs=0 永不轮换', () {
      var t = DateTime(2026, 3, 27, 12, 0, 0);
      final s = SessionManager(timeoutMs: 0, now: () => t);
      final id1 = s.id;
      s.onBackground();
      t = t.add(const Duration(days: 1));
      s.onForeground();
      expect(s.id, id1);
    });

    test('手动 rotate', () {
      final s = SessionManager();
      final id1 = s.id;
      s.rotate();
      expect(s.id, isNot(id1));
    });

    test('采样判定跟随当前会话', () {
      var t = DateTime(2026, 3, 27, 12, 0, 0);
      final s = SessionManager(timeoutMs: 1000, now: () => t);
      final before = s.inSample(1);
      expect(before, isTrue);
      s.onBackground();
      t = t.add(const Duration(seconds: 5));
      s.onForeground();
      // 轮换后的会话采样率 1 依然全采
      expect(s.inSample(1), isTrue);
    });
  });
}
