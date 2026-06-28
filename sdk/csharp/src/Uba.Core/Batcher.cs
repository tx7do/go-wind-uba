using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace Uba
{
    /// <summary>flush 结果</summary>
    public class FlushResult
    {
        public bool Success { get; set; }
        public PostReportResponse? Response { get; set; }
        /// <summary>丢弃的事件数（重试耗尽）</summary>
        public int Dropped { get; set; }
    }

    /// <summary>
    /// 事件缓冲与批量合并：内存队列，达到 batchSize 或 flushInterval 触发上报。
    /// 上报失败重试耗尽时丢弃事件（防内存无限增长）。
    /// </summary>
    public class Batcher
    {
        private readonly UbaConfig _cfg;
        private readonly IHttpTransport _transport;
        private readonly Func<ClientInfo?> _getClientInfo;
        private readonly Action<LogLevel, string> _log;

        private readonly List<ReportEvent> _queue = new List<ReportEvent>();
        private readonly object _lock = new object();
        private Timer? _timer;
        private int _flushing; // 0/1 标志，防并发 flush

        public Batcher(UbaConfig cfg, IHttpTransport transport, Func<ClientInfo?> getClientInfo, Action<LogLevel, string> log)
        {
            _cfg = cfg;
            _transport = transport;
            _getClientInfo = getClientInfo;
            _log = log;
            StartTimer();
        }

        /// <summary>入队一条事件</summary>
        public void Enqueue(ReportEvent evt)
        {
            lock (_lock)
            {
                _queue.Add(evt);
                if (_queue.Count >= _cfg.BatchSize)
                {
                    // 不阻塞调用方，触发异步 flush
                    _ = FlushAsync();
                }
            }
        }

        public int Size
        {
            get { lock (_lock) return _queue.Count; }
        }

        /// <summary>触发批量上报</summary>
        public async Task<FlushResult> FlushAsync()
        {
            // 防并发：已有 flush 在进行则直接返回
            if (Interlocked.CompareExchange(ref _flushing, 1, 0) != 0)
                return new FlushResult { Success = true };

            List<ReportEvent> events;
            lock (_lock)
            {
                if (_queue.Count == 0)
                {
                    _flushing = 0;
                    return new FlushResult { Success = true };
                }
                events = new List<ReportEvent>(_queue);
                _queue.Clear();
            }

            try
            {
                return await SendWithRetryAsync(events).ConfigureAwait(false);
            }
            finally
            {
                _flushing = 0;
            }
        }

        private async Task<FlushResult> SendWithRetryAsync(List<ReportEvent> events)
        {
            var req = new PostReportRequest
            {
                AppId = _cfg.AppId,
                AppSecret = _cfg.AppSecret,
                Events = events,
                ClientInfo = _getClientInfo(),
            };
            string body = JsonSerializer.Serialize(req);
            string url = _cfg.Endpoint + _cfg.Path;

            FetchResult last;
            for (int attempt = 0; attempt <= _cfg.MaxRetries; attempt++)
            {
                last = await _transport.SendAsync(url, body, _cfg.Timeout).ConfigureAwait(false);

                if (last.Ok)
                {
                    if (last.Response != null && (last.Response.FailedCount ?? 0) > 0)
                    {
                        _log(LogLevel.Warn, $"upload partial failure: success={last.Response.SuccessCount} failed={last.Response.FailedCount}");
                    }
                    return new FlushResult { Success = true, Response = last.Response };
                }

                // 4xx（非 429）客户端错误：不重试
                if (last.Status >= 400 && last.Status < 500 && last.Status != 429)
                {
                    _log(LogLevel.Error, $"upload rejected (no retry): {Summarize(last)}");
                    return new FlushResult { Success = false, Dropped = events.Count };
                }

                if (attempt < _cfg.MaxRetries)
                {
                    int delay = _cfg.RetryBaseDelay * (int)Math.Pow(2, attempt);
                    _log(LogLevel.Warn, $"upload failed (attempt {attempt + 1}), retry in {delay}ms: {Summarize(last)}");
                    await Task.Delay(delay).ConfigureAwait(false);
                }
                else
                {
                    _log(LogLevel.Error, $"dropping {events.Count} events after { _cfg.MaxRetries + 1} attempts: {Summarize(last)}");
                    return new FlushResult { Success = false, Dropped = events.Count };
                }
            }
            return new FlushResult { Success = false, Dropped = events.Count };
        }

        private static string Summarize(FetchResult r)
        {
            if (r.Exception != null) return r.Exception;
            if (r.Error != null) return $"status={r.Status} reason={r.Error.Reason} msg={r.Error.Message}";
            return $"status={r.Status}";
        }

        private void StartTimer()
        {
            _timer = new Timer(_ => { _ = FlushAsync(); }, null, _cfg.FlushInterval, _cfg.FlushInterval);
        }

        /// <summary>销毁：停止定时器</summary>
        public void Dispose()
        {
            _timer?.Dispose();
            _timer = null;
        }
    }
}
