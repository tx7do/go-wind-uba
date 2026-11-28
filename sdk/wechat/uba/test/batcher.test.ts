import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { UbaClient } from '../src/client';
import { OFFLINE_QUEUE_KEY } from '../src/storage';
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

describe('批量与重试策略（对齐 C#/Flutter SDK）', () => {
  let mock: WxMock;

  beforeEach(() => {
    mock = installWxMock();
    setMiniProgramHost();
  });

  afterEach(() => {
    UbaClient.getInstance()?.destroy();
    cleanupGlobals();
  });

  it('达到 batchSize 自动 flush', async () => {
    const client = UbaClient.init({ ...baseConfig(), batchSize: 2 } as never);
    client.track('a');
    client.track('b');
    await sleep(20);
    expect(mock.requests.length).toBe(1);
    expect(mock.eventCount()).toBe(2);
    expect(client.pendingCount()).toBe(0);
    client.destroy();
  });

  it('定时 flush（flushInterval > 0）', async () => {
    const client = UbaClient.init({ ...baseConfig(), flushInterval: 40 } as never);
    client.track('timer-flush');
    await sleep(200);
    expect(mock.requests.length).toBe(1);
    client.destroy();
  });

  it('401 不重试不落盘', async () => {
    mock.respondWith(() => ({ statusCode: 401 }));
    const client = UbaClient.init(baseConfig() as never);
    client.track('a');
    client.track('b');
    const r = await client.flush();
    expect(r.success).toBe(false);
    expect(r.retryable).toBe(false);
    expect(r.dropped).toBe(2);
    expect(mock.requests.length).toBe(1);
    expect(mock.storage.has(OFFLINE_QUEUE_KEY)).toBe(false);
    client.destroy();
  });

  it('5xx 重试直至成功', async () => {
    const statuses = [500, 503];
    mock.respondWith((n) => ({ statusCode: statuses[n - 1] ?? 200 }));
    const client = UbaClient.init({ ...baseConfig(), maxRetries: 2 } as never);
    client.track('a');
    const r = await client.flush();
    expect(r.success).toBe(true);
    expect(mock.requests.length).toBe(3);
    client.destroy();
  });

  it('429 可重试；重试耗尽落盘补发', async () => {
    let failing = true;
    mock.respondWith((n) => (n <= 2 && failing ? { statusCode: 429 } : { statusCode: 200 }));
    const client = UbaClient.init(baseConfig() as never);
    client.track('offline-1');
    client.track('offline-2');
    const r = await client.flush();
    expect(r.success).toBe(false);
    expect(r.retryable).toBe(true);
    expect(mock.storage.has(OFFLINE_QUEUE_KEY)).toBe(true);

    // 新实例恢复离线队列补发成功
    UbaClient.getInstance()?.destroy();
    failing = false;
    const client2 = UbaClient.init(baseConfig() as never);
    expect(client2.pendingCount()).toBe(2);
    const r2 = await client2.flush();
    expect(r2.success).toBe(true);
    expect(mock.storage.has(OFFLINE_QUEUE_KEY)).toBe(false);
    client2.destroy();
  });

  it('persistQueue=false 失败直接丢', async () => {
    mock.respondWith(() => ({ statusCode: 500 }));
    const client = UbaClient.init({ ...baseConfig(), persistQueue: false } as never);
    client.track('a');
    await client.flush();
    expect(mock.storage.has(OFFLINE_QUEUE_KEY)).toBe(false);
    client.destroy();
  });

  it('损坏的离线队列被忽略并清除', async () => {
    mock.storage.set(OFFLINE_QUEUE_KEY, '{{{not json');
    const client = UbaClient.init(baseConfig() as never);
    expect(client.pendingCount()).toBe(0);
    expect(mock.storage.has(OFFLINE_QUEUE_KEY)).toBe(false);
    client.destroy();
  });

  it('HTTP 200 部分失败按丢弃计', async () => {
    mock.respondWith(() => ({
      statusCode: 200,
      data: { success: true, totalCount: 3, successCount: 1, failedCount: 2 },
    }));
    const client = UbaClient.init(baseConfig() as never);
    client.track('a');
    client.track('b');
    client.track('c');
    const r = await client.flush();
    expect(r.success).toBe(true);
    expect(r.sent).toBe(1);
    expect(r.dropped).toBe(2);
    expect(client.pendingCount()).toBe(0);
    client.destroy();
  });

  it('网络失败（fail 回调）视为可重试', async () => {
    let failing = true;
    mock.respondWith((n) => (n === 1 && failing ? { fail: true } : { statusCode: 200 }));
    const client = UbaClient.init(baseConfig() as never);
    client.track('a');
    const r = await client.flush();
    expect(r.success).toBe(true);
    expect(mock.requests.length).toBe(2);
    client.destroy();
  });

  it('字节上限拆分多批', async () => {
    const client = UbaClient.init({ ...baseConfig(), maxBatchBytes: 400 } as never);
    for (let i = 0; i < 4; i++) {
      client.track('evt', { payload: 'a'.repeat(150) });
    }
    const r = await client.flush();
    expect(r.sent).toBe(4);
    expect(mock.requests.length).toBeGreaterThanOrEqual(2);
    for (const req of mock.requests) {
      expect((req.body['events'] as unknown[]).length).toBeLessThan(4);
    }
    client.destroy();
  });
});
