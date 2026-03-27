# Collector Service

## 测试

打开Swagger UI界面，访问： <http://localhost:5700/docs/#/ReportService/ReportService_PostReport>

### 测试行为事件

```json
{
  "appId": "app_001",
  "appSecret": "secret_xxx",
  "events": [
    {
      "eventType": "BEHAVIOR",
      "eventId": "evt_001",
      "userId": 12345,
      "deviceId": "dev_abc",
      "eventTime": "2026-03-26T15:34:36.117186300Z",
      "eventName": "purchase",
      "eventCategory": "pay",
      "sessionId": 1001,
      "tenantId": 1,
      "behavior": {
        "objectId": "item_001",
        "amount": "99.00"
      }
    }
  ]
}
```

### 测试路径事件

```json
{
  "appId": "app_001",
  "appSecret": "secret_xxx",
  "events": [
    {
      "eventType": "PATH",
      "eventId": "evt_002",
      "userId": 12345,
      "deviceId": "dev_def",
      "eventTime": "2026-03-26T15:34:36.117186300Z",
      "eventName": "purchase",
      "eventCategory": "pay",
      "sessionId": 1001,
      "tenantId": 1,
      "path": {
        "objectId": "item_001",
        "amount": "99.00"
      }
    }
  ]
}
```

### 测试风险事件

```json
{
  "appId": "app_001",
  "appSecret": "secret_xxx",
  "events": [
    {
      "eventType": "RISK",
      "eventId": "evt_003",
      "userId": 12345,
      "deviceId": "dev_gef",
      "eventTime": "2026-03-26T15:34:36.117186300Z",
      "eventName": "purchase",
      "eventCategory": "pay",
      "sessionId": 1001,
      "tenantId": 1,
      "risk": {
        "objectId": "item_001",
        "amount": "99.00"
      }
    }
  ]
}
```

### 测试会话事件

### 测试漏斗事件
