package com.gowind.uba;

import java.util.List;
import java.util.Map;

/**
 * 风险事件载荷（risk oneof）。riskEventId / occurTime / reportTime 服务端补全，无需填写。
 */
public final class RiskEvent {

    String riskType;
    String riskLevel;
    double riskScore;
    long ruleId;
    String ruleName;
    Map<String, Object> ruleContext;
    List<String> relatedEventIds;
    String description;
    Map<String, String> evidence;
    String status;

    public RiskEvent riskType(String v) { this.riskType = v; return this; }
    public RiskEvent riskLevel(String v) { this.riskLevel = v; return this; }
    public RiskEvent riskScore(double v) { this.riskScore = v; return this; }
    public RiskEvent ruleId(long v) { this.ruleId = v; return this; }
    public RiskEvent ruleName(String v) { this.ruleName = v; return this; }
    public RiskEvent ruleContext(Map<String, Object> v) { this.ruleContext = v; return this; }
    public RiskEvent relatedEventIds(List<String> v) { this.relatedEventIds = v; return this; }
    public RiskEvent description(String v) { this.description = v; return this; }
    public RiskEvent evidence(Map<String, String> v) { this.evidence = v; return this; }
    public RiskEvent status(String v) { this.status = v; return this; }
}
