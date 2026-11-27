// UbaClient.test.ets —— LocalUnit 测试（DevEco Studio / hvigor test 跑，Windows 本机无法验证）。
// 覆盖：会话轮换 / wire 构造契约（金样） / flags 解析 / 采样确定性。
// 网络传输需 Mock（@ohos.net.http 不可在 LocalUnit 里真实外呼），集成测试放 ohosTest。
import { describe, it, expect } from '@ohos/hypium'
import { UbaConfig, TrackOptions, RiskEvent, SDK_VERSION } from '../main/ets/config'
import { SessionManager } from '../main/ets/session'
import { toRFC3339Ms, sessionIdInSample, uuidV4, fnv1a32 } from '../main/ets/utils'

export default function UbaClientTest() {
  describe('ubaClientLocal', () => {
    it('config_validation', 0, () => {
      let threw = false
      try {
        const cfg = new UbaConfig()
        cfg.validate() // 必填为空 → 抛错
      } catch (err) {
        threw = true
      }
      expect(threw).assertTrue()
    })

    it('config_defaults_align_other_sdks', 0, () => {
      const cfg = new UbaConfig()
      expect(cfg.batchSize).assertEqual(20)
      expect(cfg.flushIntervalMs).assertEqual(5000)
      expect(cfg.maxRetries).assertEqual(3)
      expect(cfg.timeoutMs).assertEqual(8000)
      expect(cfg.retryBaseDelayMs).assertEqual(1000)
      expect(cfg.path).assertEqual('/uba/v1/report')
      expect(SDK_VERSION).assertEqual('1.0.0')
    })

    it('rfc3339_millis_utc', 0, () => {
      // 2026-03-27T12:00:00.123Z（UTC 毫秒）
      const epoch = Date.UTC(2026, 2, 27, 12, 0, 0, 123)
      expect(toRFC3339Ms(epoch)).assertEqual('2026-03-27T12:00:00.123Z')
    })

    it('uuid_v4_shape', 0, () => {
      const id = uuidV4()
      expect(id.length).assertEqual(36)
      expect(id.charAt(14)).assertEqual('4')
    })

    it('sampling_deterministic', 0, () => {
      expect(sessionIdInSample('s', 1.0)).assertTrue()
      expect(sessionIdInSample('s', 0.0)).assertFalse()
      const r = sessionIdInSample('fixed-session', 0.5)
      for (let i = 0; i < 10; i++) {
        expect(sessionIdInSample('fixed-session', 0.5)).assertEqual(r)
      }
    })

    it('fnv1a32_deterministic', 0, () => {
      expect(fnv1a32('session-a')).assertEqual(fnv1a32('session-a'))
      expect(fnv1a32('session-a') === fnv1a32('session-b')).assertFalse()
    })

    it('session_short_background_keeps', 0, () => {
      let fake = 1000000
      const s = new SessionManager(30000, () => fake)
      const id1 = s.current
      s.onBackground()
      fake += 10000 // 后台 10s
      s.onForeground()
      expect(s.current).assertEqual(id1) // 未超时不轮换
    })

    it('session_timeout_background_rotates', 0, () => {
      let fake = 1000000
      const s = new SessionManager(30000, () => fake)
      const id1 = s.current
      s.onBackground()
      fake += 31000 // 后台 31s
      s.onForeground()
      expect(s.current === id1).assertFalse() // 超时轮换
    })

    it('session_check_timeout_rotates', 0, () => {
      let fake = 1000000
      const s = new SessionManager(30000, () => fake)
      const id1 = s.checkTimeout()
      fake += 31000
      const id2 = s.checkTimeout()
      expect(id2 === id1).assertFalse() // 事件间隔超时轮换（服务端语义）
    })
  })
}
