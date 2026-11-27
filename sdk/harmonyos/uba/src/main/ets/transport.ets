// transport.ets —— @ohos.net.http 传输封装（statusCode 0 = 网络层失败，可重试）。
import http from '@ohos.net.http'

export class HttpResult {
  statusCode: number
  body: string

  constructor(statusCode: number, body: string) {
    this.statusCode = statusCode
    this.body = body
  }

  isOk(): boolean {
    return this.statusCode >= 200 && this.statusCode < 300
  }
}

/** Promise 化 POST JSON。 */
export async function postJson(url: string, body: string, timeoutMs: number): Promise<HttpResult> {
  const client = http.createHttp()
  try {
    const resp = await client.request(url, {
      method: http.RequestMethod.POST,
      header: { 'Content-Type': 'application/json' },
      extraData: body,
      connectTimeout: timeoutMs,
      readTimeout: timeoutMs
    })
    const code = resp.responseCode
    const text = typeof resp.result === 'string' ? resp.result : JSON.stringify(resp.result)
    return new HttpResult(code, text)
  } catch (err) {
    return new HttpResult(0, `network error: ${JSON.stringify(err)}`)
  } finally {
    client.destroy()
  }
}
