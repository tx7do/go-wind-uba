import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { UbaClient } from '../src/client';
import { DEVICE_ID_KEY, OFFLINE_QUEUE_KEY } from '../src/storage';
import { cleanupGlobals, installWxMock, setMiniProgramHost, type WxMock } from './wx-mock';

/** 契约金样：上报 body 的字段名/位置/类型必须与后端 proto 契约完全一致。 */
describe('上报契约金样', () => {
  let mock: WxMock;

  beforeEach(() => {
    mock = installWxMock();
    setMiniProgramHost();
  });

  afterEach(() => {
    UbaClient.getInstance()?.destroy();
    cleanupGlobals();
  });

  function initClient(overrides: Record<string, unknown> = {}): UbaClient {
    return UbaClient.init({
      appId: 'app_001',
      appSecret: 'secret_001',
      endpoint: 'https://collector.example.com',
      appVersion: '1.2.3',
      channel: 'official',
      batchSize: 100,
      flushInterval: -1,
      maxRetries: 1,
      retryBaseDelay: 1,
      ...overrides,
    });
  }

  function lastEvent(): Record<string, unknown> {
    expect(mock.requests.length).toBeGreaterThan(0);
    const body = mock.lastBody();
    expect(body['appId']).toBe('app_001');
    expect(body['appSecret']).toBe('secret_001');
    const events = body['events'] as Array<Record<string, unknown>>;
    return events[events.length - 1];
  }

  it('全维度行为事件：顶层 + behavior oneof 镜像，数值标量只在 oneof 内', async () => {
    const client = initClient();
    client.identify(1001);
    client.track('level_pass', { scene: 'game', diff: 'hard' }, {
      eventCategory: 'game',
      eventAction: 'pass',
      objectType: 'level',
      objectId: 'level_42',
      objectName: '第42关',
      durationMs: 3500,
      amount: '0.99',
      quantity: 2,
      score: 88,
      metrics: { speed: 1.5, combo: 3 },
      network: 'WiFi',
      opResult: 'success',
      serverId: 's1',
      level: 42,
      traceId: 'trace_abc',
    });
    await client.flush();
    client.destroy();

    const e = lastEvent();
    expect(e['eventType']).toBe('BEHAVIOR');
    expect(String(e['eventId'])).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    expect(e['eventName']).toBe('level_pass');
    expect(String(e['eventTime'])).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/);
    expect(e['userId']).toBe(1001);
    expect(e['deviceId']).toBe(mock.storage.get(DEVICE_ID_KEY));
    expect(e['platform']).toBe('mini_program');
    expect(e['properties']).toEqual({ scene: 'game', diff: 'hard' });
    expect(e['traceId']).toBe('trace_abc');
    expect(e['eventCategory']).toBe('game');

    // 顶层字符串镜像
    expect(e['eventAction']).toBe('pass');
    expect(e['objectType']).toBe('level');
    expect(e['objectId']).toBe('level_42');
    expect(e['objectName']).toBe('第42关');
    expect(e['amount']).toBe('0.99');
    expect(e['opResult']).toBe('success');

    // 服务端专属与数值标量绝不出现在顶层
    for (const k of ['tenantId', 'serverTime', 'ip', 'durationMs', 'quantity', 'score', 'sessionSeq', 'level']) {
      expect(e).not.toHaveProperty(k);
    }

    const behavior = e['behavior'] as Record<string, unknown>;
    expect(behavior['durationMs']).toBe(3500);
    expect(behavior['quantity']).toBe(2);
    expect(behavior['score']).toBe(88);
    expect(behavior['level']).toBe(42);
    expect(behavior['serverId']).toBe('s1');
    expect(behavior['os']).toBe('iOS');
    expect(behavior['appVersion']).toBe('1.2.3');
    expect(behavior['channel']).toBe('official');
    expect(behavior['network']).toBe('WiFi');
    expect(behavior['metrics']).toEqual({ speed: 1.5, combo: 3 });

    // 顶层键集合精确
    expect(Object.keys(e).sort()).toEqual(
      [
        'amount', 'behavior', 'deviceId', 'eventAction', 'eventCategory', 'eventId',
        'eventName', 'eventTime', 'eventType', 'metrics', 'objectId', 'objectName',
        'objectType', 'opResult', 'platform', 'properties', 'sessionId', 'traceId',
        'userId',
      ].sort(),
    );

    // envelope 的 clientInfo.userAgent
    const clientInfo = mock.lastBody()['clientInfo'] as Record<string, unknown>;
    expect(String(clientInfo['userAgent'])).toContain('uba-wechat/1.0.0');
    expect(String(clientInfo['userAgent'])).toContain('iPhone 15 Pro');
  });

  it('最小事件只带事件名；未 identify 不带 userId', async () => {
    const client = initClient();
    client.track('button_click');
    await client.flush();
    client.destroy();

    const e = lastEvent();
    expect(e['eventType']).toBe('BEHAVIOR');
    expect(e['eventName']).toBe('button_click');
    expect(e).not.toHaveProperty('userId');
    expect(e).not.toHaveProperty('properties');
    expect((e['behavior'] as Record<string, unknown>)['os']).toBe('iOS');
  });

  it('风险事件：risk oneof 逐字段，无 behavior', async () => {
    const client = initClient();
    client.identify(1001);
    client.trackRisk(
      'login_failed',
      {
        riskType: 'login_anomaly',
        riskLevel: 'HIGH',
        riskScore: 88.5,
        ruleId: 101,
        ruleName: '1分钟内登录失败超过5次',
        ruleContext: { threshold: 5, window_sec: 60 },
        relatedEventIds: ['e1', 'e2'],
        description: '失败8次',
        evidence: { ip: '1.2.3.4' },
        status: 'PENDING',
      },
      { try_count: '8' },
    );
    await client.flush();
    client.destroy();

    const e = lastEvent();
    expect(e['eventType']).toBe('RISK');
    expect(e['userId']).toBe(1001);
    expect(e['properties']).toEqual({ try_count: '8' });
    expect(e).not.toHaveProperty('behavior');

    const risk = e['risk'] as Record<string, unknown>;
    expect(risk['riskType']).toBe('login_anomaly');
    expect(risk['riskLevel']).toBe('HIGH');
    expect(risk['riskScore']).toBe(88.5);
    expect(risk['ruleId']).toBe(101);
    expect(risk['ruleContext']).toEqual({ threshold: 5, window_sec: 60 });
    expect(risk['relatedEventIds']).toEqual(['e1', 'e2']);
    expect(risk['status']).toBe('PENDING');
    expect(risk).not.toHaveProperty('occurTime');
    expect(risk).not.toHaveProperty('reportTime');
  });

  it('页面浏览：来源页与停留时长', async () => {
    const client = initClient();
    client.trackPageView('pages/home');
    await sleepCompat(5);
    client.trackPageView('pages/detail', '详情页', 'pages/home');
    await client.flush();
    client.destroy();

    const events = mock.lastBody()['events'] as Array<Record<string, unknown>>;
    const second = events[events.length - 1];
    expect(second['eventName']).toBe('page_view');
    expect(second['eventCategory']).toBe('navigation');
    expect(second['properties']).toEqual({ pageUrl: 'pages/detail', title: '详情页', referer: 'pages/home' });
    const behavior = second['behavior'] as Record<string, unknown>;
    expect(behavior['objectId']).toBe('pages/detail');
    expect(behavior['durationMs']).toBeGreaterThan(0);
  });

  it('首装 app_launch：deviceId 新建时触发一次', async () => {
    expect(mock.storage.get(DEVICE_ID_KEY)).toBeUndefined();
    const client = initClient({ autoTrackLaunch: true });
    expect(client.pendingCount()).toBe(1);
    await client.flush();
    client.destroy();

    const e = lastEvent();
    expect(e['eventName']).toBe('app_launch');
    expect(e['eventCategory']).toBe('app');
    expect(e['properties']).toEqual({ first_launch: 'true' });
    // deviceId 已持久化
    expect(mock.storage.get(DEVICE_ID_KEY)).toBeDefined();
  });

  it('老设备不触发 app_launch；离线队列键持久化', async () => {
    mock.storage.set(DEVICE_ID_KEY, 'existing-device');
    const client = initClient({ autoTrackLaunch: true });
    expect(client.pendingCount()).toBe(0);
    await client.flush();
    client.destroy();
    expect(mock.storage.has(OFFLINE_QUEUE_KEY)).toBe(false);
  });
});

/** 兼容真时钟的短暂等待（page dwell > 0 用）。 */
function sleepCompat(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
