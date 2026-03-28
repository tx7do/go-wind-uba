# Collector Service

## 测试

打开Swagger UI界面，访问： <http://localhost:5700/docs/#/ReportService/ReportService_PostReport>

### 测试行为事件

```json
{
  "appId": "demo_app_001",
  "appSecret": "demo_secret_123456",
  "clientInfo": {
    "userAgent": "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
    "referer": "https://yourapp.com/home",
    "country": "CN",
    "city": "Beijing"
  },
  "events": [
    {
      "eventType": "BEHAVIOR",
      "eventId": "event_ulid_00001",
      "userId": 1001,
      "deviceId": "device_iphone_123456",
      "eventTime": "2026-03-27T12:00:00Z",
      "eventName": "page_view",
      "eventCategory": "content",
      "sessionId": "202503271200001",
      "platform": "ios",
      "ip": "192.168.1.1",
      "properties": {
        "page_name": "home_page",
        "stay_time_ms": "3500"
      },
      "tenantId": 1,
      "traceId": "trace_abc_123456",
      "serverTime": "2026-03-27T12:00:01Z",
      "behavior": {
        "objectType": "page",
        "objectId": "home_page",
        "objectName": "首页",
        "durationMs": 3500,
        "action": "view",
        "opResult": "success"
      }
    }
  ]
}
```

### 测试风险事件

```json
{
  "appId": "demo_app_001",
  "appSecret": "demo_secret_123456",
  "clientInfo": {
    "userAgent": "Mozilla/5.0 (Android 13; Mobile; rv:105.0) Gecko/105.0 Firefox/105.0",
    "referer": "",
    "country": "CN",
    "city": "Shanghai"
  },
  "events": [
    {
      "eventType": "RISK",
      "eventId": "risk_event_001",
      "userId": 1001,
      "deviceId": "device_android_789",
      "eventTime": "2026-03-27T12:05:00Z",
      "eventName": "login_failed",
      "eventCategory": "security",
      "sessionId": "202503271205001",
      "platform": "android",
      "ip": "112.10.32.24",
      "properties": {},
      "tenantId": 1,
      "traceId": "trace_risk_123",
      "serverTime": "2026-03-27T12:05:01Z",
      "risk": {
        "riskEventId": "10001",
        "tenantId": 1,
        "userId": 1001,
        "deviceId": "device_android_789",
        "globalUserId": "global_user_1001",
        "riskType": "login_anomaly",
        "riskLevel": "HIGH",
        "riskScore": 88.5,
        "ruleId": 101,
        "ruleName": "1分钟内登录失败超过5次",
        "ruleContext": {
          "threshold": 5,
          "window_sec": 60,
          "current_count": 8
        },
        "relatedEventIds": ["1001", "1002", "1003"],
        "sessionId": "202503271205001",
        "description": "1分钟内登录失败8次，触发异常登录风险",
        "evidence": {
          "ip": "112.10.32.24",
          "location": "Shanghai",
          "device": "Android 13"
        },
        "status": "PENDING",
        "occurTime": "2026-03-27T12:05:00Z",
        "reportTime": "2026-03-27T12:05:01Z"
      }
    }
  ]
}
```

### 混合数据

```json
{
  "appId": "demo_app_001",
  "appSecret": "demo_secret_123456",
  "clientInfo": {
    "userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    "referer": "https://yourapp.com",
    "country": "CN",
    "city": "Guangzhou"
  },
  "events": [
    {
      "eventType": "BEHAVIOR",
      "eventId": "event_001",
      "userId": 1001,
      "deviceId": "device_web_123",
      "eventTime": "2026-03-27T12:10:00Z",
      "eventName": "click",
      "eventCategory": "action",
      "sessionId": "202603271210001",
      "platform": "web",
      "ip": "192.168.1.2",
      "properties": {
        "button": "buy"
      },
      "tenantId": 1,
      "traceId": "trace_001",
      "serverTime": "2026-03-27T12:10:00Z",
      "behavior": {
        "objectType": "button",
        "objectId": "btn_buy",
        "objectName": "购买按钮",
        "durationMs": 0,
        "action": "click",
        "opResult": "success"
      }
    },
    {
      "eventType": "RISK",
      "eventId": "risk_001",
      "userId": 1001,
      "deviceId": "device_web_123",
      "eventTime": "2026-03-27T12:10:01Z",
      "eventName": "abnormal_click",
      "eventCategory": "security",
      "sessionId": "202603271210001",
      "platform": "web",
      "ip": "192.168.1.2",
      "properties": {},
      "tenantId": 1,
      "traceId": "trace_risk_001",
      "serverTime": "2026-03-27T12:10:01Z",
      "risk": {
        "riskEventId": "1002",
        "tenantId": 1,
        "userId": 1001,
        "deviceId": "device_web_123",
        "riskType": "device_anomaly",
        "riskLevel": "MEDIUM",
        "riskScore": 65.0,
        "ruleId": 102,
        "ruleName": "1秒内点击超过10次",
        "relatedEventIds": ["1001"],
        "sessionId": "202603271210001",
        "description": "短时间内频繁点击，疑似机器操作",
        "evidence": {
          "ip": "192.168.1.2"
        },
        "status": "PENDING",
        "occurTime": "2026-03-27T12:10:01Z",
        "reportTime": "2026-03-27T12:10:01Z"
      }
    }
  ]
}
```