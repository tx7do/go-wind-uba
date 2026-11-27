import 'package:flutter_test/flutter_test.dart';
import 'package:uba_flutter/src/types.dart';

void main() {
  group('ReportEvent 序列化 round-trip（离线队列恢复路径）', () {
    test('行为事件：toJson → fromJson → toJson 幂等', () {
      final event = const ReportEvent(
        eventType: UbaEventType.behavior,
        eventId: 'evt-001',
        eventName: 'level_pass',
        eventTime: '2026-03-27T12:00:00.123Z',
        userId: 1001,
        deviceId: 'dev-001',
        sessionId: 'sess-001',
        platform: 'android',
        eventCategory: 'game',
        properties: {'scene': 'game'},
        metrics: {'combo': 3.0},
        traceId: 'trace-001',
        eventAction: 'pass',
        objectType: 'level',
        objectId: 'level_42',
        objectName: '第42关',
        amount: '0.99',
        opResult: 'success',
        errorCode: 'E01',
        behavior: BehaviorEvent(
          eventAction: 'pass',
          objectType: 'level',
          objectId: 'level_42',
          objectName: '第42关',
          sessionSeq: 7,
          os: 'Android',
          appVersion: '1.2.3',
          channel: 'official',
          network: 'WiFi',
          durationMs: 3500,
          amount: '0.99',
          quantity: 2,
          score: 88,
          metrics: {'combo': 3.0},
          properties: {'scene': 'game'},
          opResult: 'success',
          errorCode: 'E01',
          serverId: 's1',
          level: 42,
        ),
      );

      final restored = ReportEvent.fromJson(
        (event.toJson() as Map<Object?, Object?>).cast<String, Object?>(),
      );
      expect(restored.toJson(), event.toJson());
    });

    test('风险事件：round-trip 保留结构化 ruleContext', () {
      final event = const ReportEvent(
        eventType: UbaEventType.risk,
        eventId: 'risk-001',
        eventName: 'login_failed',
        eventTime: '2026-03-27T12:00:00.123Z',
        deviceId: 'dev-001',
        risk: RiskEvent(
          riskType: 'login_anomaly',
          riskLevel: 'HIGH',
          riskScore: 88.5,
          ruleId: 101,
          ruleName: '1分钟内登录失败超过5次',
          ruleContext: {'threshold': 5, 'window_sec': 60},
          relatedEventIds: ['e1', 'e2'],
          description: '失败8次',
          evidence: {'ip': '1.2.3.4'},
          status: 'PENDING',
        ),
      );

      final restored = ReportEvent.fromJson(
        (event.toJson() as Map<Object?, Object?>).cast<String, Object?>(),
      );
      expect(restored.toJson(), event.toJson());
    });

    test('未知 eventType 回退 behavior（防脏数据崩溃）', () {
      final restored = ReportEvent.fromJson({
        'eventType': 'FUTURE_TYPE',
        'eventId': 'e1',
        'eventName': 'n',
        'eventTime': '2026-03-27T12:00:00.123Z',
      });
      expect(restored.eventType, UbaEventType.behavior);
    });
  });

  group('putIfPresent', () {
    test('null 与空集合跳过，非空写入', () {
      final m = <String, Object?>{};
      putIfPresent(m, 'a', null);
      putIfPresent(m, 'b', <String, String>{});
      putIfPresent(m, 'c', <String>[]);
      putIfPresent(m, 'd', 'x');
      putIfPresent(m, 'e', 0); // 0 是有效值，要写入
      expect(m.keys.toSet(), {'d', 'e'});
    });
  });

  group('PostReportResponse 解析', () {
    test('部分失败计数字段', () {
      final r = PostReportResponse.fromJson({
        'success': true,
        'totalCount': 3,
        'successCount': 1,
        'failedCount': 2,
        'requestId': 'req-1',
        'message': 'partial',
      });
      expect(r.success, isTrue);
      expect(r.totalCount, 3);
      expect(r.successCount, 1);
      expect(r.failedCount, 2);
      expect(r.requestId, 'req-1');
    });

    test('缺省字段容错', () {
      final r = PostReportResponse.fromJson({'success': true});
      expect(r.failedCount, 0);
      expect(r.requestId, isNull);
    });
  });
}
