using System.Collections.Generic;
using System.Threading.Tasks;

namespace Uba
{
    /// <summary>环境上下文提供者：设备ID/会话ID/平台等。可由 Unity 侧注入更精确的值。</summary>
    public interface IContextProvider
    {
        string GetDeviceId();
        string GetSessionId();
        string GetPlatform();
        ClientInfo? GetClientInfo();
    }

    /// <summary>默认上下文提供者：进程级内存值（适合无引擎环境）</summary>
    public class DefaultContextProvider : IContextProvider
    {
        private readonly string _deviceId = Utils.NewUuid();
        private readonly string _sessionId = Utils.NewUuid();

        public string GetDeviceId() => _deviceId;
        public string GetSessionId() => _sessionId;
        public virtual string GetPlatform() => "dotnet";
        public virtual ClientInfo? GetClientInfo() => null;
    }

    /// <summary>
    /// UbaClient —— SDK 核心类。
    /// 持有鉴权凭证，提供 track / trackBehavior / trackRisk / identify / flush 等高层 API。
    /// </summary>
    public class UbaClient
    {
        private readonly UbaConfig _config;
        private readonly Batcher _batcher;
        private IContextProvider _context;
        private uint? _userId;
        private Dictionary<string, string> _superProperties = new Dictionary<string, string>();

        /// <summary>
        /// 初始化 SDK。
        /// </summary>
        /// <param name="config">配置（appId/appSecret/endpoint 必填）</param>
        /// <param name="transport">HTTP 传输层；默认 HttpClientTransport。Unity WebGL 传入 UnityWebRequestTransport</param>
        /// <param name="context">环境上下文；默认 DefaultContextProvider。Unity 传入 UnityContextProvider 可注入 SystemInfo</param>
        public UbaClient(UbaConfig config, IHttpTransport? transport = null, IContextProvider? context = null)
        {
            Validate(config);
            _config = config;
            // 去尾部斜杠
            _config.Endpoint = (_config.Endpoint ?? "").TrimEnd('/');
            _context = context ?? new DefaultContextProvider();
            _batcher = new Batcher(_config, transport ?? new HttpClientTransport(), () => _context.GetClientInfo(), Log);
        }

        private static void Validate(UbaConfig c)
        {
            if (string.IsNullOrEmpty(c.AppId)) throw new System.ArgumentException("appId is required", nameof(c));
            if (string.IsNullOrEmpty(c.AppSecret)) throw new System.ArgumentException("appSecret is required", nameof(c));
            if (string.IsNullOrEmpty(c.Endpoint)) throw new System.ArgumentException("endpoint is required", nameof(c));
        }

        /// <summary>替换上下文提供者（如运行时切换平台信息）</summary>
        public void SetContextProvider(IContextProvider context) => _context = context;

        /// <summary>通用埋点：上报行为事件（trackBehavior 别名）</summary>
        public void Track(string eventName, Dictionary<string, string>? properties = null, TrackOptions? options = null)
            => TrackBehavior(eventName, properties, options);

        /// <summary>行为事件埋点</summary>
        public void TrackBehavior(string eventName, Dictionary<string, string>? properties = null, TrackOptions? options = null)
        {
            var evt = BuildEvent(EventType.Behavior, eventName, properties, options);
            evt.Behavior = new BehaviorEvent
            {
                EventAction = options?.EventAction,
                ObjectType = options?.ObjectType,
                ObjectId = options?.ObjectId,
                ObjectName = options?.ObjectName,
                DurationMs = options?.DurationMs,
                Amount = options?.Amount,
                Quantity = options?.Quantity,
                Score = options?.Score,
                Metrics = options?.Metrics,
                // 游戏专属维度
                ServerId = options?.ServerId,
                Level = options?.Level,
            };
            _batcher.Enqueue(evt);
            if (_config.Debug) Log(LogLevel.Info, $"enqueued: {eventName} (queue={_batcher.Size})");
        }

        /// <summary>风险事件埋点</summary>
        public void TrackRisk(string eventName, RiskEvent risk, TrackOptions? options = null)
        {
            var evt = BuildEvent(EventType.Risk, eventName, options?.Properties, options);
            evt.Risk = risk;
            _batcher.Enqueue(evt);
            if (_config.Debug) Log(LogLevel.Info, $"enqueued risk: {eventName} (queue={_batcher.Size})");
        }

        /// <summary>设置当前登录用户 ID</summary>
        public void Identify(uint userId) => _userId = userId;

        /// <summary>清除登录用户（登出）</summary>
        public void ResetUser() => _userId = null;

        /// <summary>设置公共属性，注入后续每条事件</summary>
        public void SetSuperProperties(Dictionary<string, string> props)
        {
            foreach (var kv in props) _superProperties[kv.Key] = kv.Value;
        }

        /// <summary>清除公共属性</summary>
        public void ClearSuperProperties() => _superProperties.Clear();

        /// <summary>手动触发批量上报</summary>
        public Task<FlushResult> FlushAsync() => _batcher.FlushAsync();

        /// <summary>当前待上报事件数</summary>
        public int PendingCount => _batcher.Size;

        /// <summary>销毁：停止定时器</summary>
        public void Dispose() => _batcher.Dispose();

        // ──────────── 内部 ────────────

        private ReportEvent BuildEvent(EventType type, string eventName, Dictionary<string, string>? properties, TrackOptions? options)
        {
            // 合并 properties：superProperties + 本次传入
            var merged = Utils.MergeProperties(_superProperties, properties);

            return new ReportEvent
            {
                EventType = type == EventType.Behavior ? "BEHAVIOR" : "RISK",
                EventId = Utils.NewUuid(),
                EventName = eventName,
                EventTime = Utils.ToRFC3339(),
                UserId = options?.UserId ?? _userId,
                DeviceId = options?.DeviceId ?? _context.GetDeviceId(),
                SessionId = options?.SessionId ?? _context.GetSessionId(),
                Platform = options?.Platform ?? _context.GetPlatform(),
                EventCategory = options?.EventCategory,
                EventAction = options?.EventAction,
                ObjectType = options?.ObjectType,
                ObjectId = options?.ObjectId,
                ObjectName = options?.ObjectName,
                DurationMs = options?.DurationMs,
                Amount = options?.Amount,
                Quantity = options?.Quantity,
                Score = options?.Score,
                Metrics = options?.Metrics,
                // 游戏专属维度
                ServerId = options?.ServerId,
                Level = options?.Level,
                Properties = merged.Count > 0 ? merged : null,
            };
        }

        private void Log(LogLevel level, string msg)
        {
            if (!_config.Debug && level != LogLevel.Error) return;
            // Unity/Godot 下会通过 UnityEngine.Debug 输出；这里用条件编译或回调可扩展。
            // 默认走 System.Console（非引擎环境友好）。
            try
            {
                var prefix = level switch
                {
                    LogLevel.Error => "[UBA][ERROR]",
                    LogLevel.Warn => "[UBA][WARN]",
                    _ => "[UBA]",
                };
                System.Console.WriteLine($"{prefix} {msg}");
            }
            catch { /* console 不可用时忽略 */ }
        }
    }
}
