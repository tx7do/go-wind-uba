# CLAUDE.md — sdk/python/uba 速记

- Python 服务端 SDK，零依赖（json/urllib/threading）；deviceId=hostname，platform=`server`。
- camelCase、body 鉴权、无 gzip、单批 <1MiB、超时 8s；`tenantId`/`serverTime`/`ip` 不发。
- 数值标量只进 behavior oneof；behavior oneof 必须存在。
- 重试：1s×2^n×3；4xx(除429) 不重试不回队；可重试耗尽回队由定时器补发（cap 10000）。
- 提交前：`PYTHONPATH=src python -m unittest discover -s tests -v`。
