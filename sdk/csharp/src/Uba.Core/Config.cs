using System.Collections.Generic;

namespace Uba
{
    /// <summary>日志级别</summary>
    public enum LogLevel
    {
        Info,
        Warn,
        Error,
    }

    /// <summary>SDK 配置</summary>
    public class UbaConfig
    {
        /// <summary>应用 ID（必填，鉴权）</summary>
        public string AppId { get; set; } = "";
        /// <summary>应用密钥（必填，鉴权）</summary>
        public string AppSecret { get; set; } = "";
        /// <summary>collector 服务地址，如 http://localhost:5700</summary>
        public string Endpoint { get; set; } = "";
        /// <summary>上报路径，默认 /uba/v1/report</summary>
        public string Path { get; set; } = "/uba/v1/report";
        /// <summary>缓冲达到该数量触发 flush，默认 20</summary>
        public int BatchSize { get; set; } = 20;
        /// <summary>定时 flush 间隔（毫秒），默认 5000</summary>
        public int FlushInterval { get; set; } = 5000;
        /// <summary>失败最大重试次数，默认 3</summary>
        public int MaxRetries { get; set; } = 3;
        /// <summary>单次请求超时（毫秒），默认 8000（须 &lt; 服务端 10s）</summary>
        public int Timeout { get; set; } = 8000;
        /// <summary>重试基础退避（毫秒），默认 1000，指数递增</summary>
        public int RetryBaseDelay { get; set; } = 1000;
        /// <summary>是否开启调试日志，默认 false</summary>
        public bool Debug { get; set; } = false;
    }

    /// <summary>track 方法的可选参数</summary>
    public class TrackOptions
    {
        public string? EventCategory { get; set; }
        public uint? UserId { get; set; }
        public string? DeviceId { get; set; }
        public string? SessionId { get; set; }
        public string? Platform { get; set; }
        public string? EventAction { get; set; }
        public string? ObjectType { get; set; }
        public string? ObjectId { get; set; }
        public string? ObjectName { get; set; }
        public uint? DurationMs { get; set; }
        public string? Amount { get; set; }
        public uint? Quantity { get; set; }
        public int? Score { get; set; }
        public Dictionary<string, double>? Metrics { get; set; }
        /// <summary>自定义属性，并入 properties</summary>
        public Dictionary<string, string>? Properties { get; set; }
        /// <summary>游戏区服 ID</summary>
        public string? ServerId { get; set; }
        /// <summary>玩家等级</summary>
        public uint? Level { get; set; }
    }
}
