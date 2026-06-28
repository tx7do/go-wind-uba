using System.Collections.Generic;

namespace Uba
{
    /// <summary>事件类型（对应 ReportEvent.EventType）</summary>
    public enum EventType
    {
        Behavior = 1,
        Risk = 2,
    }

    /// <summary>风险处置状态（对应 RiskEvent.Status）</summary>
    public enum RiskStatus
    {
        Pending = 1,
        Investigating = 2,
        Confirmed = 3,
        FalsePositive = 4,
        Ignored = 5,
        AutoBlocked = 6,
    }

    /// <summary>客户端信息（对应 ClientInfo）</summary>
    public class ClientInfo
    {
        public string? UserAgent { get; set; }
        public string? Referer { get; set; }
        public string? Country { get; set; }
        public string? City { get; set; }
    }

    /// <summary>行为事件 payload（对应 BehaviorEvent，放入 behavior oneof）</summary>
    public class BehaviorEvent
    {
        public string? EventAction { get; set; }
        public string? ObjectType { get; set; }
        public string? ObjectId { get; set; }
        public string? ObjectName { get; set; }
        public uint? SessionSeq { get; set; }
        public string? Os { get; set; }
        public string? AppVersion { get; set; }
        public string? Channel { get; set; }
        public string? Network { get; set; }
        public uint? DurationMs { get; set; }
        public string? Amount { get; set; }
        public uint? Quantity { get; set; }
        public int? Score { get; set; }
        public Dictionary<string, double>? Metrics { get; set; }
        public string? OpResult { get; set; }
        public string? ErrorCode { get; set; }
    }

    /// <summary>风险事件 payload（对应 RiskEvent，放入 risk oneof）</summary>
    public class RiskEvent
    {
        public string? RiskEventId { get; set; }
        public string? RiskType { get; set; }
        public string? RiskLevel { get; set; }
        public float? RiskScore { get; set; }
        public uint? RuleId { get; set; }
        public string? RuleName { get; set; }
        public Dictionary<string, object?>? RuleContext { get; set; }
        public List<string>? RelatedEventIds { get; set; }
        public string? Description { get; set; }
        public Dictionary<string, string>? Evidence { get; set; }
        public RiskStatus? Status { get; set; }
        public uint? HandlerId { get; set; }
        public string? HandleRemark { get; set; }
        public string? OccurTime { get; set; }
        public string? ReportTime { get; set; }
    }

    /// <summary>
    /// 统一事件结构（对应 ReportEvent）。
    /// 注意：EventTypeStr 与 Behavior/Risk 构成 oneof，必须匹配；tenantId 无需上报（服务端权威覆盖）。
    /// </summary>
    public class ReportEvent
    {
        public string EventType { get; set; } = "BEHAVIOR";
        public string? EventId { get; set; }
        public uint? UserId { get; set; }
        public string? DeviceId { get; set; }
        public string? EventTime { get; set; }
        public string EventName { get; set; } = "";
        public string? EventCategory { get; set; }
        public string? SessionId { get; set; }
        public string? Platform { get; set; }
        public string? Ip { get; set; }
        public Dictionary<string, string>? Properties { get; set; }
        public string? TraceId { get; set; }

        public string? EventAction { get; set; }
        public string? ObjectType { get; set; }
        public string? ObjectId { get; set; }
        public string? ObjectName { get; set; }
        public uint? SessionSeq { get; set; }
        public uint? DurationMs { get; set; }
        public string? Amount { get; set; }
        public uint? Quantity { get; set; }
        public int? Score { get; set; }
        public Dictionary<string, double>? Metrics { get; set; }

        // oneof payload
        public BehaviorEvent? Behavior { get; set; }
        public RiskEvent? Risk { get; set; }
    }

    /// <summary>上报请求体（对应 PostReportRequest）</summary>
    public class PostReportRequest
    {
        public string AppId { get; set; } = "";
        public string AppSecret { get; set; } = "";
        public List<ReportEvent> Events { get; set; } = new List<ReportEvent>();
        public ClientInfo? ClientInfo { get; set; }
    }

    /// <summary>单条错误详情</summary>
    public class ErrorDetail
    {
        public string? Code { get; set; }
        public string? Message { get; set; }
        public string? EventId { get; set; }
    }

    /// <summary>按事件类型分组的错误</summary>
    public class TypeErrorDetail
    {
        public string? Type { get; set; }
        public List<ErrorDetail>? Errors { get; set; }
    }

    /// <summary>上报响应体（对应 PostReportResponse）</summary>
    public class PostReportResponse
    {
        public bool Success { get; set; }
        public string? Message { get; set; }
        public List<TypeErrorDetail>? ErrorsByType { get; set; }
        public string? RequestId { get; set; }
        public long? ServerTime { get; set; }
        public int? TotalCount { get; set; }
        public int? SuccessCount { get; set; }
        public int? FailedCount { get; set; }
    }

    /// <summary>Kratos 标准错误信封（非 200 响应）</summary>
    public class KratosError
    {
        public int Code { get; set; }
        public string? Reason { get; set; }
        public string? Message { get; set; }
    }
}
