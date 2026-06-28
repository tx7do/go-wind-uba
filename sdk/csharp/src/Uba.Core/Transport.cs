using System;
using System.Net.Http;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace Uba
{
    /// <summary>单次 HTTP 发送的结果</summary>
    public class FetchResult
    {
        public bool Ok { get; set; }
        public int Status { get; set; }
        /// <summary>成功时的响应体；失败时为 null</summary>
        public PostReportResponse? Response { get; set; }
        /// <summary>失败时的 Kratos 错误；非标准错误或成功时为 null</summary>
        public KratosError? Error { get; set; }
        /// <summary>异常/超时信息</summary>
        public string? Exception { get; set; }
    }

    /// <summary>
    /// HTTP 传输抽象。核心库提供 HttpClientTransport；Unity 侧可用 UnityWebRequestTransport 覆盖。
    /// 抽象出此接口是为了让 Unity WebGL（HttpClient 不可用）能替换实现。
    /// </summary>
    public interface IHttpTransport
    {
        /// <summary>发送 POST 请求（带超时，毫秒）</summary>
        Task<FetchResult> SendAsync(string url, string body, int timeoutMs, CancellationToken ct = default);
    }

    /// <summary>
    /// 基于 HttpClient 的默认传输实现。
    /// 适用：.NET Core / .NET 5+ / Mono / Unity 原生平台（IL2CPP/Mono）。
    /// 不适用：Unity WebGL（HttpClient 抛 PlatformNotSupportedException）—— WebGL 需用 UnityWebRequestTransport。
    /// </summary>
    public class HttpClientTransport : IHttpTransport
    {
        private static readonly HttpClient _shared = new HttpClient();

        public async Task<FetchResult> SendAsync(string url, string body, int timeoutMs, CancellationToken ct = default)
        {
            using (var cts = CancellationTokenSource.CreateLinkedTokenSource(ct))
            {
                cts.CancelAfter(timeoutMs);
                try
                {
                    using (var content = new StringContent(body, Encoding.UTF8, "application/json"))
                    using (var req = new HttpRequestMessage(HttpMethod.Post, url) { Content = content })
                    using (var resp = await _shared.SendAsync(req, cts.Token).ConfigureAwait(false))
                    {
                        var text = await resp.Content.ReadAsStringAsync().ConfigureAwait(false);
                        return ParseResult((int)resp.StatusCode, text);
                    }
                }
                catch (OperationCanceledException) when (!ct.IsCancellationRequested)
                {
                    return new FetchResult { Ok = false, Status = 0, Exception = $"request timeout ({timeoutMs}ms)" };
                }
                catch (Exception e)
                {
                    return new FetchResult { Ok = false, Status = 0, Exception = e.Message };
                }
            }
        }

        /// <summary>解析 HTTP 响应（公开以便 Unity/Godot 适配层复用）</summary>
        public static FetchResult ParseResult(int status, string text)
        {
            if (string.IsNullOrEmpty(text))
            {
                return new FetchResult { Ok = status >= 200 && status < 300, Status = status, Exception = status >= 300 ? "empty response" : null };
            }
            try
            {
                if (status >= 200 && status < 300)
                {
                    var resp = JsonSerializer.DeserializeResponse(text);
                    return new FetchResult { Ok = true, Status = status, Response = resp };
                }
                var err = JsonSerializer.DeserializeError(text);
                return new FetchResult { Ok = false, Status = status, Error = err };
            }
            catch (Exception e)
            {
                return new FetchResult { Ok = false, Status = status, Exception = "invalid JSON: " + e.Message };
            }
        }
    }
}
