using System;
using System.Threading;
using System.Threading.Tasks;
using UnityEngine;
using UnityEngine.Networking;

namespace Uba.Unity
{
    /// <summary>
    /// 基于 UnityWebRequest 的 HTTP 传输实现。
    ///
    /// 必须用于以下场景：
    /// - Unity WebGL（HttpClient 在 WebGL 下抛 PlatformNotSupportedException）
    /// - 需要与 Unity 主线程/生命周期集成的项目
    ///
    /// 注意：UnityWebRequest 的网络请求需在主线程发起；本实现通过 TaskCompletionSource
    /// 将协程结果转为可 await 的 Task，调用方仍可在主线程使用 await。
    /// </summary>
    public class UnityWebRequestTransport : IHttpTransport
    {
        private readonly MonoBehaviour _host;

        /// <param name="host">用于启动协程的 MonoBehaviour（通常传入场景中的某个持久 GameObject）</param>
        public UnityWebRequestTransport(MonoBehaviour host)
        {
            _host = host ? host : throw new ArgumentNullException(nameof(host));
        }

        public Task<FetchResult> SendAsync(string url, string body, int timeoutMs, CancellationToken ct = default)
        {
            var tcs = new TaskCompletionSource<FetchResult>(TaskCreationOptions.RunContinuationsAsynchronously);

            _host.StartCoroutine(SendCoroutine(url, body, timeoutMs, tcs, ct));

            return tcs.Task;
        }

        private System.Collections.IEnumerator SendCoroutine(
            string url, string body, int timeoutMs,
            TaskCompletionSource<FetchResult> tcs, CancellationToken ct)
        {
            using (var req = new UnityWebRequest(url, "POST"))
            {
                byte[] bodyRaw = System.Text.Encoding.UTF8.GetBytes(body);
                req.uploadHandler = new UploadHandlerRaw(bodyRaw);
                req.downloadHandler = new DownloadHandlerBuffer();
                req.SetRequestHeader("Content-Type", "application/json");
                req.SetRequestHeader("Accept", "application/json");
                req.timeout = Math.Max(1, timeoutMs / 1000);

                var op = req.SendWebRequest();

                // 等待完成或取消
                float deadline = Time.realtimeSinceStartup + timeoutMs / 1000f;
                while (!op.isDone)
                {
                    if (ct.IsCancellationRequested || Time.realtimeSinceStartup > deadline)
                    {
                        req.Abort();
                        tcs.TrySetResult(new FetchResult { Ok = false, Status = 0, Exception = "request timeout or cancelled" });
                        yield break;
                    }
                    yield return null;
                }

                FetchResult result;
                int status = (int)req.responseCode;
                string text = req.downloadHandler != null ? req.downloadHandler.text : string.Empty;

#if UNITY_2020_2_OR_NEWER
                bool isError = req.result == UnityWebRequest.Result.ConnectionError || req.result == UnityWebRequest.Result.ProtocolError;
#else
                bool isError = req.isHttpError || req.isNetworkError;
#endif

                if (isError && status == 0)
                {
                    result = new FetchResult { Ok = false, Status = 0, Exception = req.error };
                }
                else
                {
                    result = HttpClientTransport.ParseResult(status, text);
                }

                tcs.TrySetResult(result);
            }
        }
    }
}
