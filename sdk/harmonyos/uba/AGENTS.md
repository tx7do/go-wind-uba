# Agent Guide — sdk/harmonyos/uba

HarmonyOS 埋点 SDK（ArkTS，hvigor library）。**本仓库 Windows 无 DevEco/hvigor：源码与 hypium 测试交付，验证需 DevEco Studio（`hvigorw test`）**，交付说明必须标注。

## 铁律

1. 契约单一真相源在后端 proto；camelCase 逐字段对齐 `json_name`，禁止手造字段。
2. `tenantId`/`serverTime`/`ip` 不发送；**数值标量只进 behavior oneof**（client.ets commonWire/buildBehaviorWire 构造时保证）。
3. behavior oneof 必须存在；risk 事件不带 behavior。
4. 重试语义：4xx（除 429）不重试；可重试失败回队补发；HTTP 200 部分失败按 failedCount 丢弃计。
5. **ArkTS 严格语法**：禁 any/解构/spread；对象字面量必须显式类型（Record<string, Object|null> + putField 辅助）；class 字段显式初始化。
6. 传输用 @ohos.net.http（postJson 封装，statusCode 0 = 网络失败可重试）；deviceId 用 @ohos.data.preferences。
7. 默认值对齐其他端：批量 20/5s/3 次/1s 退避/8s 超时。

## 提交前自查（DevEco Studio）

```bash
hvigorw test     # LocalUnit（utils/session/config 契约用例）
hvigorw assembleHar
```
