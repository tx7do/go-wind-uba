import { getWx } from './wx-env';

export interface HttpResult {
  statusCode: number;
  body: unknown;
}

/** wx.request 的 Promise 化封装；statusCode 0 表示网络层失败（可重试）。 */
export function requestJson(url: string, body: string, timeoutMs: number): Promise<HttpResult> {
  return new Promise((resolve) => {
    getWx().request({
      url,
      method: 'POST',
      data: body,
      timeout: timeoutMs,
      header: { 'content-type': 'application/json' },
      success: (res) => resolve({ statusCode: res.statusCode, body: res.data }),
      fail: () => resolve({ statusCode: 0, body: undefined }),
    });
  });
}
