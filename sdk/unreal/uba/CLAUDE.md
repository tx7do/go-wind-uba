# CLAUDE.md — sdk/unreal/uba 速记

- 分层：core/ 纯 C++17（g++ 测试全绿）+ UnrealPlugin/ 壳（宿主编译，本仓库不验证）。
- camelCase、body 鉴权、无 gzip、单批 <1MiB、超时 8s；`tenantId`/`serverTime`/`ip` 不发。
- 数值标量只进 behavior oneof；behavior oneof 必须存在。
- 重试：1s×2^n×3；4xx(除429) 不重试不回队；可重试耗尽回队由定时器补发（cap 10000）。
- 队列存 wire 字符串（补发无损）；UbaJsonValue（std::variant）承载 ruleContext。
- 提交前：`g++ -std=c++17 -Icore core/uba_core.cpp core/uba_core_test.cpp -o /tmp/uba_tests && /tmp/uba_tests`。
