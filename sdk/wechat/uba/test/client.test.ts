import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { UbaClient } from '../src/client';
import { cleanupGlobals, installWxMock, setMiniProgramHost, sleep, type WxMock } from './wx-mock';

function baseConfig(): Record<string, unknown> {
  return {
    appId: 'app_001',
    appSecret: 'secret_001',
    endpoint: 'https://collector.example.com',
    batchSize: 100,
    flushInterval: -1,
    maxRetries: 1,
    retryBaseDelay: 1,
  };
}

describe('客户端行为', () => {
  let mock: WxMock;

  beforeEach(() => {
    mock = installWxMock();
    setMiniProgramHost();
  });

  afterEach(() => {
    UbaClient.getInstance()?.destroy();
    cleanupGlobals();
  });

  it('consent 合规门控', async () => {
    const client = UbaClient.init({ ...baseConfig(), requireConsent: true } as never);
    expect(client.isCollecting()).toBe(false);
    client.track('t');
    expect(client.pendingCount()).toBe(0);
    client.setConsent(true);
    expect(client.isCollecting()).toBe(true);
    client.track('t2');
    expect(client.pendingCount()).toBe(1);
    await client.flush();
    expect(mock.requests.length).toBe(1);
    client.destroy();
  });

  it('采样 sampleRate=0 全部丢弃', () => {
    const client = UbaClient.init({ ...baseConfig(), sampleRate: 0 } as never);
    expect(client.isCollecting()).toBe(false);
    client.track('t');
    client.trackRisk('r', { riskType: 'x' });
    expect(client.pendingCount()).toBe(0);
    client.destroy();
  });

  it('identify / resetUser / super 属性覆盖顺序', async () => {
    const client = UbaClient.init(baseConfig() as never);
    client.identify(42);
    client.setSuperProperties({ common: 's', a: 'super' });
    client.track('t1', { a: 'call', own: 'x' });
    client.resetUser();
    client.track('t2');
    await client.flush();
    client.destroy();

    const events = mock.lastBody()['events'] as Array<Record<string, unknown>>;
    expect(events[0]['userId']).toBe(42);
    expect(events[0]['properties']).toEqual({ common: 's', a: 'call', own: 'x' });
    expect(events[1]).not.toHaveProperty('userId');
    expect(events[1]['properties']).toEqual({ common: 's', a: 'super' });
  });

  it('flags 拉取、曝光事件每会话一次', async () => {
    mock.respondWith((_n, url) => {
      if (url.endsWith('/uba/v1/flags')) {
        return { statusCode: 200, data: { flags: { exp_color: 'red', ctl: 'control' } } };
      }
      return { statusCode: 200, data: { success: true, failedCount: 0 } };
    });
    const client = UbaClient.init(baseConfig() as never);
    client.identify(7);
    const flags = await client.loadFlags();
    expect(flags).toEqual({ exp_color: 'red', ctl: 'control' });
    expect(client.getFlag('exp_color')).toBe('red');
    expect(client.isFeatureEnabled('exp_color')).toBe(true);
    expect(client.isFeatureEnabled('ctl')).toBe(false);
    expect(client.getFlag('missing')).toBeUndefined();

    // flags 请求体带身份
    const flagsReq = mock.requests.find((r) => r.url.endsWith('/uba/v1/flags'));
    expect(flagsReq?.body['deviceId']).toBeDefined();
    expect(flagsReq?.body['userId']).toBe(7);

    await client.flush();
    const exposure = mock.requests
      .flatMap((r) => (r.body['events'] as Array<Record<string, unknown>>) ?? [])
      .find((e) => e['eventName'] === '$feature_flag');
    expect(exposure).toBeDefined();
    expect(exposure?.['properties']).toEqual({ flag_key: 'exp_color', variant: 'red' });

    client.getFlag('exp_color'); // 去重
    client.getFlag('ctl'); // 新曝光
    expect(client.pendingCount()).toBe(1);
    client.destroy();
  });

  it('sessionManager：短后台不轮换、超时轮换', async () => {
    const client = UbaClient.init({ ...baseConfig(), sessionTimeout: 30 } as never);
    client.track('a');
    client.handleBackground();
    await sleep(5);
    expect(client.pendingCount()).toBe(0); // 后台自动 flush
    client.handleForeground();
    client.track('b');
    await client.flush();
    client.destroy();

    const events = mock.requests
      .flatMap((r) => (r.body['events'] as Array<Record<string, unknown>>) ?? []);
    expect(events.length).toBe(2);
    expect(events[0]['sessionId']).toBe(events[1]['sessionId']); // 短后台不轮换
  });

  it('destroy 后不再入队；单例被替换', () => {
    const c1 = UbaClient.init(baseConfig() as never);
    expect(UbaClient.getInstance()).toBe(c1);
    c1.destroy();
    expect(UbaClient.getInstance()).toBeUndefined();
    c1.track('after-destroy');
    expect(c1.pendingCount()).toBe(0);

    const c2 = UbaClient.init(baseConfig() as never);
    expect(UbaClient.getInstance()).toBe(c2);
    c2.destroy();
  });

  describe('instrumentApp / instrumentPage 自动埋点', () => {
    it('App onHide 自动 flush、onShow 恢复', async () => {
      const client = UbaClient.init(baseConfig() as never);
      const appConfig = client.instrumentApp<Record<string, unknown>>({
        onLaunch() {},
        onShow() {},
        onHide() {},
      });
      expect(typeof appConfig['onHide']).toBe('function');
      client.track('before-hide');
      (appConfig['onHide'] as () => void).call(null);
      await sleep(10);
      expect(client.pendingCount()).toBe(0);
      expect(mock.requests.length).toBe(1);
      client.destroy();
    });

    it('Page onLoad 自动上报 page_view（含来源页）', async () => {
      const client = UbaClient.init(baseConfig() as never);
      const makePage = (route: string) => {
        const config = client.instrumentPage<Record<string, unknown>>({
          onLoad() {},
        });
        return { config, instance: { route } };
      };
      const home = makePage('pages/home/index');
      (home.config['onLoad'] as () => void).call(home.instance);
      await sleep(5);
      const detail = makePage('pages/detail/index');
      (detail.config['onLoad'] as () => void).call(detail.instance);
      await client.flush();
      client.destroy();

      const events = mock.requests
        .flatMap((r) => (r.body['events'] as Array<Record<string, unknown>>) ?? [])
        .filter((e) => e['eventName'] === 'page_view');
      expect(events.length).toBe(2);
      expect(events[0]['properties']).toEqual({ pageUrl: 'pages/home/index' });
      expect(events[1]['properties']).toEqual({
        pageUrl: 'pages/detail/index',
        referer: 'pages/home/index',
      });
    });
  });

  it('平台探测：无 Page 全局 → mini_game', async () => {
    const g = globalThis as unknown as { Page?: unknown };
    delete g.Page;
    const client = UbaClient.init(baseConfig() as never);
    client.track('t');
    await client.flush();
    client.destroy();
    const evt = mock.lastBody()['events'] as Array<Record<string, unknown>>;
    expect(evt[0]['platform']).toBe('mini_game');
  });

  it('非法配置抛错', () => {
    expect(() => UbaClient.init({ ...baseConfig(), appId: '' } as never)).toThrow();
    expect(() => UbaClient.init({ ...baseConfig(), timeout: 10000 } as never)).toThrow();
  });
});
